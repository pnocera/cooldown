package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/circuitbreaker"
	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/cooldownp/cooldown-proxy/internal/token"
	"github.com/stretchr/testify/assert"
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

func TestCerebrasProxyHeaderIntegration(t *testing.T) {
	// Create a mock HTTP server that returns rate limit headers
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-limit-tokens-minute", "1000")
		w.Header().Set("x-ratelimit-remaining-tokens-minute", "800")
		w.Header().Set("x-ratelimit-reset-tokens-minute", "45.5")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"response": "ok"}`))
	}))
	defer mockServer.Close()

	// Parse mock server URL
	mockURL, _ := url.Parse(mockServer.URL)

	// Create proxy handler with mock backend
	cerebrasConfig := &config.CerebrasLimits{
		RPMLimit:       60,
		TPMLimit:       1000,
		MaxQueueDepth:  100,
		RequestTimeout: 10 * time.Minute,
	}

	limiter := ratelimit.NewCerebrasLimiter(cerebrasConfig.RPMLimit, cerebrasConfig.TPMLimit)
	estimator := token.NewTokenEstimator()
	handler := NewCerebrasProxyHandler(limiter, estimator, cerebrasConfig)

	// Set the target to our mock server
	handler.SetTarget(mockURL)

	// Create test request
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model": "claude-3"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "api.cerebras.ai"

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Check that response headers include rate limit info
	assert.Equal(t, "60", resp.Header.Get("X-RateLimit-Limit-RPM"))
	assert.Equal(t, "1000", resp.Header.Get("X-RateLimit-Limit-TPM"))
	assert.Contains(t, resp.Header.Get("X-RateLimit-Remaining-TPM"), "800")

	// Verify limiter was updated
	assert.Equal(t, 1000, handler.Limiter.CurrentTPMLimit())
	assert.Equal(t, 800, handler.Limiter.CurrentTPMRemaining())
}

func TestCerebrasProxyHeaderParsingErrors(t *testing.T) {
	// Create a mock server with invalid headers
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-limit-tokens-minute", "invalid")
		w.Header().Set("x-ratelimit-remaining-tokens-minute", "800")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"response": "ok"}`))
	}))
	defer mockServer.Close()

	// Parse mock server URL
	mockURL, _ := url.Parse(mockServer.URL)

	cerebrasConfig := &config.CerebrasLimits{
		RPMLimit:       60,
		TPMLimit:       1000,
		MaxQueueDepth:  100,
		RequestTimeout: 10 * time.Minute,
	}

	limiter := ratelimit.NewCerebrasLimiter(cerebrasConfig.RPMLimit, cerebrasConfig.TPMLimit)
	estimator := token.NewTokenEstimator()
	handler := NewCerebrasProxyHandler(limiter, estimator, cerebrasConfig)

	// Set the target to our mock server
	handler.SetTarget(mockURL)

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model": "claude-3"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "api.cerebras.ai"

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Should still add static headers even if dynamic parsing failed
	assert.Equal(t, "60", resp.Header.Get("X-RateLimit-Limit-RPM"))
	assert.Equal(t, "1000", resp.Header.Get("X-RateLimit-Limit-TPM"))
	// Current TPM limit should be 0 since header parsing failed
	assert.Equal(t, "0", resp.Header.Get("X-RateLimit-Current-TPM-Limit"))
}
