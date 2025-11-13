package main

import (
	"bytes"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/modelrouting"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/cooldownp/cooldown-proxy/internal/router"
)

func TestModelRoutingIntegration(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		ModelRouting: &config.ModelRoutingConfig{
			Enabled:       true,
			DefaultTarget: "https://api.openai.com/v1",
			Models: map[string]string{
				"gpt-4":    "https://api.openai.com/v1",
				"claude-3": "https://api.anthropic.com/v1",
			},
		},
	}

	// Create rate limiter
	rateLimiter := ratelimit.New(cfg.RateLimits)

	// Create router with empty routes
	routes := make(map[string]*url.URL)
	r := router.New(routes, rateLimiter)

	// Create composite handler
	compositeHandler := &CompositeHandler{
		cerebrasHandler: nil,
		standardRouter:  r,
		modelRouting:    modelrouting.NewModelRoutingMiddleware(cfg.ModelRouting, r),
	}

	t.Run("routes GPT-4 requests to OpenAI", func(t *testing.T) {
		body := bytes.NewBufferString(`{"model": "gpt-4", "messages": []}`)
		req := httptest.NewRequest("POST", "http://localhost:8080/chat/completions", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		compositeHandler.ServeHTTP(w, req)

		// The request should be modified to route to OpenAI
		if req.Host != "api.openai.com" {
			t.Errorf("Expected host to be api.openai.com, got %s", req.Host)
		}
	})

	t.Run("routes Claude requests to Anthropic", func(t *testing.T) {
		body := bytes.NewBufferString(`{"model": "claude-3", "messages": []}`)
		req := httptest.NewRequest("POST", "http://localhost:8080/chat/completions", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		compositeHandler.ServeHTTP(w, req)

		if req.Host != "api.anthropic.com" {
			t.Errorf("Expected host to be api.anthropic.com, got %s", req.Host)
		}
	})
}

func TestCompositeHandlerWithModelRouting(t *testing.T) {
	// Test the complex handler setup from main.go
	cfg := &config.Config{
		ModelRouting: &config.ModelRoutingConfig{
			Enabled:       true,
			DefaultTarget: "https://api.openai.com/v1",
			Models: map[string]string{
				"gpt-4": "https://api.openai.com/v1",
			},
		},
	}

	rateLimiter := ratelimit.New([]config.RateLimitRule{})
	routes := make(map[string]*url.URL)
	r := router.New(routes, rateLimiter)

	// Initialize model routing
	var modelRoutingMiddleware *modelrouting.ModelRoutingMiddleware
	if cfg.ModelRouting != nil && cfg.ModelRouting.Enabled {
		modelRoutingMiddleware = modelrouting.NewModelRoutingMiddleware(cfg.ModelRouting, nil)
	}

	// Create composite handler
	compositeHandler := &CompositeHandler{
		cerebrasHandler: nil,
		standardRouter:  r,
		modelRouting:    modelRoutingMiddleware,
	}

	// If model routing is enabled, wrap it properly (same logic as main.go)
	if modelRoutingMiddleware != nil {
		baseHandler := &CompositeHandler{
			cerebrasHandler: nil,
			standardRouter:  r,
			modelRouting:    nil, // Don't recurse
		}

		modelRoutingMiddleware = modelrouting.NewModelRoutingMiddleware(cfg.ModelRouting, baseHandler)

		compositeHandler = &CompositeHandler{
			cerebrasHandler: nil,
			standardRouter:  nil,
			modelRouting:    modelRoutingMiddleware,
		}
	}

	t.Run("wrapped handler routes correctly", func(t *testing.T) {
		body := bytes.NewBufferString(`{"model": "gpt-4", "messages": []}`)
		req := httptest.NewRequest("POST", "http://localhost:8080/chat/completions", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		compositeHandler.ServeHTTP(w, req)

		// The request should be modified to route to OpenAI
		if req.Host != "api.openai.com" {
			t.Errorf("Expected host to be api.openai.com, got %s", req.Host)
		}
	})
}

func TestCompositeHandlerWithoutModelRouting(t *testing.T) {
	// Test that the handler works normally when model routing is disabled
	rateLimiter := ratelimit.New([]config.RateLimitRule{})
	routes := make(map[string]*url.URL)
	r := router.New(routes, rateLimiter)

	compositeHandler := &CompositeHandler{
		cerebrasHandler: nil,
		standardRouter:  r,
		modelRouting:    nil,
	}

	t.Run("passes through when model routing disabled", func(t *testing.T) {
		body := bytes.NewBufferString(`{"model": "gpt-4", "messages": []}`)
		req := httptest.NewRequest("POST", "http://localhost:8080/chat/completions", body)
		req.Header.Set("Content-Type", "application/json")

		originalHost := req.Host

		w := httptest.NewRecorder()
		compositeHandler.ServeHTTP(w, req)

		// The request should not be modified when model routing is disabled
		if req.Host != originalHost {
			t.Errorf("Expected host to remain %s, got %s", originalHost, req.Host)
		}
	})
}