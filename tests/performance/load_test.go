package performance

import (
	"bytes"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/config"
)

func TestAnthropicEndpointPerformance(t *testing.T) {
	baseURL := "http://localhost:5730/anthropic"

	// Skip if server not running
	if !isServerRunning(baseURL) {
		t.Skip("Server not running - skipping performance test")
	}

	concurrency := 10
	requests := 100

	var wg sync.WaitGroup
	results := make(chan time.Duration, requests)

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requests/concurrency; j++ {
				reqStart := time.Now()

				resp, err := http.Post(baseURL+"/v1/messages", "application/json",
					bytes.NewBufferString(`{
						"model": "claude-3-5-sonnet-20241022",
						"max_tokens": 50,
						"messages": [{"role": "user", "content": "Hello"}]
					}`))

				if err != nil {
					t.Logf("Request failed: %v", err)
					continue
				}
				resp.Body.Close()

				results <- time.Since(reqStart)
			}
		}()
	}

	wg.Wait()
	close(results)

	totalTime := time.Since(start)

	var totalResponseTime time.Duration
	count := 0
	for responseTime := range results {
		totalResponseTime += responseTime
		count++
	}

	if count == 0 {
		t.Fatal("No successful requests")
	}

	avgResponseTime := totalResponseTime / time.Duration(count)
	qps := float64(count) / totalTime.Seconds()

	t.Logf("Performance Results:")
	t.Logf("  Total Requests: %d", count)
	t.Logf("  Total Time: %v", totalTime)
	t.Logf("  Average Response Time: %v", avgResponseTime)
	t.Logf("  Queries Per Second: %.2f", qps)

	// Performance assertions
	if avgResponseTime > 5*time.Second {
		t.Errorf("Average response time too high: %v", avgResponseTime)
	}

	if qps < 1.0 {
		t.Errorf("QPS too low: %.2f", qps)
	}
}

func TestConfigurationLoadingPerformance(t *testing.T) {
	configPath := "../../config.yaml.example-claude-code"

	iterations := 1000

	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := config.Load(configPath)
		if err != nil {
			t.Fatalf("Failed to load configuration: %v", err)
		}
	}
	totalTime := time.Since(start)

	avgTime := totalTime / time.Duration(iterations)
	t.Logf("Configuration Loading Performance:")
	t.Logf("  Iterations: %d", iterations)
	t.Logf("  Total Time: %v", totalTime)
	t.Logf("  Average Time per Load: %v", avgTime)

	// Should be reasonably fast - under 5ms per load
	if avgTime > 5*time.Millisecond {
		t.Errorf("Configuration loading too slow: %v", avgTime)
	}
}

func isServerRunning(baseURL string) bool {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
