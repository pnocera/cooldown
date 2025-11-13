# Claude Code Intelligent Endpoint Design

## Overview

Design for integrating dual intelligent endpoints into the Cooldown Proxy: `/anthropic` for Claude Code with multi-model routing and API key load balancing, plus `/openai` for existing OpenAI compatibility. Features environment-based model mapping, multi-provider support, and provider-specific dynamic rate limiting.

## Architecture

### Dual Endpoint Structure
- **`/anthropic`**: Claude Code endpoint (port 5730) with model routing
- **`/openai`**: Existing OpenAI-compatible endpoint (preserved behavior)

### Anthropic Request Flow
```
Claude Code → /anthropic → Model Identification → Provider Selection →
API Key Load Balancing → Per-Key Rate Check → Prompt Injection →
Provider API → Parse Headers → Update Per-Key Limits → Response Transform → Claude Code
```

### Core Components

#### 1. Dual Endpoint Handler
- **`/anthropic`**: Claude Code SDK compatible, model-based routing
- **`/openai`**: Existing OpenAI API compatibility
- **Port**: 5730 (configurable)
- **Authentication**: Optional API key with localhost fallback

#### 2. Environment-Based Model Router
- **Function**: Maps Claude models to actual provider models
- **Configuration**: `ANTHROPIC_DEFAULT_HAIKU_MODEL`, `ANTHROPIC_DEFAULT_SONNET_MODEL`, `ANTHROPIC_DEFAULT_OPUS_MODEL`
- **Flexibility**: Runtime model reassignment without code changes

#### 3. Multi-Provider System
- **Cerebras**: GLM models with API key load balancing
- **Zhipu**: Alternative GLM provider with fixed rate limits
- **OpenRouter**: Additional model options with token-based limits
- **Extensible**: Plugin-style provider addition

#### 4. Cerebras API Key Load Balancer
- **Strategies**: Round Robin, Least Used, Weighted Random
- **Health Management**: Per-key tracking with automatic failover
- **Rate Limiting**: Individual quota tracking per API key
- **Circuit Breaker**: Temporary key disabling on failures

#### 5. Reasoning Injection System
- **Auto-Injection**: For GLM models requiring reasoning activation
- **Pattern**: `<reasoning_content>` blocks based on GLM-4.6 research
- **Model-Specific**: Configurable per-model reasoning requirements

#### 6. Multi-Tier Rate Limiting
- **Global**: Proxy-wide limits
- **Provider**: Per-provider rate limiting schemes
- **Per-Key**: Individual API key quota tracking
- **Dynamic**: Real-time adaptation based on provider headers

## Cerebras Integration

### Rate Limit Headers Monitoring
```go
type CerebrasRateLimit struct {
    LimitRequestsDay      int  // x-ratelimit-limit-requests-day
    LimitTokensMinute     int  // x-ratelimit-limit-tokens-minute
    RemainingRequestsDay  int  // x-ratelimit-remaining-requests-day
    RemainingTokensMinute int  // x-ratelimit-remaining-tokens-minute
    ResetRequestsDay      int  // x-ratelimit-reset-requests-day
    ResetTokensMinute     int  // x-ratelimit-reset-tokens-minute
}
```

### Dynamic Rate Adjustment Algorithm
- **Pre-Request Check**: Validate sufficient Cerebras quota
- **Token Budget Management**: Use 80% of remaining token budget
- **Request Conservation**: Back off when daily requests < 100
- **Reset Handling**: Automatic timer management for quota resets

### Multi-Layer Rate Limiting
1. **Base Configuration**: Existing per-domain YAML settings
2. **Dynamic Multiplier**: 0.5x to 2.0x based on Cerebras headers
3. **Safety Margins**: 20% buffer to prevent quota exhaustion
4. **Adaptive Throttling**: Real-time adjustment per request

## Reasoning System Design

### Interleaved Thinking Integration
Based on Minimax M2 research findings:

- **Plan → Act → Reflect loops**: Reasoning alternates with tool use
- **Reasoning Continuity**: Thinking content preserved between conversation steps
- **Response Structure**: `thinking` blocks integrated in `response.content`
- **State Preservation**: All content blocks maintained in conversation history

### Prompt Injection Strategy
```go
reasoningPrompt := `You are an expert reasoning model.
Always think step by step before answering.
Use interleaved thinking: plan → act → reflect
Format your reasoning in <reasoning_content> blocks.
Carry reasoning forward between tool calls.`
```

### Response Transformation
- **Thinking Blocks**: Convert Cerebras reasoning to Anthropic `thinking` format
- **Tool Calls**: Transform tool use blocks for Claude Code compatibility
- **Content Ordering**: Maintain proper sequence of reasoning → action → results
- **History Management**: Preserve complete conversation context

## Configuration

### Environment Variables (Claude Code Integration)
```bash
# Claude Code Configuration
export ANTHROPIC_BASE_URL="http://localhost:5730/anthropic"
export ANTHROPIC_AUTH_TOKEN="proxy-auth-token"
export ANTHROPIC_DEFAULT_HAIKU_MODEL="glm-4.5-air"
export ANTHROPIC_DEFAULT_SONNET_MODEL="glm-4.6"
export ANTHROPIC_DEFAULT_OPUS_MODEL="glm-4.6"

# Provider API Keys
export CEREBRAS_API_KEY_1="sk-cerebras-key-1"
export CEREBRAS_API_KEY_2="sk-cerebras-key-2"
export CEREBRAS_API_KEY_3="sk-cerebras-key-3"
export ZHIPU_API_KEY="your-zhipu-key"
export OPENROUTER_API_KEY="your-openrouter-key"
```

### Enhanced YAML Configuration
```yaml
server:
  anthropic_endpoint: "/anthropic"
  openai_endpoint: "/openai"
  port: 5730
  bind_address: "127.0.0.1"
  api_key_required: false

environment_models:
  haiku: "${ANTHROPIC_DEFAULT_HAIKU_MODEL:glm-4.5-air}"
  sonnet: "${ANTHROPIC_DEFAULT_SONNET_MODEL:glm-4.6}"
  opus: "${ANTHROPIC_DEFAULT_OPUS_MODEL:glm-4.6}"

providers:
  cerebras:
    endpoint: "https://api.cerebras.ai/v1/chat/completions"
    models: ["glm-4.6", "glm-4.5-air"]
    load_balancing:
      strategy: "least_used"  # round_robin, least_used, weighted_random
      api_keys:
        - key: "${CEREBRAS_API_KEY_1}"
          weight: 1
          max_requests_per_minute: 60
        - key: "${CEREBRAS_API_KEY_2}"
          weight: 2
          max_requests_per_minute: 120
        - key: "${CEREBRAS_API_KEY_3}"
          weight: 1
          max_requests_per_minute: 60
    rate_limiting:
      type: "per_key_cerebras_headers"
      safety_margin: 0.2
      backoff_threshold: 100

  zhipu:
    endpoint: "https://open.bigmodel.cn/api/paas/v4/chat/completions"
    models: ["glm-4-flash", "glm-4-airx"]
    api_key: "${ZHIPU_API_KEY}"
    rate_limiting:
      type: "fixed_rpm"
      requests_per_minute: 60

  openrouter:
    endpoint: "https://openrouter.ai/api/v1/chat/completions"
    models: ["deepseek/deepseek-chat", "anthropic/claude-3.5-sonnet"]
    api_key: "${OPENROUTER_API_KEY}"
    rate_limiting:
      type: "tokens_per_minute"
      tokens_per_minute: 100000

reasoning_injection:
  enabled: true
  models: ["glm-4.6", "glm-4.5-air"]
  prompt_template: "You are an expert reasoning model. Always think step by step..."

monitoring:
  metrics_enabled: true
  health_endpoint: "/health"
  prometheus_endpoint: "/metrics"
  log_level: "info"
```

## Implementation Phases

### Phase 1: Core Infrastructure
- Basic `/v1/chat/completions` endpoint
- Request/response transformers
- Cerebras API integration
- Configuration management

### Phase 2: Reasoning System
- Prompt injection mechanism
- Response transformation with thinking blocks
- Conversation state management
- Minimax-style interleaved thinking

### Phase 3: Dynamic Rate Limiting
- Cerebras header parsing
- Rate limit adaptation algorithm
- Multi-layer throttling
- Quota management and reset handling

### Phase 4: Monitoring & Optimization
- Request metrics and performance tracking
- Rate limit effectiveness monitoring
- Configuration validation
- Error handling and fallback mechanisms

## Security Considerations

### Authentication
- Optional API key protection for endpoint
- Localhost-only binding by default
- CORS restrictions for cross-origin requests

### Rate Limit Abuse Prevention
- Per-session rate limiting
- Request size validation
- Token usage monitoring
- Automatic backoff on quota exhaustion

### Data Protection
- No request/response logging by default
- Secure API key handling
- Path traversal prevention
- Input sanitization

## Testing Strategy

### Unit Tests
- Token counting accuracy
- Rate limit calculations
- Prompt injection logic
- Response transformation correctness

### Integration Tests
- End-to-end Claude Code compatibility
- Cerebras API integration
- Rate limit header handling
- Configuration validation

### Performance Tests
- Concurrent request handling
- Rate limit adaptation under load
- Memory usage with large contexts
- Response time benchmarks

## Success Criteria

### Functional Requirements
- ✅ Claude Code can successfully connect and make requests
- ✅ GLM-4.6 reasoning is consistently activated
- ✅ Rate limits adapt dynamically based on Cerebras quotas
- ✅ All tool calls and responses work correctly

### Performance Requirements
- ✅ < 100ms additional latency vs direct Cerebras calls
- ✅ 99.9% uptime for proxy endpoint
- ✅ Zero quota exhaustion incidents
- ✅ Sub-second response transformation times

### Reliability Requirements
- ✅ Graceful degradation on Cerebras outages
- ✅ Automatic recovery from rate limit errors
- ✅ Configuration hot-reloading
- ✅ Comprehensive error logging

## Future Extensions

### Multi-Model Support
- Additional Cerebras models
- Provider failover mechanisms
- Model-specific routing rules
- Cost optimization strategies

### Advanced Features
- Request caching for identical prompts
- Batch processing capabilities
- Custom reasoning prompt templates
- Plugin architecture for extensions

### Monitoring Dashboard
- Real-time rate limit visualization
- Request pattern analysis
- Performance metrics tracking
- Configuration management UI

---

*Design Date: 2025-01-13*
*Based on analysis of cerebras-code-mcp, claude-code-router, and GLM-4.6 reasoning requirements*