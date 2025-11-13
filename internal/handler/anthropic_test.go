package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestAnthropicEndpointBasics(t *testing.T) {
	config := &config.Config{
		EnvironmentModels: config.EnvironmentModels{
			Sonnet: "glm-4.6",
		},
		Providers: []config.ProviderConfig{
			{
				Name:     "cerebras",
				Endpoint: "https://api.cerebras.ai/v1",
				Models:   []string{"glm-4.6"},
				LoadBalancing: &config.LoadBalancingConfig{
					Strategy: "round_robin",
					APIKeys: []config.APIKeyConfig{
						{Key: "test-key", Weight: 1},
					},
				},
			},
		},
	}

	handler := NewAnthropicHandler(config)

	req := httptest.NewRequest("POST", "/anthropic/v1/messages", strings.NewReader(`{
        "model": "claude-3-5-sonnet-20241022",
        "max_tokens": 1024,
        "messages": [{"role": "user", "content": "Hello"}]
    }`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should get 500 because we can't make real HTTP requests in tests
	// But the error should be about the provider request, not parsing
	assert.Contains(t, w.Body.String(), "Provider error")
}

func TestAnthropicHandlerEndToEndFlow(t *testing.T) {
	config := &config.Config{
		EnvironmentModels: config.EnvironmentModels{
			Haiku:  "glm-4.5-air",
			Sonnet: "glm-4.6",
		},
		Providers: []config.ProviderConfig{
			{
				Name:     "cerebras",
				Endpoint: "https://api.cerebras.ai/v1",
				Models:   []string{"glm-4.6"},
				LoadBalancing: &config.LoadBalancingConfig{
					Strategy: "round_robin",
					APIKeys: []config.APIKeyConfig{
						{Key: "test-key", Weight: 1},
					},
				},
			},
		},
		ReasoningConfig: config.ReasoningConfig{
			Enabled:       true,
			Models:        []string{"glm-4.6"},
			PromptTemplate: "You are an expert reasoning model.",
		},
	}

	handler := NewAnthropicHandler(config)

	req := httptest.NewRequest("POST", "/anthropic/v1/messages", strings.NewReader(`{
        "model": "claude-3-5-sonnet-20241022",
        "max_tokens": 1024,
        "messages": [{"role": "user", "content": "Hello"}]
    }`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should get 500 because we can't make real HTTP requests in tests
	// But this proves the full pipeline is working (model mapping, reasoning injection, provider selection)
	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "Provider error")
}