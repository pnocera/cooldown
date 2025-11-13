package modelrouting

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cooldownp/cooldown-proxy/internal/config"
)

func TestMetricsCollection(t *testing.T) {
	cfg := &config.ModelRoutingConfig{
		Enabled:       true,
		DefaultTarget: "https://api.openai.com/v1",
		Models: map[string]string{
			"gpt-4":    "https://api.openai.com/v1",
			"claude-3": "https://api.anthropic.com/v1",
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewModelRoutingMiddleware(cfg, nextHandler)

	// Send multiple requests to generate metrics
	requests := []struct {
		name        string
		body        string
		expectRoute string
	}{
		{"known model", `{"model": "gpt-4", "messages": []}`, "https://api.openai.com/v1"},
		{"unknown model", `{"model": "unknown", "messages": []}`, "https://api.openai.com/v1"},
		{"missing model", `{"messages": []}`, "https://api.openai.com/v1"},
		{"different known model", `{"model": "claude-3", "messages": []}`, "https://api.anthropic.com/v1"},
	}

	for _, req := range requests {
		t.Run(req.name, func(t *testing.T) {
			initialMetrics := middleware.GetMetrics()

			request := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(req.body))
			request.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, request)

			finalMetrics := middleware.GetMetrics()

			// Verify that routing attempts increased
			if finalMetrics.RoutingAttempts != initialMetrics.RoutingAttempts+1 {
				t.Errorf("Expected routing attempts to increase by 1, went from %d to %d",
					initialMetrics.RoutingAttempts, finalMetrics.RoutingAttempts)
			}

			// Verify that processing time was recorded
			if finalMetrics.TotalProcessingTimeNs <= initialMetrics.TotalProcessingTimeNs {
				t.Errorf("Expected processing time to increase, was %v before and %v after",
					initialMetrics.TotalProcessingTimeNs, finalMetrics.TotalProcessingTimeNs)
			}

			// Verify routing was applied correctly
			if req.expectRoute == "https://api.openai.com/v1" {
				if request.Host != "api.openai.com" {
					t.Errorf("Expected routing to OpenAI, got host %s", request.Host)
				}
			} else if req.expectRoute == "https://api.anthropic.com/v1" {
				if request.Host != "api.anthropic.com" {
					t.Errorf("Expected routing to Anthropic, got host %s", request.Host)
				}
			}
		})
	}
}

func TestMetricsErrorHandling(t *testing.T) {
	cfg := &config.ModelRoutingConfig{
		Enabled:       true,
		DefaultTarget: "https://api.openai.com/v1",
		Models: map[string]string{
			"gpt-4": "https://api.openai.com/v1",
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewModelRoutingMiddleware(cfg, nextHandler)

	t.Run("invalid JSON increases parsing errors", func(t *testing.T) {
		initialMetrics := middleware.GetMetrics()

		request := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(`{invalid json`))
		request.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, request)

		finalMetrics := middleware.GetMetrics()

		// Verify that parsing errors increased
		if finalMetrics.ParsingErrors != initialMetrics.ParsingErrors+1 {
			t.Errorf("Expected parsing errors to increase by 1, went from %d to %d",
				initialMetrics.ParsingErrors, finalMetrics.ParsingErrors)
		}

		// Verify that fallback increased
		if finalMetrics.RoutingFallback != initialMetrics.RoutingFallback+1 {
			t.Errorf("Expected routing fallback to increase by 1, went from %d to %d",
				initialMetrics.RoutingFallback, finalMetrics.RoutingFallback)
		}
	})

	t.Run("non-JSON requests don't affect metrics", func(t *testing.T) {
		initialMetrics := middleware.GetMetrics()

		request := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, request)

		finalMetrics := middleware.GetMetrics()

		// Metrics should remain unchanged for non-JSON requests
		if finalMetrics.RoutingAttempts != initialMetrics.RoutingAttempts {
			t.Errorf("Expected routing attempts to remain unchanged, went from %d to %d",
				initialMetrics.RoutingAttempts, finalMetrics.RoutingAttempts)
		}
	})
}

func TestHealthCheckStatus(t *testing.T) {
	t.Run("healthy status with high success rate", func(t *testing.T) {
		cfg := &config.ModelRoutingConfig{
			Enabled:       true,
			DefaultTarget: "https://api.openai.com/v1",
			Models: map[string]string{
				"gpt-4": "https://api.openai.com/v1",
			},
		}

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		middleware := NewModelRoutingMiddleware(cfg, nextHandler)

		// Simulate successful requests
		for i := 0; i < 20; i++ {
			request := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(`{"model": "gpt-4"}`))
			request.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, request)
		}

		// Manually set metrics to simulate high success rate
		middleware.metrics.RoutingAttempts = 100
		middleware.metrics.RoutingSuccess = 96
		middleware.metrics.RoutingFallback = 4
		middleware.metrics.ParsingErrors = 0

		health := middleware.HealthCheck()

		if health["status"] != "healthy" {
			t.Errorf("Expected healthy status, got %s", health["status"])
		}

		if health["success_rate"].(float64) < 95.0 {
			t.Errorf("Expected success rate >= 95%%, got %f", health["success_rate"].(float64))
		}
	})

	t.Run("degraded status with moderate success rate", func(t *testing.T) {
		cfg := &config.ModelRoutingConfig{
			Enabled:       true,
			DefaultTarget: "https://api.openai.com/v1",
			Models: map[string]string{
				"gpt-4": "https://api.openai.com/v1",
			},
		}

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		middleware := NewModelRoutingMiddleware(cfg, nextHandler)

		// Manually set metrics to simulate moderate success rate
		middleware.metrics.RoutingAttempts = 100
		middleware.metrics.RoutingSuccess = 85
		middleware.metrics.RoutingFallback = 10
		middleware.metrics.ParsingErrors = 5

		health := middleware.HealthCheck()

		if health["status"] != "degraded" {
			t.Errorf("Expected degraded status, got %s", health["status"])
		}

		successRate := health["success_rate"].(float64)
		if successRate < 80.0 || successRate >= 95.0 {
			t.Errorf("Expected success rate between 80%% and 95%%, got %f", successRate)
		}
	})

	t.Run("unhealthy status with low success rate", func(t *testing.T) {
		cfg := &config.ModelRoutingConfig{
			Enabled:       true,
			DefaultTarget: "https://api.openai.com/v1",
			Models: map[string]string{
				"gpt-4": "https://api.openai.com/v1",
			},
		}

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		middleware := NewModelRoutingMiddleware(cfg, nextHandler)

		// Manually set metrics to simulate low success rate
		middleware.metrics.RoutingAttempts = 100
		middleware.metrics.RoutingSuccess = 60
		middleware.metrics.RoutingFallback = 20
		middleware.metrics.ParsingErrors = 20

		health := middleware.HealthCheck()

		if health["status"] != "unhealthy" {
			t.Errorf("Expected unhealthy status, got %s", health["status"])
		}

		if health["success_rate"].(float64) >= 80.0 {
			t.Errorf("Expected success rate < 80%%, got %f", health["success_rate"].(float64))
		}
	})

	t.Run("disabled status when routing disabled", func(t *testing.T) {
		cfg := &config.ModelRoutingConfig{
			Enabled: false, // Disabled
		}

		middleware := NewModelRoutingMiddleware(cfg, nil)

		health := middleware.HealthCheck()

		if health["status"] != "disabled" {
			t.Errorf("Expected disabled status, got %s", health["status"])
		}

		if health["enabled"].(bool) != false {
			t.Errorf("Expected enabled=false, got %v", health["enabled"])
		}
	})

	t.Run("nil config returns disabled", func(t *testing.T) {
		middleware := NewModelRoutingMiddleware(nil, nil)

		health := middleware.HealthCheck()

		if health["status"] != "disabled" {
			t.Errorf("Expected disabled status for nil config, got %s", health["status"])
		}
	})
}

func TestMetricsAccuracy(t *testing.T) {
	cfg := &config.ModelRoutingConfig{
		Enabled:       true,
		DefaultTarget: "https://api.openai.com/v1",
		Models: map[string]string{
			"gpt-4":    "https://api.openai.com/v1",
			"claude-3": "https://api.anthropic.com/v1",
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewModelRoutingMiddleware(cfg, nextHandler)

	const numRequests = 100
	successCount := 0
	fallbackCount := 0
	errorCount := 0

	// Send requests and track expected results
	for i := 0; i < numRequests; i++ {
		var body string

		switch i % 4 {
		case 0:
			body = `{"model": "gpt-4", "messages": []}`
			successCount++
		case 1:
			body = `{"model": "claude-3", "messages": []}`
			successCount++
		case 2:
			body = `{"model": "unknown-model", "messages": []}`
			fallbackCount++
		case 3:
			body = `{invalid json`
			errorCount++
			fallbackCount++
		}

		request := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, request)
	}

	// Get final metrics
	metrics := middleware.GetMetrics()

	// Verify accuracy
	if metrics.RoutingAttempts != int64(numRequests) {
		t.Errorf("Expected %d routing attempts, got %d", numRequests, metrics.RoutingAttempts)
	}

	if metrics.RoutingSuccess != int64(successCount) {
		t.Errorf("Expected %d routing successes, got %d", successCount, metrics.RoutingSuccess)
	}

	if metrics.RoutingFallback != int64(fallbackCount) {
		t.Errorf("Expected %d routing fallbacks, got %d", fallbackCount, metrics.RoutingFallback)
	}

	if metrics.ParsingErrors != int64(errorCount) {
		t.Errorf("Expected %d parsing errors, got %d", errorCount, metrics.ParsingErrors)
	}

	// Verify processing time is reasonable
	avgProcessingTime := float64(metrics.TotalProcessingTimeNs) / float64(numRequests) / 1e6
	if avgProcessingTime > 100 { // More than 100ms average seems too high
		t.Errorf("Average processing time seems too high: %.2f ms", avgProcessingTime)
	}

	t.Logf("Metrics accuracy verified - Attempts: %d, Success: %d, Fallback: %d, Errors: %d, Avg Time: %.2f ms",
		metrics.RoutingAttempts, metrics.RoutingSuccess, metrics.RoutingFallback, metrics.ParsingErrors, avgProcessingTime)
}

func TestMetricsThreadSafety(t *testing.T) {
	cfg := &config.ModelRoutingConfig{
		Enabled:       true,
		DefaultTarget: "https://api.openai.com/v1",
		Models: map[string]string{
			"gpt-4": "https://api.openai.com/v1",
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewModelRoutingMiddleware(cfg, nextHandler)

	const numGoroutines = 10
	const requestsPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	// Launch multiple goroutines to test thread safety
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < requestsPerGoroutine; j++ {
				request := httptest.NewRequest("POST", "/chat/completions", strings.NewReader(`{"model": "gpt-4"}`))
				request.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				middleware.ServeHTTP(w, request)

				// Also test metrics reading concurrently
				_ = middleware.GetMetrics()
				_ = middleware.HealthCheck()
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final metrics are consistent
	metrics := middleware.GetMetrics()
	health := middleware.HealthCheck()

	expectedTotalRequests := numGoroutines * requestsPerGoroutine
	if metrics.RoutingAttempts != int64(expectedTotalRequests) {
		t.Errorf("Expected %d total routing attempts, got %d", expectedTotalRequests, metrics.RoutingAttempts)
	}

	// Health check should not panic and should return reasonable data
	if health["routing_attempts"].(int64) != int64(expectedTotalRequests) {
		t.Errorf("Health check routing attempts mismatch: expected %d, got %d",
			expectedTotalRequests, health["routing_attempts"].(int64))
	}

	t.Logf("Thread safety verified - %d concurrent requests processed correctly", expectedTotalRequests)
}
