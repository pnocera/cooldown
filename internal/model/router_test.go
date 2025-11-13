package model

import (
	"testing"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestModelRouterMapsClaudeToProviderModels(t *testing.T) {
	config := &config.Config{
		EnvironmentModels: config.EnvironmentModels{
			Haiku:  "glm-4.5-air",
			Sonnet: "glm-4.6",
			Opus:   "glm-4.6",
		},
	}

	router := NewModelRouter(config)

	assert.Equal(t, "glm-4.5-air", router.MapModel("claude-3-5-haiku-20241022"))
	assert.Equal(t, "glm-4.6", router.MapModel("claude-3-5-sonnet-20241022"))
	assert.Equal(t, "glm-4.6", router.MapModel("claude-3-opus-20240229"))
}
