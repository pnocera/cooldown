//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/config"
)

// TestServer represents a test HTTP server
type TestServer struct {
	server      *httptest.Server
	requests    []*http.Request
	failures    int
	responses   map[string]MockResponse
}

// MockResponse represents a mock HTTP response
type MockResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       interface{}
	Delay      time.Duration
}

// NewTestServer creates a new test server with optional failure simulation
func NewTestServer(failureThreshold int, responseDelay time.Duration) *TestServer {
	ts := &TestServer{
		responses: make(map[string]MockResponse),
		requests:  make([]*http.Request, 0),
	}

	ts.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.requests = append(ts.requests, r)

		// Check for failure simulation
		if failureThreshold > 0 && ts.failures >= failureThreshold {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Add default delay
		if responseDelay > 0 {
			time.Sleep(responseDelay)
		}

		// Get mock response for this path
		mockResp, exists := ts.responses[r.URL.Path]
		if !exists {
			// Default response
			w.Header().Set("Content-Type", "application/json")
			defaultResp := map[string]interface{}{
				"id":      "test-response",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   "llama3.1-8b",
			}
			json.NewEncoder(w).Encode(defaultResp)
			return
		}

		// Apply mock response
		for key, value := range mockResp.Headers {
			w.Header().Set(key, value)
		}

		if mockResp.Delay > 0 {
			time.Sleep(mockResp.Delay)
		}

		w.WriteHeader(mockResp.StatusCode)
		if mockResp.Body != nil {
			json.NewEncoder(w).Encode(mockResp.Body)
		}
	}))

	return ts
}

// SetMockResponse sets a mock response for a specific path
func (ts *TestServer) SetMockResponse(path string, response MockResponse) {
	ts.responses[path] = response
}

// SimulateFailure simulates a server failure
func (ts *TestServer) SimulateFailure() {
	ts.failures++
}

// ResetFailures resets the failure counter
func (ts *TestServer) ResetFailures() {
	ts.failures = 0
}

// GetRequestCount returns the number of requests received
func (ts *TestServer) GetRequestCount() int {
	return len(ts.requests)
}

// GetRequests returns all received requests
func (ts *TestServer) GetRequests() []*http.Request {
	return ts.requests
}

// URL returns the server URL
func (ts *TestServer) URL() string {
	return ts.server.URL
}

// Close closes the test server
func (ts *TestServer) Close() {
	ts.server.Close()
}

// TestConfigBuilder helps build test configurations
type TestConfigBuilder struct {
	config map[string]interface{}
}

// NewTestConfigBuilder creates a new test config builder
func NewTestConfigBuilder() *TestConfigBuilder {
	return &TestConfigBuilder{
		config: make(map[string]interface{}),
	}
}

// WithServer sets server configuration
func (tcb *TestConfigBuilder) WithServer(host string, port int) *TestConfigBuilder {
	tcb.config["server"] = map[string]interface{}{
		"host": host,
		"port": port,
	}
	return tcb
}

// WithCerebrasLimits sets Cerebras rate limiting configuration
func (tcb *TestConfigBuilder) WithCerebrasLimits(rpm, tpm int, options map[string]interface{}) *TestConfigBuilder {
	cerebrasConfig := map[string]interface{}{
		"rpm_limit":        rpm,
		"tpm_limit":        tpm,
		"max_queue_depth":  100,
		"request_timeout":  "10m",
		"priority_threshold": 0.7,
	}

	for key, value := range options {
		cerebrasConfig[key] = value
	}

	tcb.config["cerebras_limits"] = cerebrasConfig
	return tcb
}

// WithStandardRateLimits sets standard rate limiting configuration
func (tcb *TestConfigBuilder) WithStandardRateLimits(rateLimits []map[string]interface{}) *TestConfigBuilder {
	tcb.config["rate_limits"] = rateLimits
	return tcb
}

// WithDefaultRateLimit sets default rate limit
func (tcb *TestConfigBuilder) WithDefaultRateLimit(rps int) *TestConfigBuilder {
	tcb.config["default_rate_limit"] = map[string]interface{}{
		"requests_per_second": rps,
	}
	return tcb
}

// Build creates the configuration and writes it to a file
func (tcb *TestConfigBuilder) Build(t *testing.T) string {
	configData, err := json.MarshalIndent(tcb.config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test config: %v", err)
	}

	configFile := filepath.Join(t.TempDir(), fmt.Sprintf("test-config-%d.yaml", time.Now().UnixNano()))
	err = os.WriteFile(configFile, configData, 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	return configFile
}

// LoadConfig loads the built configuration
func (tcb *TestConfigBuilder) LoadConfig(t *testing.T) *config.Config {
	configFile := tcb.Build(t)
	cfg, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}
	return cfg
}

// HTTPTestClient represents a test HTTP client
type HTTPTestClient struct {
	client  *http.Client
	baseURL string
}

// NewHTTPTestClient creates a new HTTP test client
func NewHTTPTestClient(baseURL string) *HTTPTestClient {
	return &HTTPTestClient{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
	}
}

// MakeCerebrasRequest makes a request to the Cerebras API through the proxy
func (htc *HTTPTestClient) MakeCerebrasRequest(prompt string, maxTokens int) (*http.Response, error) {
	requestBody := map[string]interface{}{
		"model": "llama3.1-8b",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": maxTokens,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", htc.baseURL+"/v1/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Host", "api.cerebras.ai")
	req.Header.Set("Content-Type", "application/json")

	return htc.client.Do(req)
}

// MakeStandardRequest makes a standard request through the proxy
func (htc *HTTPTestClient) MakeStandardRequest(host string, path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", htc.baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Host", host)

	return htc.client.Do(req)
}

// PerformanceMetrics tracks performance metrics for testing
type PerformanceMetrics struct {
	Requests       int
	Successes      int
	Errors         int
	TotalLatency   time.Duration
	MinLatency     time.Duration
	MaxLatency     time.Duration
	ErrorBreakdown map[string]int
}

// RecordRequest records a request and its latency
func (pm *PerformanceMetrics) RecordRequest(success bool, latency time.Duration, errorType string) {
	pm.Requests++
	pm.TotalLatency += latency

	if latency < pm.MinLatency || pm.MinLatency == 0 {
		pm.MinLatency = latency
	}

	if latency > pm.MaxLatency {
		pm.MaxLatency = latency
	}

	if success {
		pm.Successes++
	} else {
		pm.Errors++
		if pm.ErrorBreakdown == nil {
			pm.ErrorBreakdown = make(map[string]int)
		}
		pm.ErrorBreakdown[errorType]++
	}
}

// GetAverageLatency returns the average latency
func (pm *PerformanceMetrics) GetAverageLatency() time.Duration {
	if pm.Requests == 0 {
		return 0
	}
	return pm.TotalLatency / time.Duration(pm.Requests)
}

// GetSuccessRate returns the success rate as a percentage
func (pm *PerformanceMetrics) GetSuccessRate() float64 {
	if pm.Requests == 0 {
		return 0
	}
	return float64(pm.Successes) / float64(pm.Requests) * 100
}

// AssertPerformance validates performance against expected thresholds
func (pm *PerformanceMetrics) AssertPerformance(t *testing.T, expectedMinSuccessRate float64, expectedMaxLatency time.Duration) {
	successRate := pm.GetSuccessRate()
	avgLatency := pm.GetAverageLatency()

	t.Logf("Performance: %d requests, %.2f%% success rate, avg latency %v",
		pm.Requests, successRate, avgLatency)

	if successRate < expectedMinSuccessRate {
		t.Errorf("Success rate %.2f%% is below expected %.2f%%",
			successRate, expectedMinSuccessRate)
	}

	if avgLatency > expectedMaxLatency {
		t.Errorf("Average latency %v is above expected %v",
			avgLatency, expectedMaxLatency)
	}
}

// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, message string) bool {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-time.After(timeout):
			t.Errorf("Timeout waiting for condition: %s", message)
			return false
		case <-ticker.C:
			if condition() {
				return true
			}
			if time.Now().After(deadline) {
				t.Errorf("Timeout waiting for condition: %s", message)
				return false
			}
		}
	}
}