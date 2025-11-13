package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 9090
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}

	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", config.Server.Port)
	}
}

func TestHeaderBasedRateLimitConfig(t *testing.T) {
	configYAML := `
cerebras_limits:
  rate_limits:
    use_headers: true
    header_fallback: true
    header_timeout: 5s
    reset_buffer: 100ms
  rpm_limit: 60
  tpm_limit: 1000
`

	var config Config
	err := yaml.Unmarshal([]byte(configYAML), &config)
	assert.NoError(t, err)

	assert.True(t, config.CerebrasLimits.RateLimits.UseHeaders)
	assert.True(t, config.CerebrasLimits.RateLimits.HeaderFallback)
	assert.Equal(t, 5*time.Second, config.CerebrasLimits.RateLimits.HeaderTimeout)
	assert.Equal(t, 100*time.Millisecond, config.CerebrasLimits.RateLimits.ResetBuffer)
	assert.Equal(t, 60, config.CerebrasLimits.RPMLimit)
	assert.Equal(t, 1000, config.CerebrasLimits.TPMLimit)
}

func TestDefaultHeaderBasedRateLimitConfig(t *testing.T) {
	configYAML := `
cerebras_limits:
  rpm_limit: 60
  tpm_limit: 1000
`

	var config Config
	err := yaml.Unmarshal([]byte(configYAML), &config)
	assert.NoError(t, err)

	// Set defaults to trigger default population
	config.CerebrasLimits.SetDefaults()

	// Should have sensible defaults
	assert.False(t, config.CerebrasLimits.RateLimits.UseHeaders)    // Disabled by default
	assert.True(t, config.CerebrasLimits.RateLimits.HeaderFallback) // Enabled by default (zero value = false, but we want true)
	assert.Equal(t, 5*time.Second, config.CerebrasLimits.RateLimits.HeaderTimeout)
	assert.Equal(t, 100*time.Millisecond, config.CerebrasLimits.RateLimits.ResetBuffer)
}

func TestAnthropicEndpointConfiguration(t *testing.T) {
	config := Config{
		Server: ServerConfig{
			AnthropicEndpoint: "/anthropic",
			OpenAIEndpoint:    "/openai",
			Port:             5730,
			BindAddress:      "127.0.0.1",
			APIKeyRequired:   false,
		},
		EnvironmentModels: EnvironmentModels{
			Haiku:   "glm-4.5-air",
			Sonnet:  "glm-4.6",
			Opus:    "glm-4.6",
		},
	}

	assert.Equal(t, "/anthropic", config.Server.AnthropicEndpoint)
	assert.Equal(t, "glm-4.6", config.EnvironmentModels.Sonnet)
}
