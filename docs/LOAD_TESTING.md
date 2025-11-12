# Load Testing Guide

## Overview

The Cooldown Proxy includes a comprehensive load testing framework specifically designed for testing Cerebras AI rate limiting functionality. This guide covers how to use the load testing tools, interpret results, and optimize performance.

## Getting Started

### Build Load Testing Tool

```bash
# Build the load testing CLI tool
make build-loadtest

# Or build manually
go build -o loadtest ./cmd/loadtest
```

### Basic Usage

```bash
# List available scenarios
./loadtest -list

# Run a predefined scenario
./loadtest -scenario light_load

# Run custom configuration
./loadtest -clients 100 -rate 500 -duration 5m -endpoint http://localhost:8080

# Save results to JSON file
./loadtest -scenario moderate_load -output test-results.json
```

## Predefined Scenarios

### 1. Light Load Test
```bash
./loadtest -scenario light_load
```

**Configuration:**
- Clients: 10
- Request Rate: 30 RPS
- Duration: 2 minutes
- Purpose: Basic functionality verification

**Use Case:** Initial testing and development validation

### 2. Moderate Load Test
```bash
./loadtest -scenario moderate_load
```

**Configuration:**
- Clients: 50
- Request Rate: 200 RPS
- Duration: 5 minutes
- Purpose: Mixed request types and moderate traffic

**Use Case:** Staging environment testing and performance validation

### 3. Heavy Load Test
```bash
./loadtest -scenario heavy_load
```

**Configuration:**
- Clients: 200
- Request Rate: 1000 RPS
- Duration: 10 minutes
- Purpose: Performance limit testing

**Use Case:** Production capacity planning and stress testing

### 4. Burst Test
```bash
./loadtest -scenario burst_test
```

**Configuration:**
- Clients: 100
- Request Rate: 2000 RPS
- Duration: 3 minutes
- Purpose: Burst traffic pattern testing

**Use Case:** Testing system behavior under sudden traffic spikes

### 5. Rate Limit Stress Test
```bash
./loadtest -scenario rate_limit_stress
```

**Configuration:**
- Clients: 150
- Request Rate: 1500 RPS
- Duration: 15 minutes
- Purpose: Rate limiting functionality validation

**Use Case:** Rate limit configuration tuning and validation

### 6. Circuit Breaker Test
```bash
./loadtest -scenario circuit_breaker_test
```

**Configuration:**
- Clients: 50
- Request Rate: 300 RPS
- Duration: 5 minutes
- Timeout: 10 seconds (shorter to trigger timeouts)

**Use Case:** Circuit breaker behavior validation and testing

## Custom Configuration

### Command Line Options

| Option | Description | Example |
|--------|-------------|---------|
| `-scenario` | Predefined scenario to use | `-scenario light_load` |
| `-clients` | Number of concurrent clients | `-clients 100` |
| `-duration` | Test duration | `-duration 5m` |
| `-rate` | Requests per second | `-rate 500` |
| `-endpoint` | Target endpoint | `-endpoint http://localhost:8080` |
| `-output` | JSON results file | `-output results.json` |
| `-list` | List available scenarios | `-list` |

### Custom Scenarios

```bash
# Small custom test
./loadtest -clients 5 -rate 10 -duration 1m -endpoint http://localhost:8080

# Medium custom test
./loadtest -clients 50 -rate 200 -duration 5m -endpoint http://localhost:8080 -output medium-test.json

# Large custom test
./loadtest -clients 500 -rate 2000 -duration 10m -endpoint http://localhost:8080 -output large-test.json

# Very short test for quick validation
./loadtest -clients 10 -rate 50 -duration 30s -endpoint http://localhost:8080
```

## Understanding Results

### Console Output

```
=== Load Test Results ===
Total Requests: 12000
Successful Requests: 11850 (98.75%)
Failed Requests: 150 (1.25%)
Test Duration: 2m0s
Average Latency: 125ms
Min Latency: 45ms
Max Latency: 850ms
Requests per Second: 100.00

Circuit Breaker Statistics:
  State Changes: 0
  Circuit Opens: 0
  Circuit Closes: 0
  Rejected Requests: 0

=== Performance Assessment ===
✅ Excellent performance (>1000 RPS)
✅ Excellent success rate (≥99%)
⚠️  Moderate latency (500ms-1s)
```

### JSON Output Format

```json
{
  "total_requests": 12000,
  "successful_requests": 11850,
  "failed_requests": 150,
  "duration": "2m0s",
  "average_latency": "125ms",
  "min_latency": "45ms",
  "max_latency": "850ms",
  "requests_per_second": 100.0,
  "error_breakdown": {
    "http_429": 100,
    "http_503": 50
  },
  "circuit_breaker_stats": {
    "state_changes": 0,
    "circuit_opens": 0,
    "circuit_closes": 0,
    "rejected_requests": 0
  }
}
```

### Performance Metrics

#### Throughput Metrics
- **Requests per Second (RPS)**: Total throughput
- **Success Rate**: Percentage of successful requests
- **Error Rate**: Percentage of failed requests

#### Latency Metrics
- **Average Latency**: Mean response time
- **Min Latency**: Fastest response time
- **Max Latency**: Slowest response time

#### Circuit Breaker Metrics
- **State Changes**: Number of circuit breaker state transitions
- **Circuit Opens**: Times circuit moved to OPEN state
- **Rejected Requests**: Requests rejected by circuit breaker

#### Error Breakdown
- **HTTP 429**: Rate limit exceeded
- **HTTP 502**: Upstream service errors
- **HTTP 503**: Circuit breaker open
- **Network Errors**: Connection/timeout issues
- **Marshal Errors**: Request payload issues

## Performance Benchmarks

### Expected Performance Tiers

| Tier | RPS | Success Rate | Latency | Use Case |
|------|-----|-------------|---------|----------|
| **Excellent** | >1000 | ≥99% | ≤100ms | High-performance production |
| **Good** | 500-1000 | 95-99% | 100-500ms | Standard production |
| **Moderate** | 100-500 | 90-95% | 500ms-1s | Development/staging |
| **Poor** | <100 | <90% | >1s | Needs optimization |

### Configuration Impact

#### High Rate Limits (Production)
```yaml
cerebras_limits:
  rpm_limit: 5000
  tpm_limit: 5000000
  max_queue_depth: 500
  request_timeout: 5m
```
**Expected Results:** 500-1000+ RPS, <500ms latency

#### Moderate Rate Limits (Staging)
```yaml
cerebras_limits:
  rpm_limit: 1000
  tpm_limit: 1000000
  max_queue_depth: 100
  request_timeout: 10m
```
**Expected Results:** 200-500 RPS, 500ms-1s latency

#### Conservative Rate Limits (Development)
```yaml
cerebras_limits:
  rpm_limit: 60
  tpm_limit: 100000
  max_queue_depth: 20
  request_timeout: 30s
```
**Expected Results:** 10-100 RPS, >500ms latency

## Testing Methodology

### 1. Baseline Testing
```bash
# Start with conservative settings
./loadtest -scenario light_load

# Gradually increase load
./loadtest -clients 20 -rate 100 -duration 2m
./loadtest -clients 50 -rate 200 -duration 2m
./loadtest -clients 100 -rate 400 -duration 2m
```

### 2. Rate Limit Validation
```bash
# Test rate limit enforcement
./loadtest -scenario rate_limit_stress

# Monitor queue behavior
./loadtest -clients 200 -rate 1500 -duration 5m -output rate-test.json
```

### 3. Circuit Breaker Testing
```bash
# Test circuit breaker behavior
./loadtest -scenario circuit_breaker_test

# Monitor recovery
./loadtest -clients 50 -rate 300 -duration 10m
```

### 4. Sustained Load Testing
```bash
# Long-duration testing
./loadtest -clients 100 -rate 500 -duration 30m -output sustained.json

# Burst testing
./loadtest -scenario burst_test
```

## Troubleshooting

### High Latency Issues

**Symptoms:**
- Average latency > 1s
- Max latency > 5s
- High queue length headers

**Solutions:**
```yaml
# Increase rate limits
cerebras_limits:
  rpm_limit: 2000        # Increase
  tpm_limit: 2000000     # Increase

# Reduce queue depth
max_queue_depth: 50       # Decrease

# Adjust priority threshold
priority_threshold: 0.6   # Lower for more aggressive prioritization
```

### Low Throughput Issues

**Symptoms:**
- RPS below expected
- High error rate
- Many 429 responses

**Solutions:**
```yaml
# Increase limits
cerebras_limits:
  rpm_limit: 1500        # Increase
  tpm_limit: 1500000     # Increase

# Larger queue
max_queue_depth: 200      # Increase

# Longer timeout
request_timeout: 15m      # Increase
```

### Circuit Breaker Issues

**Symptoms:**
- Many 503 responses
- Circuit breaker frequently opens
- Low success rate

**Solutions:**
```yaml
# More conservative circuit breaker (in code)
maxFailures: 10          # Increase failure threshold
resetTimeout: 120s       # Increase reset timeout
halfOpenMaxCalls: 5      # More recovery attempts
```

## Advanced Testing

### Custom Request Payloads

The load test framework uses predefined test data, but you can modify `cmd/loadtest/main.go` to use custom payloads:

```go
TestData: []TestData{
    {
        Model:     "llama3.1-8b",
        Prompt:    "Custom prompt for testing",
        MaxTokens: 200,
    },
},
```

### Multiple Endpoint Testing

```bash
# Test different endpoints
./loadtest -endpoint http://localhost:8080 -scenario moderate_load
./loadtest -endpoint http://localhost:9090 -scenario moderate_load
./loadtest -endpoint https://your-proxy.com -scenario moderate_load
```

### Automated Testing Script

```bash
#!/bin/bash
# test-scenarios.sh

echo "Running comprehensive load tests..."

for scenario in light_load moderate_load heavy_load; do
    echo "Testing $scenario..."
    ./loadtest -scenario $scenario -output "results-$scenario.json"

    # Check if results meet criteria
    rps=$(jq -r '.requests_per_second' "results-$scenario.json")
    success_rate=$(jq '(.successful_requests / .total_requests) * 100' "results-$scenario.json")

    echo "RPS: $rps, Success Rate: $success_rate%"

    if (( $(echo "$rps < 100" | bc -l) )); then
        echo "⚠️  Low throughput detected"
    fi

    if (( $(echo "$success_rate < 95" | bc -l) )); then
        echo "❌ Low success rate detected"
    fi
done

echo "Load testing completed!"
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Load Testing

on:
  push:
    branches: [ main ]

jobs:
  load-test:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v2

    - name: Setup Go
      uses: actions/setup-go@v2
      with:
        go-version: 1.21

    - name: Build applications
      run: |
        make build
        make build-loadtest

    - name: Start proxy server
      run: |
        ./cooldown-proxy -config config.yaml &
        sleep 10

    - name: Run load tests
      run: |
        ./loadtest -scenario light_load -output light-load.json
        ./loadtest -scenario moderate_load -output moderate-load.json

    - name: Validate results
      run: |
        # Check if performance meets minimum criteria
        rps=$(jq -r '.requests_per_second' moderate-load.json)
        success_rate=$(jq '(.successful_requests / .total_requests) * 100' moderate-load.json)

        if (( $(echo "$rps < 50" | bc -l) )); then
          echo "Performance regression detected: RPS $rps < 50"
          exit 1
        fi

        if (( $(echo "$success_rate < 90" | bc -l) )); then
          echo "Success rate regression: $success_rate% < 90%"
          exit 1
        fi

    - name: Upload results
      uses: actions/upload-artifact@v2
      with:
        name: load-test-results
        path: "*.json"
```

## Best Practices

### Test Planning
1. **Start Small**: Begin with light_load and gradually increase
2. **Monitor Resources**: Watch CPU, memory, and network usage
3. **Test Realistic Scenarios**: Use patterns that match your usage
4. **Establish Baselines**: Document normal performance metrics

### Test Execution
1. **Warm-up Period**: Allow system to stabilize before measuring
2. **Multiple Runs**: Run tests multiple times for consistency
3. **Result Validation**: Always review results for anomalies
4. **Resource Monitoring**: Monitor system resources during tests

### Result Analysis
1. **Compare Baselines**: Track performance over time
2. **Identify Bottlenecks**: Look for latency spikes or error patterns
3. **Validate Configuration**: Ensure rate limits work as expected
4. **Document Findings**: Keep records of test results and optimizations

### Production Readiness
1. **Performance Targets**: Establish minimum acceptable performance
2. **Load Testing**: Test with realistic production volumes
3. **Failure Testing**: Validate circuit breaker and error handling
4. **Monitoring Setup**: Ensure production monitoring is configured