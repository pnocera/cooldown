package loadtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/token"
)

// LoadTestConfig holds configuration for load tests
type LoadTestConfig struct {
	ConcurrentClients int           `json:"concurrent_clients"`
	Duration          time.Duration `json:"duration"`
	RequestRate       int           `json:"request_rate_per_second"`
	Endpoint          string        `json:"endpoint"`
	TestData          []TestData    `json:"test_data"`
	Timeout           time.Duration `json:"timeout"`
}

// TestData represents a single test request
type TestData struct {
	Model     string `json:"model"`
	Prompt    string `json:"prompt"`
	MaxTokens int    `json:"max_tokens"`
}

// LoadTestResult contains the results of a load test
type LoadTestResult struct {
	TotalRequests       int64               `json:"total_requests"`
	SuccessfulRequests  int64               `json:"successful_requests"`
	FailedRequests      int64               `json:"failed_requests"`
	Duration            time.Duration       `json:"duration"`
	AverageLatency      time.Duration       `json:"average_latency"`
	MinLatency          time.Duration       `json:"min_latency"`
	MaxLatency          time.Duration       `json:"max_latency"`
	RequestsPerSecond   float64             `json:"requests_per_second"`
	ErrorBreakdown      map[string]int      `json:"error_breakdown"`
	CircuitBreakerStats CircuitBreakerStats `json:"circuit_breaker_stats"`
}

// CircuitBreakerStats tracks circuit breaker behavior during tests
type CircuitBreakerStats struct {
	StateChanges     int `json:"state_changes"`
	CircuitOpens     int `json:"circuit_opens"`
	CircuitCloses    int `json:"circuit_closes"`
	RejectedRequests int `json:"rejected_requests"`
}

// Client represents a load testing client
type Client struct {
	id         int
	config     LoadTestConfig
	httpClient *http.Client
	estimator  *token.TokenEstimator
	stats      *ClientStats
}

// ClientStats tracks individual client statistics
type ClientStats struct {
	Requests        int64
	Successes       int64
	Errors          int64
	TotalLatency    time.Duration
	MinLatency      time.Duration
	MaxLatency      time.Duration
	ErrorsBreakdown map[string]int
	mu              sync.Mutex
}

// LoadTestRunner orchestrates load testing
type LoadTestRunner struct {
	config       LoadTestConfig
	clients      []*Client
	stats        LoadTestResult
	circuitStats CircuitBreakerStats
	startTime    time.Time
	wg           sync.WaitGroup
}

// NewLoadTestRunner creates a new load test runner
func NewLoadTestRunner(config LoadTestConfig) *LoadTestRunner {
	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &LoadTestRunner{
		config:       config,
		stats:        LoadTestResult{ErrorBreakdown: make(map[string]int)},
		circuitStats: CircuitBreakerStats{},
	}
}

// Run executes the load test
func (ltr *LoadTestRunner) Run() LoadTestResult {
	ltr.startTime = time.Now()

	// Initialize clients
	ltr.initializeClients()

	// Start monitoring circuit breaker
	go ltr.monitorCircuitBreaker()

	// Start all clients
	for _, client := range ltr.clients {
		ltr.wg.Add(1)
		go client.Run(&ltr.wg, ltr.config.Duration)
	}

	// Wait for all clients to complete
	ltr.wg.Wait()

	// Aggregate results
	ltr.aggregateResults()

	return ltr.stats
}

// initializeClients creates the load testing clients
func (ltr *LoadTestRunner) initializeClients() {
	for i := 0; i < ltr.config.ConcurrentClients; i++ {
		client := &Client{
			id:     i,
			config: ltr.config,
			httpClient: &http.Client{
				Timeout: ltr.config.Timeout,
			},
			estimator: token.NewTokenEstimator(),
			stats: &ClientStats{
				MinLatency:      time.Hour, // Initialize to a large value
				ErrorsBreakdown: make(map[string]int),
			},
		}
		ltr.clients = append(ltr.clients, client)
	}
}

// monitorCircuitBreaker monitors circuit breaker behavior
func (ltr *LoadTestRunner) monitorCircuitBreaker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-time.After(ltr.config.Duration):
			return
		case <-ticker.C:
			// Poll circuit breaker status
			resp, err := http.Get(ltr.config.Endpoint + "/health")
			if err != nil {
				continue
			}

			// Check circuit breaker headers
			if state := resp.Header.Get("X-CircuitBreaker-State"); state != "" {
				// Track state changes here if needed
				resp.Body.Close()
			}
		}
	}
}

// aggregateResults combines statistics from all clients
func (ltr *LoadTestRunner) aggregateResults() {
	ltr.stats.Duration = time.Since(ltr.startTime)

	for _, client := range ltr.clients {
		client.stats.mu.Lock()

		atomic.AddInt64(&ltr.stats.TotalRequests, client.stats.Requests)
		atomic.AddInt64(&ltr.stats.SuccessfulRequests, client.stats.Successes)
		atomic.AddInt64(&ltr.stats.FailedRequests, client.stats.Errors)

		// Update latency statistics
		if client.stats.Requests > 0 {
			avgLatency := client.stats.TotalLatency / time.Duration(client.stats.Requests)
			ltr.stats.AverageLatency += avgLatency

			if client.stats.MinLatency < ltr.stats.MinLatency || ltr.stats.MinLatency == 0 {
				ltr.stats.MinLatency = client.stats.MinLatency
			}

			if client.stats.MaxLatency > ltr.stats.MaxLatency {
				ltr.stats.MaxLatency = client.stats.MaxLatency
			}
		}

		// Aggregate error breakdown
		for errorType, count := range client.stats.ErrorsBreakdown {
			ltr.stats.ErrorBreakdown[errorType] += count
		}

		client.stats.mu.Unlock()
	}

	// Calculate final averages
	if len(ltr.clients) > 0 {
		ltr.stats.AverageLatency = ltr.stats.AverageLatency / time.Duration(len(ltr.clients))
	}

	// Calculate requests per second
	if ltr.stats.Duration > 0 {
		ltr.stats.RequestsPerSecond = float64(ltr.stats.TotalRequests) / ltr.stats.Duration.Seconds()
	}

	ltr.stats.CircuitBreakerStats = ltr.circuitStats
}

// Run executes the client load test
func (c *Client) Run(wg *sync.WaitGroup, duration time.Duration) {
	defer wg.Done()

	ticker := time.NewTicker(time.Second / time.Duration(c.RequestRateRatePerClient()))
	defer ticker.Stop()

	timeout := time.After(duration)

	for {
		select {
		case <-timeout:
			return
		case <-ticker.C:
			c.makeRequest()
		}
	}
}

// RequestRateRatePerClient calculates the rate each client should maintain
func (c *Client) RequestRateRatePerClient() int {
	if c.config.ConcurrentClients == 0 {
		return 1
	}
	return max(1, c.config.RequestRate/c.config.ConcurrentClients)
}

// makeRequest makes a single HTTP request
func (c *Client) makeRequest() {
	startTime := time.Now()

	// Select test data
	testData := c.config.TestData[c.id%len(c.config.TestData)]

	// Create request payload
	payload := map[string]interface{}{
		"model": testData.Model,
		"messages": []map[string]string{
			{"role": "user", "content": testData.Prompt},
		},
		"max_tokens": testData.MaxTokens,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		c.recordError("marshal_error")
		return
	}

	// Make HTTP request
	req, err := http.NewRequest("POST", c.config.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		c.recordError("request_creation_error")
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "api.cerebras.ai") // Simulate Cerebras request

	resp, err := c.httpClient.Do(req)
	latency := time.Since(startTime)

	if err != nil {
		c.recordError("network_error")
		return
	}
	defer resp.Body.Close()

	// Record request
	c.stats.mu.Lock()
	c.stats.Requests++
	c.stats.TotalLatency += latency

	if latency < c.stats.MinLatency {
		c.stats.MinLatency = latency
	}

	if latency > c.stats.MaxLatency {
		c.stats.MaxLatency = latency
	}

	// Check response status
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		c.stats.Successes++
	} else {
		c.stats.Errors++
		c.stats.ErrorsBreakdown[fmt.Sprintf("http_%d", resp.StatusCode)]++
	}

	c.stats.mu.Unlock()

	// Read response body to ensure full request completion
	_, _ = io.Copy(io.Discard, resp.Body)
}

// recordError records an error occurrence
func (c *Client) recordError(errorType string) {
	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()

	c.stats.Errors++
	c.stats.ErrorsBreakdown[errorType]++
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
