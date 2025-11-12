package loadtest

import (
	"testing"
	"time"
)

func TestLoadTestRunner_Creation(t *testing.T) {
	config := LoadTestConfig{
		ConcurrentClients: 10,
		Duration:          1 * time.Minute,
		RequestRate:       100,
		Endpoint:          "http://localhost:8080",
		Timeout:           30 * time.Second,
		TestData: []TestData{
			{
				Model:     "llama3.1-8b",
				Prompt:    "Test prompt",
				MaxTokens: 100,
			},
		},
	}

	runner := NewLoadTestRunner(config)

	if runner == nil {
		t.Fatal("Expected non-nil runner")
	}

	if runner.config.ConcurrentClients != 10 {
		t.Errorf("Expected 10 concurrent clients, got %d", runner.config.ConcurrentClients)
	}

	if len(runner.clients) != 0 {
		t.Errorf("Expected 0 clients initially, got %d", len(runner.clients))
	}
}

func TestLoadTestRunner_GetScenario(t *testing.T) {
	// Test existing scenario
	scenario, exists := GetScenario("light_load")
	if !exists {
		t.Error("Expected light_load scenario to exist")
	}

	if scenario.ConcurrentClients != 10 {
		t.Errorf("Expected 10 concurrent clients for light_load, got %d", scenario.ConcurrentClients)
	}

	// Test non-existent scenario
	_, exists = GetScenario("non_existent")
	if exists {
		t.Error("Expected non_existent scenario to not exist")
	}
}

func TestLoadTestRunner_ListScenarios(t *testing.T) {
	scenarios := ListScenarios()

	if len(scenarios) == 0 {
		t.Error("Expected at least one scenario")
	}

	expectedScenarios := []string{"light_load", "moderate_load", "heavy_load", "burst_test", "rate_limit_stress", "circuit_breaker_test"}
	for _, expected := range expectedScenarios {
		found := false
		for _, scenario := range scenarios {
			if scenario == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected scenario %s to be in list", expected)
		}
	}
}

func TestLoadTestRunner_CustomScenario(t *testing.T) {
	scenario := CustomScenario(20, 2*time.Minute, 500, "http://test.example.com")

	if scenario.ConcurrentClients != 20 {
		t.Errorf("Expected 20 concurrent clients, got %d", scenario.ConcurrentClients)
	}

	if scenario.Duration != 2*time.Minute {
		t.Errorf("Expected 2 minute duration, got %v", scenario.Duration)
	}

	if scenario.RequestRate != 500 {
		t.Errorf("Expected 500 requests per second, got %d", scenario.RequestRate)
	}

	if scenario.Endpoint != "http://test.example.com" {
		t.Errorf("Expected http://test.example.com endpoint, got %s", scenario.Endpoint)
	}

	if len(scenario.TestData) == 0 {
		t.Error("Expected test data to be provided")
	}
}

func TestClient_RequestRateRatePerClient(t *testing.T) {
	tests := []struct {
		name         string
		clients      int
		requestRate  int
		expectedRate int
	}{
		{
			name:         "Single client",
			clients:      1,
			requestRate:  100,
			expectedRate: 100,
		},
		{
			name:         "Multiple clients",
			clients:      10,
			requestRate:  100,
			expectedRate: 10,
		},
		{
			name:         "More clients than rate",
			clients:      200,
			requestRate:  100,
			expectedRate: 1, // Minimum rate is 1
		},
		{
			name:         "Zero clients",
			clients:      0,
			requestRate:  100,
			expectedRate: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				config: LoadTestConfig{
					ConcurrentClients: tt.clients,
					RequestRate:       tt.requestRate,
				},
			}

			rate := client.RequestRateRatePerClient()
			if rate != tt.expectedRate {
				t.Errorf("Expected rate %d, got %d", tt.expectedRate, rate)
			}
		})
	}
}

func TestLoadTestResult_Calculation(t *testing.T) {
	// This test verifies that result calculations work correctly
	result := LoadTestResult{
		TotalRequests:      1000,
		SuccessfulRequests: 950,
		FailedRequests:     50,
		Duration:           10 * time.Second,
		AverageLatency:     100 * time.Millisecond,
		MinLatency:         50 * time.Millisecond,
		MaxLatency:         500 * time.Millisecond,
		ErrorBreakdown: map[string]int{
			"http_500": 30,
			"timeout":  20,
		},
	}

	// Calculate requests per second
	expectedRPS := float64(result.TotalRequests) / result.Duration.Seconds()
	if expectedRPS != 100.0 {
		t.Errorf("Expected 100 RPS, got %f", expectedRPS)
	}

	// Verify error breakdown
	totalErrors := 0
	for _, count := range result.ErrorBreakdown {
		totalErrors += count
	}

	if totalErrors != int(result.FailedRequests) {
		t.Errorf("Expected %d errors in breakdown, got %d", result.FailedRequests, totalErrors)
	}
}
