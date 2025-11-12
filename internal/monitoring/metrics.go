package monitoring

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector collects and tracks performance metrics for header-based rate limiting
type MetricsCollector struct {
	// Request metrics
	TotalRequests         int64
	SuccessRequests       int64
	ErrorRequests         int64
	RateLimitedRequests   int64

	// Header parsing metrics
	HeaderParsingAttempts int64
	HeaderParsingSuccess  int64
	HeaderParsingErrors   int64

	// Latency metrics
	TotalLatency          int64 // Total latency in nanoseconds
	MinLatency            int64 // Min latency in nanoseconds
	MaxLatency            int64 // Max latency in nanoseconds

	// Rate limiting metrics
	DynamicRateLimitHits  int64
	StaticFallbackHits    int64
	QueueOperations       int64

	// Circuit breaker metrics
	CircuitBreakerOpens   int64
	CircuitBreakerCloses  int64

	// Performance metrics
	TPMLimits             []int64 // Recent TPM limit values
	TPMRemaining          []int64 // Recent TPM remaining values

	// Timing
	StartTime             time.Time
	LastUpdateTime        int64 // Unix nanoseconds for atomic operations

	// Thread safety
	mu                    sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		StartTime:  time.Now(),
		MinLatency: int64(^uint64(0) >> 1), // Max int64
		TPMLimits:  make([]int64, 0, 100),
		TPMRemaining: make([]int64, 0, 100),
	}
}

// RecordRequest records a request attempt
func (m *MetricsCollector) RecordRequest(success bool, rateLimited bool, latency time.Duration) {
	atomic.AddInt64(&m.TotalRequests, 1)
	atomic.AddInt64(&m.TotalLatency, latency.Nanoseconds())

	// Update min/max latencies
	latencyNs := latency.Nanoseconds()
	for {
		current := atomic.LoadInt64(&m.MinLatency)
		if latencyNs >= current || atomic.CompareAndSwapInt64(&m.MinLatency, current, latencyNs) {
			break
		}
	}

	for {
		current := atomic.LoadInt64(&m.MaxLatency)
		if latencyNs <= current || atomic.CompareAndSwapInt64(&m.MaxLatency, current, latencyNs) {
			break
		}
	}

	if success {
		atomic.AddInt64(&m.SuccessRequests, 1)
	} else {
		atomic.AddInt64(&m.ErrorRequests, 1)
	}

	if rateLimited {
		atomic.AddInt64(&m.RateLimitedRequests, 1)
	}

	atomic.StoreInt64(&m.LastUpdateTime, time.Now().UnixNano())
}

// RecordHeaderParsing records header parsing results
func (m *MetricsCollector) RecordHeaderParsing(success bool) {
	atomic.AddInt64(&m.HeaderParsingAttempts, 1)
	if success {
		atomic.AddInt64(&m.HeaderParsingSuccess, 1)
	} else {
		atomic.AddInt64(&m.HeaderParsingErrors, 1)
	}
}

// RecordRateLimitType records whether dynamic or static rate limiting was used
func (m *MetricsCollector) RecordRateLimitType(dynamic bool) {
	if dynamic {
		atomic.AddInt64(&m.DynamicRateLimitHits, 1)
	} else {
		atomic.AddInt64(&m.StaticFallbackHits, 1)
	}
}

// RecordQueueOperation records queue operations
func (m *MetricsCollector) RecordQueueOperation() {
	atomic.AddInt64(&m.QueueOperations, 1)
}

// RecordCircuitBreakerState records circuit breaker state changes
func (m *MetricsCollector) RecordCircuitBreakerState(open bool) {
	if open {
		atomic.AddInt64(&m.CircuitBreakerOpens, 1)
	} else {
		atomic.AddInt64(&m.CircuitBreakerCloses, 1)
	}
}

// RecordTPMLimit records current TPM limit and remaining
func (m *MetricsCollector) RecordTPMLimit(limit, remaining int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Keep only last 100 values for memory efficiency
	if len(m.TPMLimits) >= 100 {
		m.TPMLimits = m.TPMLimits[1:]
		m.TPMRemaining = m.TPMRemaining[1:]
	}

	m.TPMLimits = append(m.TPMLimits, limit)
	m.TPMRemaining = append(m.TPMRemaining, remaining)
}

// GetMetrics returns current metrics snapshot
func (m *MetricsCollector) GetMetrics() MetricsSnapshot {
	totalReqs := atomic.LoadInt64(&m.TotalRequests)
	successReqs := atomic.LoadInt64(&m.SuccessRequests)
	errorReqs := atomic.LoadInt64(&m.ErrorRequests)
	rateLimitedReqs := atomic.LoadInt64(&m.RateLimitedRequests)

	headerAttempts := atomic.LoadInt64(&m.HeaderParsingAttempts)
	headerSuccess := atomic.LoadInt64(&m.HeaderParsingSuccess)
	headerErrors := atomic.LoadInt64(&m.HeaderParsingErrors)

	totalLatency := atomic.LoadInt64(&m.TotalLatency)
	minLatency := atomic.LoadInt64(&m.MinLatency)
	maxLatency := atomic.LoadInt64(&m.MaxLatency)

	dynamicHits := atomic.LoadInt64(&m.DynamicRateLimitHits)
	staticHits := atomic.LoadInt64(&m.StaticFallbackHits)
	queueOps := atomic.LoadInt64(&m.QueueOperations)

	cbOpens := atomic.LoadInt64(&m.CircuitBreakerOpens)
	cbCloses := atomic.LoadInt64(&m.CircuitBreakerCloses)

	uptime := time.Since(m.StartTime)
	lastUpdate := time.Unix(0, atomic.LoadInt64(&m.LastUpdateTime))

	m.mu.RLock()
	currentTPMLimit := int64(0)
	currentTPMRemaining := int64(0)
	if len(m.TPMLimits) > 0 {
		currentTPMLimit = m.TPMLimits[len(m.TPMLimits)-1]
		currentTPMRemaining = m.TPMRemaining[len(m.TPMRemaining)-1]
	}
	m.mu.RUnlock()

	return MetricsSnapshot{
		TotalRequests:        totalReqs,
		SuccessRequests:      successReqs,
		ErrorRequests:        errorReqs,
		RateLimitedRequests:  rateLimitedReqs,

		HeaderParsingAttempts: headerAttempts,
		HeaderParsingSuccess:  headerSuccess,
		HeaderParsingErrors:   headerErrors,

		AverageLatency:       time.Duration(totalLatency / max(totalReqs, 1)),
		MinLatency:           time.Duration(minLatency),
		MaxLatency:           time.Duration(maxLatency),

		DynamicRateLimitHits: dynamicHits,
		StaticFallbackHits:   staticHits,
		QueueOperations:      queueOps,

		CircuitBreakerOpens:  cbOpens,
		CircuitBreakerCloses: cbCloses,

		CurrentTPMLimit:      currentTPMLimit,
		CurrentTPMRemaining:  currentTPMRemaining,

		RequestsPerSecond:    float64(totalReqs) / uptime.Seconds(),
		SuccessRate:          float64(successReqs) / float64(max(totalReqs, 1)) * 100,
		ErrorRate:            float64(errorReqs) / float64(max(totalReqs, 1)) * 100,
		RateLimitRate:        float64(rateLimitedReqs) / float64(max(totalReqs, 1)) * 100,
		HeaderParsingRate:    float64(headerSuccess) / float64(max(headerAttempts, 1)) * 100,
		DynamicRateLimitRate: float64(dynamicHits) / float64(max(dynamicHits+staticHits, 1)) * 100,

		Uptime:               uptime,
		LastUpdateTime:       lastUpdate,
	}
}

// MetricsSnapshot represents a snapshot of current metrics
type MetricsSnapshot struct {
	// Request counts
	TotalRequests        int64   `json:"total_requests"`
	SuccessRequests      int64   `json:"success_requests"`
	ErrorRequests        int64   `json:"error_requests"`
	RateLimitedRequests  int64   `json:"rate_limited_requests"`

	// Header parsing
	HeaderParsingAttempts int64  `json:"header_parsing_attempts"`
	HeaderParsingSuccess  int64  `json:"header_parsing_success"`
	HeaderParsingErrors   int64  `json:"header_parsing_errors"`

	// Latency metrics
	AverageLatency time.Duration `json:"average_latency"`
	MinLatency     time.Duration `json:"min_latency"`
	MaxLatency     time.Duration `json:"max_latency"`

	// Rate limiting metrics
	DynamicRateLimitHits int64   `json:"dynamic_rate_limit_hits"`
	StaticFallbackHits   int64   `json:"static_fallback_hits"`
	QueueOperations      int64   `json:"queue_operations"`

	// Circuit breaker
	CircuitBreakerOpens  int64 `json:"circuit_breaker_opens"`
	CircuitBreakerCloses int64 `json:"circuit_breaker_closes"`

	// Current state
	CurrentTPMLimit     int64 `json:"current_tpm_limit"`
	CurrentTPMRemaining int64 `json:"current_tpm_remaining"`

	// Calculated rates
	RequestsPerSecond    float64 `json:"requests_per_second"`
	SuccessRate          float64 `json:"success_rate"`
	ErrorRate            float64 `json:"error_rate"`
	RateLimitRate        float64 `json:"rate_limit_rate"`
	HeaderParsingRate    float64 `json:"header_parsing_rate"`
	DynamicRateLimitRate float64 `json:"dynamic_rate_limit_rate"`

	// Timing
	Uptime         time.Duration `json:"uptime"`
	LastUpdateTime time.Time     `json:"last_update_time"`
}

// HealthStatus represents the health status of the header-based rate limiting system
type HealthStatus struct {
	Status    string                 `json:"status"`
	Healthy   bool                   `json:"healthy"`
	Issues    []string               `json:"issues,omitempty"`
	Metrics   MetricsSnapshot        `json:"metrics"`
	Timestamp time.Time              `json:"timestamp"`
}

// CheckHealth performs health checks and returns health status
func (m *MetricsCollector) CheckHealth() HealthStatus {
	snapshot := m.GetMetrics()
	status := HealthStatus{
		Status:    "healthy",
		Healthy:   true,
		Metrics:   snapshot,
		Timestamp: time.Now(),
		Issues:    make([]string, 0),
	}

	// Check error rate
	if snapshot.ErrorRate > 10 { // More than 10% errors
		status.Healthy = false
		status.Status = "degraded"
		status.Issues = append(status.Issues, fmt.Sprintf("High error rate: %.1f%%", snapshot.ErrorRate))
	}

	// Check header parsing success rate
	if snapshot.HeaderParsingRate < 90 { // Less than 90% header parsing success
		status.Healthy = false
		status.Status = "degraded"
		status.Issues = append(status.Issues, fmt.Sprintf("Low header parsing rate: %.1f%%", snapshot.HeaderParsingRate))
	}

	// Check for excessive rate limiting
	if snapshot.RateLimitRate > 50 { // More than 50% of requests rate limited
		status.Healthy = false
		status.Status = "degraded"
		status.Issues = append(status.Issues, fmt.Sprintf("High rate limit rate: %.1f%%", snapshot.RateLimitRate))
	}

	// Check for circuit breaker activity
	if snapshot.CircuitBreakerOpens > 0 {
		status.Healthy = false
		status.Status = "unhealthy"
		status.Issues = append(status.Issues, fmt.Sprintf("Circuit breaker has opened %d times", snapshot.CircuitBreakerOpens))
	}

	// Check latency
	if snapshot.AverageLatency > 5*time.Second {
		status.Healthy = false
		status.Status = "degraded"
		status.Issues = append(status.Issues, fmt.Sprintf("High average latency: %v", snapshot.AverageLatency))
	}

	return status
}

// Reset resets all metrics to zero
func (m *MetricsCollector) Reset() {
	atomic.StoreInt64(&m.TotalRequests, 0)
	atomic.StoreInt64(&m.SuccessRequests, 0)
	atomic.StoreInt64(&m.ErrorRequests, 0)
	atomic.StoreInt64(&m.RateLimitedRequests, 0)
	atomic.StoreInt64(&m.HeaderParsingAttempts, 0)
	atomic.StoreInt64(&m.HeaderParsingSuccess, 0)
	atomic.StoreInt64(&m.HeaderParsingErrors, 0)
	atomic.StoreInt64(&m.TotalLatency, 0)
	atomic.StoreInt64(&m.MinLatency, int64(^uint64(0) >> 1))
	atomic.StoreInt64(&m.MaxLatency, 0)
	atomic.StoreInt64(&m.DynamicRateLimitHits, 0)
	atomic.StoreInt64(&m.StaticFallbackHits, 0)
	atomic.StoreInt64(&m.QueueOperations, 0)
	atomic.StoreInt64(&m.CircuitBreakerOpens, 0)
	atomic.StoreInt64(&m.CircuitBreakerCloses, 0)

	m.mu.Lock()
	m.TPMLimits = m.TPMLimits[:0]
	m.TPMRemaining = m.TPMRemaining[:0]
	m.mu.Unlock()

	m.StartTime = time.Now()
	atomic.StoreInt64(&m.LastUpdateTime, time.Now().UnixNano())
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}