package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/proxy"
	"github.com/stretchr/testify/assert"
)

// TestSimpleProxyIntegration tests basic proxy functionality without complex routing
func TestSimpleProxyIntegration(t *testing.T) {
	// Create test upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Hello from upstream", "path": "` + r.URL.Path + `"}`))
	}))
	defer upstream.Close()

	// Create proxy handler (simple, no rate limiting)
	proxyHandler := proxy.NewHandler(nil)

	// Set target directly for this test
	targetURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("Failed to parse upstream URL: %v", err)
	}
	proxyHandler.SetTarget(targetURL)

	// Create test server
	testServer := httptest.NewServer(proxyHandler)
	defer testServer.Close()

	// Test request through proxy
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", testServer.URL+"/api/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}

// TestProxyErrorHandlingSimple tests basic error handling
func TestProxyErrorHandlingSimple(t *testing.T) {
	// Create proxy handler without setting target (should cause errors)
	proxyHandler := proxy.NewHandler(nil)

	// Create test server
	testServer := httptest.NewServer(proxyHandler)
	defer testServer.Close()

	// Test request without proper target setup
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", testServer.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return error due to misconfigured proxy
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}

// TestProxyWithInvalidUpstream tests handling of invalid upstream
func TestProxyWithInvalidUpstream(t *testing.T) {
	// Create proxy handler with invalid target
	proxyHandler := proxy.NewHandler(nil)
	invalidURL, _ := url.Parse("http://invalid-host-that-does-not-exist:9999")
	proxyHandler.SetTarget(invalidURL)

	// Create test server
	testServer := httptest.NewServer(proxyHandler)
	defer testServer.Close()

	// Test request to invalid upstream
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", testServer.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return 502 Bad Gateway
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

// TestProxyConcurrentRequests tests concurrent request handling
func TestProxyConcurrentRequests(t *testing.T) {
	// Create test upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"request_id": "` + r.Header.Get("X-Request-ID") + `"}`))
	}))
	defer upstream.Close()

	// Parse upstream URL
	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("Failed to parse upstream URL: %v", err)
	}

	// Create proxy handler
	proxyHandler := proxy.NewHandler(nil)
	proxyHandler.SetTarget(upstreamURL)

	// Create test server
	testServer := httptest.NewServer(proxyHandler)
	defer testServer.Close()

	// Test concurrent requests
	concurrency := 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	client := &http.Client{Timeout: 10 * time.Second}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			req, err := http.NewRequest("GET", testServer.URL, nil)
			if err != nil {
				return
			}
			req.Header.Set("X-Request-ID", fmt.Sprintf("worker-%d", workerID))

			resp, err := client.Do(req)
			if err != nil {
				return
			}

			if resp.StatusCode == http.StatusOK {
				mu.Lock()
				successCount++
				mu.Unlock()
			}

			resp.Body.Close()
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent test: %d/%d requests succeeded", successCount, concurrency)
	assert.Greater(t, successCount, concurrency/2, "At least half of concurrent requests should succeed")
}