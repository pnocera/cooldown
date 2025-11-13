# Model-Based Routing Design - Refined

## Overview

This design adds model-based routing capabilities to the Cooldown Proxy, allowing requests to be routed to different API endpoints based on the model field in JSON request bodies. This enables flexible configuration where different AI models can be proxied to their respective providers (Cerebras, OpenAI, Anthropic, etc.) through a single proxy endpoint.

## Problem Statement

Currently, the Cooldown Proxy only supports host-based routing, which requires different hostnames for different API endpoints. Users want to route requests based on the model field in the JSON request body, enabling:

- Single proxy endpoint for multiple AI providers
- Flexible model-to-API mappings
- Configurable routing without DNS changes
- Model-based rate limiting and monitoring

## Architecture

### Core Design Decisions

1. **Override Host-Based Routing**: Model routing overrides existing host-based routing when a model field is detected
2. **Direct URL Mapping**: Models map to full API endpoints (e.g., `gpt-4` → `https://api.openai.com/v1`)
3. **Streaming JSON Parsing**: Uses TeeReader to parse while preserving original payload
4. **Fallback Behavior**: Invalid/missing models route to default target rather than failing fast
5. **Inline Configuration**: Model routing config added to existing `config.yaml`
6. **All JSON Endpoints**: Applies to any JSON request for maximum flexibility

### Request Flow

```
HTTP Request → Model Routing Middleware → Rate Limiter → Router → Proxy
                ↓
          Check Content-Type
                ↓
          Stream Parse JSON
                ↓
          Extract Model Field
                ↓
          Look up Target URL
                ↓
          Rewrite Request
                ↓
          Continue Normal Flow
```

### Integration Architecture

The middleware sits between the HTTP server and existing handlers, intercepting requests before they reach the rate limiter. This ensures:

- Model routing doesn't interfere with existing rate limiting
- All requests (routed and non-routed) pass through the same monitoring
- Backward compatibility with existing host-based routing

## Configuration

### Inline Configuration Structure

```yaml
model_routing:
  enabled: true
  default_target: "https://api.openai.com/v1"
  models:
    "gpt-4": "https://api.openai.com/v1"
    "gpt-3.5-turbo": "https://api.openai.com/v1"
    "claude-3-opus": "https://api.anthropic.com/v1"
    "claude-3-sonnet": "https://api.anthropic.com/v1"
    "zai-glm-4.6": "https://api.cerebras.ai/v1"
```

### Configuration Types

```go
type ModelRoutingConfig struct {
    Enabled       bool              `yaml:"enabled"`
    DefaultTarget string            `yaml:"default_target"`
    Models        map[string]string `yaml:"models"`
}

// Extended Config struct
type Config struct {
    Server           ServerConfig        `yaml:"server"`
    RateLimits       []RateLimitRule     `yaml:"rate_limits"`
    DefaultRateLimit *RateLimitRule      `yaml:"default_rate_limit"`
    CerebrasLimits   CerebrasLimits      `yaml:"cerebras_limits"`
    ModelRouting     *ModelRoutingConfig `yaml:"model_routing"`
}
```

## Implementation Details

### Middleware Structure

```go
type ModelRoutingMiddleware struct {
    config      *config.ModelRoutingConfig
    nextHandler http.Handler
    logger      *log.Logger
}

func (m *ModelRoutingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if !m.shouldApplyRouting(r) {
        m.nextHandler.ServeHTTP(w, r)
        return
    }

    target, err := m.extractTargetFromModel(r)
    if err != nil || target == "" {
        // Fallback to default target on any error
        target = m.config.DefaultTarget
        m.logger.Printf("Model routing failed, using default: %v", err)
    }

    if target != "" {
        m.rewriteRequest(r, target)
    }

    m.nextHandler.ServeHTTP(w, r)
}
```

### Key Functions

1. **shouldApplyRouting(r *http.Request)**: Checks if request has `application/json` content type
2. **extractTargetFromModel(r *http.Request)**: Streams JSON parsing with TeeReader
3. **rewriteRequest(r *http.Request, target string)**: Updates request URL and Host header

### Streaming JSON Parser

The middleware uses a custom streaming approach:

```go
func (m *ModelRoutingMiddleware) extractModelFromBody(r *http.Request) (string, error) {
    if r.Body == nil {
        return "", nil
    }
    
    // Create TeeReader to stream while parsing
    var buf bytes.Buffer
    tee := io.TeeReader(r.Body, &buf)
    
    // Replace body with buffered content for downstream handlers
    r.Body = io.NopCloser(&buf)
    
    // Stream parse for model field
    return m.parseModelField(tee)
}
```

### Integration Point

In `cmd/proxy/main.go`, wrap the existing handler:

```go
// Create composite handler with existing components
compositeHandler := &CompositeHandler{
    cerebrasHandler: cerebrasHandler,
    standardRouter:  r,
}

// Wrap with model routing middleware if enabled
if cfg.ModelRouting != nil && cfg.ModelRouting.Enabled {
    compositeHandler = NewModelRoutingMiddleware(cfg.ModelRouting, compositeHandler)
}

// Start server with wrapped handler
server := &http.Server{
    Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
    Handler: compositeHandler,
}
```

## Error Handling

### Fallback Strategy

- **Malformed JSON**: Route to default target, log warning, continue processing
- **Missing model field**: Route to default target, no logging (normal for non-AI requests)
- **Unknown model**: Route to default target, log warning for monitoring
- **Invalid target URL**: Use fallback default, log error
- **Request body read errors**: Route to default target, log error

### Robustness Guarantees

- No request fails due to model routing errors
- Existing rate limiting and monitoring preserved
- Backward compatibility maintained for all configurations

## Performance Considerations

### Optimizations

1. **Selective Processing**: Only parse requests with `application/json` content type
2. **Streaming Parser**: Uses `io.TeeReader` to avoid buffering entire request bodies
3. **Fast Path**: Non-JSON requests bypass model routing immediately
4. **Minimal Allocation**: Reuses buffers and parsers where possible
5. **Configuration Caching**: Model mappings pre-validated on startup

### Memory Usage

- Request bodies are streamed, not loaded into memory
- Parser state is minimal (only tracks "model" key detection)
- No per-request persistent storage
- Error logging uses structured format to avoid allocations

### Performance Impact

- **Overhead**: < 1ms for typical JSON requests
- **Memory**: Constant overhead, no scaling with request size
- **Throughput**: No impact on non-JSON requests
- **Latency**: Minimal additional latency for JSON requests

## Monitoring Integration

### Metrics Collection

1. **Routing Metrics**: Track model usage and routing success rates
2. **Performance Metrics**: Monitor middleware overhead
3. **Error Tracking**: Log routing failures for alerting
4. **Usage Analytics**: Model popularity and API endpoint utilization

### Logging Strategy

```go
// Structured logging for monitoring
logger.Info("model_routing_attempt",
    zap.String("model", modelName),
    zap.String("target", targetURL),
    zap.Bool("success", true),
    zap.Duration("duration", time.Since(start)),
)
```

## Testing Strategy

### Unit Tests

1. **Middleware Behavior**: Test request interception and routing logic
2. **Streaming Parser**: Verify JSON streaming with various payload sizes
3. **Configuration Loading**: Validate configuration parsing and validation
4. **Error Handling**: Test all error conditions and fallback behavior
5. **Edge Cases**: Empty requests, large payloads, concurrent requests

### Integration Tests

1. **End-to-End Routing**: Test complete request flow through proxy
2. **Multiple Models**: Verify different models route to correct endpoints
3. **Fallback Behavior**: Test default routing for unknown models
4. **Backward Compatibility**: Ensure existing functionality unchanged

### Load Tests

1. **Performance Impact**: Measure overhead with and without model routing
2. **Concurrent Requests**: Test thread safety and performance under load
3. **Memory Usage**: Monitor memory consumption with large JSON payloads
4. **Streaming Validation**: Verify no request body corruption

## Migration Path

### Phase 1: Core Implementation
- Implement middleware with streaming model extraction and routing
- Add configuration structure and loading
- Basic unit tests for core functionality

### Phase 2: Integration
- Integrate with main application
- Add comprehensive error handling
- Integration tests with existing components

### Phase 3: Production Readiness
- Performance optimization and monitoring
- Comprehensive testing suite
- Documentation and deployment examples

### Deployment Strategy

1. **Feature Flag**: Model routing disabled by default (enabled: false)
2. **Configuration Validation**: Validate all URLs on startup
3. **Gradual Rollout**: Enable for specific models initially
4. **Monitoring**: Add metrics for routing success/failure rates

## Security Considerations

### Input Validation

1. **JSON Parsing**: Validate JSON structure, prevent injection attacks
2. **Model Names**: Sanitize model names to prevent path traversal
3. **URL Validation**: Validate target URLs to prevent SSRF attacks
4. **Size Limits**: Enforce request body size limits (inherited from main proxy)

### Access Control

1. **Model Restrictions**: Configure which models can be routed to which endpoints
2. **Authentication**: Preserve and forward authentication headers properly
3. **Audit Logging**: Log routing decisions for security auditing

## Future Enhancements

### Advanced Features

1. **Header-based Model Detection**: Support model specification in headers
2. **Path-based Routing**: Support model specification in URL paths
3. **Load Balancing**: Multiple target URLs per model with load balancing
4. **Model Aliases**: Support model name mappings and synonyms
5. **Dynamic Configuration**: Hot-reload model routing configuration

### Monitoring Enhancements

1. **Real-time Analytics**: Live model usage dashboards
2. **Cost Tracking**: Model-specific cost estimation and budgeting
3. **Performance Tuning**: Automatic optimization based on usage patterns
4. **Integration Metrics**: Per-endpoint success rates and latency tracking

## Conclusion

This refined design provides a robust, performant, and backward-compatible solution for model-based routing in the Cooldown Proxy. The key improvements over the initial design include:

- **Override semantics**: Model routing takes precedence over host-based routing
- **Direct URL mapping**: Eliminates ambiguity in routing targets
- **Streaming approach**: Better performance for large JSON payloads
- **Fallback behavior**: Ensures robustness in error conditions
- **Inline configuration**: Simplifies deployment and management

The implementation maintains the existing architecture's strengths while adding powerful routing capabilities that scale with multiple AI providers and models.