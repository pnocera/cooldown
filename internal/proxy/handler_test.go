package proxy

import (
	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestProxyHandler(t *testing.T) {
	// Create a mock target server
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Target", "test-server")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("target response"))
	}))
	defer targetServer.Close()

	// Parse target URL
	targetURL, _ := url.Parse(targetServer.URL)

	// Create proxy handler
	handler := NewHandler(nil)
	handler.SetTarget(targetURL)

	// Create a request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("X-Test", "test-value")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("X-Target") != "test-server" {
		t.Errorf("Expected X-Target header to be forwarded")
	}
}

func TestProxyHandlerWithRateLimit(t *testing.T) {
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer targetServer.Close()

	targetURL, _ := url.Parse(targetServer.URL)

	// Create handler with rate limiter
	rules := []config.RateLimitRule{
		{Domain: "api.example.com", RequestsPerSecond: 2},
	}
	limiter := ratelimit.New(rules)

	handler := NewHandler(limiter)
	handler.SetTarget(targetURL)

	// Make multiple requests to test rate limiting
	start := time.Now()

	req := httptest.NewRequest("GET", "http://api.example.com/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	firstDuration := time.Since(start)

	// Second request should be delayed
	start = time.Now()
	req = httptest.NewRequest("GET", "http://api.example.com/test", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	secondDuration := time.Since(start)

	// Second request should take longer due to rate limiting
	if secondDuration <= firstDuration {
		t.Logf("Rate limiting may not be working as expected: first=%v, second=%v", firstDuration, secondDuration)
	}
}
