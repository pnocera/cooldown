package monitoring

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// MetricsHandler provides HTTP endpoints for metrics and health monitoring
type MetricsHandler struct {
	collector *MetricsCollector
}

// NewMetricsHandler creates a new metrics HTTP handler
func NewMetricsHandler(collector *MetricsCollector) *MetricsHandler {
	return &MetricsHandler{
		collector: collector,
	}
}

// RegisterRoutes registers the monitoring HTTP endpoints
func (h *MetricsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/metrics", h.MetricsHandler)
	mux.HandleFunc("/health", h.HealthHandler)
	mux.HandleFunc("/health/detailed", h.DetailedHealthHandler)
	mux.HandleFunc("/metrics/reset", h.ResetMetricsHandler)
	mux.HandleFunc("/metrics/prometheus", h.PrometheusHandler)
}

// MetricsHandler returns current metrics in JSON format
func (h *MetricsHandler) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := h.collector.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
		return
	}
}

// HealthHandler returns basic health status
func (h *MetricsHandler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := h.collector.CheckHealth()

	if health.Healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    health.Status,
		"healthy":   health.Healthy,
		"timestamp": health.Timestamp,
	})
}

// DetailedHealthHandler returns detailed health status with metrics
func (h *MetricsHandler) DetailedHealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := h.collector.CheckHealth()

	statusCode := http.StatusOK
	if !health.Healthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(health); err != nil {
		http.Error(w, "Failed to encode health status", http.StatusInternalServerError)
		return
	}
}

// ResetMetricsHandler resets all metrics (for testing/recovery)
func (h *MetricsHandler) ResetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.collector.Reset()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Metrics reset successfully",
	})
}

// PrometheusHandler returns metrics in Prometheus format
func (h *MetricsHandler) PrometheusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := h.collector.GetMetrics()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// Request metrics
	fmt.Fprintf(w, "# HELP cooldown_proxy_requests_total Total number of requests\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_requests_total counter\n")
	fmt.Fprintf(w, "cooldown_proxy_requests_total %d\n", snapshot.TotalRequests)

	fmt.Fprintf(w, "# HELP cooldown_proxy_requests_success_total Total number of successful requests\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_requests_success_total counter\n")
	fmt.Fprintf(w, "cooldown_proxy_requests_success_total %d\n", snapshot.SuccessRequests)

	fmt.Fprintf(w, "# HELP cooldown_proxy_requests_error_total Total number of error requests\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_requests_error_total counter\n")
	fmt.Fprintf(w, "cooldown_proxy_requests_error_total %d\n", snapshot.ErrorRequests)

	fmt.Fprintf(w, "# HELP cooldown_proxy_requests_rate_limited_total Total number of rate limited requests\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_requests_rate_limited_total counter\n")
	fmt.Fprintf(w, "cooldown_proxy_requests_rate_limited_total %d\n", snapshot.RateLimitedRequests)

	// Header parsing metrics
	fmt.Fprintf(w, "# HELP cooldown_proxy_header_parsing_attempts_total Total header parsing attempts\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_header_parsing_attempts_total counter\n")
	fmt.Fprintf(w, "cooldown_proxy_header_parsing_attempts_total %d\n", snapshot.HeaderParsingAttempts)

	fmt.Fprintf(w, "# HELP cooldown_proxy_header_parsing_success_total Total successful header parsing attempts\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_header_parsing_success_total counter\n")
	fmt.Fprintf(w, "cooldown_proxy_header_parsing_success_total %d\n", snapshot.HeaderParsingSuccess)

	fmt.Fprintf(w, "# HELP cooldown_proxy_header_parsing_errors_total Total header parsing errors\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_header_parsing_errors_total counter\n")
	fmt.Fprintf(w, "cooldown_proxy_header_parsing_errors_total %d\n", snapshot.HeaderParsingErrors)

	// Rate limiting metrics
	fmt.Fprintf(w, "# HELP cooldown_proxy_dynamic_rate_limit_hits_total Total dynamic rate limit hits\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_dynamic_rate_limit_hits_total counter\n")
	fmt.Fprintf(w, "cooldown_proxy_dynamic_rate_limit_hits_total %d\n", snapshot.DynamicRateLimitHits)

	fmt.Fprintf(w, "# HELP cooldown_proxy_static_fallback_hits_total Total static fallback hits\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_static_fallback_hits_total counter\n")
	fmt.Fprintf(w, "cooldown_proxy_static_fallback_hits_total %d\n", snapshot.StaticFallbackHits)

	fmt.Fprintf(w, "# HELP cooldown_proxy_queue_operations_total Total queue operations\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_queue_operations_total counter\n")
	fmt.Fprintf(w, "cooldown_proxy_queue_operations_total %d\n", snapshot.QueueOperations)

	// Circuit breaker metrics
	fmt.Fprintf(w, "# HELP cooldown_proxy_circuit_breaker_opens_total Total circuit breaker opens\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_circuit_breaker_opens_total counter\n")
	fmt.Fprintf(w, "cooldown_proxy_circuit_breaker_opens_total %d\n", snapshot.CircuitBreakerOpens)

	fmt.Fprintf(w, "# HELP cooldown_proxy_circuit_breaker_closes_total Total circuit breaker closes\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_circuit_breaker_closes_total counter\n")
	fmt.Fprintf(w, "cooldown_proxy_circuit_breaker_closes_total %d\n", snapshot.CircuitBreakerCloses)

	// Current state metrics
	fmt.Fprintf(w, "# HELP cooldown_proxy_current_tpm_limit Current TPM limit from headers\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_current_tpm_limit gauge\n")
	fmt.Fprintf(w, "cooldown_proxy_current_tpm_limit %d\n", snapshot.CurrentTPMLimit)

	fmt.Fprintf(w, "# HELP cooldown_proxy_current_tpm_remaining Current TPM remaining from headers\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_current_tpm_remaining gauge\n")
	fmt.Fprintf(w, "cooldown_proxy_current_tpm_remaining %d\n", snapshot.CurrentTPMRemaining)

	// Rate metrics
	fmt.Fprintf(w, "# HELP cooldown_proxy_requests_per_second Requests per second\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_requests_per_second gauge\n")
	fmt.Fprintf(w, "cooldown_proxy_requests_per_second %.2f\n", snapshot.RequestsPerSecond)

	fmt.Fprintf(w, "# HELP cooldown_proxy_success_rate Success rate percentage\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_success_rate gauge\n")
	fmt.Fprintf(w, "cooldown_proxy_success_rate %.2f\n", snapshot.SuccessRate)

	fmt.Fprintf(w, "# HELP cooldown_proxy_error_rate Error rate percentage\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_error_rate gauge\n")
	fmt.Fprintf(w, "cooldown_proxy_error_rate %.2f\n", snapshot.ErrorRate)

	fmt.Fprintf(w, "# HELP cooldown_proxy_rate_limit_rate Rate limit hit rate percentage\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_rate_limit_rate gauge\n")
	fmt.Fprintf(w, "cooldown_proxy_rate_limit_rate %.2f\n", snapshot.RateLimitRate)

	fmt.Fprintf(w, "# HELP cooldown_proxy_header_parsing_rate Header parsing success rate percentage\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_header_parsing_rate gauge\n")
	fmt.Fprintf(w, "cooldown_proxy_header_parsing_rate %.2f\n", snapshot.HeaderParsingRate)

	fmt.Fprintf(w, "# HELP cooldown_proxy_dynamic_rate_limit_rate Dynamic rate limiting usage percentage\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_dynamic_rate_limit_rate gauge\n")
	fmt.Fprintf(w, "cooldown_proxy_dynamic_rate_limit_rate %.2f\n", snapshot.DynamicRateLimitRate)

	// Latency metrics
	fmt.Fprintf(w, "# HELP cooldown_proxy_average_latency_seconds Average request latency in seconds\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_average_latency_seconds gauge\n")
	fmt.Fprintf(w, "cooldown_proxy_average_latency_seconds %.6f\n", snapshot.AverageLatency.Seconds())

	fmt.Fprintf(w, "# HELP cooldown_proxy_min_latency_seconds Minimum request latency in seconds\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_min_latency_seconds gauge\n")
	fmt.Fprintf(w, "cooldown_proxy_min_latency_seconds %.6f\n", snapshot.MinLatency.Seconds())

	fmt.Fprintf(w, "# HELP cooldown_proxy_max_latency_seconds Maximum request latency in seconds\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_max_latency_seconds gauge\n")
	fmt.Fprintf(w, "cooldown_proxy_max_latency_seconds %.6f\n", snapshot.MaxLatency.Seconds())

	// Uptime metric
	fmt.Fprintf(w, "# HELP cooldown_proxy_uptime_seconds Proxy uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE cooldown_proxy_uptime_seconds counter\n")
	fmt.Fprintf(w, "cooldown_proxy_uptime_seconds %.0f\n", snapshot.Uptime.Seconds())
}
