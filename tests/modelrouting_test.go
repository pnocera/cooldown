package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/modelrouting"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/cooldownp/cooldown-proxy/internal/router"
)

func TestModelRoutingEndToEnd(t *testing.T) {
	// Test different model routings
	testCases := []struct {
		name           string
		requestBody    string
		expectedHost   string
		expectedScheme string
	}{
		{
			name:           "OpenAI GPT-4",
			requestBody:    `{"model": "gpt-4", "messages": []}`,
			expectedHost:   "api.openai.com",
			expectedScheme: "https",
		},
		{
			name:           "Anthropic Claude",
			requestBody:    `{"model": "claude-3-opus", "messages": []}`,
			expectedHost:   "api.anthropic.com",
			expectedScheme: "https",
		},
		{
			name:           "Unknown model falls back to default",
			requestBody:    `{"model": "unknown-model", "messages": []}`,
			expectedHost:   "api.openai.com",
			expectedScheme: "https",
		},
		{
			name:           "No model field falls back to default",
			requestBody:    `{"messages": []}`,
			expectedHost:   "api.openai.com",
			expectedScheme: "https",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.ModelRoutingConfig{
				Enabled:       true,
				DefaultTarget: "https://api.openai.com/v1",
				Models: map[string]string{
					"gpt-4":           "https://api.openai.com/v1",
					"claude-3-opus":   "https://api.anthropic.com/v1",
					"claude-3-sonnet": "https://api.anthropic.com/v1",
				},
			}

			// Create proxy handler
			rateLimiter := ratelimit.New([]config.RateLimitRule{})
			routes := make(map[string]*url.URL)
			router := router.New(routes, rateLimiter)

			middleware := modelrouting.NewModelRoutingMiddleware(cfg, router)

			// Create request
			req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Execute request
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			// Verify routing
			if req.Host != tc.expectedHost {
				t.Errorf("Expected host %s, got %s", tc.expectedHost, req.Host)
			}

			if req.URL.Scheme != tc.expectedScheme {
				t.Errorf("Expected scheme %s, got %s", tc.expectedScheme, req.URL.Scheme)
			}
		})
	}
}

func TestModelRoutingPerformance(t *testing.T) {
	cfg := &config.ModelRoutingConfig{
		Enabled:       true,
		DefaultTarget: "https://api.openai.com/v1",
		Models: map[string]string{
			"gpt-4": "https://api.openai.com/v1",
		},
	}

	middleware := modelrouting.NewModelRoutingMiddleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test performance with various payload sizes
	payloadSizes := []int{100, 1000, 10000, 100000}

	for _, size := range payloadSizes {
		t.Run(fmt.Sprintf("payload_size_%d", size), func(t *testing.T) {
			// Create large JSON payload
			json := fmt.Sprintf(`{"model": "gpt-4", "messages": [{"content": "%s"}]}`, strings.Repeat("x", size))

			req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(json))
			req.Header.Set("Content-Type", "application/json")

			start := time.Now()
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)
			duration := time.Since(start)

			// Should complete quickly even with large payloads
			if duration > 100*time.Millisecond {
				t.Errorf("Request took too long: %v for payload size %d", duration, size)
			}

			t.Logf("Payload size %d: %v", size, duration)
		})
	}
}

func TestModelRoutingConcurrent(t *testing.T) {
	cfg := &config.ModelRoutingConfig{
		Enabled:       true,
		DefaultTarget: "https://api.openai.com/v1",
		Models: map[string]string{
			"gpt-4":    "https://api.openai.com/v1",
			"claude-3": "https://api.anthropic.com/v1",
		},
	}

	middleware := modelrouting.NewModelRoutingMiddleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const numGoroutines = 100
	const requestsPerGoroutine = 10

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < requestsPerGoroutine; j++ {
				model := "gpt-4"
				if id%2 == 0 {
					model = "claude-3"
				}

				json := fmt.Sprintf(`{"model": "%s", "messages": []}`, model)
				req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(json))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				middleware.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200, got %d", w.Code)
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("Test timed out waiting for goroutines")
		}
	}
}

func TestModelRoutingErrorHandling(t *testing.T) {
	cfg := &config.ModelRoutingConfig{
		Enabled:       true,
		DefaultTarget: "https://api.openai.com/v1",
		Models: map[string]string{
			"gpt-4": "https://api.openai.com/v1",
		},
	}

	middleware := modelrouting.NewModelRoutingMiddleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("handles invalid JSON gracefully", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(`{invalid json`))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		// Should fall back to default target for invalid JSON
		if req.Host != "api.openai.com" {
			t.Errorf("Expected fallback to default target, got host %s", req.Host)
		}
	})

	t.Run("handles empty request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/chat/completions", nil)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		// Should fall back to default target for empty body
		if req.Host != "api.openai.com" {
			t.Errorf("Expected fallback to default target for empty body, got host %s", req.Host)
		}
	})

	t.Run("handles non-string model field", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(`{"model": 123, "messages": []}`))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		// Should fall back to default target for non-string model
		if req.Host != "api.openai.com" {
			t.Errorf("Expected fallback to default target for non-string model, got host %s", req.Host)
		}
	})
}

func TestModelRoutingMemoryEfficiency(t *testing.T) {
	cfg := &config.ModelRoutingConfig{
		Enabled:       true,
		DefaultTarget: "https://api.openai.com/v1",
		Models: map[string]string{
			"gpt-4": "https://api.openai.com/v1",
		},
	}

	// Test with very large payload to ensure streaming parsing works
	largePayload := strings.Repeat("x", 1024*1024) // 1MB of 'x' characters
	json := fmt.Sprintf(`{"model": "gpt-4", "content": "%s"}`, largePayload)

	// Create a handler that captures the body to ensure it's preserved
	var capturedBody []byte
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, len(json))
		n, _ := r.Body.Read(body)
		capturedBody = body[:n]
		w.WriteHeader(http.StatusOK)
	})

	middleware := modelrouting.NewModelRoutingMiddleware(cfg, handler)

	req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(json))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Verify routing worked
	if req.Host != "api.openai.com" {
		t.Errorf("Expected routing to OpenAI, got host %s", req.Host)
	}

	// Verify body was preserved exactly
	if string(capturedBody) != json {
		t.Errorf("Request body was not preserved correctly. Expected length %d, got %d", len(json), len(capturedBody))
	}
}

func TestModelRoutingBackwardCompatibility(t *testing.T) {
	// Test that model routing doesn't interfere with normal requests when disabled
	cfg := &config.ModelRoutingConfig{
		Enabled: false, // Disabled
		Models: map[string]string{
			"gpt-4": "https://api.openai.com/v1",
		},
	}

	// Create a standard router handler
	rateLimiter := ratelimit.New([]config.RateLimitRule{})
	routes := make(map[string]*url.URL)
	router := router.New(routes, rateLimiter)

	middleware := modelrouting.NewModelRoutingMiddleware(cfg, router)

	t.Run("non-JSON requests pass through unchanged", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		originalHost := "original.example.com"
		req.Host = originalHost

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		// Host should remain unchanged
		if req.Host != originalHost {
			t.Errorf("Expected host to remain unchanged when model routing disabled, got %s", req.Host)
		}
	})

	t.Run("JSON requests pass through when disabled", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(`{"model": "gpt-4"}`))
		req.Header.Set("Content-Type", "application/json")
		originalHost := "original.example.com"
		req.Host = originalHost

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)

		// Host should remain unchanged
		if req.Host != originalHost {
			t.Errorf("Expected host to remain unchanged when model routing disabled, got %s", req.Host)
		}
	})
}
