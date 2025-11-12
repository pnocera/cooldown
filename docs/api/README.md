# API Documentation

## Overview

Cooldown Proxy acts as a reverse proxy that applies intelligent rate limiting to outgoing HTTP requests. This document describes how to use the proxy effectively.

## Proxy Behavior

The proxy intercepts HTTP requests and:

1. **Rate Limits**: Applies per-domain rate limiting based on configuration
2. **Routes**: Forwards requests to the appropriate target server
3. **Responds**: Returns the target server's response to the client

## Request Format

### HTTP Headers

The proxy uses the `Host` header to determine rate limiting and routing:

```http
GET /path/to/resource HTTP/1.1
Host: api.example.com
Authorization: Bearer token123
Content-Type: application/json
```

### Supported Methods

All HTTP methods are supported:
- `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `HEAD`, `OPTIONS`

## Rate Limiting

### Per-Domain Limits

Each domain has its own rate limiter based on the configuration:

```yaml
rate_limits:
  - domain: "api.github.com"
    requests_per_second: 10
  - domain: "*.example.com"
    requests_per_second: 20
```

### Wildcard Matching

Wildcards match subdomains:

- `*.example.com` matches:
  - `api.example.com`
  - `www.example.com`
  - `v1.api.example.com`

### Default Rate Limit

Unspecified domains use the default rate limit:

```yaml
default_rate_limit:
  requests_per_second: 1
```

### Response Headers

The proxy adds rate limiting information to response headers:

```http
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
X-RateLimit-Reset: 1640995200
```

## Error Handling

### HTTP Status Codes

| Status | Description |
|--------|-------------|
| 200 | Success |
| 404 | No route found for domain |
| 429 | Rate limit exceeded (rare - requests are typically delayed) |
| 502 | Bad gateway (target server error) |
| 503 | Service unavailable |

### Error Response Format

```json
{
  "error": "No route found for host: unknown.example.com",
  "status": 404
}
```

## Usage Examples

### Basic Request

```bash
curl -H "Host: api.github.com" http://localhost:8080/users/octocat
```

### With Authentication

```bash
curl -H "Host: api.stripe.com" \
     -H "Authorization: Bearer sk_test_..." \
     http://localhost:8080/v1/customers
```

### POST Request

```bash
curl -X POST \
     -H "Host: api.example.com" \
     -H "Content-Type: application/json" \
     -d '{"name": "John Doe"}' \
     http://localhost:8080/users
```

## Configuration Examples

### High-Traffic API Gateway

```yaml
server:
  host: "0.0.0.0"
  port: 8080

rate_limits:
  - domain: "api.payment.com"
    requests_per_second: 1000
  - domain: "api.analytics.com"
    requests_per_second: 500
  - domain: "api.notification.com"
    requests_per_second: 200

default_rate_limit:
  requests_per_second: 10
```

### Development Environment

```yaml
server:
  host: "localhost"
  port: 3000

rate_limits:
  - domain: "*.dev"
    requests_per_second: 100
  - domain: "localhost"
    requests_per_second: 1000

default_rate_limit:
  requests_per_second: 50
```

### Multi-Tenant Setup

```yaml
server:
  host: "0.0.0.0"
  port: 80

rate_limits:
  # Enterprise customers
  - domain: "api.enterprise-client.com"
    requests_per_second: 1000
  
  # Standard customers
  - domain: "*.standard-client.com"
    requests_per_second: 100
  
  # Free tier
  - domain: "*.free-client.com"
    requests_per_second: 10

default_rate_limit:
  requests_per_second: 1
```

## Performance Considerations

### Request Processing

- **Latency**: ~1-2ms additional overhead per request
- **Memory**: Minimal per-request memory allocation
- **Concurrency**: Safe for high concurrent loads

### Rate Limiting Behavior

- **Smooth Distribution**: Requests are evenly spaced
- **Burst Capacity**: Short bursts allowed within limits
- **Graceful Degradation**: Excess requests are delayed, not rejected

## Monitoring

### Metrics to Monitor

1. **Request Rate**: Total requests per second
2. **Response Times**: Latency percentiles
3. **Error Rates**: 4xx/5xx response percentages
4. **Rate Limit Hits**: Requests being delayed

### Health Checks

```bash
# Basic health check
curl http://localhost:8080/health

# Check if proxy is responsive
curl -I http://localhost:8080
```

## Security Considerations

### Input Validation

The proxy validates:
- Host header format
- Request URI structure
- HTTP method validity

### Recommended Practices

1. **Use HTTPS**: Configure TLS termination
2. **Authentication**: Implement upstream authentication
3. **Logging**: Monitor for abuse patterns
4. **Network Security**: Use firewall rules appropriately

## Integration Examples

### Node.js Client

```javascript
const axios = require('axios');

const client = axios.create({
  baseURL: 'http://localhost:8080',
  headers: {
    'Host': 'api.example.com'
  }
});

async function fetchData() {
  try {
    const response = await client.get('/data');
    return response.data;
  } catch (error) {
    console.error('Request failed:', error.message);
  }
}
```

### Python Client

```python
import requests

session = requests.Session()
session.headers.update({'Host': 'api.example.com'})

def fetch_data():
    try:
        response = session.get('http://localhost:8080/data')
        response.raise_for_status()
        return response.json()
    except requests.RequestException as e:
        print(f"Request failed: {e}")
        return None
```

### Go Client

```go
package main

import (
    "net/http"
    "io/ioutil"
    "fmt"
)

func main() {
    client := &http.Client{}
    
    req, _ := http.NewRequest("GET", "http://localhost:8080/data", nil)
    req.Header.Set("Host", "api.example.com")
    
    resp, err := client.Do(req)
    if err != nil {
        fmt.Printf("Request failed: %v\n", err)
        return
    }
    defer resp.Body.Close()
    
    body, _ := ioutil.ReadAll(resp.Body)
    fmt.Printf("Response: %s\n", body)
}
```