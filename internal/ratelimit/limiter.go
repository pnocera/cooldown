package ratelimit

import (
	"github.com/cooldownp/cooldown-proxy/internal/config"
	"strings"
	"sync"
	"time"
)

type LeakyBucket struct {
	rate         float64         // requests per second
	capacity     int             // bucket capacity
	tokens       int             // current tokens
	lastLeak     time.Time       // last time tokens leaked
	mu           sync.Mutex      // mutex for thread safety

	// Metrics
	totalRequests   int64         // total requests processed
	delayedRequests int64         // requests that were delayed
	lastAccess      time.Time     // last access time
}

type Metrics struct {
	TotalRequests   int64     `json:"total_requests"`
	DelayedRequests int64     `json:"delayed_requests"`
	CurrentTokens   int       `json:"current_tokens"`
	LastAccess      time.Time `json:"last_access"`
	DelayRate       float64   `json:"delay_rate"` // percentage of delayed requests
}

type Limiter struct {
	buckets       map[string]*LeakyBucket
	defaultBucket *LeakyBucket
	mu            sync.RWMutex
}

func New(rules []config.RateLimitRule) *Limiter {
	buckets := make(map[string]*LeakyBucket)

	// Default rate limit: 1 request per second
	defaultBucket := &LeakyBucket{
		rate:     1.0,
		capacity: 1,
		tokens:   1,
		lastLeak: time.Now(),
	}

	for _, rule := range rules {
		rate := float64(rule.RequestsPerSecond)
		if rate <= 0 {
			rate = 1.0
		}

		// Capacity allows for burst of up to 2 seconds worth of requests
		capacity := int(rate * 2)
		if capacity < 1 {
			capacity = 1
		}

		buckets[rule.Domain] = &LeakyBucket{
			rate:     rate,
			capacity: capacity,
			tokens:   capacity,
			lastLeak: time.Now(),
		}
	}

	return &Limiter{
		buckets:       buckets,
		defaultBucket: defaultBucket,
	}
}

func (l *Limiter) GetDelay(domain string) time.Duration {
	bucket := l.getBucket(domain)
	if bucket == nil {
		bucket = l.defaultBucket
	}

	return bucket.getDelay()
}

func (l *Limiter) getBucket(domain string) *LeakyBucket {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Exact match first
	if bucket, exists := l.buckets[domain]; exists {
		return bucket
	}

	// Wildcard matching
	for pattern, bucket := range l.buckets {
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*.")
			if strings.HasSuffix(domain, suffix) {
				return bucket
			}
		}
	}

	return nil
}

func (lb *LeakyBucket) getDelay() time.Duration {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	now := time.Now()
	lb.lastAccess = now
	lb.totalRequests++

	// Leak tokens based on time elapsed
	elapsed := now.Sub(lb.lastLeak).Seconds()
	tokensToLeak := int(elapsed * lb.rate)

	if tokensToLeak > 0 {
		lb.tokens = max(0, lb.tokens-tokensToLeak)
		lb.lastLeak = now
	}

	// If we have tokens available, consume one immediately
	if lb.tokens > 0 {
		lb.tokens--
		return 0
	}

	// No tokens available, calculate delay needed for next token
	// The leaky bucket leaks at a constant rate, so we need to wait
	// until enough time has passed for one token to leak
	delayDuration := time.Duration(float64(time.Second) / lb.rate)
	lb.delayedRequests++

	return delayDuration
}

// GetMetrics returns metrics for a specific domain
func (l *Limiter) GetMetrics(domain string) Metrics {
	bucket := l.getBucket(domain)
	if bucket == nil {
		bucket = l.defaultBucket
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	var delayRate float64
	if bucket.totalRequests > 0 {
		delayRate = float64(bucket.delayedRequests) / float64(bucket.totalRequests) * 100
	}

	return Metrics{
		TotalRequests:   bucket.totalRequests,
		DelayedRequests: bucket.delayedRequests,
		CurrentTokens:   bucket.tokens,
		LastAccess:      bucket.lastAccess,
		DelayRate:       delayRate,
	}
}

// GetAllMetrics returns metrics for all domains
func (l *Limiter) GetAllMetrics() map[string]Metrics {
	l.mu.RLock()
	defer l.mu.RUnlock()

	metrics := make(map[string]Metrics)

	// Add metrics for all configured buckets
	for domain, bucket := range l.buckets {
		bucket.mu.Lock()
		var delayRate float64
		if bucket.totalRequests > 0 {
			delayRate = float64(bucket.delayedRequests) / float64(bucket.totalRequests) * 100
		}

		metrics[domain] = Metrics{
			TotalRequests:   bucket.totalRequests,
			DelayedRequests: bucket.delayedRequests,
			CurrentTokens:   bucket.tokens,
			LastAccess:      bucket.lastAccess,
			DelayRate:       delayRate,
		}
		bucket.mu.Unlock()
	}

	// Add default bucket metrics
	l.defaultBucket.mu.Lock()
	var delayRate float64
	if l.defaultBucket.totalRequests > 0 {
		delayRate = float64(l.defaultBucket.delayedRequests) / float64(l.defaultBucket.totalRequests) * 100
	}

	metrics["default"] = Metrics{
		TotalRequests:   l.defaultBucket.totalRequests,
		DelayedRequests: l.defaultBucket.delayedRequests,
		CurrentTokens:   l.defaultBucket.tokens,
		LastAccess:      l.defaultBucket.lastAccess,
		DelayRate:       delayRate,
	}
	l.defaultBucket.mu.Unlock()

	return metrics
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
