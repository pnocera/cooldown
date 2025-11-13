package provider

import (
	"testing"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestProviderManagerSelectsCerebrasForGLM(t *testing.T) {
	config := &config.Config{
		Providers: []config.ProviderConfig{
			{
				Name:     "cerebras",
				Endpoint: "https://api.cerebras.ai/v1",
				Models:   []string{"glm-4.6", "glm-4.5-air"},
				LoadBalancing: &config.LoadBalancingConfig{
					Strategy: "round_robin",
					APIKeys: []config.APIKeyConfig{
						{Key: "test-key-1", Weight: 1},
					},
				},
			},
		},
	}

	manager := NewProviderManager(config)
	provider, err := manager.GetProviderForModel("glm-4.6")

	assert.NoError(t, err)
	assert.Equal(t, "cerebras", provider.Name())
}
