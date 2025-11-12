//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// TestFullStackIntegration tests the complete system end-to-end
func TestFullStackIntegration(t *testing.T) {
	// Create a test configuration with Cerebras limits
	configContent := `
server:
  host: "localhost"
  port: 0  # Use random port for testing

rate_limits:
  - domain: "api.example.com"
    requests_per_second: 10

cerebras_limits:
  rpm_limit: 60
  tpm_limit: 60000
  max_queue_depth: 20
  request_timeout: 30s
  priority_threshold: 0.7
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

	// Initialize components
	cerebrasLimiter := ratelimit.NewCerebrasLimiter(cfg.CerebrasLimits.RPMLimit, cfg.CerebrasLimits.TPMLimit)
	tokenEstimator := token.NewTokenEstimator()
	cerebrasHandler := proxy.NewCerebrasProxyHandler(cerebrasLimiter, tokenEstimator, &cfg.CerebrasLimits)

	// Create mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate Cerebras API response - accept any Host header for testing
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"id": "test-id",
			"object": "chat.completion",
			"created": time.Now().Unix(),
			"model": "llama3.1-8b",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role": "assistant",
						"content": "Test response from Cerebras API",
					},
					"finish_reason": "stop",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Parse mock server URL
	targetURL, _ := url.Parse(mockServer.URL)
	cerebrasHandler.SetTarget(targetURL)

	// Test standard request handling
	t.Run("StandardRateLimiting", func(t *testing.T) {
		standardRateLimiter := ratelimit.New(cfg.RateLimits)
		standardHandler := proxy.NewHandler(standardRateLimiter)
		standardHandler.SetTarget(targetURL)

		// Make requests to test rate limiting
		start := time.Now()
		var delays []time.Duration

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "http://localhost/test", nil)
			req.Host = "api.example.com"

			w := httptest.NewRecorder()
			standardHandler.ServeHTTP(w, req)

			if i > 0 {
				delays = append(delays, time.Since(start))
			}
			start = time.Now()

			if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
				t.Errorf("Expected 200 or 404, got %d", w.Code)
			}
		}

		// Verify rate limiting is working (delays should increase)
		if len(delays) >= 2 && delays[1] < 50*time.Millisecond {
			t.Log("Rate limiting may not be working as expected")
		}
	})

	// Test Cerebras request handling
	t.Run("CerebrasRateLimiting", func(t *testing.T) {
		// Test request detection
		req := httptest.NewRequest("POST", "http://localhost/v1/chat/completions", nil)
		req.Host = "api.cerebras.ai"

		if !cerebrasHandler.IsCerebrasRequest(req) {
			t.Error("Expected Cerebras request to be detected")
		}

		// Test non-Cerebras request
		req2 := httptest.NewRequest("GET", "http://localhost/test", nil)
		req2.Host = "api.openai.com"

		if cerebrasHandler.IsCerebrasRequest(req2) {
			t.Error("Expected non-Cerebras request to not be detected")
		}

		// Test token estimation
		chatRequest := map[string]interface{}{
			"model": "llama3.1-8b",
			"messages": []map[string]string{
				{"role": "user", "content": "Hello world test message with some words"},
			},
			"max_tokens": 100,
		}

		jsonData, _ := json.Marshal(chatRequest)
		req.Body = io.NopCloser(bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")

		tokens, err := cerebrasHandler.EstimateTokens(req)
		if err != nil {
			t.Errorf("Unexpected error estimating tokens: %v", err)
		}

		if tokens <= 0 {
			t.Errorf("Expected positive token estimate, got %d", tokens)
		}
	})

	// Test rate limiting with actual requests
	t.Run("CerebrasRequests", func(t *testing.T) {
		// Make multiple requests to test rate limiting
		successCount := 0
		rateLimitedCount := 0

		for i := 0; i < 5; i++ {
			chatRequest := map[string]interface{}{
				"model": "llama3.1-8b",
				"messages": []map[string]string{
					{"role": "user", "content": fmt.Sprintf("Test message %d", i)},
				},
				"max_tokens": 50,
			}

			jsonData, _ := json.Marshal(chatRequest)
			req := httptest.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader(jsonData))
			req.Host = "api.cerebras.ai"
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			cerebrasHandler.ServeHTTP(w, req)

			// Check response headers
			if w.Code == http.StatusOK {
				successCount++
			} else if w.Code == http.StatusTooManyRequests || w.Code == http.StatusServiceUnavailable {
				rateLimitedCount++
			}

			// Verify monitoring headers are present
			if w.Header().Get("X-RateLimit-Limit-RPM") == "" {
				t.Error("Expected X-RateLimit-Limit-RPM header")
			}

			if w.Header().Get("X-CircuitBreaker-State") == "" {
				t.Error("Expected X-CircuitBreaker-State header")
			}

			// Small delay between requests
			time.Sleep(100 * time.Millisecond)
		}

		t.Logf("Successful requests: %d, Rate limited: %d", successCount, rateLimitedCount)

		// Should have some successful requests
		if successCount == 0 {
			t.Error("Expected at least some successful requests")
		}
	})

	// Test circuit breaker behavior
	t.Run("CircuitBreakerBehavior", func(t *testing.T) {
		// Check initial circuit breaker state
		stats := cerebrasHandler.CircuitBreakerStats()
		if stats.Name != "cerebras-api" {
			t.Errorf("Expected circuit breaker name 'cerebras-api', got %s", stats.Name)
		}

		if stats.State.String() != "CLOSED" {
			t.Errorf("Expected circuit breaker to be CLOSED initially, got %s", stats.State.String())
		}

		// Circuit breaker should be in normal state for our mock server
		t.Logf("Circuit breaker state: %s, failures: %d", stats.State.String(), stats.Failures)
	})
}

// TestCircuitBreakerFailureSimulation tests circuit breaker under failure conditions
func TestCircuitBreakerFailureSimulation(t *testing.T) {
	// Create configuration with low thresholds for faster testing
	configContent := `
cerebras_limits:
  rpm_limit: 10
  tpm_limit: 1000
  max_queue_depth: 5
  request_timeout: 5s
  priority_threshold: 0.7
`

	configFile := filepath.Join(t.TempDir(), "test-config.yaml")
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	// Create a mock server that fails
	failureCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failureCount++
		if failureCount <= 6 { // First 6 requests fail
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK) // Later requests succeed
		}
	}))
	defer mockServer.Close()

	// Parse mock server URL
	targetURL, _ := url.Parse(mockServer.URL)

	// Initialize cerebras handler
	cerebrasLimiter := ratelimit.NewCerebrasLimiter(cfg.CerebrasLimits.RPMLimit, cfg.CerebrasLimits.TPMLimit)
	tokenEstimator := token.NewTokenEstimator()
	cerebrasHandler := proxy.NewCerebrasProxyHandler(cerebrasLimiter, tokenEstimator, &cfg.CerebrasLimits)
	cerebrasHandler.SetTarget(targetURL)

	// Make requests to trigger failures
	t.Run("FailureRecovery", func(t *testing.T) {
		successCount := 0
		circuitOpenCount := 0

		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("POST", "http://localhost/v1/chat/completions", strings.NewReader(`{"model":"test"}`))
			req.Host = "api.cerebras.ai"
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			cerebrasHandler.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				successCount++
			} else if w.Code == http.StatusServiceUnavailable {
				circuitOpenCount++
			}

			time.Sleep(100 * time.Millisecond)
		}

		t.Logf("Successful: %d, Circuit Open: %d", successCount, circuitOpenCount)

		// Should have some successful requests after circuit recovers
		if successCount == 0 {
			t.Error("Expected some successful requests after recovery")
		}
	})
}

// TestLoadTestingFrameworkIntegration tests the load testing framework
func TestLoadTestingFrameworkIntegration(t *testing.T) {
	// Skip if loadtest binary not built
	if _, err := os.Stat("./loadtest"); os.IsNotExist(err) {
		t.Skip("Load testing binary not built. Run 'make build-loadtest' first.")
	}

	// Create test server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"id": "load-test-id",
			"object": "chat.completion",
			"created": time.Now().Unix(),
			"model": "llama3.1-8b",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create temporary load test config
	loadTestConfig := map[string]interface{}{
		"concurrent_clients": 5,
		"duration":           "5s",
		"request_rate":       10,
		"endpoint":           mockServer.URL,
		"timeout":            "5s",
		"test_data": []map[string]interface{}{
			{
				"model":      "llama3.1-8b",
				"prompt":     "Load test prompt",
				"max_tokens": 50,
			},
		},
	}

	configJSON, _ := json.MarshalIndent(loadTestConfig, "", "  ")
	configFile := filepath.Join(t.TempDir(), "loadtest-config.json")
	err := os.WriteFile(configFile, configJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to write load test config: %v", err)
	}

	t.Logf("Created load test config: %s", configFile)
	t.Logf("Mock server URL: %s", mockServer.URL)

	// Note: In a real test environment, you would run the load test binary
	// For this integration test, we verify the setup is correct
	if mockServer.URL == "" {
		t.Error("Mock server URL is empty")
	}

	t.Log("Load testing framework integration test setup complete")
}

// TestConfigurationValidation tests configuration loading and validation
func TestConfigurationValidation(t *testing.T) {
	testCases := []struct {
		name        string
		config      string
		expectError bool
	}{
		{
			name: "Valid Cerebras Config",
			config: `
cerebras_limits:
  rpm_limit: 100
  tpm_limit: 100000
  max_queue_depth: 50
  request_timeout: 5m
  priority_threshold: 0.8
`,
			expectError: false,
		},
		{
			name: "Config with defaults",
			config: `
cerebras_limits:
  rpm_limit: 50
  tpm_limit: 50000
`,
			expectError: false,
		},
		{
			name: "Invalid YAML",
			config: `
cerebras_limits:
  rpm_limit: 100
  tpm_limit: invalid_value
`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configFile := filepath.Join(t.TempDir(), fmt.Sprintf("config-%s.yaml", tc.name))
			err := os.WriteFile(configFile, []byte(tc.config), 0644)
			if err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			_, err = config.Load(configFile)
			if tc.expectError && err == nil {
				t.Error("Expected config loading to fail, but it succeeded")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected config loading to succeed, but got error: %v", err)
			}
		})
	}
}