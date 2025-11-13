package benchmarks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/proxy"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/cooldownp/cooldown-proxy/internal/token"
)

func BenchmarkHeaderBasedRateLimiting(b *testing.B) {
	// Create a mock server that returns rate limit headers
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-limit-tokens-minute", "1000000")
		w.Header().Set("x-ratelimit-remaining-tokens-minute", "900000")
		w.Header().Set("x-ratelimit-reset-tokens-minute", "30.0")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"response": "ok"}`))
	}))
	defer mockServer.Close()

	// Create proxy handler with header-based rate limiting enabled
	cerebrasConfig := &config.CerebrasLimits{
		RateLimits: config.CerebrasRateLimitConfig{
			UseHeaders:     true,
			HeaderFallback: true,
			HeaderTimeout:  5 * time.Second,
			ResetBuffer:    100 * time.Millisecond,
		},
		RPMLimit:       60,
		TPMLimit:       1000,
		MaxQueueDepth:  100,
		RequestTimeout: 10 * time.Minute,
	}

	limiter := ratelimit.NewCerebrasLimiter(cerebrasConfig.RPMLimit, cerebrasConfig.TPMLimit)
	estimator := token.NewTokenEstimator()
	handler := proxy.NewCerebrasProxyHandler(limiter, estimator, cerebrasConfig)

	// Set target to mock server
	targetURL, _ := url.Parse(mockServer.URL)
	handler.SetTarget(targetURL)

	// Prepare test request
	requestBody := map[string]interface{}{
		"model": "test-model",
		"messages": []map[string]string{
			{"role": "user", "content": "This is a benchmark test message."},
		},
		"max_tokens": 50,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Host = "api.cerebras.ai"

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

func BenchmarkStaticRateLimiting(b *testing.B) {
	// Create a mock server without rate limit headers
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"response": "ok"}`))
	}))
	defer mockServer.Close()

	// Create proxy handler with static rate limiting only
	cerebrasConfig := &config.CerebrasLimits{
		RateLimits: config.CerebrasRateLimitConfig{
			UseHeaders: false, // Disable header-based rate limiting
		},
		RPMLimit:       60,
		TPMLimit:       1000,
		MaxQueueDepth:  100,
		RequestTimeout: 10 * time.Minute,
	}

	limiter := ratelimit.NewCerebrasLimiter(cerebrasConfig.RPMLimit, cerebrasConfig.TPMLimit)
	estimator := token.NewTokenEstimator()
	handler := proxy.NewCerebrasProxyHandler(limiter, estimator, cerebrasConfig)

	// Set target to mock server
	targetURL, _ := url.Parse(mockServer.URL)
	handler.SetTarget(targetURL)

	// Prepare test request
	requestBody := map[string]interface{}{
		"model": "test-model",
		"messages": []map[string]string{
			{"role": "user", "content": "This is a benchmark test message."},
		},
		"max_tokens": 50,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Host = "api.cerebras.ai"

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Errorf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

func BenchmarkHeaderParsing(b *testing.B) {
	// Test just the header parsing performance
	headers := http.Header{
		"x-ratelimit-limit-tokens-minute":     []string{"1000000"},
		"x-ratelimit-remaining-tokens-minute": []string{"900000"},
		"x-ratelimit-reset-tokens-minute":     []string{"30.5"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ratelimit.ParseRateLimitHeaders(headers)
		if err != nil {
			b.Errorf("Header parsing failed: %v", err)
		}
	}
}

func BenchmarkConcurrentRequests(b *testing.B) {
	// Test concurrent request handling with header-based rate limiting
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate real API latency
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("x-ratelimit-limit-tokens-minute", "1000000")
		w.Header().Set("x-ratelimit-remaining-tokens-minute", "950000")
		w.Header().Set("x-ratelimit-reset-tokens-minute", "45.0")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"response": "ok"}`))
	}))
	defer mockServer.Close()

	cerebrasConfig := &config.CerebrasLimits{
		RateLimits: config.CerebrasRateLimitConfig{
			UseHeaders:     true,
			HeaderFallback: true,
			HeaderTimeout:  5 * time.Second,
			ResetBuffer:    100 * time.Millisecond,
		},
		RPMLimit:       1000, // Higher limit for concurrent testing
		TPMLimit:       1000000,
		MaxQueueDepth:  500,
		RequestTimeout: 10 * time.Minute,
	}

	limiter := ratelimit.NewCerebrasLimiter(cerebrasConfig.RPMLimit, cerebrasConfig.TPMLimit)
	estimator := token.NewTokenEstimator()
	handler := proxy.NewCerebrasProxyHandler(limiter, estimator, cerebrasConfig)

	parsedURL, _ := url.Parse(mockServer.URL)
	handler.SetTarget(parsedURL)

	requestBody := map[string]interface{}{
		"model": "test-model",
		"messages": []map[string]string{
			{"role": "user", "content": "Concurrent benchmark test message."},
		},
		"max_tokens": 50,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	// Test with different numbers of concurrent goroutines
	concurrencyLevels := []int{1, 5, 10, 20, 50}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			b.SetParallelism(concurrency)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
					req.Header.Set("Content-Type", "application/json")
					req.Host = "api.cerebras.ai"

					w := httptest.NewRecorder()
					handler.ServeHTTP(w, req)

					// Don't check status in benchmark to avoid affecting performance measurement
				}
			})
		})
	}
}

func BenchmarkMemoryUsage(b *testing.B) {
	// Benchmark memory allocation patterns
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-limit-tokens-minute", "1000000")
		w.Header().Set("x-ratelimit-remaining-tokens-minute", "900000")
		w.Header().Set("x-ratelimit-reset-tokens-minute", "30.0")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"response": "ok"}`))
	}))
	defer mockServer.Close()

	cerebrasConfig := &config.CerebrasLimits{
		RateLimits: config.CerebrasRateLimitConfig{
			UseHeaders:     true,
			HeaderFallback: true,
			HeaderTimeout:  5 * time.Second,
			ResetBuffer:    100 * time.Millisecond,
		},
		RPMLimit:       60,
		TPMLimit:       1000,
		MaxQueueDepth:  100,
		RequestTimeout: 10 * time.Minute,
	}

	limiter := ratelimit.NewCerebrasLimiter(cerebrasConfig.RPMLimit, cerebrasConfig.TPMLimit)
	estimator := token.NewTokenEstimator()
	handler := proxy.NewCerebrasProxyHandler(limiter, estimator, cerebrasConfig)

	parsedURL, _ := url.Parse(mockServer.URL)
	handler.SetTarget(parsedURL)

	requestBody := map[string]interface{}{
		"model": "test-model",
		"messages": []map[string]string{
			{"role": "user", "content": "Memory usage benchmark test message."},
		},
		"max_tokens": 50,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Host = "api.cerebras.ai"

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}
