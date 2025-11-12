# Cerebras AI Rate Limiting Documentation

## Overview

The Cooldown Proxy now includes intelligent rate limiting specifically designed for Cerebras AI inference APIs. This implementation provides sophisticated dual-metric rate limiting, intelligent queuing, circuit breaker protection, and comprehensive monitoring.

## Features

### 🔧 **Core Rate Limiting**
- **Dual-Metric Enforcement**: RPM (Requests Per Minute) + TPM (Tokens Per Minute)
- **Sliding Windows**: 60-second rolling windows for accurate rate limiting
- **Smart Queuing**: Priority-based request queuing with intelligent sorting
- **Token Estimation**: Automatic token counting from request payloads

### 🛡️ **Reliability & Protection**
- **Circuit Breaker**: Prevents cascading failures with automatic recovery
- **Error Handling**: Comprehensive error detection and graceful degradation
- **Timeout Management**: Configurable request timeouts and queue management
- **Health Monitoring**: Real-time status via HTTP headers

### 📊 **Performance & Testing**
- **Load Testing**: Built-in load testing framework with predefined scenarios
- **Performance Metrics**: Latency tracking, throughput monitoring, and error analysis
- **Benchmarking Tools**: Automated performance validation and regression testing

## Quick Start

### 1. Configuration

Add the following to your `config.yaml`:

```yaml
# Cerebras AI specific rate limiting configuration
cerebras_limits:
  rpm_limit: 1000              # Requests per minute limit
  tpm_limit: 1000000           # Tokens per minute limit
  max_queue_depth: 100         # Maximum number of queued requests
  request_timeout: 10m         # Maximum time a request can wait in queue
  priority_threshold: 0.7      # Usage threshold for priority adjustment (70%)
```

### 2. Start the Proxy

```bash
# Build and run
make build
./cooldown-proxy -config config.yaml
```

### 3. Send Requests

Make requests to Cerebras AI through the proxy:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Host: api.cerebras.ai" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama3.1-8b",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 100
  }'
```

## Configuration Options

### Basic Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `rpm_limit` | int | 1000 | Maximum requests per minute |
| `tpm_limit` | int | 1000000 | Maximum tokens per minute |
| `max_queue_depth` | int | 100 | Maximum queued requests |
| `request_timeout` | duration | 10m | Maximum queue wait time |
| `priority_threshold` | float | 0.7 | Priority boost threshold (70%) |

### Environment-Specific Examples

#### Development Environment
```yaml
cerebras_limits:
  rpm_limit: 60               # 1 request per second
  tpm_limit: 100000           # 100K tokens per minute
  max_queue_depth: 20         # Small queue
  request_timeout: 30s        # Quick feedback
  priority_threshold: 0.8     # Higher threshold
```

#### Production Environment
```yaml
cerebras_limits:
  rpm_limit: 5000             # Higher limit for production
  tpm_limit: 5000000          # 5M tokens per minute
  max_queue_depth: 500        # Larger queue for traffic
  request_timeout: 5m         # Shorter timeout
  priority_threshold: 0.6     # More aggressive prioritization
```

#### High-Traffic Environment
```yaml
cerebras_limits:
  rpm_limit: 10000            # Very high limit
  tpm_limit: 10000000         # 10M tokens per minute
  max_queue_depth: 1000       # Large queue capacity
  request_timeout: 15m        # Longer timeout for high load
  priority_threshold: 0.5     # Very aggressive prioritization
```

## Rate Limiting Behavior

### Request Processing Flow

1. **Request Detection**: Requests to `api.cerebras.ai` or `inference.cerebras.ai` are identified
2. **Token Estimation**: Tokens are counted from the request payload
3. **Rate Limit Check**: RPM and TPM limits are evaluated
4. **Priority Calculation**: Request priority is determined based on usage and token count
5. **Queue Management**: Requests are queued or processed immediately based on limits
6. **Circuit Breaker**: Failed requests trigger circuit breaker protection

### Priority System

The system uses intelligent priority calculation:

- **High Usage (>70%)**: Small requests (<1000 tokens) get priority boost (2.0x)
- **High Usage (>70%)**: Large requests (>5000 tokens) get priority penalty (0.5x)
- **Normal Usage**: All requests get normal priority (1.0x)

### Queue Behavior

- **Immediate Processing**: Requests within limits are processed immediately
- **Queued Processing**: Requests over limits are queued with priority ordering
- **Queue Full**: When queue is full, requests are rejected with `429 Too Many Requests`
- **Timeout**: Requests in queue longer than `request_timeout` are rejected

## Circuit Breaker

### Protection Mechanisms

The circuit breaker provides three-state protection:

1. **CLOSED**: Normal operation, all requests pass through
2. **OPEN**: All requests are rejected immediately after 5 consecutive failures
3. **HALF-OPEN**: Limited requests allowed to test recovery (3 requests max)

### Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| Failure Threshold | 5 | Number of failures before opening circuit |
| Reset Timeout | 60s | Time before attempting recovery |
| Half-Open Calls | 3 | Max calls allowed in half-open state |

### Monitoring Headers

All responses include circuit breaker status:

```http
X-CircuitBreaker-State: CLOSED
X-CircuitBreaker-Failures: 0
X-CircuitBreaker-Reason: none
```

## Load Testing

### Built-in Scenarios

```bash
# Build load testing tool
make build-loadtest

# List available scenarios
./loadtest -list

# Run light load test
./loadtest -scenario light_load -output results.json

# Run custom test
./loadtest -clients 100 -rate 500 -duration 5m -endpoint http://localhost:8080
```

### Available Scenarios

| Scenario | Clients | Rate | Duration | Purpose |
|----------|---------|------|----------|---------|
| `light_load` | 10 | 30 RPS | 2m | Basic functionality |
| `moderate_load` | 50 | 200 RPS | 5m | Mixed request types |
| `heavy_load` | 200 | 1000 RPS | 10m | Performance limits |
| `burst_test` | 100 | 2000 RPS | 3m | Burst traffic |
| `rate_limit_stress` | 150 | 1500 RPS | 15m | Rate limiting stress |
| `circuit_breaker_test` | 50 | 300 RPS | 5m | Circuit breaker behavior |

### Performance Assessment

Load test results include automatic performance assessment:

- **Excellent**: >1000 RPS, ≥99% success, ≤100ms latency
- **Good**: 500-1000 RPS, 95-99% success, 100-500ms latency
- **Moderate**: 100-500 RPS, 90-95% success, 500ms-1s latency
- **Poor**: <100 RPS, <90% success, >1s latency

## Monitoring & Debugging

### HTTP Headers

All responses include comprehensive monitoring headers:

```http
# Rate Limiting
X-RateLimit-Limit-RPM: 1000
X-RateLimit-Limit-TPM: 1000000
X-RateLimit-Queue-Length: 15

# Circuit Breaker
X-CircuitBreaker-State: CLOSED
X-CircuitBreaker-Failures: 0

# Request Processing
X-RateLimit-Delay: 0ms
X-RateLimit-Reason: none
```

### Error Responses

Different error types provide specific information:

```http
# Rate Limited
HTTP/1.1 429 Too Many Requests
X-RateLimit-Reason: queue_full
Retry-After: 60

# Circuit Breaker Open
HTTP/1.1 503 Service Unavailable
X-CircuitBreaker-Reason: circuit_open
Retry-After: 60

# Upstream Error
HTTP/1.1 502 Bad Gateway
X-CircuitBreaker-Reason: upstream_error
```

## Best Practices

### Configuration Guidelines

1. **Start Conservative**: Begin with lower limits and monitor performance
2. **Monitor Queues**: Keep queue depth reasonable for your use case
3. **Adjust Timeouts**: Set appropriate timeouts for your SLA requirements
4. **Test Thoroughly**: Use load testing to validate configuration

### Performance Optimization

1. **Token Estimation**: The system uses word-based estimation (1 token ≈ 0.75 words)
2. **Batch Requests**: Smaller requests generally get better priority during high load
3. **Monitor Circuit Breaker**: Frequent trips indicate upstream issues
4. **Queue Management**: Balance queue depth vs. response time requirements

### Troubleshooting

#### High Latency
- Check queue length in response headers
- Verify `priority_threshold` is appropriate
- Consider increasing `rpm_limit` or `tpm_limit`
- Monitor circuit breaker state

#### Many Rejections
- Review rate limit configuration
- Check for circuit breaker trips
- Verify request patterns match expected usage
- Consider increasing `max_queue_depth`

#### Circuit Breaker Issues
- Monitor upstream service health
- Check network connectivity to Cerebras API
- Review error logs for failure patterns
- Adjust circuit breaker thresholds if needed

## API Reference

### Configuration Structure

```yaml
cerebras_limits:
  rpm_limit: int              # Required: Requests per minute
  tpm_limit: int              # Required: Tokens per minute
  max_queue_depth: int         # Optional: Queue capacity (default: 100)
  request_timeout: duration    # Optional: Queue timeout (default: 10m)
  priority_threshold: float64  # Optional: Priority threshold (default: 0.7)
```

### Response Headers

| Header | Description | Example |
|--------|-------------|---------|
| `X-RateLimit-Limit-RPM` | RPM limit setting | `1000` |
| `X-RateLimit-Limit-TPM` | TPM limit setting | `1000000` |
| `X-RateLimit-Queue-Length` | Current queue depth | `15` |
| `X-CircuitBreaker-State` | Circuit breaker state | `CLOSED` |
| `X-CircuitBreaker-Failures` | Recent failure count | `0` |
| `X-RateLimit-Delay` | Processing delay | `0ms` |
| `X-RateLimit-Reason` | Rejection reason | `queue_full` |

### Error Codes

| Code | Meaning | Cause |
|------|---------|-------|
| 429 | Too Many Requests | Rate limit exceeded or queue full |
| 502 | Bad Gateway | Upstream service error |
| 503 | Service Unavailable | Circuit breaker open |

## Integration Examples

### Node.js

```javascript
const axios = require('axios');

async function cerebrasRequest(prompt) {
  try {
    const response = await axios.post('http://localhost:8080/v1/chat/completions', {
      model: 'llama3.1-8b',
      messages: [{ role: 'user', content: prompt }],
      max_tokens: 100
    }, {
      headers: {
        'Host': 'api.cerebras.ai',
        'Content-Type': 'application/json'
      }
    });

    console.log('Queue length:', response.headers['x-ratelimit-queue-length']);
    console.log('Circuit breaker:', response.headers['x-circuitbreaker-state']);

    return response.data;
  } catch (error) {
    if (error.response?.status === 429) {
      console.log('Rate limited, retry after:', error.response.headers['retry-after']);
    }
    throw error;
  }
}
```

### Python

```python
import requests
import time

def cerebras_request(prompt):
    headers = {
        'Host': 'api.cerebras.ai',
        'Content-Type': 'application/json'
    }

    data = {
        'model': 'llama3.1-8b',
        'messages': [{'role': 'user', 'content': prompt}],
        'max_tokens': 100
    }

    while True:
        response = requests.post(
            'http://localhost:8080/v1/chat/completions',
            headers=headers,
            json=data
        )

        if response.status_code == 200:
            print(f"Queue: {response.headers.get('x-ratelimit-queue-length')}")
            return response.json()
        elif response.status_code == 429:
            retry_after = int(response.headers.get('retry-after', 60))
            print(f"Rate limited, retrying in {retry_after}s")
            time.sleep(retry_after)
        else:
            response.raise_for_status()
```

## Support

For issues, questions, or contributions:

1. **Documentation**: Check this guide and configuration examples
2. **Issues**: Report bugs via GitHub Issues
3. **Load Testing**: Use built-in scenarios for validation
4. **Monitoring**: Check response headers for real-time status