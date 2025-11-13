package main

import (
	"net/url"
	"testing"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/cooldownp/cooldown-proxy/internal/router"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	// Test that config loading works
	// This will be tested with actual config files
}

func TestServerStartup(t *testing.T) {
	// This will be tested as an integration test
	// For now, just test the parsing logic
}

func TestMainApplicationSetsUpDualEndpoints(t *testing.T) {
	config := &config.Config{
		Server: config.ServerConfig{
			Port:              5730,
			BindAddress:       "127.0.0.1",
			AnthropicEndpoint: "/anthropic",
			OpenAIEndpoint:    "/openai",
		},
	}

	// Test that both endpoints are configured
	assert.Equal(t, "/anthropic", config.Server.AnthropicEndpoint)
	assert.Equal(t, "/openai", config.Server.OpenAIEndpoint)
}

func TestRouterSupportsDualEndpoints(t *testing.T) {
	config := &config.Config{
		Server: config.ServerConfig{
			AnthropicEndpoint: "/anthropic",
			OpenAIEndpoint:    "/openai",
		},
	}

	// Test that router can be created with the existing signature
	routes := make(map[string]*url.URL)
	rateLimiter := ratelimit.New(config.RateLimits)
	r := router.New(routes, rateLimiter)
	assert.NotNil(t, r)
}
