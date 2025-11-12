//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/proxy"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/cooldownp/cooldown-proxy/internal/token"
)

// TestCerebrasEndToEndIntegration tests the complete Cerebras rate limiting system end-to-end
func TestCerebrasEndToEndIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create mock Cerebras API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request forwarding
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		var chatReq map[string]interface{}
		json.Unmarshal(body, &chatReq)

		if chatReq["model"] != "llama3.1-70b" {
			t.Errorf("Expected model llama3.1-70b, got %v", chatReq["model"])
		}

		// Mock successful response
		response := map[string]interface{}{
			"id":      "chat-test123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "llama3.1-70b",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello! I'm a mock response.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 15,
				"total_tokens":      25,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit-RPM", "1000")
		w.Header().Set("X-RateLimit-Limit-TPM", "1000000")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Parse mock server URL
	targetURL, _ := url.Parse(mockServer.URL)

	// Create configuration
	configContent := `
server:
  host: "localhost"
  port: 0  # Use random port

cerebras_limits:
  rpm_limit: 10
  tpm_limit: 10000
  max_queue_depth: 5
  request_timeout: 1s

rate_limits:
  - domain: "api.cerebras.ai"
    requests_per_second: 5
`

	// Write config to temporary file
	configFile := filepath.Join(t.TempDir(), "test-config.yaml")
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	// Create cerebras proxy handler
	cerebrasLimiter := ratelimit.NewCerebrasLimiter(cfg.CerebrasLimits.RPMLimit, cfg.CerebrasLimits.TPMLimit)
	tokenEstimator := token.NewTokenEstimator()
	cerebrasHandler := proxy.NewCerebrasProxyHandler(cerebrasLimiter, tokenEstimator, &cfg.CerebrasLimits)
	cerebrasHandler.SetTarget(targetURL)

	// Router is not needed for this test - use handler directly

	// Test request
	requestBody := map[string]interface{}{
		"model": "llama3.1-70b",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello, world!"},
		},
		"max_tokens": 100,
		"stream":     false,
	}

	bodyBytes, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "api.cerebras.ai"
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()

	// Process request
	cerebrasHandler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify response body
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["model"] != "llama3.1-70b" {
		t.Errorf("Expected response model llama3.1-70b, got %v", response["model"])
	}

	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Error("Expected choices in response")
		return
	}

	choice := choices[0].(map[string]interface{})
	message := choice["message"].(map[string]interface{})
	if message["role"] != "assistant" {
		t.Errorf("Expected assistant role, got %s", message["role"])
	}
}

// TestCerebrasRateLimitEnforcement tests rate limiting behavior with immediate responses
func TestCerebrasRateLimitEnforcement(t *testing.T) {
	// Mock server that returns success immediately (no delays)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"id":      "rate-limit-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "llama3.1-70b",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Rate limit test response",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	targetURL, _ := url.Parse(mockServer.URL)

	// Create configuration with reasonable limits
	configContent := `
cerebras_limits:
  rpm_limit: 100
  tpm_limit: 10000
  max_queue_depth: 10
  request_timeout: 5s
`

	configFile := filepath.Join(t.TempDir(), "test-config-ratelimit.yaml")
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	cerebrasLimiter := ratelimit.NewCerebrasLimiter(cfg.CerebrasLimits.RPMLimit, cfg.CerebrasLimits.TPMLimit)
	tokenEstimator := token.NewTokenEstimator()
	cerebrasHandler := proxy.NewCerebrasProxyHandler(cerebrasLimiter, tokenEstimator, &cfg.CerebrasLimits)
	cerebrasHandler.SetTarget(targetURL)

	// Test a few requests to ensure rate limiting headers are present
	requestBody := map[string]interface{}{
		"model": "llama3.1-70b",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Simple test message"},
		},
	}

	bodyBytes, _ := json.Marshal(requestBody)
	successCount := 0

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Host = "api.cerebras.ai"

		w := httptest.NewRecorder()
		cerebrasHandler.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			successCount++
		}

		// Verify rate limit headers are present
		if w.Header().Get("X-RateLimit-Limit-RPM") == "" {
			t.Error("Expected X-RateLimit-Limit-RPM header")
		}

		if w.Header().Get("X-CircuitBreaker-State") == "" {
			t.Error("Expected X-CircuitBreaker-State header")
		}
	}

	if successCount == 0 {
		t.Error("Expected at least some requests to succeed")
	}

	t.Logf("Rate limit enforcement test: %d/3 requests successful", successCount)
}

// TestCerebrasTokenEstimationAccuracy tests token estimation functionality
func TestCerebrasTokenEstimationAccuracy(t *testing.T) {
	// Create mock server for successful requests
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"id":      "token-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "llama3.1-70b",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Test response for token estimation",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	targetURL, _ := url.Parse(mockServer.URL)

	// Create token estimator
	estimator := token.NewTokenEstimator()

	// Test cases with different request sizes
	testCases := []struct {
		name     string
		messages []map[string]interface{}
		expected int // Minimum expected tokens
	}{
		{
			name: "Small request",
			messages: []map[string]interface{}{
				{"role": "user", "content": "Hi"},
			},
			expected: 5,
		},
		{
			name: "Medium request",
			messages: []map[string]interface{}{
				{"role": "system", "content": "You are a helpful assistant."},
				{"role": "user", "content": "Can you help me with a task?"},
			},
			expected: 20,
		},
		{
			name: "Large request",
			messages: []map[string]interface{}{
				{"role": "system", "content": "You are a helpful assistant."},
				{"role": "user", "content": strings.Repeat("This is a longer message for testing. ", 10)},
			},
			expected: 50,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			requestData := map[string]interface{}{
				"model":       "llama3.1-70b",
				"messages":    tc.messages,
				"max_tokens": 100,
			}

			jsonData, _ := json.Marshal(requestData)
			req := httptest.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")

			// Create a minimal proxy handler for testing token estimation
			cfg := &config.CerebrasLimits{
				RPMLimit:       1000,
				TPMLimit:       1000000,
				MaxQueueDepth: 100,
				RequestTimeout: 10 * time.Second,
			}

			cerebrasLimiter := ratelimit.NewCerebrasLimiter(cfg.RPMLimit, cfg.TPMLimit)
			cerebrasHandler := proxy.NewCerebrasProxyHandler(cerebrasLimiter, estimator, cfg)
			cerebrasHandler.SetTarget(targetURL)

			// Estimate tokens
			tokens, err := cerebrasHandler.EstimateTokens(req)
			if err != nil {
				t.Fatalf("Unexpected error estimating tokens: %v", err)
			}

			if tokens < tc.expected {
				t.Errorf("Expected at least %d tokens, got %d", tc.expected, tokens)
			}

			t.Logf("Request '%s': estimated %d tokens", tc.name, tokens)
		})
	}
}

// TestResult is a helper type for testing
type TestResult struct {
	ID        int
	StatusCode int
	Duration  time.Duration
	Success   bool
}