# Reverse Proxy with Rate Limiting - Design Document

**Date:** 2025-11-12  
**Author:** Claude  
**Status:** Design Phase

## Overview

A local-first reverse proxy server that applies intelligent rate limiting to outgoing REST API requests. The proxy uses the leaky bucket algorithm to add delays rather than blocking requests, preventing 409/429 errors from external APIs.

## Architecture

### Core Components

1. **HTTP Server** (`net/http`) - Listens for incoming requests on localhost:8080
2. **Rate Limiter** (`uber-go/ratelimit`) - Leaky bucket implementation per domain
3. **Proxy Handler** (`net/http/httputil.ReverseProxy`) - Handles request forwarding
4. **Configuration Manager** - Loads static config on startup
5. **Request Router** - Routes to downstream APIs

### Data Flow

```
Client App → Local Proxy → [Rate Limiting Delay] → External API
```

1. Client sends request to proxy
2. Proxy extracts target domain from request
3. Rate limiter calculates required delay for that domain
4. Request waits for calculated delay time
5. Proxy forwards request with all headers, query params, and body
6. Response is returned to client

## Technology Stack

### Core Dependencies

- **Go 1.21+** - Base runtime
- **net/http/httputil.ReverseProxy** - Standard library reverse proxy (handles headers, query params, body forwarding automatically)
- **uber-go/ratelimit** - Production-ready leaky bucket rate limiter
- **gopkg.in/yaml.v3** - Configuration file parsing

### Why These Libraries

**httputil.ReverseProxy**: 
- Built into Go standard library
- Automatically forwards all HTTP headers, query parameters, and request body
- Handles chunked encoding and streaming
- Battle-tested and performant

**uber-go/ratelimit**:
- Uber's production leaky bucket implementation
- High-performance with minimal allocations
- No external dependencies
- Precise timing control for delay-based rate limiting

## Configuration

### Config Format (YAML)

```yaml
server:
  host: "localhost"
  port: 8080

rate_limits:
  # Domain-based rate limiting
  - domain: "api.github.com"
    requests_per_second: 10
  - domain: "api.twitter.com" 
    requests_per_second: 5
  - domain: "*.example.com"
    requests_per_second: 20

# Optional: Default rate limit for unspecified domains
default_rate_limit:
  requests_per_second: 1
```

### Key Features

- **Per-domain configuration** - Different limits per API
- **Wildcard support** - Pattern matching for subdomains
- **Default fallback** - Safe default when domain not specified
- **No hot reload** - Simple restart for config changes

## Rate Limiting Algorithm

### Leaky Bucket Implementation

The leaky bucket algorithm smooths out request timing:

1. **Bucket State**: Tracks last request time per domain
2. **Delay Calculation**: `delay = max(0, (1/rate) - time_since_last_request)`
3. **Request Processing**: Sleep for calculated delay, then forward
4. **Benefits**: 
   - Prevents request bursts
   - Maintains consistent request flow
   - No request blocking/rejection

### Example Behavior

For 10 requests/second rate limit:
- Request 1: No delay (first request)
- Request 2: 100ms delay if immediate
- Request 3: 200ms delay if immediate
- Smooths out to consistent 100ms intervals

## Implementation Details

### Request Forwarding

Using `httputil.ReverseProxy` provides automatic:
- **Header Forwarding**: All incoming headers forwarded to target
- **Query Parameters**: URL query parameters preserved
- **Request Body**: Streaming of POST/PUT/PATCH bodies
- **Response Headers**: All response headers returned to client
- **Status Codes**: Original HTTP status codes preserved
- **Chunked Transfer**: Handles streaming responses

### Domain Extraction

From incoming request URL:
1. Parse Host header
2. Extract domain name
3. Match against configuration patterns
4. Apply appropriate rate limit

### Error Handling

- **Config Errors**: Fail fast on startup with clear error messages
- **Invalid URLs**: Return 400 Bad Request
- **Target Unreachable**: Return 502 Bad Gateway
- **Rate Limit Errors**: Should not occur due to delay approach

## Performance Considerations

### Memory Usage
- Minimal: Only tracking last request timestamp per domain
- Scales with number of unique domains in config

### Concurrency
- Each request handled in separate goroutine
- Rate limiter uses atomic operations for thread safety
- No global locks, high throughput

### Network Efficiency
- Direct streaming proxy (no buffering of large responses)
- Keep-alive connections reused where possible
- Minimal latency overhead beyond calculated delays

## Testing Strategy

### Unit Tests
- Rate limiter delay calculations
- Configuration parsing and validation
- Domain pattern matching

### Integration Tests
- End-to-end request forwarding
- Rate limiting behavior verification
- Header and query parameter preservation

### Performance Tests
- Throughput under various rate limits
- Memory usage with many domains
- Concurrent request handling

## Deployment

### Running the Proxy

```bash
# Build
go build -o cooldown-proxy ./cmd/proxy

# Run with default config
./cooldown-proxy

# Run with custom config
./cooldown-proxy -config /path/to/config.yaml
```

### Client Configuration

Configure applications to use `http://localhost:8080` as their HTTP proxy.

### Monitoring

- Basic metrics via HTTP endpoint (/metrics)
- Request counts per domain
- Average delays applied
- Error rates

## Future Enhancements

### Potential Features (Not in MVP)
- Configuration hot-reloading
- Per-API-key rate limiting
- Request/response logging
- Metrics with Prometheus format
- Admin HTTP interface
- WebSocket proxy support

### Scaling Considerations
- Distributed rate limiting (Redis backend)
- Multiple proxy instances behind load balancer
- Configuration management system integration

## Conclusion

This design provides a simple, efficient solution for rate limiting outgoing API requests using proven Go libraries. The leaky bucket approach ensures smooth request timing without blocking, and the standard library `httputil.ReverseProxy` handles all the complexities of HTTP forwarding automatically.

The architecture prioritizes:
- **Simplicity** - Minimal dependencies and configuration
- **Reliability** - Battle-tested libraries and algorithms  
- **Performance** - Efficient memory usage and concurrency
- **Maintainability** - Clear separation of concerns