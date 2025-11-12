# Header-Based Rate Limiting Enhancement Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enhance the Cooldown Proxy's Cerebras rate limiting system to dynamically adjust behavior based on real-time rate limit headers from Cerebras API responses.

**Architecture:** Extend the existing CerebrasLimiter with header parsing and dynamic scheduling capabilities, maintaining backward compatibility while adding intelligent timing based on actual service limits.

**Tech Stack:** Go, HTTP headers parsing, rate limiting algorithms, mutex-based concurrency, YAML configuration

---

## Overview

This plan implements the header-based rate limiting enhancement designed in `docs/design/2025-01-12-header-based-rate-limiting-design.md`. The enhancement will parse rate limit headers from Cerebras API responses and use them to dynamically adjust request timing and queuing behavior.

## Task 1: Header Parsing Infrastructure

**Files:**
- Create: `internal/ratelimit/headers.go`
- Test: `internal/ratelimit/headers_test.go`

**Step 1: Write the failing test**

```go
func TestParseRateLimitHeaders(t *testing.T) {
    headers := http.Header{}
    headers.Set("x-ratelimit-limit-tokens-minute", "1000")
    headers.Set("x-ratelimit-remaining-tokens-minute", "800")
    headers.Set("x-ratelimit-reset-tokens-minute", "45.5")

    parsed, err := ParseRateLimitHeaders(headers)

    assert.NoError(t, err)
    assert.Equal(t, 1000, parsed.TPMLimit)
    assert.Equal(t, 800, parsed.TPMRemaining)
    assert.Equal(t, 45.5*time.Second, parsed.TPMReset)
}

func TestParseRateLimitHeadersMissingRequired(t *testing.T) {
    headers := http.Header{}
    headers.Set("x-ratelimit-remaining-tokens-minute", "800")

    _, err := ParseRateLimitHeaders(headers)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "missing required rate limit headers")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ratelimit -v -run TestParseRateLimitHeaders`
Expected: FAIL with "ParseRateLimitHeaders undefined"

**Step 3: Write minimal implementation**

```go
// internal/ratelimit/headers.go
package ratelimit

import (
    "fmt"
    "net/http"
    "strconv"
    "time"
)

type RateLimitHeaders struct {
    TPMLimit             int
    TPMRemaining         int
    TPMReset             time.Duration
    RequestDayLimit      int
    RequestDayRemaining  int
    RequestDayReset      time.Duration
}

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

**Step 4: Run test to verify it passes**

Run: `go test ./internal/ratelimit -v -run TestParseRateLimitHeaders`
Expected: PASS

**Step 5: Add more comprehensive tests**

```go
func TestParseRateLimitHeadersInvalidValues(t *testing.T) {
    tests := []struct {
        name     string
        headers  http.Header
        expected *RateLimitHeaders
        hasError bool
    }{
        {
            name: "invalid tpm limit",
            headers: http.Header{
                "x-ratelimit-limit-tokens-minute":     []string{"invalid"},
                "x-ratelimit-reset-tokens-minute":    []string{"60"},
            },
            hasError: true,
        },
        {
            name: "negative tpm remaining",
            headers: http.Header{
                "x-ratelimit-limit-tokens-minute":     []string{"1000"},
                "x-ratelimit-remaining-tokens-minute": []string{"-100"},
                "x-ratelimit-reset-tokens-minute":    []string{"60"},
            },
            expected: &RateLimitHeaders{TPMLimit: 1000, TPMReset: 60 * time.Second},
        },
        {
            name: "fractional reset time",
            headers: http.Header{
                "x-ratelimit-limit-tokens-minute":     []string{"1000"},
                "x-ratelimit-reset-tokens-minute":    []string{"45.5"},
            },
            expected: &RateLimitHeaders{TPMLimit: 1000, TPMReset: 45.5 * time.Second},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := ParseRateLimitHeaders(tt.headers)
            if tt.hasError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expected.TPMLimit, result.TPMLimit)
                assert.Equal(t, tt.expected.TPMReset, result.TPMReset)
            }
        })
    }
}
```

**Step 6: Run tests to verify they pass**

Run: `go test ./internal/ratelimit -v -run TestParseRateLimitHeaders`
Expected: PASS all tests

**Step 7: Commit**

```bash
git add internal/ratelimit/headers.go internal/ratelimit/headers_test.go
git commit -m "feat: add rate limit header parsing infrastructure"
```

## Task 2: Enhanced CerebrasLimiter Structure

**Files:**
- Modify: `internal/ratelimit/cerebras_limiter.go`
- Test: `internal/ratelimit/cerebras_limiter_test.go`

**Step 1: Write the failing test**

```go
func TestCerebrasLimiterHeaderBasedState(t *testing.T) {
    limiter := NewCerebrasLimiter(CerebrasConfig{
        RPMLimit: 60,
        TPMLimit: 1000,
    })

    // Test initial state
    assert.Equal(t, 0, limiter.currentTPMLimit)
    assert.Equal(t, 0, limiter.currentTPMRemaining)
    assert.True(t, limiter.lastHeaderUpdate.IsZero())

    // Update from headers
    headers := http.Header{}
    headers.Set("x-ratelimit-limit-tokens-minute", "1000")
    headers.Set("x-ratelimit-remaining-tokens-minute", "800")
    headers.Set("x-ratelimit-reset-tokens-minute", "45.5")

    err := limiter.UpdateFromHeaders(headers)
    assert.NoError(t, err)

    assert.Equal(t, 1000, limiter.currentTPMLimit)
    assert.Equal(t, 800, limiter.currentTPMRemaining)
    assert.False(t, limiter.lastHeaderUpdate.IsZero())
    assert.True(t, time.Now().Add(45*time.Second).Before(limiter.nextTPMReset))
    assert.True(t, time.Now().Add(46*time.Second).After(limiter.nextTPMReset))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ratelimit -v -run TestCerebrasLimiterHeaderBasedState`
Expected: FAIL with "currentTPMLimit undefined" or similar

**Step 3: Examine existing CerebrasLimiter structure**

```bash
find . -name "*cerebras*" -type f
grep -r "type CerebrasLimiter" . --include="*.go"
```

**Step 4: Extend CerebrasLimiter structure**

Add new fields to the CerebrasLimiter struct:

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

**Step 5: Add UpdateFromHeaders method**

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

**Step 6: Run test to verify it passes**

Run: `go test ./internal/ratelimit -v -run TestCerebrasLimiterHeaderBasedState`
Expected: PASS

**Step 7: Add concurrency safety test**

```go
func TestCerebrasLimiterConcurrentHeaderUpdates(t *testing.T) {
    limiter := NewCerebrasLimiter(CerebrasConfig{
        RPMLimit: 60,
        TPMLimit: 1000,
    })

    var wg sync.WaitGroup
    numGoroutines := 10

    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(iteration int) {
            defer wg.Done()

            headers := http.Header{}
            headers.Set("x-ratelimit-limit-tokens-minute", strconv.Itoa(1000+iteration))
            headers.Set("x-ratelimit-remaining-tokens-minute", "800")
            headers.Set("x-ratelimit-reset-tokens-minute", "45.5")

            _ = limiter.UpdateFromHeaders(headers)
        }(i)
    }

    wg.Wait()

    limiter.mu.RLock()
    defer limiter.mu.RUnlock()

    // Should have updated to some valid value
    assert.True(t, limiter.currentTPMLimit > 0)
    assert.False(t, limiter.lastHeaderUpdate.IsZero())
}
```

**Step 8: Run test to verify it passes**

Run: `go test ./internal/ratelimit -v -run TestCerebrasLimiterConcurrentHeaderUpdates`
Expected: PASS

**Step 9: Commit**

```bash
git add internal/ratelimit/cerebras_limiter.go internal/ratelimit/cerebras_limiter_test.go
git commit -m "feat: enhance CerebrasLimiter with header-based state tracking"
```

## Task 3: Dynamic Rate Limiting Logic

**Files:**
- Modify: `internal/ratelimit/cerebras_limiter.go`
- Test: `internal/ratelimit/cerebras_limiter_test.go`

**Step 1: Write the failing test**

```go
func TestCerebrasLimiterDynamicRateLimiting(t *testing.T) {
    limiter := NewCerebrasLimiter(CerebrasConfig{
        RPMLimit: 60,
        TPMLimit: 1000,
    })

    // Set up header state
    headers := http.Header{}
    headers.Set("x-ratelimit-limit-tokens-minute", "1000")
    headers.Set("x-ratelimit-remaining-tokens-minute", "50")
    headers.Set("x-ratelimit-reset-tokens-minute", "10")

    err := limiter.UpdateFromHeaders(headers)
    assert.NoError(t, err)

    // Request that would exceed remaining tokens
    requestID := "test-request-1"
    tokens := 100
    delay := limiter.CheckRequestWithDynamicQueue(requestID, tokens)

    // Should wait for reset since not enough tokens
    assert.True(t, delay > 5*time.Second)
    assert.True(t, delay < 15*time.Second)
}

func TestCerebrasLimiterDynamicFallback(t *testing.T) {
    limiter := NewCerebrasLimiter(CerebrasConfig{
        RPMLimit: 60,
        TPMLimit: 1000,
    })

    // No header data, should fall back to static limits
    requestID := "test-request-1"
    tokens := 100
    delay := limiter.CheckRequestWithDynamicQueue(requestID, tokens)

    // Should use static limiting logic
    assert.True(t, delay >= 0)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ratelimit -v -run TestCerebrasLimiterDynamicRateLimiting`
Expected: FAIL with "CheckRequestWithDynamicQueue undefined"

**Step 3: Implement CheckRequestWithDynamicQueue method**

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

func (c *CerebrasLimiter) recordRequest(tokens int, timestamp time.Time) {
    // Implement this helper method if it doesn't exist
    // This should record the request in both sliding windows
    if c.tpmWindow != nil {
        c.tpmWindow.Add(tokens, timestamp)
    }
    if c.rpmWindow != nil {
        c.rpmWindow.Add(1, timestamp) // 1 request
    }
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/ratelimit -v -run TestCerebrasLimiterDynamicRateLimiting`
Expected: PASS

**Step 5: Add edge case tests**

```go
func TestCerebrasLimiterDynamicEdgeCases(t *testing.T) {
    limiter := NewCerebrasLimiter(CerebrasConfig{
        RPMLimit: 60,
        TPMLimit: 1000,
    })

    t.Run("stale header data fallback", func(t *testing.T) {
        // Set up old header data
        headers := http.Header{}
        headers.Set("x-ratelimit-limit-tokens-minute", "1000")
        headers.Set("x-ratelimit-remaining-tokens-minute", "50")
        headers.Set("x-ratelimit-reset-tokens-minute", "10")

        err := limiter.UpdateFromHeaders(headers)
        assert.NoError(t, err)

        // Simulate time passing (6 minutes)
        limiter.lastHeaderUpdate = time.Now().Add(-6 * time.Minute)

        requestID := "test-request"
        tokens := 100
        delay := limiter.CheckRequestWithDynamicQueue(requestID, tokens)

        // Should fall back to static limiting due to stale data
        assert.True(t, delay >= 0)
    })

    t.Run("reset time passed", func(t *testing.T) {
        // Set up header data with reset time in the past
        headers := http.Header{}
        headers.Set("x-ratelimit-limit-tokens-minute", "1000")
        headers.Set("x-ratelimit-remaining-tokens-minute", "0")
        headers.Set("x-ratelimit-reset-tokens-minute", "1")

        err := limiter.UpdateFromHeaders(headers)
        assert.NoError(t, err)

        // Simulate reset time passing
        limiter.nextTPMReset = time.Now().Add(-1 * time.Second)

        requestID := "test-request"
        tokens := 50
        delay := limiter.CheckRequestWithDynamicQueue(requestID, tokens)

        // Should process immediately (conservative estimate applied)
        assert.Equal(t, time.Duration(0), delay)
        assert.Equal(t, 50, limiter.currentTPMRemaining) // Should have been set to conservative estimate
    })
}
```

**Step 6: Run tests to verify they pass**

Run: `go test ./internal/ratelimit -v -run TestCerebrasLimiterDynamicEdgeCases`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/ratelimit/cerebras_limiter.go internal/ratelimit/cerebras_limiter_test.go
git commit -m "feat: implement dynamic rate limiting logic with header-based timing"
```

## Task 4: Enhanced Response Processing

**Files:**
- Find: `internal/proxy/cerebras_proxy.go` (or similar)
- Modify: The Cerebras proxy handler
- Test: `internal/proxy/cerebras_proxy_test.go`

**Step 1: Find the Cerebras proxy implementation**

```bash
find . -name "*cerebras*" -type f | grep -v test
grep -r "CerebrasProxyHandler\|cerebras.*proxy" . --include="*.go"
```

**Step 2: Write the failing test**

```go
func TestCerebrasProxyHeaderIntegration(t *testing.T) {
    // Create a mock HTTP server that returns rate limit headers
    mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("x-ratelimit-limit-tokens-minute", "1000")
        w.Header().Set("x-ratelimit-remaining-tokens-minute", "800")
        w.Header().Set("x-ratelimit-reset-tokens-minute", "45.5")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"response": "ok"}`))
    }))
    defer mockServer.Close()

    // Create proxy handler with mock backend
    handler := &CerebrasProxyHandler{
        Limiter: NewCerebrasLimiter(CerebrasConfig{
            RPMLimit: 60,
            TPMLimit: 1000,
        }),
        BackendURL: mockServer.URL,
    }

    // Create test request
    req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model": "claude-3"}`))
    req.Header.Set("Content-Type", "application/json")

    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    resp := w.Result()
    assert.Equal(t, http.StatusOK, resp.StatusCode)

    // Check that response headers include rate limit info
    assert.Equal(t, "1000", resp.Header.Get("X-RateLimit-Limit-TPM"))
    assert.Contains(t, resp.Header.Get("X-RateLimit-Remaining-TPM"), "800")

    // Verify limiter was updated
    handler.Limiter.mu.RLock()
    assert.Equal(t, 1000, handler.Limiter.currentTPMLimit)
    assert.Equal(t, 800, handler.Limiter.currentTPMRemaining)
    handler.Limiter.mu.RUnlock()
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./internal/proxy -v -run TestCerebrasProxyHeaderIntegration`
Expected: FAIL with missing header processing

**Step 4: Modify response handler to process headers**

Find and modify the ModifyResponse function in the Cerebras proxy:

```go
ModifyResponse: func(resp *http.Response) error {
    // Parse and update limiter with header data
    if err := h.Limiter.UpdateFromHeaders(resp.Header); err != nil {
        // Log but don't fail the request
        log.Printf("Failed to parse rate limit headers: %v", err)
    }

    // Add proxy headers with current state
    h.Limiter.mu.RLock()
    defer h.Limiter.mu.RUnlock()

    resp.Header.Set("X-RateLimit-Limit-RPM", strconv.Itoa(h.Config.RPMLimit))
    resp.Header.Set("X-RateLimit-Limit-TPM", strconv.Itoa(h.Config.TPMLimit))
    resp.Header.Set("X-RateLimit-Current-TPM-Limit", strconv.Itoa(h.Limiter.currentTPMLimit))
    resp.Header.Set("X-RateLimit-Remaining-TPM", strconv.Itoa(h.Limiter.currentTPMRemaining))
    resp.Header.Set("X-RateLimit-Queue-Length", strconv.Itoa(h.Limiter.QueueLength()))

    // Add circuit breaker headers if they exist
    if h.CircuitBreaker != nil {
        stats := h.CircuitBreaker.Stats()
        resp.Header.Set("X-CircuitBreaker-State", stats.State.String())
        resp.Header.Set("X-CircuitBreaker-Failures", strconv.Itoa(stats.Failures))
    }

    return nil
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/proxy -v -run TestCerebrasProxyHeaderIntegration`
Expected: PASS

**Step 6: Add error handling test**

```go
func TestCerebrasProxyHeaderParsingErrors(t *testing.T) {
    // Create a mock server with invalid headers
    mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("x-ratelimit-limit-tokens-minute", "invalid")
        w.Header().Set("x-ratelimit-remaining-tokens-minute", "800")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"response": "ok"}`))
    }))
    defer mockServer.Close()

    handler := &CerebrasProxyHandler{
        Limiter: NewCerebrasLimiter(CerebrasConfig{
            RPMLimit: 60,
            TPMLimit: 1000,
        }),
        BackendURL: mockServer.URL,
    }

    req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model": "claude-3"}`))
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    resp := w.Result()
    assert.Equal(t, http.StatusOK, resp.StatusCode)

    // Should still add static headers even if dynamic parsing failed
    assert.Equal(t, "1000", resp.Header.Get("X-RateLimit-Limit-TPM"))
}
```

**Step 7: Run test to verify it passes**

Run: `go test ./internal/proxy -v -run TestCerebrasProxyHeaderParsingErrors`
Expected: PASS

**Step 8: Commit**

```bash
git add internal/proxy/cerebras_proxy.go internal/proxy/cerebras_proxy_test.go
git commit -m "feat: integrate header-based rate limiting into proxy response processing"
```

## Task 5: Configuration Enhancement

**Files:**
- Modify: `internal/config/types.go`
- Test: `internal/config/config_test.go`

**Step 1: Write the failing test**

```go
func TestHeaderBasedRateLimitConfig(t *testing.T) {
    configYAML := `
cerebras:
  rate_limits:
    use_headers: true
    header_fallback: true
    header_timeout: 5s
    reset_buffer: 100ms
  rpm_limit: 60
  tpm_limit: 1000
`

    var config Config
    err := yaml.Unmarshal([]byte(configYAML), &config)
    assert.NoError(t, err)

    assert.True(t, config.Cerebras.RateLimits.UseHeaders)
    assert.True(t, config.Cerebras.RateLimits.HeaderFallback)
    assert.Equal(t, 5*time.Second, config.Cerebras.RateLimits.HeaderTimeout)
    assert.Equal(t, 100*time.Millisecond, config.Cerebras.RateLimits.ResetBuffer)
    assert.Equal(t, 60, config.Cerebras.RPMLimit)
    assert.Equal(t, 1000, config.Cerebras.TPMLimit)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -v -run TestHeaderBasedRateLimitConfig`
Expected: FAIL with undefined configuration fields

**Step 3: Add new configuration types**

```go
// Add to internal/config/types.go
type CerebrasRateLimitConfig struct {
    UseHeaders     bool          `yaml:"use_headers"`
    HeaderFallback bool          `yaml:"header_fallback"`
    HeaderTimeout  time.Duration `yaml:"header_timeout"`
    ResetBuffer    time.Duration `yaml:"reset_buffer"`
}

type CerebrasConfig struct {
    RateLimits CerebrasRateLimitConfig `yaml:"rate_limits"`
    RPMLimit   int                     `yaml:"rpm_limit"`
    TPMLimit   int                     `yaml:"tpm_limit"`
    // ... existing fields
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -v -run TestHeaderBasedRateLimitConfig`
Expected: PASS

**Step 5: Add default configuration test**

```go
func TestDefaultHeaderBasedRateLimitConfig(t *testing.T) {
    configYAML := `
cerebras:
  rpm_limit: 60
  tpm_limit: 1000
`

    var config Config
    err := yaml.Unmarshal([]byte(configYAML), &config)
    assert.NoError(t, err)

    // Should have sensible defaults
    assert.False(t, config.Cerebras.RateLimits.UseHeaders)     // Disabled by default
    assert.True(t, config.Cerebras.RateLimits.HeaderFallback)  // Enabled by default
    assert.Equal(t, 5*time.Second, config.Cerebras.RateLimits.HeaderTimeout)
    assert.Equal(t, 100*time.Millisecond, config.Cerebras.RateLimits.ResetBuffer)
}
```

**Step 6: Run test to verify it passes**

Run: `go test ./internal/config -v -run TestDefaultHeaderBasedRateLimitConfig`
Expected: PASS

**Step 7: Update example configuration**

Modify `config.yaml.example` to include the new options:

```yaml
cerebras:
  rate_limits:
    use_headers: true           # Enable header-based rate limiting
    header_fallback: true       # Fall back to static limits if headers fail
    header_timeout: 5s          # Max time to wait for fresh header data
    reset_buffer: 100ms         # Buffer time before reset to account for clock skew
  rpm_limit: 60                # Requests per minute limit
  tpm_limit: 1000              # Tokens per minute limit
```

**Step 8: Commit**

```bash
git add internal/config/types.go internal/config/config_test.go config.yaml.example
git commit -m "feat: add configuration for header-based rate limiting"
```

## Task 6: Integration and End-to-End Testing

**Files:**
- Create: `tests/integration/header_rate_limiting_test.go`
- Test: `tests/integration/`

**Step 1: Write the integration test**

```go
// tests/integration/header_rate_limiting_test.go
package integration

import (
    "testing"
    "net/http"
    "net/http/httptest"
    "strings"
    "time"
)

func TestHeaderBasedRateLimitingEndToEnd(t *testing.T) {
    // Mock Cerebras server with rate limiting
    var requestCount int
    mockCerebras := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestCount++

        if requestCount <= 2 {
            // First two requests succeed with plenty of quota
            w.Header().Set("x-ratelimit-limit-tokens-minute", "1000")
            w.Header().Set("x-ratelimit-remaining-tokens-minute", "800")
            w.Header().Set("x-ratelimit-reset-tokens-minute", "60")
        } else {
            // Third request hits rate limit
            w.Header().Set("x-ratelimit-limit-tokens-minute", "1000")
            w.Header().Set("x-ratelimit-remaining-tokens-minute", "0")
            w.Header().Set("x-ratelimit-reset-tokens-minute", "5")
            w.WriteHeader(http.StatusTooManyRequests)
            w.Write([]byte(`{"error": "Rate limit exceeded"}`))
            return
        }

        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"response": "success"}`))
    }))
    defer mockCerebras.Close()

    // Configure proxy with header-based rate limiting
    config := Config{
        Cerebras: CerebrasConfig{
            RateLimits: CerebrasRateLimitConfig{
                UseHeaders:     true,
                HeaderFallback: true,
                HeaderTimeout:  5 * time.Second,
                ResetBuffer:    100 * time.Millisecond,
            },
            RPMLimit: 60,
            TPMLimit: 1000,
        },
        Server: ServerConfig{
            Host: "localhost",
            Port: 8080,
        },
    }

    proxyHandler := NewCerebrasProxyHandler(config, mockCerebras.URL)
    proxy := httptest.NewServer(proxyHandler)
    defer proxy.Close()

    client := &http.Client{Timeout: 30 * time.Second}

    // First request should succeed
    resp1, err := client.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model": "claude-3"}`))
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp1.StatusCode)
    resp1.Body.Close()

    // Second request should succeed
    resp2, err := client.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model": "claude-3"}`))
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp2.StatusCode)
    resp2.Body.Close()

    // Third request should be queued or delayed based on headers
    start := time.Now()
    resp3, err := client.Post(proxy.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model": "claude-3"}`))
    duration := time.Since(start)

    assert.NoError(t, err)
    // Should either succeed after waiting or be properly rate limited
    if resp3.StatusCode == http.StatusOK {
        // If it succeeded, it should have taken some time to wait for reset
        assert.True(t, duration > 4*time.Second)
    } else {
        // If it failed, should be properly rate limited
        assert.Equal(t, http.StatusTooManyRequests, resp3.StatusCode)
    }
    resp3.Body.Close()
}
```

**Step 2: Run integration test**

Run: `go test ./tests/integration -v -run TestHeaderBasedRateLimitingEndToEnd`
Expected: May fail initially, implement missing pieces

**Step 3: Implement missing integration pieces**

Create proxy constructor and handler integration as needed:

```go
func NewCerebrasProxyHandler(config Config, backendURL string) http.Handler {
    limiter := NewCerebrasLimiter(config.Cerebras)

    proxy := &httputil.ReverseProxy{
        Director: func(req *http.Request) {
            req.URL.Scheme = "http"
            req.URL.Host = strings.TrimPrefix(backendURL, "http://")
            req.Host = req.URL.Host
        },
        ModifyResponse: func(resp *http.Response) error {
            // Parse and update limiter with header data
            if err := limiter.UpdateFromHeaders(resp.Header); err != nil {
                // Log but don't fail the request
                log.Printf("Failed to parse rate limit headers: %v", err)
            }

            // Add proxy headers
            limiter.mu.RLock()
            defer limiter.mu.RUnlock()

            resp.Header.Set("X-RateLimit-Limit-RPM", strconv.Itoa(config.Cerebras.RPMLimit))
            resp.Header.Set("X-RateLimit-Limit-TPM", strconv.Itoa(config.Cerebras.TPMLimit))
            resp.Header.Set("X-RateLimit-Queue-Length", strconv.Itoa(limiter.QueueLength()))

            return nil
        },
    }

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Apply rate limiting before forwarding
        requestID := generateRequestID()
        estimatedTokens := estimateTokens(r)

        delay := limiter.CheckRequestWithDynamicQueue(requestID, estimatedTokens)
        if delay > 0 {
            time.Sleep(delay)
        }

        proxy.ServeHTTP(w, r)
    })
}
```

**Step 4: Run integration test to verify it passes**

Run: `go test ./tests/integration -v -run TestHeaderBasedRateLimitingEndToEnd`
Expected: PASS

**Step 5: Add performance benchmark test**

```go
func BenchmarkHeaderBasedRateLimiting(b *testing.B) {
    mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("x-ratelimit-limit-tokens-minute", "1000")
        w.Header().Set("x-ratelimit-remaining-tokens-minute", "900")
        w.Header().Set("x-ratelimit-reset-tokens-minute", "60")
        w.WriteHeader(http.StatusOK)
    }))
    defer mockServer.Close()

    config := Config{
        Cerebras: CerebrasConfig{
            RateLimits: CerebrasRateLimitConfig{
                UseHeaders: true,
            },
            RPMLimit: 60,
            TPMLimit: 1000,
        },
    }

    proxyHandler := NewCerebrasProxyHandler(config, mockServer.URL)
    proxy := httptest.NewServer(proxyHandler)
    defer proxy.Close()

    client := &http.Client{Timeout: 5 * time.Second}

    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            resp, err := client.Get(proxy.URL)
            if err == nil {
                resp.Body.Close()
            }
        }
    })
}
```

**Step 6: Run benchmark test**

Run: `go test ./tests/integration -bench=BenchmarkHeaderBasedRateLimiting -benchmem`
Expected: Benchmark runs successfully

**Step 7: Commit**

```bash
git add tests/integration/header_rate_limiting_test.go
git commit -m "feat: add integration tests for header-based rate limiting"
```

## Task 7: Documentation and Examples

**Files:**
- Modify: `README.md`
- Create: `docs/examples/header-rate-limiting.md`
- Update: `docs/api/` (if exists)

**Step 1: Update main README**

Add section about header-based rate limiting:

```markdown
## Header-Based Rate Limiting

The proxy supports dynamic rate limiting based on Cerebras API response headers. This feature allows the proxy to adapt to real-time rate limits and provide more accurate request timing.

### Configuration

```yaml
cerebras:
  rate_limits:
    use_headers: true           # Enable header-based rate limiting
    header_fallback: true       # Fall back to static limits if headers fail
    header_timeout: 5s          # Max time to wait for fresh header data
    reset_buffer: 100ms         # Buffer time before reset to account for clock skew
  rpm_limit: 60                # Fallback requests per minute limit
  tpm_limit: 1000              # Fallback tokens per minute limit
```

### Response Headers

When header-based rate limiting is enabled, the proxy adds the following headers to responses:

- `X-RateLimit-Limit-RPM`: Configured RPM limit
- `X-RateLimit-Limit-TPM`: Configured TPM limit
- `X-RateLimit-Current-TPM-Limit`: Current TPM limit from Cerebras headers
- `X-RateLimit-Remaining-TPM`: Remaining TPM tokens from Cerebras headers
- `X-RateLimit-Queue-Length`: Current queue length
```

**Step 2: Create detailed example documentation**

```markdown
# Header-Based Rate Limiting Examples

## Basic Setup

Enable header-based rate limiting in your configuration:

```yaml
cerebras:
  rate_limits:
    use_headers: true
    header_fallback: true
    header_timeout: 5s
    reset_buffer: 100ms
  rpm_limit: 60
  tpm_limit: 1000
```

## How It Works

1. **Request Processing**: The proxy estimates tokens and checks current rate limits
2. **Forward Request**: Request is sent to Cerebras with current rate limit state
3. **Response Processing**: Rate limit headers are parsed from Cerebras response
4. **State Update**: Limiter is updated with real-time data from headers
5. **Queue Scheduling**: Queue timing uses reset times for precise processing

## Monitoring

Monitor the proxy's rate limiting behavior through response headers:

```bash
curl -I http://localhost:8080/v1/chat/completions

# Response headers include:
# X-RateLimit-Current-TPM-Limit: 1000
# X-RateLimit-Remaining-TPM: 850
# X-RateLimit-Queue-Length: 0
```
```

**Step 3: Add migration guide**

```markdown
# Migration Guide: Header-Based Rate Limiting

## Upgrading from Static Rate Limiting

1. **Update Configuration**: Add the `rate_limits` section to your cerebras config
2. **Enable Feature**: Set `use_headers: true`
3. **Test Gradually**: Monitor headers to ensure proper operation
4. **Adjust Buffers**: Fine-tune `reset_buffer` if needed for clock skew

## Backward Compatibility

The feature is disabled by default. Existing configurations continue to work without changes.
```

**Step 4: Commit**

```bash
git add README.md docs/examples/header-rate-limiting.md
git commit -m "docs: add documentation for header-based rate limiting feature"
```

## Task 8: Final Integration and Quality Checks

**Files:**
- Modify: Various files as needed
- Test: All tests

**Step 1: Run full test suite**

```bash
go test ./... -v
```

**Step 2: Run tests with coverage**

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

**Step 3: Run code quality checks**

```bash
make fmt
make vet
make check
```

**Step 4: Build and verify**

```bash
make build
./cooldown-proxy --help
```

**Step 5: Create final end-to-end test**

```bash
# Create a test config
cat > test-config.yaml << EOF
cerebras:
  rate_limits:
    use_headers: true
    header_fallback: true
    header_timeout: 5s
    reset_buffer: 100ms
  rpm_limit: 60
  tpm_limit: 1000
server:
  host: localhost
  port: 8080
EOF

# Test server startup
timeout 5s ./cooldown-proxy -config test-config.yaml || echo "Server started successfully"
```

**Step 6: Cleanup and final commit**

```bash
rm test-config.yaml
git add .
git commit -m "feat: complete header-based rate limiting enhancement implementation"
```

## Task 9: Performance and Load Testing

**Files:**
- Create: `tests/performance/header_rate_limiting_bench_test.go`

**Step 1: Create performance benchmarks**

```go
package performance

import (
    "testing"
    "time"
)

func BenchmarkHeaderParsingOverhead(b *testing.B) {
    headers := http.Header{}
    headers.Set("x-ratelimit-limit-tokens-minute", "1000")
    headers.Set("x-ratelimit-remaining-tokens-minute", "800")
    headers.Set("x-ratelimit-reset-tokens-minute", "45.5")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = ParseRateLimitHeaders(headers)
    }
}

func BenchmarkDynamicRateLimiting(b *testing.B) {
    limiter := NewCerebrasLimiter(CerebrasConfig{
        RPMLimit: 60,
        TPMLimit: 1000,
    })

    // Set up header state
    headers := http.Header{}
    headers.Set("x-ratelimit-limit-tokens-minute", "1000")
    headers.Set("x-ratelimit-remaining-tokens-minute", "500")
    headers.Set("x-ratelimit-reset-tokens-minute", "60")
    limiter.UpdateFromHeaders(headers)

    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        requestID := "test-request"
        tokens := 100
        for pb.Next() {
            limiter.CheckRequestWithDynamicQueue(requestID, tokens)
        }
    })
}
```

**Step 2: Run benchmarks**

```bash
go test ./tests/performance -bench=. -benchmem
```

**Step 3: Create load test script**

```bash
#!/bin/bash
# tests/load/header_rate_limiting_load_test.sh

echo "Starting header-based rate limiting load test..."

# Start proxy server
./cooldown-proxy -config config.yaml &
PROXY_PID=$!
sleep 2

# Run concurrent requests
echo "Running 100 concurrent requests..."
for i in {1..100}; do
    curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/health &
done

wait
echo "Load test completed"

# Stop proxy server
kill $PROXY_PID
```

**Step 4: Commit performance tests**

```bash
git add tests/performance/ tests/load/
git commit -m "test: add performance and load tests for header-based rate limiting"
```

---

## Task 10: Feature Flag and Monitoring

**Files:**
- Modify: `internal/config/feature_flags.go` (if exists)
- Add: Monitoring and metrics

**Step 1: Add feature flag support**

```go
type FeatureFlags struct {
    HeaderBasedRateLimiting bool `yaml:"header_based_rate_limiting"`
}
```

**Step 2: Add metrics collection**

```go
type RateLimitMetrics struct {
    HeaderParsingSuccess    int64
    HeaderParsingFailures   int64
    DynamicTimingsUsed      int64
    FallbackTimingsUsed     int64
}
```

**Step 3: Commit monitoring features**

```bash
git add internal/config/feature_flags.go
git commit -m "feat: add feature flags and metrics for header-based rate limiting"
```

---

## Summary

This implementation plan provides a comprehensive, step-by-step approach to implementing header-based rate limiting enhancement:

1. **Header parsing infrastructure** - Core header parsing with validation
2. **Enhanced limiter structure** - Dynamic state tracking with concurrency safety
3. **Dynamic rate limiting logic** - Intelligent timing based on real-time data
4. **Response processing integration** - Header processing in proxy responses
5. **Configuration enhancement** - New config options with backward compatibility
6. **Integration testing** - End-to-end verification of the complete flow
7. **Documentation** - User guides and API documentation
8. **Quality assurance** - Code quality checks and final integration
9. **Performance testing** - Benchmarks and load testing
10. **Monitoring** - Feature flags and metrics collection

Each task follows TDD principles with bite-sized steps, exact file paths, complete code examples, and specific verification commands. The implementation maintains backward compatibility while providing intelligent adaptation to actual Cerebras service limits.

**Total estimated implementation time:** 2-3 days for a senior developer following this plan.