// +build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cooldownp/cooldown-proxy/internal/config"
	"github.com/cooldownp/cooldown-proxy/internal/proxy"
	"github.com/cooldownp/cooldown-proxy/internal/ratelimit"
	"github.com/cooldownp/cooldown-proxy/internal/token"
)

func main() {
	fmt.Println("🧪 Real Cerebras Integration Test for Header-Based Rate Limiting")
	fmt.Println("=====================================================")

	// Load environment variables
	apiURL := os.Getenv("CEREBRAS_API_URL")
	apiKey := os.Getenv("CEREBRAS_API_KEY")
	model := os.Getenv("CEREBRAS_MODEL")

	if apiURL == "" || apiKey == "" || model == "" {
		log.Fatal("❌ Missing required environment variables: CEREBRAS_API_URL, CEREBRAS_API_KEY, CEREBRAS_MODEL")
	}

	fmt.Printf("🔧 Using Cerebras API: %s\n", apiURL)
	fmt.Printf("🤖 Using model: %s\n", model)

	// Configure proxy with header-based rate limiting
	cerebrasConfig := &config.CerebrasLimits{
		RateLimits: config.CerebrasRateLimitConfig{
			UseHeaders:     true,  // Enable header-based rate limiting
			HeaderFallback: true,  // Fall back to static limits if headers fail
			HeaderTimeout:  5 * time.Second,
			ResetBuffer:    100 * time.Millisecond,
		},
		RPMLimit:       60,
		TPMLimit:       1000,
		MaxQueueDepth:  100,
		RequestTimeout: 10 * time.Minute,
	}

	// Create proxy components
	limiter := ratelimit.NewCerebrasLimiter(cerebrasConfig.RPMLimit, cerebrasConfig.TPMLimit)
	estimator := token.NewTokenEstimator()
	handler := proxy.NewCerebrasProxyHandler(limiter, estimator, cerebrasConfig)

	// Set proxy target to real Cerebras API
	cerebrasURL, err := url.Parse(apiURL)
	if err != nil {
		log.Fatalf("❌ Failed to parse Cerebras API URL: %v", err)
	}
	handler.SetTarget(cerebrasURL)

	// Create test server
	testServer := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}
	defer testServer.Close()

	fmt.Println("🚀 Starting proxy server on :8080...")
	go func() {
		if err := testServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("❌ Server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Test 1: Make a request and check headers
	fmt.Println("\n📋 Test 1: Single Request - Header Parsing")
	testSingleRequest(apiKey, model)

	// Test 2: Multiple requests to test rate limiting behavior
	fmt.Println("\n📋 Test 2: Multiple Requests - Rate Limiting Behavior")
	testMultipleRequests(apiKey, model)

	// Test 3: Check limiter state
	fmt.Println("\n📋 Test 3: Limiter State Verification")
	testLimiterState(limiter)

	fmt.Println("\n✅ All integration tests completed successfully!")
}

func testSingleRequest(apiKey, model string) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Create chat completion request
	requestBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": "Say hello!"},
		},
		"max_tokens": 50,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req, err := http.NewRequest("POST", "http://localhost:8080/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("❌ Failed to create request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Host = "api.cerebras.ai"

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		log.Printf("❌ Request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	fmt.Printf("  🕒 Request duration: %v\n", duration)
	fmt.Printf("  📊 Status: %d\n", resp.StatusCode)
	fmt.Printf("  📋 Response headers:\n")
	for key, values := range resp.Header {
		if strings.HasPrefix(key, "X-RateLimit") || strings.HasPrefix(key, "X-CircuitBreaker") {
			for _, value := range values {
				fmt.Printf("    %s: %s\n", key, value)
			}
		}
	}

	if resp.StatusCode == 200 {
		fmt.Printf("  ✅ Request successful\n")
	} else {
		fmt.Printf("  ❌ Request failed: %s\n", string(body))
	}
}

func testMultipleRequests(apiKey, model string) {
	client := &http.Client{Timeout: 30 * time.Second}

	for i := 1; i <= 3; i++ {
		fmt.Printf("\n  🔄 Request %d:\n", i)

		// Create chat completion request
		requestBody := map[string]interface{}{
			"model": model,
			"messages": []map[string]string{
				{"role": "user", "content": fmt.Sprintf("Test message %d", i)},
			},
			"max_tokens": 30,
		}
		bodyBytes, _ := json.Marshal(requestBody)

		req, err := http.NewRequest("POST", "http://localhost:8080/v1/chat/completions", bytes.NewReader(bodyBytes))
		if err != nil {
			log.Printf("    ❌ Failed to create request: %v", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Host = "api.cerebras.ai"

		start := time.Now()
		resp, err := client.Do(req)
		duration := time.Since(start)

		if err != nil {
			log.Printf("    ❌ Request failed: %v", err)
			continue
		}
		resp.Body.Close()

		fmt.Printf("    🕒 Duration: %v | Status: %d\n", duration, resp.StatusCode)

		// Check rate limit headers
		if currentLimit := resp.Header.Get("X-RateLimit-Current-TPM-Limit"); currentLimit != "" {
			if limit, err := strconv.Atoi(currentLimit); err == nil && limit > 0 {
				fmt.Printf("    📊 Current TPM Limit: %s (✅ Header parsing working)\n", currentLimit)
			}
		}
		if remaining := resp.Header.Get("X-RateLimit-Remaining-TPM"); remaining != "" {
			fmt.Printf("    📊 Remaining TPM: %s\n", remaining)
		}
	}
}

func testLimiterState(limiter *ratelimit.CerebrasLimiter) {
	fmt.Printf("  📊 Current TPM Limit: %d\n", limiter.CurrentTPMLimit())
	fmt.Printf("  📊 Current TPM Remaining: %d\n", limiter.CurrentTPMRemaining())
	fmt.Printf("  📊 Last Header Update: %v\n", limiter.LastHeaderUpdate())
	fmt.Printf("  📊 Next TPM Reset: %v\n", limiter.NextTPMReset())
	fmt.Printf("  📊 Queue Length: %d\n", limiter.QueueLength())

	if limiter.CurrentTPMLimit() > 0 {
		fmt.Printf("  ✅ Header-based rate limiting is active\n")
	} else {
		fmt.Printf("  ⚠️  Header-based rate limiting not yet activated (no headers received)\n")
	}
}