# Claude Code Integration

This guide explains how to configure Claude Code to use the Cooldown Proxy for intelligent model routing and rate limiting.

## Quick Start

### 1. Configure Environment Variables

```bash
# Claude Code Configuration
export ANTHROPIC_BASE_URL="http://localhost:5730/anthropic"
export ANTHROPIC_AUTH_TOKEN="proxy-auth-token"
export ANTHROPIC_DEFAULT_HAIKU_MODEL="glm-4.5-air"
export ANTHROPIC_DEFAULT_SONNET_MODEL="glm-4.6"
export ANTHROPIC_DEFAULT_OPUS_MODEL="glm-4.6"

# Provider API Keys
export CEREBRAS_API_KEY_1="your-cerebras-key-1"
export CEREBRAS_API_KEY_2="your-cerebras-key-2"
export ZHIPU_API_KEY="your-zhipu-key"
```

### 2. Start the Proxy

```bash
# Copy example configuration
cp config.yaml.example-claude-code config.yaml

# Start the proxy
make build
./cooldown-proxy
```

### 3. Configure Claude Code

In your Claude Code settings or environment:

```bash
export ANTHROPIC_BASE_URL="http://localhost:5730/anthropic"
export ANTHROPIC_API_KEY="your-proxy-token"
```

## Architecture

### Dual Endpoints

- **`/anthropic`**: Claude Code SDK compatible endpoint with model routing
- **`/openai`**: Existing OpenAI API compatibility (preserved behavior)

### Model Routing

The proxy automatically maps Claude models to provider models:

| Claude Model | Default Provider Model | Environment Variable |
|-------------|----------------------|-------------------|
| claude-3-5-haiku-20241022 | glm-4.5-air | `ANTHROPIC_DEFAULT_HAIKU_MODEL` |
| claude-3-5-sonnet-20241022 | glm-4.6 | `ANTHROPIC_DEFAULT_SONNET_MODEL` |
| claude-3-opus-20240229 | glm-4.6 | `ANTHROPIC_DEFAULT_OPUS_MODEL` |

### Provider Support

#### Cerebras Integration

- **Models**: GLM-4.6, GLM-4.5-air
- **Load Balancing**: Round robin, least used, weighted random
- **Rate Limiting**: Dynamic based on API headers
- **Reasoning**: Automatic injection for GLM models

#### Zhipu Integration

- **Models**: GLM-4-flash, GLM-4-airx
- **Rate Limiting**: Fixed RPM limits

## Features

### Reasoning Injection

GLM models automatically receive reasoning prompts to activate thinking capabilities:

```
You are an expert reasoning model.
Always think step by step before answering.
Use interleaved thinking: plan → act → reflect
Format your reasoning in <reasoning_content> blocks.
```

### Dynamic Rate Limiting

- **Cerebras Headers**: Real-time quota monitoring
- **Safety Margins**: 20% buffer to prevent exhaustion
- **Adaptive Throttling**: Back off when quotas low
- **Per-Key Tracking**: Individual API key limits

### Multi-Tier Rate Limiting

1. **Global**: Proxy-wide limits
2. **Provider**: Per-provider rate limiting schemes  
3. **Per-Key**: Individual API key quota tracking
4. **Dynamic**: Real-time adaptation based on provider headers

## Configuration

### Basic Configuration

```yaml
server:
  anthropic_endpoint: "/anthropic"
  openai_endpoint: "/openai"
  port: 5730

environment_models:
  haiku: "glm-4.5-air"
  sonnet: "glm-4.6"
  opus: "glm-4.6"
```

### Provider Configuration

```yaml
providers:
  - name: "cerebras"
    endpoint: "https://api.cerebras.ai/v1"
    models: ["glm-4.6", "glm-4.5-air"]
    load_balancing:
      strategy: "least_used"
      api_keys:
        - key: "${CEREBRAS_API_KEY_1}"
          weight: 1
          max_requests_per_minute: 60
```

### Load Balancing Strategies

- **round_robin**: Cycle through API keys sequentially
- **least_used**: Use API key with fewest requests
- **weighted_random**: Random selection based on weights

## Monitoring

### Health Endpoints

- **`/health`**: Basic health check
- **`/metrics`**: Prometheus-compatible metrics
- **Logging**: Configurable log levels

### Rate Limit Monitoring

The proxy tracks:
- Requests per provider per key
- Token usage and quotas
- Response times
- Error rates

## Troubleshooting

### Common Issues

1. **"No provider found for model"**
   - Check model mapping in environment_models
   - Verify provider configuration

2. **"Approaching daily request limit"**
   - Check Cerebras quota headers
   - Adjust safety_margin in configuration

3. **Reasoning not working**
   - Verify reasoning_injection.enabled: true
   - Check model is in reasoning_injection.models list

### Debug Mode

Enable debug logging:

```yaml
monitoring:
  log_level: "debug"
```

### Testing Configuration

```bash
# Test configuration loading
./cooldown-proxy -config config.yaml.example-claude-code

# Test endpoints
curl -X POST http://localhost:5730/anthropic/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-3-5-sonnet-20241022","max_tokens":100,"messages":[{"role":"user","content":"Hello"}]}'
```