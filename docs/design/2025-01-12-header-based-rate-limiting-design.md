# Header-Based Rate Limiting Enhancement Design

## Overview

This design document outlines an enhancement to the Cooldown Proxy's Cerebras rate limiting system to dynamically adjust behavior based on real-time rate limit headers from Cerebras API responses. Instead of relying solely on static configured limits, the proxy will use the `x-ratelimit-*` headers to make intelligent decisions about request timing and queuing.

## Problem Statement

The current Cerebras rate limiter uses static configured limits (RPM and TPM) without visibility into the actual limits reported by Cerebras. This creates several issues:

1. Mismatch between proxy limits and actual Cerebras limits
2. Inability to adapt to real-time capacity constraints
3. Imprecise timing for rate-limited requests
4. Lack of visibility into true remaining quota

## Solution Architecture

### Core Components

#### Header Parser
A new component that extracts and validates rate limit headers from Cerebras API responses:

```go
type RateLimitHeaders struct {
    TPMLimit           int
    TPMRemaining       int
    TPMReset           time.Duration
    RequestDayLimit    int
    RequestDayRemaining int
    RequestDayReset    time.Duration
}

func ParseRateLimitHeaders(headers http.Header) (*RateLimitHeaders, error)
```

#### Enhanced CerebrasLimiter
Extend the existing `CerebrasLimiter` to incorporate header-based state:

```go
type CerebrasLimiter struct {
    // Existing fields
    rpmLimit          int
    tpmLimit          int
    rpmWindow         *slidingWindow
    tpmWindow         *slidingWindow
    queue             *queue.PriorityQueue
    mu                sync.RWMutex

    // New header-based fields
    currentTPMLimit   int
    currentTPMRemaining int
    nextTPMReset      time.Time
    lastHeaderUpdate  time.Time
}
```

#### Dynamic Request Scheduler
Enhanced logic that uses header information for precise timing instead of generic delays.

## Data Flow

### Request Processing Flow

1. **Request Arrival**: Estimate tokens and check current rate limits
2. **Rate Limit Check**: If limited, queue with enhanced metadata including predicted wait times
3. **Forward Request**: Send to Cerebras with current rate limit state
4. **Response Processing**: Parse rate limit headers from Cerebras response
5. **State Update**: Update limiter with real-time data from headers
6. **Queue Scheduling**: Use reset times to time queued request processing precisely
7. **Client Response**: Return updated limit information via response headers

### Header Processing Logic

```go
func (c *CerebrasLimiter) UpdateFromHeaders(headers http.Header) error {
    parsed, err := ParseRateLimitHeaders(headers)
    if err != nil {
        // Fall back to configured limits
        return err
    }

    c.mu.Lock()
    defer c.mu.Unlock()

    // Update limits with header values
    if parsed.TPMLimit > 0 {
        c.currentTPMLimit = parsed.TPMLimit
    }
    c.currentTPMRemaining = parsed.TPMRemaining
    c.nextTPMReset = time.Now().Add(parsed.TPMReset)
    c.lastHeaderUpdate = time.Now()

    return nil
}
```

## Implementation Details

### Enhanced Rate Limiting Logic

```go
func (c *CerebrasLimiter) CheckRequestWithDynamicQueue(requestID string, tokens int) time.Duration {
    c.mu.Lock()
    defer c.mu.Unlock()

    now := time.Now()

    // Check if we have recent header data
    if c.lastHeaderUpdate.IsZero() || now.Sub(c.lastHeaderUpdate) > 5*time.Minute {
        // Fall back to static limiting logic
        return c.CheckRequestWithQueue(requestID, tokens)
    }

    // Use header-based logic
    if c.currentTPMRemaining <= 0 {
        // Calculate precise wait time until reset
        if now.Before(c.nextTPMReset) {
            return c.nextTPMReset.Sub(now)
        }
        // Reset should have occurred, assume some capacity
        c.currentTPMRemaining = c.currentTPMLimit / 10 // Conservative estimate
    }

    if tokens > c.currentTPMRemaining {
        // Not enough tokens, wait for reset
        if now.Before(c.nextTPMReset) {
            return c.nextTPMReset.Sub(now)
        }
    }

    // Update remaining tokens
    c.currentTPMRemaining -= tokens

    // Record in sliding window for consistency
    c.recordRequest(tokens, now)

    return 0 // Process immediately
}
```

### Response Processing Enhancement

Modify the `ModifyResponse` function in `CerebrasProxyHandler`:

```go
ModifyResponse: func(resp *http.Response) error {
    // Parse and update limiter with header data
    if err := h.Limiter.UpdateFromHeaders(resp.Header); err != nil {
        // Log but don't fail the request
        fmt.Printf("Failed to parse rate limit headers: %v\n", err)
    }

    // Add proxy headers
    resp.Header.Set("X-RateLimit-Limit-RPM", strconv.Itoa(h.Config.RPMLimit))
    resp.Header.Set("X-RateLimit-Limit-TPM", strconv.Itoa(h.Config.TPMLimit))
    resp.Header.Set("X-RateLimit-Queue-Length", strconv.Itoa(h.Limiter.QueueLength()))

    // Add circuit breaker headers
    stats := circuitBreaker.Stats()
    resp.Header.Set("X-CircuitBreaker-State", stats.State.String())
    resp.Header.Set("X-CircuitBreaker-Failures", strconv.Itoa(stats.Failures))

    return nil
}
```

### Header Validation and Error Handling

```go
func ParseRateLimitHeaders(headers http.Header) (*RateLimitHeaders, error) {
    result := &RateLimitHeaders{}

    // Parse TPM limit
    if tpmLimitStr := headers.Get("x-ratelimit-limit-tokens-minute"); tpmLimitStr != "" {
        if tpmLimit, err := strconv.Atoi(tpmLimitStr); err == nil && tpmLimit > 0 {
            result.TPMLimit = tpmLimit
        }
    }

    // Parse TPM remaining
    if tpmRemainingStr := headers.Get("x-ratelimit-remaining-tokens-minute"); tpmRemainingStr != "" {
        if tpmRemaining, err := strconv.Atoi(tpmRemainingStr); err == nil && tpmRemaining >= 0 {
            result.TPMRemaining = tpmRemaining
        }
    }

    // Parse TPM reset (may be fractional)
    if tpmResetStr := headers.Get("x-ratelimit-reset-tokens-minute"); tpmResetStr != "" {
        if tpmReset, err := strconv.ParseFloat(tpmResetStr, 64); err == nil && tpmReset > 0 {
            result.TPMReset = time.Duration(tpmReset * float64(time.Second))
        }
    }

    // Validate required fields
    if result.TPMLimit == 0 || result.TPMReset == 0 {
        return nil, fmt.Errorf("missing required rate limit headers")
    }

    return result, nil
}
```

## Edge Cases and Error Handling

### Header Validation
- All header values are validated as positive numbers
- Invalid or missing headers trigger fallback to configured limits
- Inconsistent values (remaining > limit) are logged and handled conservatively

### Race Conditions
- Mutex protection when updating rate limit state from response headers
- Atomic updates to prevent concurrent request handling issues

### Reset Time Precision
- Handle fractional seconds in reset headers using `time.Duration` with nanosecond precision
- Account for clock skew between proxy and Cerebras servers

### Fallback Behavior
- If headers are missing or invalid, fall back to static configured limits
- Use exponential backoff for repeated header parsing failures
- Log warnings when falling back to static limits

## Testing Strategy

### Unit Tests
- Test header parsing with various formats and edge cases
- Test limiter state updates and fallback behavior
- Test queue timing calculations with different reset values
- Test race condition handling with concurrent goroutines

### Integration Tests
- Test end-to-end flow with mocked Cerebras responses containing different header values
- Test behavior with missing headers and invalid values
- Test concurrent request handling with dynamic limit updates
- Test fallback to static limits when headers are unavailable

### Performance Tests
- Measure overhead of header parsing on request latency
- Test performance under high load with dynamic limit updates
- Validate that queue timing improvements reduce unnecessary delays

## Configuration

### New Configuration Options
```yaml
cerebras:
  rate_limits:
    use_headers: true           # Enable header-based rate limiting
    header_fallback: true       # Fall back to static limits if headers fail
    header_timeout: 5s          # Max time to wait for fresh header data
    reset_buffer: 100ms         # Buffer time before reset to account for clock skew
```

### Backward Compatibility
- Feature is disabled by default to maintain current behavior
- Gradual rollout possible with feature flag
- Existing configuration remains valid

## Success Metrics

### Primary Metrics
- Reduced rate limiting errors (HTTP 429) from Cerebras
- Improved request processing latency through precise timing
- Better queue utilization and reduced unnecessary delays

### Secondary Metrics
- Increased visibility into actual Cerebras limits via headers
- Better client experience with more accurate rate limit information
- Reduced need for manual limit configuration adjustments

## Migration Plan

### Phase 1: Header Parsing Infrastructure
- Implement header parser component
- Add validation and error handling
- Unit tests for header parsing logic

### Phase 2: Enhanced Rate Limiting
- Extend CerebrasLimiter with header-based state
- Implement dynamic rate limiting logic
- Integration tests with mock responses

### Phase 3: Response Processing
- Modify proxy response handling
- Add header-based state updates
- End-to-end testing

### Phase 4: Configuration and Monitoring
- Add configuration options
- Implement monitoring and metrics
- Performance testing and optimization

## Risks and Mitigations

### Risk: Header Format Changes
- **Mitigation**: Flexible parsing with fallback to static limits
- **Monitoring**: Alert on header parsing failures

### Risk: Clock Skew
- **Mitigation**: Add buffer time before reset, use conservative timing
- **Monitoring**: Track timing accuracy and adjust buffers as needed

### Risk: Performance Overhead
- **Mitigation**: Efficient parsing, minimal memory allocation
- **Monitoring**: Measure impact on request latency

## Conclusion

This enhancement will significantly improve the proxy's rate limiting accuracy and efficiency by leveraging real-time data from Cerebras API responses. The implementation maintains backward compatibility while providing intelligent adaptation to actual service limits.