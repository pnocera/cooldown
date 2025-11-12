package ratelimit

import (
	"testing"
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
