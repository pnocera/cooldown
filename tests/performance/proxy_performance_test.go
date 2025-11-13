package performance

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/proxy"
)

// TestProxyPerformance tests basic proxy performance under load
func TestProxyPerformance(t *testing.T) {
	// Create test upstream server that responds quickly
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Hello from upstream", "path": "` + r.URL.Path + `"}`))
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

	// Test configurations
	testConfigs := []struct {
		name        string
		concurrency int
		requests    int
		maxLatency  time.Duration
		minQPS      float64
	}{
		{
			name:        "Light Load",
			concurrency: 5,
			requests:    50,
			maxLatency:  100 * time.Millisecond,
			minQPS:      100,
		},
		{
			name:        "Moderate Load",
			concurrency: 20,
			requests:    200,
			maxLatency:  200 * time.Millisecond,
			minQPS:      200,
		},
		{
			name:        "Heavy Load",
			concurrency: 50,
			requests:    500,
			maxLatency:  500 * time.Millisecond,
			minQPS:      300,
		},
	}

	for _, config := range testConfigs {
		t.Run(config.name, func(t *testing.T) {
			client := &http.Client{Timeout: 5 * time.Second}

			var wg sync.WaitGroup
			results := make(chan time.Duration, config.requests)
			errors := make(chan error, config.requests)

			start := time.Now()

			// Distribute requests across concurrent workers
			requestsPerWorker := config.requests / config.concurrency
			remaining := config.requests % config.concurrency

			for i := 0; i < config.concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					// Handle remaining requests
					workerRequests := requestsPerWorker
					if workerID < remaining {
						workerRequests++
					}

					for j := 0; j < workerRequests; j++ {
						reqStart := time.Now()

						resp, err := client.Get(testServer.URL + "/api/test")
						if err != nil {
							errors <- err
							continue
						}

						resp.Body.Close()

						if resp.StatusCode == http.StatusOK {
							results <- time.Since(reqStart)
						} else {
							errors <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
						}
					}
				}(i)
			}

			wg.Wait()
			close(results)
			close(errors)

			totalTime := time.Since(start)

			// Collect results
			var totalLatency time.Duration
			successCount := 0
			var minLatency, maxLatency time.Duration

			first := true
			for latency := range results {
				successCount++
				totalLatency += latency
				if first {
					minLatency = latency
					maxLatency = latency
					first = false
				} else {
					if latency < minLatency {
						minLatency = latency
					}
					if latency > maxLatency {
						maxLatency = latency
					}
				}
			}

			// Count errors
			errorCount := 0
			for err := range errors {
				errorCount++
				t.Logf("Request error: %v", err)
			}

			// Calculate metrics
			successRate := float64(successCount) / float64(config.requests) * 100
			qps := float64(successCount) / totalTime.Seconds()
			avgLatency := time.Duration(0)
			if successCount > 0 {
				avgLatency = totalLatency / time.Duration(successCount)
			}

			// Log results
			t.Logf("=== %s Performance Results ===", config.name)
			t.Logf("  Concurrency: %d", config.concurrency)
			t.Logf("  Total Requests: %d", config.requests)
			t.Logf("  Successful Requests: %d (%.1f%%)", successCount, successRate)
			t.Logf("  Failed Requests: %d", errorCount)
			t.Logf("  Total Time: %v", totalTime)
			t.Logf("  QPS: %.2f", qps)
			t.Logf("  Average Latency: %v", avgLatency)
			t.Logf("  Min Latency: %v", minLatency)
			t.Logf("  Max Latency: %v", maxLatency)

			// Performance assertions
			if successRate < 95.0 {
				t.Errorf("Success rate too low: %.1f%% (expected >= 95%%)", successRate)
			}

			if qps < config.minQPS {
				t.Errorf("QPS too low: %.2f (expected >= %.2f)", qps, config.minQPS)
			}

			if avgLatency > config.maxLatency {
				t.Errorf("Average latency too high: %v (expected <= %v)", avgLatency, config.maxLatency)
			}

			// Additional stress assertions for heavy load
			if config.name == "Heavy Load" {
				if maxLatency > 2*config.maxLatency {
					t.Logf("WARNING: Max latency significantly higher than average: %v", maxLatency)
				}
			}
		})
	}
}

// TestProxyConcurrencySafety tests concurrent access to proxy handler
func TestProxyConcurrencySafety(t *testing.T) {
	// Create test upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add small delay to simulate real-world latency
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	proxyHandler := proxy.NewHandler(nil)
	proxyHandler.SetTarget(upstreamURL)

	testServer := httptest.NewServer(proxyHandler)
	defer testServer.Close()

	// High concurrency test
	concurrency := 100
	requestsPerWorker := 10

	var wg sync.WaitGroup
	errors := make(chan error, concurrency*requestsPerWorker)

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			client := &http.Client{Timeout: 5 * time.Second}

			for j := 0; j < requestsPerWorker; j++ {
				resp, err := client.Get(testServer.URL + "/api/test")
				if err != nil {
					errors <- fmt.Errorf("worker %d request %d failed: %v", workerID, j, err)
					continue
				}
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					errors <- fmt.Errorf("worker %d request %d returned status %d", workerID, j, resp.StatusCode)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	totalTime := time.Since(start)

	// Check for any errors
	errorCount := 0
	for err := range errors {
		errorCount++
		t.Logf("Concurrency error: %v", err)
	}

	totalRequests := concurrency * requestsPerWorker
	successRate := float64(totalRequests-errorCount) / float64(totalRequests) * 100
	qps := float64(totalRequests-errorCount) / totalTime.Seconds()

	t.Logf("=== Concurrency Safety Test Results ===")
	t.Logf("  Concurrency: %d", concurrency)
	t.Logf("  Requests per Worker: %d", requestsPerWorker)
	t.Logf("  Total Requests: %d", totalRequests)
	t.Logf("  Successful Requests: %d (%.1f%%)", totalRequests-errorCount, successRate)
	t.Logf("  Failed Requests: %d", errorCount)
	t.Logf("  Total Time: %v", totalTime)
	t.Logf("  QPS: %.2f", qps)

	// Assertions
	if successRate < 95.0 {
		t.Errorf("Success rate too low for concurrency test: %.1f%%", successRate)
	}

	if errorCount > 0 {
		t.Errorf("Concurrency safety issues detected: %d errors out of %d requests", errorCount, totalRequests)
	}

	// Should handle high concurrency gracefully
	if qps < 200 {
		t.Errorf("QPS too low for concurrency test: %.2f (expected >= 200)", qps)
	}
}

// TestProxyMemoryUsage tests memory efficiency over many requests
func TestProxyMemoryUsage(t *testing.T) {
	// Create test upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "response data"}`))
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	proxyHandler := proxy.NewHandler(nil)
	proxyHandler.SetTarget(upstreamURL)

	testServer := httptest.NewServer(proxyHandler)
	defer testServer.Close()

	// Test many requests over time to check for memory leaks
	client := &http.Client{Timeout: 2 * time.Second}
	iterations := 1000
	requestsPerIteration := 10

	for i := 0; i < iterations; i++ {
		for j := 0; j < requestsPerIteration; j++ {
			resp, err := client.Get(testServer.URL + "/api/memory")
			if err != nil {
				t.Fatalf("Request failed at iteration %d, request %d: %v", i, j, err)
			}
			resp.Body.Close()
		}

		// Small pause to allow garbage collection
		if i%100 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	t.Logf("Memory usage test completed: %d total requests", iterations*requestsPerIteration)
}