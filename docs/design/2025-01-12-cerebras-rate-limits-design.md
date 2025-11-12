# Cerebras Rate Limits Enhancement Design

## Overview

Enhance the Cooldown Proxy to efficiently and accurately manage Cerebras inference API rate limits using smart priority queuing, ensuring strict compliance with RPM (1,000) and TPM (1,000,000) limits while maximizing throughput for Developer tier users.

## Problem Statement

Current proxy only supports simple per-second rate limiting. Cerebras API has dual-metric limits (RPM + TPM) where the more restrictive limit applies. Need intelligent queuing that adapts to workload patterns and prioritizes requests optimally.

## Solution Architecture

### Enhanced Rate Limiter (internal/ratelimit/cerebras.go)
- Sliding window counters for both RPM and TPM (60-second rolling windows)
- Real-time proximity calculations to limit thresholds
- Concurrent-safe with mutex protection
- Configurable limits (default: 1000 RPM, 1M TPM for Developer tier)

### Token Estimator (internal/token/estimator.go)
- Pre-request token counting using tiktoken-go
- Model-specific estimation patterns for Llama, Mixtral, etc.
- Conservative output token estimation
- Caching for similar request patterns

### Priority Queue Manager (internal/queue/priority.go)
- Smart prioritization based on limit proximity metrics
- Dual priority levels: small token requests get priority when TPM-limited
- FIFO maintained when RPM-limited
- Exponential backoff for failed requests
- Max queue depth: 100 requests, timeout: 10 minutes

## Request Processing Flow

1. Request arrives at router
2. Token estimator counts input + estimates output tokens
3. Rate limiter checks RPM/TPM capacity
4. Queue decision:
   - Immediate processing if both metrics have capacity
   - Smart prioritization if approaching TPM limit
   - FIFO if approaching RPM limit
   - Queue with calculated delay if limits exceeded
5. Proxy forwards request when cleared
6. Metrics update for future accuracy

## Smart Prioritization Logic

```go
priorityFactor = max(currentTPM/1M, currentRPM/1K)

if priorityFactor > 0.7 {
    // Smart mode active
    if requestTokens < 1000 { priorityBoost = 2.0 }
    else if requestTokens > 5000 { priorityBoost = 0.5 }
    else { priorityBoost = 1.0 }
}
```

## Configuration Changes

```yaml
cerebras_limits:
  rpm_limit: 1000
  tpm_limit: 1000000
  max_queue_depth: 100
  request_timeout: 10m
  priority_threshold: 0.7
```

## Error Handling

- **Immediate Rejection**: 429 errors when queue depth > 100 or wait time > 5 minutes
- **Graceful Degradation**: Conservative 2x input token estimate when estimation fails
- **Circuit Breaker**: 60-second pause after 10 consecutive rate limit errors
- **Queue Starvation Prevention**: Aging mechanism for requests waiting > 2 minutes

## Integration Points

- Router enhancement for Cerebras-specific routing
- Config extension for cerebras_limits section
- Enhanced rate limiter replacing current go.uber.org/ratelimit
- Simple metrics endpoint for queue depth and limit proximity

## Testing Strategy

### Unit Tests
- Rate limiter sliding window accuracy
- Token estimation precision
- Priority queue ordering logic
- Concurrent access safety

### Load Tests
- Mixed workload patterns (small vs large requests)
- Limit boundary behavior (90%, 95%, 100% of limits)
- Queue stress testing
- Concurrent request handling

### Performance Targets
- Token estimation overhead: <5ms per request
- Queue processing throughput: >500 req/sec sustained
- Memory usage: <100MB for 1000 queued requests

## Implementation Priority

1. Enhanced rate limiter with sliding windows
2. Token estimator with model support
3. Priority queue manager
4. Router integration
5. Configuration enhancements
6. Error handling and circuit breaker
7. Comprehensive testing

## Success Criteria

- Zero rate limit violations from Cerebras API
- >95% queue throughput under normal load
- <10ms average request queuing delay
- Graceful handling of burst traffic patterns
- Simple configuration and monitoring