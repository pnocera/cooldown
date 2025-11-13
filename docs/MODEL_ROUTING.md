# Model-Based Routing

Model-based routing allows the Cooldown Proxy to route requests to different API endpoints based on the `model` field in JSON request bodies. This enables:

- Single proxy endpoint for multiple AI providers
- Flexible model-to-API mappings
- Configurable routing without DNS changes
- Model-based rate limiting and monitoring

## Configuration

Add a `model_routing` section to your `config.yaml`:

```yaml
model_routing:
  # Enable model-based routing
  enabled: true
  # Default target for unknown/missing models
  default_target: "https://api.openai.com/v1"
  # Model mappings
  models:
    "gpt-4": "https://api.openai.com/v1"
    "claude-3": "https://api.anthropic.com/v1"
    "gemini-pro": "https://generativelanguage.googleapis.com/v1beta"
```

## How It Works

1. **Request Interception**: Middleware intercepts requests with `application/json` content type
2. **Streaming Parsing**: Uses `io.TeeReader` to parse JSON while preserving the original request body
3. **Model Extraction**: Extracts the `model` field from the JSON payload
4. **URL Lookup**: Looks up the target URL based on the model
5. **Request Rewriting**: Rewrites the request URL and Host header
6. **Fallback**: Uses `default_target` for unknown models or parsing errors

## Example Requests

### OpenAI GPT-4 Request
```bash
curl -X POST http://localhost:8080/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```
This request gets routed to `https://api.openai.com/v1/chat/completions`

### Anthropic Claude Request
```bash
curl -X POST http://localhost:8080/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-sonnet",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```
This request gets routed to `https://api.anthropic.com/v1/messages`

### Unknown Model (Fallback)
```bash
curl -X POST http://localhost:8080/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "unknown-model",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```
This request gets routed to the `default_target`: `https://api.openai.com/v1/chat/completions`

## Configuration Options

### enabled
- **Type**: `boolean`
- **Default**: `false`
- **Description**: Enable or disable model-based routing

### default_target
- **Type**: `string`
- **Required**: No
- **Description**: Fallback URL for requests without a model or with unknown models

### models
- **Type**: `map[string]string`
- **Required**: No
- **Description**: Mapping of model names to target API endpoints

## Supported Providers

### OpenAI
```yaml
models:
  "gpt-4": "https://api.openai.com/v1"
  "gpt-4-turbo": "https://api.openai.com/v1"
  "gpt-3.5-turbo": "https://api.openai.com/v1"
```

### Anthropic
```yaml
models:
  "claude-3-opus": "https://api.anthropic.com/v1"
  "claude-3-sonnet": "https://api.anthropic.com/v1"
  "claude-3-haiku": "https://api.anthropic.com/v1"
```

### Google
```yaml
models:
  "gemini-pro": "https://generativelanguage.googleapis.com/v1beta"
  "gemini-pro-vision": "https://generativelanguage.googleapis.com/v1beta"
```

### Cerebras
```yaml
models:
  "llama3-8b": "https://api.cerebras.ai/v1"
  "llama3-70b": "https://api.cerebras.ai/v1"
  "mixtral-8x7b": "https://api.cerebras.ai/v1"
```

## Backward Compatibility

Model routing is fully backward compatible:

- When disabled, all requests use existing host-based routing
- Non-JSON requests bypass model routing entirely
- Existing rate limiting and monitoring continue to work
- No DNS or client configuration changes required

## Security Considerations

- All target URLs are validated on startup
- JSON parsing is stream-based to prevent memory issues
- Fallback behavior ensures no requests fail due to routing errors
- Request bodies are preserved exactly for downstream processing

## Monitoring

Model routing includes structured logging for monitoring:

- Model routing attempts and results
- Fallback to default target
- Parsing errors and warnings
- Performance metrics overhead

Example log output:
```
[model-routing] 2025/01/13 10:30:15 Routed request to: https://api.anthropic.com/v1
[model-routing] 2025/01/13 10:30:16 Model routing failed, using default: unexpected EOF
```

## Performance Considerations

### Memory Efficiency
- Uses `io.TeeReader` to stream JSON parsing without loading entire request bodies into memory
- Minimal overhead for non-JSON requests

### Latency
- Sub-millisecond overhead for JSON parsing
- Fast lookup using Go map for model-to-URL mappings
- Early exit for non-JSON requests

### Scalability
- Processes each request independently
- No shared state between requests
- Compatible with existing rate limiting infrastructure

## Limitations

- Only works with JSON request bodies
- Model field must be at the top level of JSON object
- Target URLs must be complete API endpoints (include `/v1` path if needed)
- Does not modify request headers other than Host and URL

## Troubleshooting

### Model Not Routing
1. Check that `model_routing.enabled: true` in config
2. Verify request has `Content-Type: application/json`
3. Ensure model field exists in JSON request body
4. Check model is in the `models` mapping or `default_target` is set

### Performance Issues
1. Monitor request size - very large JSON payloads may impact performance
2. Check logs for parsing errors
3. Ensure non-JSON requests are not being processed unnecessarily

### Configuration Errors
1. Validate YAML syntax
2. Ensure target URLs are valid and accessible
3. Check that default_target is a complete URL
4. Verify model names exactly match request payloads