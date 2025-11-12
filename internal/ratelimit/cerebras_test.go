package ratelimit

import (
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCerebrasRateLimiter_Creation(t *testing.T) {
	limiter := NewCerebrasLimiter(1000, 1000000)

	if limiter == nil {
		t.Fatal("Expected non-nil limiter")
	}

	if limiter.RPMLimit() != 1000 {
		t.Errorf("Expected RPM limit 1000, got %d", limiter.RPMLimit())
	}

	if limiter.TPMLimit() != 1000000 {
		t.Errorf("Expected TPM limit 1000000, got %d", limiter.TPMLimit())
	}
}

func TestCerebrasRateLimiter_SlidingWindowRPM(t *testing.T) {
	limiter := NewCerebrasLimiter(2, 1000000) // 2 RPM for testing

	// First request should be allowed
	if delay := limiter.CheckRequest(100); delay > 0 {
		t.Errorf("First request should be allowed, got delay %v", delay)
	}

	// Second request should be allowed
	if delay := limiter.CheckRequest(100); delay > 0 {
		t.Errorf("Second request should be allowed, got delay %v", delay)
	}

	// Third request should be delayed (over RPM limit)
	if delay := limiter.CheckRequest(100); delay == 0 {
		t.Error("Third request should be delayed due to RPM limit")
	}
}

func TestCerebrasRateLimiter_TPMLimit(t *testing.T) {
	limiter := NewCerebrasLimiter(1000, 1000) // 1000 TPM for testing

	// First request with 600 tokens should be allowed
	if delay := limiter.CheckRequest(600); delay > 0 {
		t.Errorf("First 600-token request should be allowed, got delay %v", delay)
	}

	// Second request with 600 tokens should be delayed (over TPM limit)
	if delay := limiter.CheckRequest(600); delay == 0 {
		t.Error("Second 600-token request should be delayed due to TPM limit")
	}
}

func TestCerebrasRateLimiter_QueueIntegration(t *testing.T) {
	limiter := NewCerebrasLimiter(1, 1000) // Very low limits for testing

	// First request should be immediate
	delay := limiter.CheckRequestWithQueue("req-1", 100)
	if delay > 0 {
		t.Errorf("First request should be immediate, got delay %v", delay)
	}

	// Second request should be queued
	delay = limiter.CheckRequestWithQueue("req-2", 100)
	if delay == 0 {
		t.Error("Second request should be queued/delayed")
	}

	// Check queue length
	if limiter.QueueLength() != 1 {
		t.Errorf("Expected queue length 1, got %d", limiter.QueueLength())
	}
}

func TestCerebrasLimiterHeaderBasedState(t *testing.T) {
	limiter := NewCerebrasLimiter(60, 1000)

	// Test initial state
	assert.Equal(t, 0, limiter.currentTPMLimit)
	assert.Equal(t, 0, limiter.currentTPMRemaining)
	assert.True(t, limiter.lastHeaderUpdate.IsZero())

	// Update from headers
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-tokens-minute", "1000")
	headers.Set("x-ratelimit-remaining-tokens-minute", "800")
	headers.Set("x-ratelimit-reset-tokens-minute", "45.5")

	err := limiter.UpdateFromHeaders(headers)
	assert.NoError(t, err)

	assert.Equal(t, 1000, limiter.currentTPMLimit)
	assert.Equal(t, 800, limiter.currentTPMRemaining)
	assert.False(t, limiter.lastHeaderUpdate.IsZero())
	assert.True(t, time.Now().Add(45*time.Second).Before(limiter.nextTPMReset))
	assert.True(t, time.Now().Add(46*time.Second).After(limiter.nextTPMReset))
}

func TestCerebrasLimiterConcurrentHeaderUpdates(t *testing.T) {
	limiter := NewCerebrasLimiter(60, 1000)

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()

			headers := http.Header{}
			headers.Set("x-ratelimit-limit-tokens-minute", strconv.Itoa(1000+iteration))
			headers.Set("x-ratelimit-remaining-tokens-minute", "800")
			headers.Set("x-ratelimit-reset-tokens-minute", "45.5")

			_ = limiter.UpdateFromHeaders(headers)
		}(i)
	}

	wg.Wait()

	limiter.mu.RLock()
	defer limiter.mu.RUnlock()

	// Should have updated to some valid value
	assert.True(t, limiter.currentTPMLimit > 0)
	assert.False(t, limiter.lastHeaderUpdate.IsZero())
}

func TestCerebrasLimiterDynamicRateLimiting(t *testing.T) {
	limiter := NewCerebrasLimiter(60, 1000)

	// Set up header state
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-tokens-minute", "1000")
	headers.Set("x-ratelimit-remaining-tokens-minute", "50")
	headers.Set("x-ratelimit-reset-tokens-minute", "10")

	err := limiter.UpdateFromHeaders(headers)
	assert.NoError(t, err)

	// Request that would exceed remaining tokens
	requestID := "test-request-1"
	tokens := 100
	delay := limiter.CheckRequestWithDynamicQueue(requestID, tokens)

	// Should wait for reset since not enough tokens
	assert.True(t, delay > 5*time.Second)
	assert.True(t, delay < 15*time.Second)
}

func TestCerebrasLimiterDynamicFallback(t *testing.T) {
	limiter := NewCerebrasLimiter(60, 1000)

	// No header data, should fall back to static limits
	requestID := "test-request-1"
	tokens := 100
	delay := limiter.CheckRequestWithDynamicQueue(requestID, tokens)

	// Should use static limiting logic
	assert.True(t, delay >= 0)
}

func TestCerebrasLimiterDynamicEdgeCases(t *testing.T) {
	limiter := NewCerebrasLimiter(60, 1000)

	t.Run("stale header data fallback", func(t *testing.T) {
		// Set up old header data
		headers := http.Header{}
		headers.Set("x-ratelimit-limit-tokens-minute", "1000")
		headers.Set("x-ratelimit-remaining-tokens-minute", "50")
		headers.Set("x-ratelimit-reset-tokens-minute", "10")

		err := limiter.UpdateFromHeaders(headers)
		assert.NoError(t, err)

		// Simulate time passing (6 minutes)
		limiter.mu.Lock()
		limiter.lastHeaderUpdate = time.Now().Add(-6 * time.Minute)
		limiter.mu.Unlock()

		requestID := "test-request"
		tokens := 100
		delay := limiter.CheckRequestWithDynamicQueue(requestID, tokens)

		// Should fall back to static limiting due to stale data
		assert.True(t, delay >= 0)
	})

	t.Run("reset time passed", func(t *testing.T) {
		// Set up header data with reset time in the past
		headers := http.Header{}
		headers.Set("x-ratelimit-limit-tokens-minute", "1000")
		headers.Set("x-ratelimit-remaining-tokens-minute", "0")
		headers.Set("x-ratelimit-reset-tokens-minute", "1")

		err := limiter.UpdateFromHeaders(headers)
		assert.NoError(t, err)

		// Simulate reset time passing
		limiter.mu.Lock()
		limiter.nextTPMReset = time.Now().Add(-1 * time.Second)
		limiter.mu.Unlock()

		requestID := "test-request"
		tokens := 50
		delay := limiter.CheckRequestWithDynamicQueue(requestID, tokens)

		// Should process immediately (conservative estimate applied)
		assert.Equal(t, time.Duration(0), delay)

		limiter.mu.RLock()
		assert.Equal(t, 50, limiter.currentTPMRemaining) // Should have been set to conservative estimate
		limiter.mu.RUnlock()
	})
}
