package ratelimit

import (
	"github.com/cooldownp/cooldown-proxy/internal/config"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	rules := []config.RateLimitRule{
		{Domain: "api.github.com", RequestsPerSecond: 10},
		{Domain: "api.twitter.com", RequestsPerSecond: 5},
	}

	limiter := New(rules)

	// Test domain matching
	delay := limiter.GetDelay("api.github.com")
	if delay < 0 {
		t.Errorf("Expected non-negative delay for api.github.com")
	}

	// Test default delay for unknown domain
	defaultDelay := limiter.GetDelay("unknown.api.com")
	if defaultDelay < 0 {
		t.Errorf("Expected non-negative default delay")
	}
}

func TestLeakyBucketBehavior(t *testing.T) {
	// Create a rate limiter with 5 requests per second (smaller capacity for easier testing)
	rules := []config.RateLimitRule{
		{Domain: "test.com", RequestsPerSecond: 5},
	}

	limiter := New(rules)

	// Should allow first request immediately
	delay1 := limiter.GetDelay("test.com")
	if delay1 != 0 {
		t.Errorf("Expected first request to have no delay, got %v", delay1)
	}

	// Should allow burst requests up to capacity (5 * 2 = 10 tokens capacity)
	for i := 0; i < 9; i++ {
		delay := limiter.GetDelay("test.com")
		if delay != 0 {
			t.Errorf("Expected request %d to have no delay, got %v", i+2, delay)
		}
	}

	// Next request should be delayed (capacity exceeded)
	delay11 := limiter.GetDelay("test.com")
	if delay11 <= 0 {
		t.Errorf("Expected 11th request to be delayed, got %v", delay11)
	}

	// Test that delay is reasonable (should be around 200ms for 5 req/sec)
	if delay11 > 500*time.Millisecond {
		t.Errorf("Expected delay around 200ms, got %v", delay11)
	}
}

func TestConcurrentRateLimiting(t *testing.T) {
	rules := []config.RateLimitRule{
		{Domain: "test.com", RequestsPerSecond: 5},
	}

	limiter := New(rules)
	var wg sync.WaitGroup
	var delays []time.Duration
	var mu sync.Mutex

	// Launch 15 concurrent requests (more than capacity)
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			delay := limiter.GetDelay("test.com")

			mu.Lock()
			delays = append(delays, delay)
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Count how many requests had no delay (should be 10 for burst capacity of 5*2)
	noDelayCount := 0
	for _, delay := range delays {
		if delay == 0 {
			noDelayCount++
		}
	}

	if noDelayCount != 10 {
		t.Errorf("Expected 10 requests to have no delay (burst capacity), got %d", noDelayCount)
	}

	// At least some requests should be delayed
	delayedCount := 0
	for _, delay := range delays {
		if delay > 0 {
			delayedCount++
		}
	}

	if delayedCount == 0 {
		t.Error("Expected some requests to be delayed when exceeding burst capacity")
	}
}

func TestWildcardDomainMatching(t *testing.T) {
	rules := []config.RateLimitRule{
		{Domain: "*.example.com", RequestsPerSecond: 3},
		{Domain: "api.specific.com", RequestsPerSecond: 10},
	}

	limiter := New(rules)

	// Test wildcard matching
	delay1 := limiter.GetDelay("api.example.com")
	if delay1 != 0 {
		t.Errorf("Expected first request to api.example.com to have no delay, got %v", delay1)
	}

	// Test specific domain matching
	delay2 := limiter.GetDelay("api.specific.com")
	if delay2 != 0 {
		t.Errorf("Expected first request to api.specific.com to have no delay, got %v", delay2)
	}

	// Test unknown domain (should use default rate limit)
	delay3 := limiter.GetDelay("unknown.com")
	if delay3 != 0 {
		t.Errorf("Expected first request to unknown.com to have no delay (default limit), got %v", delay3)
	}
}

func TestRateLimitingMetrics(t *testing.T) {
	rules := []config.RateLimitRule{
		{Domain: "test.com", RequestsPerSecond: 2},
	}

	limiter := New(rules)

	// Make some requests to generate metrics
	for i := 0; i < 5; i++ {
		limiter.GetDelay("test.com")
	}

	// Get metrics for test.com
	metrics := limiter.GetMetrics("test.com")

	if metrics.TotalRequests != 5 {
		t.Errorf("Expected 5 total requests, got %d", metrics.TotalRequests)
	}

	if metrics.CurrentTokens < 0 {
		t.Errorf("Expected non-negative current tokens, got %d", metrics.CurrentTokens)
	}

	if metrics.LastAccess.IsZero() {
		t.Error("Expected non-zero last access time")
	}

	// Test default domain metrics
	for i := 0; i < 3; i++ {
		limiter.GetDelay("unknown.com")
	}

	defaultMetrics := limiter.GetMetrics("unknown.com")
	if defaultMetrics.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests for default domain, got %d", defaultMetrics.TotalRequests)
	}
}

func TestRateLimitingDelayRate(t *testing.T) {
	rules := []config.RateLimitRule{
		{Domain: "test.com", RequestsPerSecond: 1}, // Very low rate for testing
	}

	limiter := New(rules)

	// Make requests that will exceed capacity and cause delays
	for i := 0; i < 5; i++ {
		limiter.GetDelay("test.com")
	}

	metrics := limiter.GetMetrics("test.com")

	// Should have some delayed requests due to exceeding burst capacity
	if metrics.DelayedRequests == 0 {
		t.Error("Expected some requests to be delayed")
	}

	// Delay rate should be greater than 0
	if metrics.DelayRate <= 0 {
		t.Errorf("Expected positive delay rate, got %f", metrics.DelayRate)
	}

	// Total requests should equal sum of immediate and delayed requests
	expectedTotal := int64(5)
	if metrics.TotalRequests != expectedTotal {
		t.Errorf("Expected %d total requests, got %d", expectedTotal, metrics.TotalRequests)
	}
}

func TestGetAllMetrics(t *testing.T) {
	rules := []config.RateLimitRule{
		{Domain: "api.example.com", RequestsPerSecond: 5},
		{Domain: "cdn.example.com", RequestsPerSecond: 10},
	}

	limiter := New(rules)

	// Make requests to different domains
	limiter.GetDelay("api.example.com")
	limiter.GetDelay("api.example.com")
	limiter.GetDelay("cdn.example.com")
	limiter.GetDelay("unknown.com") // Should use default bucket

	allMetrics := limiter.GetAllMetrics()

	// Should have metrics for all domains
	if len(allMetrics) != 3 { // 2 configured + 1 default
		t.Errorf("Expected metrics for 3 domains, got %d", len(allMetrics))
	}

	// Check specific domain metrics
	if apiMetrics, exists := allMetrics["api.example.com"]; exists {
		if apiMetrics.TotalRequests != 2 {
			t.Errorf("Expected 2 requests for api.example.com, got %d", apiMetrics.TotalRequests)
		}
	} else {
		t.Error("Expected metrics for api.example.com")
	}

	// Check default domain metrics
	if defaultMetrics, exists := allMetrics["default"]; exists {
		if defaultMetrics.TotalRequests != 1 {
			t.Errorf("Expected 1 request for default domain, got %d", defaultMetrics.TotalRequests)
		}
	} else {
		t.Error("Expected metrics for default domain")
	}
}
