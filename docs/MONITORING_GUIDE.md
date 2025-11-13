# Monitoring Guide - Header-Based Rate Limiting

## Overview

The Cooldown Proxy with header-based rate limiting includes comprehensive monitoring capabilities to track system performance, health, and operational metrics. This guide covers all monitoring aspects including health checks, metrics collection, and alerting.

## Feature Flags

The system uses feature flags to enable/disable functionality dynamically:

### Available Feature Flags

| Flag | Description | Default | Environment Variable |
|------|-------------|---------|---------------------|
| `header_based_rate_limiting` | Main header-based rate limiting feature | `true` | `COOLDOWN_HEADER_BASED_RATE_LIMITING` |
| `header_fallback` | Fallback to static limits when headers fail | `true` | `COOLDOWN_HEADER_FALLBACK` |
| `circuit_breaker_enhancement` | Enhanced circuit breaker integration | `true` | `COOLDOWN_CIRCUIT_BREAKER_ENHANCEMENT` |
| `metrics_collection` | Detailed metrics collection | `true` | `COOLDOWN_METRICS_COLLECTION` |
| `dynamic_queue_prioritization` | Smart queue prioritization | `true` | `COOLDOWN_DYNAMIC_QUEUE_PRIORITIZATION` |
| `rate_limit_buffer_adjustment` | Automatic buffer adjustment | `true` (50% rollout) | `COOLDOWN_RATE_LIMIT_BUFFER_ADJUSTMENT` |
| `header_validation_strict` | Strict header validation | `false` | `COOLDOWN_HEADER_VALIDATION_STRICT` |

### Using Feature Flags

```bash
# Disable header-based rate limiting
export COOLDOWN_HEADER_BASED_RATE_LIMITING=false

# Enable strict header validation
export COOLDOWN_HEADER_VALIDATION_STRICT=true

# Set rollout percentage for gradual deployment
export COOLDOWN_HEADER_BASED_RATE_LIMITING_ROLLOUT=25.0
```

## HTTP Monitoring Endpoints

The proxy exposes several HTTP endpoints for monitoring:

### Health Endpoints

#### `/health` - Basic Health Check
Returns simple health status.

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "healthy": true,
  "timestamp": "2025-01-12T17:30:00Z"
}
```

#### `/health/detailed` - Detailed Health Check
Returns comprehensive health status with metrics and issues.

```bash
curl http://localhost:8080/health/detailed
```

Response:
```json
{
  "status": "healthy",
  "healthy": true,
  "issues": [],
  "metrics": {
    "total_requests": 15420,
    "success_rate": 98.7,
    "error_rate": 1.3,
    "average_latency": "245ms",
    "header_parsing_rate": 96.2,
    "dynamic_rate_limit_rate": 94.1
  },
  "timestamp": "2025-01-12T17:30:00Z"
}
```

### Metrics Endpoints

#### `/metrics` - JSON Metrics
Returns current metrics in JSON format.

```bash
curl http://localhost:8080/metrics
```

#### `/metrics/prometheus` - Prometheus Metrics
Returns metrics in Prometheus exposition format.

```bash
curl http://localhost:8080/metrics/prometheus
```

#### `/metrics/reset` - Reset Metrics
Resets all collected metrics (for testing or recovery).

```bash
curl -X POST http://localhost:8080/metrics/reset
```

## Key Metrics

### Request Metrics

- **`cooldown_proxy_requests_total`** - Total number of requests processed
- **`cooldown_proxy_requests_success_total`** - Total successful requests
- **`cooldown_proxy_requests_error_total`** - Total failed requests
- **`cooldown_proxy_requests_rate_limited_total`** - Total rate-limited requests

### Rate Limiting Metrics

- **`cooldown_proxy_header_parsing_success_total`** - Successful header parsing attempts
- **`cooldown_proxy_header_parsing_errors_total`** - Header parsing failures
- **`cooldown_proxy_dynamic_rate_limit_hits_total`** - Requests using dynamic rate limits
- **`cooldown_proxy_static_fallback_hits_total`** - Requests using static fallback limits
- **`cooldown_proxy_queue_operations_total`** - Queue management operations

### Performance Metrics

- **`cooldown_proxy_requests_per_second`** - Current requests per second
- **`cooldown_proxy_success_rate`** - Success rate percentage
- **`cooldown_proxy_error_rate`** - Error rate percentage
- **`cooldown_proxy_average_latency_seconds`** - Average request latency
- **`cooldown_proxy_current_tpm_limit`** - Current TPM limit from headers
- **`cooldown_proxy_current_tpm_remaining`** - Current TPM remaining from headers

### System Metrics

- **`cooldown_proxy_circuit_breaker_opens_total`** - Circuit breaker open events
- **`cooldown_proxy_circuit_breaker_closes_total`** - Circuit breaker close events
- **`cooldown_proxy_uptime_seconds`** - System uptime in seconds

## Health Status Levels

### Healthy (`200 OK`)
- Error rate < 10%
- Header parsing rate > 90%
- No circuit breaker activity
- Average latency < 5 seconds

### Degraded (`503 Service Unavailable`)
- One or more thresholds exceeded:
  - Error rate > 10%
  - Header parsing rate < 90%
  - Rate limit hit rate > 50%
  - Average latency > 5 seconds

### Unhealthy (`503 Service Unavailable`)
- Circuit breaker has opened
- Multiple degradation factors present

## Monitoring Setup

### Prometheus Configuration

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'cooldown-proxy'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics/prometheus'
    scrape_interval: 15s
    scrape_timeout: 10s
```

### Grafana Dashboard

Key panels for monitoring:

1. **Request Volume**
   - Requests per second
   - Success rate
   - Error rate
   - Rate limit hit rate

2. **Header-Based Rate Limiting**
   - Header parsing success rate
   - Dynamic vs static rate limiting usage
   - Current TPM limits and remaining

3. **Performance**
   - Average, min, max latencies
   - P95 and P99 latencies
   - Queue depth

4. **System Health**
   - Circuit breaker status
   - Health check status
   - Uptime

### Example Grafana Queries

```promql
# Request rate
rate(cooldown_proxy_requests_total[5m])

# Success rate
(cooldown_proxy_requests_success_total / cooldown_proxy_requests_total) * 100

# Header parsing success rate
(cooldown_proxy_header_parsing_success_total / cooldown_proxy_header_parsing_attempts_total) * 100

# Dynamic rate limiting usage
(cooldown_proxy_dynamic_rate_limit_hits_total / (cooldown_proxy_dynamic_rate_limit_hits_total + cooldown_proxy_static_fallback_hits_total)) * 100

# Average latency
cooldown_proxy_average_latency_seconds

# Current TPM utilization
((cooldown_proxy_current_tpm_limit - cooldown_proxy_current_tpm_remaining) / cooldown_proxy_current_tpm_limit) * 100
```

## Alerting Rules

### Prometheus Alert Rules

```yaml
groups:
  - name: cooldown-proxy-alerts
    rules:
      # High error rate
      - alert: CooldownProxyHighErrorRate
        expr: cooldown_proxy_error_rate > 10
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High error rate in Cooldown Proxy"
          description: "Error rate is {{ $value }}% for the last 2 minutes"

      # Low header parsing rate
      - alert: CooldownProxyLowHeaderParsingRate
        expr: cooldown_proxy_header_parsing_rate < 90
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Low header parsing success rate"
          description: "Header parsing success rate is {{ $value }}%"

      # Circuit breaker opened
      - alert: CooldownProxyCircuitBreakerOpen
        expr: increase(cooldown_proxy_circuit_breaker_opens_total[5m]) > 0
        labels:
          severity: critical
        annotations:
          summary: "Circuit breaker has opened"
          description: "Circuit breaker has opened, indicating upstream issues"

      # High rate limit hit rate
      - alert: CooldownProxyHighRateLimitRate
        expr: cooldown_proxy_rate_limit_rate > 50
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High rate limit hit rate"
          description: "Rate limit hit rate is {{ $value }}%"

      # High latency
      - alert: CooldownProxyHighLatency
        expr: cooldown_proxy_average_latency_seconds > 5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High average latency"
          description: "Average latency is {{ $value }}s"
```

## Log Analysis

### Key Log Patterns

Monitor logs for these patterns:

```bash
# Header parsing issues
grep "Failed to parse rate limit headers" /var/log/cooldown-proxy.log

# Circuit breaker events
grep "Circuit breaker" /var/log/cooldown-proxy.log

# High queue depths
grep "queue depth" /var/log/cooldown-proxy.log

# Feature flag changes
grep "Feature flag" /var/log/cooldown-proxy.log
```

### Log Metrics

Track these log-derived metrics:

1. **Header parsing failures per minute**
2. **Circuit breaker state changes**
3. **Queue depth alerts**
4. **Feature flag toggles**

## Production Monitoring Best Practices

### 1. Alert Thresholds

- **Error rate**: Alert at >5% (warning), >15% (critical)
- **Header parsing rate**: Alert at <95% (warning), <85% (critical)
- **Average latency**: Alert at >2s (warning), >10s (critical)
- **Rate limit hit rate**: Alert at >30% (warning), >70% (critical)

### 2. Dashboard Refresh Rates

- **Real-time metrics**: 15-30 second refresh
- **Historical trends**: 5-15 minute refresh
- **Health status**: 30-60 second refresh

### 3. Data Retention

- **Raw metrics**: 15 days (Prometheus)
- **Aggregated metrics**: 90 days
- **Health status changes**: 180 days
- **Feature flag changes**: 365 days

### 4. Monitoring Redundancy

- Deploy multiple monitoring instances
- Use external monitoring (Pingdom, UptimeRobot)
- Set up alerting through multiple channels (email, Slack, PagerDuty)

## Troubleshooting

### Common Issues

1. **High Error Rate**
   - Check API credentials
   - Verify network connectivity
   - Review rate limit configuration

2. **Low Header Parsing Rate**
   - Verify API is returning proper headers
   - Check for API changes
   - Review header parsing logs

3. **Circuit Breaker Open**
   - Check upstream API status
   - Verify network connectivity
   - Review error patterns

4. **High Latency**
   - Check queue depth
   - Review rate limit configuration
   - Monitor system resources

### Diagnostic Commands

```bash
# Check current health
curl -s http://localhost:8080/health/detailed | jq .

# Check recent metrics
curl -s http://localhost:8080/metrics | jq '.requests_per_second, .success_rate, .header_parsing_rate'

# Check feature flags
curl -s http://localhost:8080/metrics | jq '.dynamic_rate_limit_rate'

# Reset metrics for troubleshooting
curl -X POST http://localhost:8080/metrics/reset
```

## Integration with Existing Systems

### Kubernetes Monitoring

```yaml
apiVersion: v1
kind: Service
metadata:
  name: cooldown-proxy
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/path: "/metrics/prometheus"
    prometheus.io/port: "8080"
spec:
  ports:
  - port: 8080
    name: http
  selector:
    app: cooldown-proxy
```

### Docker Health Check

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1
```

### Kubernetes Health Probe

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

## Conclusion

The monitoring system provides comprehensive visibility into the header-based rate limiting functionality. Regular monitoring and alerting ensure system reliability and early detection of issues.

Key metrics to watch:
- **Header parsing success rate** (>95% target)
- **Dynamic rate limiting usage** (>90% target)
- **Error rate** (<5% target)
- **Average latency** (<1s target)

With proper monitoring and alerting, the system can maintain high reliability and performance for production workloads.