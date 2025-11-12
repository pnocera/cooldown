# Header-Based Rate Limiting Examples

## Overview

This guide demonstrates how to use the header-based rate limiting feature in Cooldown Proxy. The feature allows the proxy to dynamically adapt to Cerebras API rate limits in real-time, providing more efficient and accurate request scheduling.

## Basic Setup

Enable header-based rate limiting in your configuration:

```yaml
# config.yaml
server:
  host: "localhost"
  port: 8080

cerebras_limits:
  rate_limits:
    use_headers: true           # Enable header-based rate limiting
    header_fallback: true       # Fall back to static limits if headers fail
    header_timeout: 5s          # Max time to wait for fresh header data
    reset_buffer: 100ms         # Buffer time before reset to account for clock skew
  rpm_limit: 60                # Fallback requests per minute limit
  tpm_limit: 1000              # Fallback tokens per minute limit
  max_queue_depth: 100         # Maximum queue size
  request_timeout: 10m         # Maximum wait time in queue
  priority_threshold: 0.7      # Usage threshold for priority adjustment
```

## How It Works

### Request Processing Flow

1. **Incoming Request**: The proxy receives a request with Host header matching Cerebras
2. **Rate Check**: Uses `CheckRequestWithDynamicQueue` with intelligent timing
3. **Header-Based Logic**:
   - If fresh headers available → Uses precise timing from API
   - If headers stale/missing → Falls back to static limits
4. **Forward Request**: Sends request to Cerebras API with current state
5. **Response Processing**: Parses rate limit headers from Cerebras response
6. **State Update**: Updates limiter with real-time data
7. **Response Headers**: Adds rate limiting info to client response

### Header Processing

The proxy parses these Cerebras API response headers:

- `x-ratelimit-limit-tokens-minute`: Current TPM limit
- `x-ratelimit-remaining-tokens-minute`: Remaining TPM tokens
- `x-ratelimit-reset-tokens-minute`: Time until token reset (may be fractional)

## Configuration Options

### Rate Limits Configuration

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `use_headers` | bool | `false` | Enable header-based rate limiting |
| `header_fallback` | bool | `true` | Fall back to static limits if headers fail |
| `header_timeout` | duration | `5s` | Max time to consider header data fresh |
| `reset_buffer` | duration | `100ms` | Buffer time before reset to avoid clock skew |

### Static Fallback Configuration

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `rpm_limit` | int | `1000` | Fallback requests per minute |
| `tpm_limit` | int | `1000000` | Fallback tokens per minute |
| `max_queue_depth` | int | `100` | Maximum queue length |
| `request_timeout` | duration | `10m` | Maximum queue wait time |

## Monitoring and Observability

### Response Headers

When header-based rate limiting is enabled, the proxy adds these headers to responses:

```bash
curl -I http://localhost:8080/v1/chat/completions

# Example response headers:
HTTP/1.1 200 OK
X-RateLimit-Limit-RPM: 60
X-RateLimit-Limit-TPM: 1000
X-RateLimit-Current-TPM-Limit: 1000000
X-RateLimit-Remaining-TPM: 999842
X-RateLimit-Queue-Length: 0
X-CircuitBreaker-State: CLOSED
X-CircuitBreaker-Failures: 0
```

### Header Descriptions

| Header | Meaning | Example |
|--------|---------|---------|
| `X-RateLimit-Limit-RPM` | Configured RPM limit | `60` |
| `X-RateLimit-Limit-TPM` | Configured TPM limit | `1000` |
| `X-RateLimit-Current-TPM-Limit` | Live TPM limit from Cerebras | `1000000` |
| `X-RateLimit-Remaining-TPM` | Remaining TPM tokens | `999842` |
| `X-RateLimit-Queue-Length` | Current queue size | `0` |
| `X-CircuitBreaker-State` | Circuit breaker status | `CLOSED` |
| `X-CircuitBreaker-Failures` | Circuit breaker failures | `0` |

## Production Examples

### High-Traffic Configuration

```yaml
cerebras_limits:
  rate_limits:
    use_headers: true
    header_fallback: true
    header_timeout: 10s         # Longer timeout for reliability
    reset_buffer: 200ms        # Larger buffer for clock skew
  rpm_limit: 5000            # Higher static limits
  tpm_limit: 5000000         # 5M tokens per minute
  max_queue_depth: 500
  request_timeout: 5m          # Shorter timeout for better UX
  priority_threshold: 0.6
```

### Development Configuration

```yaml
cerebras_limits:
  rate_limits:
    use_headers: true
    header_fallback: true
    header_timeout: 2s          # Quick fallback for testing
    reset_buffer: 50ms         # Smaller buffer for faster resets
  rpm_limit: 60
  tpm_limit: 100000
  max_queue_depth: 20
  request_timeout: 30s
  priority_threshold: 0.8
```

### Conservative Configuration

```yaml
cerebras_limits:
  rate_limits:
    use_headers: true
    header_fallback: true
    header_timeout: 15s         # Very conservative timeout
    reset_buffer: 500ms        # Large buffer for safety
  rpm_limit: 30               # Very low static limits
  tpm_limit: 50000
  max_queue_depth: 50
  request_timeout: 15m         # Long timeout
  priority_threshold: 0.9
```

## Migration Guide

### Upgrading from Static Rate Limiting

1. **Update Configuration**: Add the `rate_limits` section to your cerebras config
2. **Enable Feature**: Set `use_headers: true`
3. **Test Gradually**: Monitor headers to ensure proper operation
4. **Adjust Buffers**: Fine-tune `reset_buffer` if needed for clock skew

```bash
# Before (static only)
cerebras_limits:
  rpm_limit: 1000
  tpm_limit: 1000000

# After (with headers)
cerebras_limits:
  rate_limits:
    use_headers: true
    header_fallback: true
    header_timeout: 5s
    reset_buffer: 100ms
  rpm_limit: 1000
  tpm_limit: 1000000
```

## Troubleshooting

### Headers Not Being Parsed

**Symptoms**: `X-RateLimit-Current-TPM-Limit: 0`

**Solutions**:
1. Check Cerebras API key and URL
2. Verify `use_headers: true` is set
3. Check logs for "Failed to parse rate limit headers" messages
4. Ensure requests reach Cerebras API (check Host header)

### High Rate of Fallback

**Symptoms**: Frequently using static limits instead of headers

**Solutions**:
1. Increase `header_timeout` value
2. Check network connectivity to Cerebras
3. Verify Cerebras API is returning proper headers
4. Consider enabling only header fallback

### Performance Issues

**Symptoms**: Requests taking too long or timing out

**Solutions**:
1. Adjust `reset_buffer` to avoid unnecessary waiting
2. Optimize `priority_threshold` for better queueing
3. Monitor `X-RateLimit-Queue-Length` for bottlenecks
4. Consider reducing `request_timeout` for better UX

## Integration Examples

### With Circuit Breaker

The header-based rate limiting works seamlessly with the existing circuit breaker:

```yaml
# Headers processed even when circuit is closed
# Circuit breaker protects against API failures
# Rate limiting protects against quota exhaustion
```

### With Load Testing

```yaml
# Header-based rate limiting improves load testing accuracy
# Real API responses provide realistic rate limits
# Better simulation of actual production behavior
```

### With Monitoring

```bash
# Monitor rate limiting in real-time
watch -n 1 'curl -s http://localhost:8080/health | jq .rateLimit'

# Track queue length over time
curl -s http://localhost:8080/v1/chat/completions \
  -H "X-RateLimit-Queue-Length: $(curl -s -I http://localhost:8080/v1/chat/completions | grep X-RateLimit-Queue-Length | cut -d: -f2)"
```

## Advanced Usage

### Custom Header Parsing

The system automatically handles standard Cerebras headers. For custom integrations, you can extend the `ParseRateLimitHeaders` function.

### Metrics Collection

Monitor rate limiting effectiveness through response headers and logs:

```go
// Track header parsing success rate
// Monitor fallback vs dynamic usage
// Measure queue lengths and wait times
```

### Dynamic Configuration

While the feature requires configuration reload (server restart) for changes, the rate limiting behavior adapts dynamically based on real-time API responses.