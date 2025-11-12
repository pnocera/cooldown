package proxy

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/circuitbreaker"
	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/cooldownp/cooldown-proxy/internal/token"
)

func TestCerebrasProxyHandler_Creation(t *testing.T) {
	cerebrasConfig := &config.CerebrasLimits{
		RPMLimit:       1000,
		TPMLimit:       1000000,
		MaxQueueDepth:  100,
		RequestTimeout: 10 * time.Minute,
	}

	limiter := ratelimit.NewCerebrasLimiter(cerebrasConfig.RPMLimit, cerebrasConfig.TPMLimit)
	estimator := token.NewTokenEstimator()

	handler := NewCerebrasProxyHandler(limiter, estimator, cerebrasConfig)

	if handler == nil {
		t.Fatal("Expected non-nil handler")
	}

	if handler.Limiter == nil {
		t.Error("Expected non-nil rate limiter")
	}

	if handler.Estimator == nil {
		t.Error("Expected non-nil token estimator")
	}
}

func TestCerebrasProxyHandler_IsCerebrasRequest(t *testing.T) {
	cerebrasConfig := &config.CerebrasLimits{
		RPMLimit:       1000,
		TPMLimit:       1000000,
		MaxQueueDepth:  100,
		RequestTimeout: 10 * time.Minute,
	}

	limiter := ratelimit.NewCerebrasLimiter(cerebrasConfig.RPMLimit, cerebrasConfig.TPMLimit)
	estimator := token.NewTokenEstimator()
	handler := NewCerebrasProxyHandler(limiter, estimator, cerebrasConfig)

	// Test with Cerebras request
	req := httptest.NewRequest("POST", "https://api.cerebras.ai/v1/chat/completions", nil)
	req.Host = "api.cerebras.ai"

	if !handler.IsCerebrasRequest(req) {
		t.Error("Expected Cerebras request to be detected")
	}

	// Test with non-Cerebras request
	req2 := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	req2.Host = "api.openai.com"

	if handler.IsCerebrasRequest(req2) {
		t.Error("Expected non-Cerebras request to NOT be detected")
	}
}

func TestCerebrasProxyHandler_EstimateTokens(t *testing.T) {
	cerebrasConfig := &config.CerebrasLimits{
		RPMLimit:       1000,
		TPMLimit:       1000000,
		MaxQueueDepth:  100,
		RequestTimeout: 10 * time.Minute,
	}

	limiter := ratelimit.NewCerebrasLimiter(cerebrasConfig.RPMLimit, cerebrasConfig.TPMLimit)
	estimator := token.NewTokenEstimator()
	handler := NewCerebrasProxyHandler(limiter, estimator, cerebrasConfig)

	// Test with valid chat completion request
	chatReq := map[string]interface{}{
		"model": "llama3.1-8b",
		"messages": []map[string]string{
			{"role": "user", "content": "This is a test message with about ten words or so"},
		},
		"max_tokens": 100,
	}

	body, _ := json.Marshal(chatReq)
	req := httptest.NewRequest("POST", "https://api.cerebras.ai/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	tokens, err := handler.EstimateTokens(req)

	if err != nil {
		t.Errorf("Unexpected error estimating tokens: %v", err)
	}

	// Should estimate input tokens (words/0.75) + output tokens
	if tokens <= 0 {
		t.Errorf("Expected positive token estimate, got %d", tokens)
	}

	// Test with invalid JSON
	req2 := httptest.NewRequest("POST", "https://api.cerebras.ai/v1/chat/completions", bytes.NewReader([]byte("invalid json")))
	req2.Header.Set("Content-Type", "application/json")

	_, err = handler.EstimateTokens(req2)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestCerebrasProxyHandler_CheckRateLimit(t *testing.T) {
	cerebrasConfig := &config.CerebrasLimits{
		RPMLimit:       1, // Very low limit for testing
		TPMLimit:       1000,
		MaxQueueDepth:  100,
		RequestTimeout: 10 * time.Minute,
	}

	limiter := ratelimit.NewCerebrasLimiter(cerebrasConfig.RPMLimit, cerebrasConfig.TPMLimit)
	estimator := token.NewTokenEstimator()
	handler := NewCerebrasProxyHandler(limiter, estimator, cerebrasConfig)

	req := httptest.NewRequest("POST", "https://api.cerebras.ai/v1/chat/completions", nil)
	req.Host = "api.cerebras.ai"

	// First request should be allowed
	delay := handler.CheckRateLimit(req, 100)
	if delay != 0 {
		t.Errorf("First request should be allowed, got delay %v", delay)
	}

	// Second request should be rate limited
	delay = handler.CheckRateLimit(req, 100)
	if delay == 0 {
		t.Error("Second request should be rate limited")
	}
}

func TestCerebrasProxyHandler_CircuitBreakerIntegration(t *testing.T) {
	cerebrasConfig := &config.CerebrasLimits{
		RPMLimit:       1000,
		TPMLimit:       1000000,
		MaxQueueDepth:  100,
		RequestTimeout: 10 * time.Minute,
	}

	limiter := ratelimit.NewCerebrasLimiter(cerebrasConfig.RPMLimit, cerebrasConfig.TPMLimit)
	estimator := token.NewTokenEstimator()
	handler := NewCerebrasProxyHandler(limiter, estimator, cerebrasConfig)

	// Test that circuit breaker is initialized
	if handler.circuitBreaker == nil {
		t.Error("Expected circuit breaker to be initialized")
	}

	// Test initial state
	if handler.circuitBreaker.State() != circuitbreaker.StateClosed {
		t.Errorf("Expected circuit breaker to be in CLOSED state, got %s", handler.circuitBreaker.State().String())
	}

	// Test circuit breaker stats
	stats := handler.circuitBreaker.Stats()
	if stats.Name != "cerebras-api" {
		t.Errorf("Expected circuit breaker name 'cerebras-api', got %s", stats.Name)
	}
}
