package ratelimit

import (
	"github.com/cooldownp/cooldown-proxy/internal/config"
	"testing"
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
