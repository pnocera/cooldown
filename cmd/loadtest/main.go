package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/loadtest"
)

func main() {
	var (
		scenario      = flag.String("scenario", "", "Load test scenario to run (light_load, moderate_load, heavy_load, burst_test, rate_limit_stress, circuit_breaker_test)")
		clients       = flag.Int("clients", 0, "Number of concurrent clients (overrides scenario)")
		duration      = flag.Duration("duration", 0, "Test duration (overrides scenario)")
		requestRate   = flag.Int("rate", 0, "Requests per second (overrides scenario)")
		endpoint      = flag.String("endpoint", "http://localhost:8080", "Target endpoint")
		outputFile    = flag.String("output", "", "Output file for results (JSON format)")
		listScenarios = flag.Bool("list", false, "List available scenarios")
	)
	flag.Parse()

	if *listScenarios {
		listAvailableScenarios()
		return
	}

	var config loadtest.LoadTestConfig

	if *scenario != "" {
		// Use predefined scenario
		scenarioConfig, exists := loadtest.GetScenario(*scenario)
		if !exists {
			fmt.Fprintf(os.Stderr, "Error: Scenario '%s' not found\n", *scenario)
			fmt.Fprintf(os.Stderr, "Available scenarios: %s\n", strings.Join(loadtest.ListScenarios(), ", "))
			os.Exit(1)
		}
		config = scenarioConfig
	} else {
		// Use custom scenario
		defaultDuration := 2 * time.Minute
		if *duration == 0 {
			*duration = defaultDuration
		}

		config = loadtest.CustomScenario(*clients, *duration, *requestRate, *endpoint)
	}

	// Override config with command line parameters
	if *clients > 0 {
		config.ConcurrentClients = *clients
	}
	if *duration > 0 {
		config.Duration = *duration
	}
	if *requestRate > 0 {
		config.RequestRate = *requestRate
	}
	if *endpoint != "" {
		config.Endpoint = *endpoint
	}

	// Validate configuration
	if config.ConcurrentClients <= 0 {
		fmt.Fprintf(os.Stderr, "Error: Number of clients must be positive\n")
		os.Exit(1)
	}

	if config.RequestRate <= 0 {
		fmt.Fprintf(os.Stderr, "Error: Request rate must be positive\n")
		os.Exit(1)
	}

	if config.Duration <= 0 {
		fmt.Fprintf(os.Stderr, "Error: Duration must be positive\n")
		os.Exit(1)
	}

	fmt.Printf("Starting load test with the following configuration:\n")
	fmt.Printf("  Concurrent Clients: %d\n", config.ConcurrentClients)
	fmt.Printf("  Duration: %v\n", config.Duration)
	fmt.Printf("  Request Rate: %d req/s\n", config.RequestRate)
	fmt.Printf("  Endpoint: %s\n", config.Endpoint)
	fmt.Printf("  Timeout: %v\n", config.Timeout)
	fmt.Printf("\n")

	// Run the load test
	fmt.Println("Running load test...")
	runner := loadtest.NewLoadTestRunner(config)
	result := runner.Run()

	// Display results
	displayResults(result)

	// Save results to file if specified
	if *outputFile != "" {
		saveResults(result, *outputFile)
	}
}

func listAvailableScenarios() {
	fmt.Println("Available load test scenarios:")
	scenarios := loadtest.ListScenarios()
	for _, scenario := range scenarios {
		config, _ := loadtest.GetScenario(scenario)
		fmt.Printf("  %s:\n", scenario)
		fmt.Printf("    Clients: %d\n", config.ConcurrentClients)
		fmt.Printf("    Duration: %v\n", config.Duration)
		fmt.Printf("    Rate: %d req/s\n", config.RequestRate)
		fmt.Printf("    Description: %s\n", getScenarioDescription(scenario))
		fmt.Println()
	}
}

func getScenarioDescription(scenario string) string {
	descriptions := map[string]string{
		"light_load":           "Light load test for basic functionality",
		"moderate_load":        "Moderate load with mixed request types",
		"heavy_load":           "Heavy load test for performance limits",
		"burst_test":           "Burst traffic pattern test",
		"rate_limit_stress":    "Stress test for rate limiting functionality",
		"circuit_breaker_test": "Test circuit breaker behavior",
	}

	if desc, exists := descriptions[scenario]; exists {
		return desc
	}
	return "Custom load test scenario"
}

func displayResults(result loadtest.LoadTestResult) {
	fmt.Println("\n=== Load Test Results ===")
	fmt.Printf("Total Requests: %d\n", result.TotalRequests)
	fmt.Printf("Successful Requests: %d (%.2f%%)\n", result.SuccessfulRequests,
		float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
	fmt.Printf("Failed Requests: %d (%.2f%%)\n", result.FailedRequests,
		float64(result.FailedRequests)/float64(result.TotalRequests)*100)
	fmt.Printf("Test Duration: %v\n", result.Duration)
	fmt.Printf("Average Latency: %v\n", result.AverageLatency)
	fmt.Printf("Min Latency: %v\n", result.MinLatency)
	fmt.Printf("Max Latency: %v\n", result.MaxLatency)
	fmt.Printf("Requests per Second: %.2f\n", result.RequestsPerSecond)

	fmt.Println("\nCircuit Breaker Statistics:")
	fmt.Printf("  State Changes: %d\n", result.CircuitBreakerStats.StateChanges)
	fmt.Printf("  Circuit Opens: %d\n", result.CircuitBreakerStats.CircuitOpens)
	fmt.Printf("  Circuit Closes: %d\n", result.CircuitBreakerStats.CircuitCloses)
	fmt.Printf("  Rejected Requests: %d\n", result.CircuitBreakerStats.RejectedRequests)

	if len(result.ErrorBreakdown) > 0 {
		fmt.Println("\nError Breakdown:")
		for errorType, count := range result.ErrorBreakdown {
			fmt.Printf("  %s: %d\n", errorType, count)
		}
	}

	// Performance assessment
	fmt.Println("\n=== Performance Assessment ===")
	if result.RequestsPerSecond >= 1000 {
		fmt.Println("✅ Excellent performance (>1000 RPS)")
	} else if result.RequestsPerSecond >= 500 {
		fmt.Println("✅ Good performance (500-1000 RPS)")
	} else if result.RequestsPerSecond >= 100 {
		fmt.Println("⚠️  Moderate performance (100-500 RPS)")
	} else {
		fmt.Println("❌ Poor performance (<100 RPS)")
	}

	successRate := float64(result.SuccessfulRequests) / float64(result.TotalRequests) * 100
	if successRate >= 99 {
		fmt.Println("✅ Excellent success rate (≥99%)")
	} else if successRate >= 95 {
		fmt.Println("✅ Good success rate (95-99%)")
	} else if successRate >= 90 {
		fmt.Println("⚠️  Moderate success rate (90-95%)")
	} else {
		fmt.Println("❌ Poor success rate (<90%)")
	}

	if result.AverageLatency <= 100*time.Millisecond {
		fmt.Println("✅ Excellent latency (≤100ms)")
	} else if result.AverageLatency <= 500*time.Millisecond {
		fmt.Println("✅ Good latency (100-500ms)")
	} else if result.AverageLatency <= 1000*time.Millisecond {
		fmt.Println("⚠️  Moderate latency (500ms-1s)")
	} else {
		fmt.Println("❌ High latency (>1s)")
	}
}

func saveResults(result loadtest.LoadTestResult, filename string) {
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("Error marshaling results: %v", err)
		return
	}

	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		log.Printf("Error saving results to file: %v", err)
		return
	}

	fmt.Printf("\nResults saved to: %s\n", filename)
}
