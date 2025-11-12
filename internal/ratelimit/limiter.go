package ratelimit

import (
	"github.com/cooldownp/cooldown-proxy/internal/config"
	"go.uber.org/ratelimit"
	"strings"
	"time"
)

type Limiter struct {
	limiters       map[string]ratelimit.Limiter
	defaultLimiter ratelimit.Limiter
}

func New(rules []config.RateLimitRule) *Limiter {
	limiters := make(map[string]ratelimit.Limiter)

	for _, rule := range rules {
		rate := int(rule.RequestsPerSecond)
		if rate <= 0 {
			rate = 1
		}
		limiters[rule.Domain] = ratelimit.New(rate)
	}

	return &Limiter{
		limiters:       limiters,
		defaultLimiter: ratelimit.New(1), // 1 req/sec default
	}
}

func (l *Limiter) GetDelay(domain string) time.Duration {
	// Find matching limiter
	limiter := l.findLimiter(domain)
	if limiter == nil {
		limiter = l.defaultLimiter
	}

	// Take from rate limiter
	limiter.Take()
	return 0
}

func (l *Limiter) findLimiter(domain string) ratelimit.Limiter {
	// Exact match first
	if limiter, exists := l.limiters[domain]; exists {
		return limiter
	}

	// Wildcard matching
	for pattern, limiter := range l.limiters {
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*.")
			if strings.HasSuffix(domain, suffix) {
				return limiter
			}
		}
	}

	return nil
}
