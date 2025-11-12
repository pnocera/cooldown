package loadtest

import (
	"time"
)

// PredefinedLoadTestScenarios contains common test scenarios
var PredefinedLoadTestScenarios = map[string]LoadTestConfig{
	"light_load": {
		ConcurrentClients: 10,
		Duration:          2 * time.Minute,
		RequestRate:       30, // 30 requests per second
		Endpoint:          "http://localhost:8080",
		Timeout:           30 * time.Second,
		TestData: []TestData{
			{
				Model:     "llama3.1-8b",
				Prompt:    "Hello, how are you today?",
				MaxTokens: 100,
			},
		},
	},
	"moderate_load": {
		ConcurrentClients: 50,
		Duration:          5 * time.Minute,
		RequestRate:       200, // 200 requests per second
		Endpoint:          "http://localhost:8080",
		Timeout:           30 * time.Second,
		TestData: []TestData{
			{
				Model:     "llama3.1-8b",
				Prompt:    "Explain the concept of machine learning in simple terms.",
				MaxTokens: 500,
			},
			{
				Model:     "llama3.1-70b",
				Prompt:    "Write a short poem about technology.",
				MaxTokens: 200,
			},
		},
	},
	"heavy_load": {
		ConcurrentClients: 200,
		Duration:          10 * time.Minute,
		RequestRate:       1000, // 1000 requests per second
		Endpoint:          "http://localhost:8080",
		Timeout:           30 * time.Second,
		TestData: []TestData{
			{
				Model:     "llama3.1-8b",
				Prompt:    "Generate a comprehensive summary of artificial intelligence research from the past decade.",
				MaxTokens: 1000,
			},
			{
				Model:     "llama3.1-70b",
				Prompt:    "Create a detailed business plan for a startup in the renewable energy sector.",
				MaxTokens: 2000,
			},
		},
	},
	"burst_test": {
		ConcurrentClients: 100,
		Duration:          3 * time.Minute,
		RequestRate:       2000, // Burst: 2000 requests per second
		Endpoint:          "http://localhost:8080",
		Timeout:           60 * time.Second, // Longer timeout for burst scenarios
		TestData: []TestData{
			{
				Model:     "llama3.1-8b",
				Prompt:    "Quick response: What is the capital of France?",
				MaxTokens: 50,
			},
		},
	},
	"rate_limit_stress": {
		ConcurrentClients: 150,
		Duration:          15 * time.Minute,
		RequestRate:       1500, // Above typical rate limits
		Endpoint:          "http://localhost:8080",
		Timeout:           45 * time.Second,
		TestData: []TestData{
			{
				Model:     "llama3.1-8b",
				Prompt:    "Test message for rate limiting.",
				MaxTokens: 100,
			},
		},
	},
	"circuit_breaker_test": {
		ConcurrentClients: 50,
		Duration:          5 * time.Minute,
		RequestRate:       300,
		Endpoint:          "http://localhost:8080",
		Timeout:           10 * time.Second, // Short timeout to trigger timeouts
		TestData: []TestData{
			{
				Model:     "llama3.1-8b",
				Prompt:    "This request should help test circuit breaker behavior.",
				MaxTokens: 150,
			},
		},
	},
}

// GetScenario returns a predefined test scenario
func GetScenario(name string) (LoadTestConfig, bool) {
	scenario, exists := PredefinedLoadTestScenarios[name]
	return scenario, exists
}

// ListScenarios returns all available scenario names
func ListScenarios() []string {
	var scenarios []string
	for name := range PredefinedLoadTestScenarios {
		scenarios = append(scenarios, name)
	}
	return scenarios
}

// CustomScenario allows creating custom test configurations
func CustomScenario(clients int, duration time.Duration, requestRate int, endpoint string) LoadTestConfig {
	return LoadTestConfig{
		ConcurrentClients: clients,
		Duration:          duration,
		RequestRate:       requestRate,
		Endpoint:          endpoint,
		Timeout:           30 * time.Second,
		TestData: []TestData{
			{
				Model:     "llama3.1-8b",
				Prompt:    "Custom test prompt for load testing.",
				MaxTokens: 100,
			},
		},
	}
}
