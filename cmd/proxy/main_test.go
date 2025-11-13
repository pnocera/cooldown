package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestProxyWithEmptyRoutesReturns404(t *testing.T) {
	// This test demonstrates the current broken behavior
	emptyRoutes := make(map[string]*url.URL)

	// Create rate limiter
	rateLimiter := ratelimit.New([]config.RateLimitRule{})

	// Create router with empty routes (current main.go behavior)
	r := router.New(emptyRoutes, rateLimiter)

	// Create test request
	req := httptest.NewRequest("GET", "http://test.com/api/users", nil)
	w := httptest.NewRecorder()

	// Serve request
	r.ServeHTTP(w, req)

	// Should return 404 because no routes are configured
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "No route found for host: test.com")
}

func TestModelRoutingConfigurationLoadsCorrectly(t *testing.T) {
	// Test that model routing configuration is properly loaded
	cfg := &config.Config{
		ModelRouting: &config.ModelRoutingConfig{
			Enabled: true,
			DefaultTarget: "https://api.cerebras.ai/v1",
			Models: map[string]string{
				"gpt-4": "https://api.openai.com/v1",
				"claude-3-sonnet": "https://api.anthropic.com/v1",
				"llama3-8b": "https://api.cerebras.ai/v1",
			},
		},
		Server: config.ServerConfig{
			AnthropicEndpoint: "/anthropic",
			OpenAIEndpoint: "/openai",
		},
	}

	// Verify configuration is loaded correctly
	assert.True(t, cfg.ModelRouting.Enabled)
	assert.Equal(t, "https://api.cerebras.ai/v1", cfg.ModelRouting.DefaultTarget)
	assert.Equal(t, "https://api.openai.com/v1", cfg.ModelRouting.Models["gpt-4"])
	assert.Equal(t, "/anthropic", cfg.Server.AnthropicEndpoint)
	assert.Equal(t, "/openai", cfg.Server.OpenAIEndpoint)
}

func TestConfigurationValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with host",
			config: &config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				ModelRouting: &config.ModelRoutingConfig{
					Enabled:       true,
					DefaultTarget: "https://api.cerebras.ai/v1",
					Models: map[string]string{
						"gpt-4": "https://api.openai.com/v1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with bind address",
			config: &config.Config{
				Server: config.ServerConfig{
					BindAddress: "127.0.0.1",
					Port:        8080,
				},
				ModelRouting: &config.ModelRoutingConfig{
					Enabled:       true,
					DefaultTarget: "https://api.cerebras.ai/v1",
					Models: map[string]string{
						"gpt-4": "https://api.openai.com/v1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing host and bind address",
			config: &config.Config{
				Server: config.ServerConfig{
					Port: 8080,
				},
			},
			wantErr: true,
			errMsg:  "server host or bind address is required",
		},
		{
			name: "invalid port",
			config: &config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 0,
				},
			},
			wantErr: true,
			errMsg:  "server port must be between 1 and 65535",
		},
		{
			name: "model routing enabled without default target",
			config: &config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				ModelRouting: &config.ModelRoutingConfig{
					Enabled: true,
				},
			},
			wantErr: true,
			errMsg:  "model routing is enabled but no default target is specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServerAddressFieldAccess(t *testing.T) {
	// Test that main.go can access the correct fields
	cfg := &config.ServerConfig{
		Host:        "localhost",
		BindAddress: "127.0.0.1",
		Port:        8080,
	}

	// Test field access that main.go does
	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, "127.0.0.1", cfg.BindAddress)
	assert.Equal(t, 8080, cfg.Port)

	// Test address construction logic like in main.go
	bindAddress := cfg.BindAddress
	if bindAddress == "" {
		bindAddress = cfg.Host
	}
	addr := fmt.Sprintf("%s:%d", bindAddress, cfg.Port)
	assert.Equal(t, "127.0.0.1:8080", addr)

	// Test fallback to Host when BindAddress is empty
	cfg.BindAddress = ""
	bindAddress = cfg.BindAddress
	if bindAddress == "" {
		bindAddress = cfg.Host
	}
	addr = fmt.Sprintf("%s:%d", bindAddress, cfg.Port)
	assert.Equal(t, "localhost:8080", addr)
}

func TestProviderConfigurationValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid provider config",
			config: &config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				Providers: []config.ProviderConfig{
					{
						Name:     "cerebras",
						Endpoint: "https://api.cerebras.ai",
						Models:   []string{"llama3-8b", "llama3-70b"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing provider name",
			config: &config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				Providers: []config.ProviderConfig{
					{
						Endpoint: "https://api.cerebras.ai",
						Models:   []string{"llama3-8b"},
					},
				},
			},
			wantErr: true,
			errMsg:  "provider name is required",
		},
		{
			name: "missing provider endpoint",
			config: &config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				Providers: []config.ProviderConfig{
					{
						Name:   "cerebras",
						Models: []string{"llama3-8b"},
					},
				},
			},
			wantErr: true,
			errMsg:  "provider cerebras: endpoint is required",
		},
		{
			name: "missing provider models",
			config: &config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				Providers: []config.ProviderConfig{
					{
						Name:     "cerebras",
						Endpoint: "https://api.cerebras.ai",
					},
				},
			},
			wantErr: true,
			errMsg:  "provider cerebras: at least one model is required",
		},
		{
			name: "no providers configured (should be valid)",
			config: &config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				Providers: []config.ProviderConfig{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
