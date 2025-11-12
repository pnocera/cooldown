package featureflags

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

// FeatureFlag represents a feature flag with configuration
type FeatureFlag struct {
	Name         string        `json:"name"`
	Enabled      bool          `json:"enabled"`
	Description  string        `json:"description"`
	LastModified time.Time     `json:"last_modified"`
	RolloutPercentage float64  `json:"rollout_percentage"` // 0-100 for gradual rollout
}

// FeatureFlagManager manages feature flags for the header-based rate limiting system
type FeatureFlagManager struct {
	flags map[string]*FeatureFlag
	mu    sync.RWMutex
}

// NewFeatureFlagManager creates a new feature flag manager
func NewFeatureFlagManager() *FeatureFlagManager {
	ffm := &FeatureFlagManager{
		flags: make(map[string]*FeatureFlag),
	}

	// Initialize default flags
	ffm.initializeDefaultFlags()

	// Load flags from environment
	ffm.loadFromEnvironment()

	return ffm
}

// initializeDefaultFlags sets up the default feature flags
func (ffm *FeatureFlagManager) initializeDefaultFlags() {
	ffm.flags["header_based_rate_limiting"] = &FeatureFlag{
		Name:             "header_based_rate_limiting",
		Enabled:          true,  // Enabled by default for production
		Description:      "Enable dynamic rate limiting based on Cerebras API response headers",
		RolloutPercentage: 100.0, // Full rollout
		LastModified:     time.Now(),
	}

	ffm.flags["header_fallback"] = &FeatureFlag{
		Name:             "header_fallback",
		Enabled:          true,
		Description:      "Enable fallback to static rate limits when headers fail",
		RolloutPercentage: 100.0,
		LastModified:     time.Now(),
	}

	ffm.flags["circuit_breaker_enhancement"] = &FeatureFlag{
		Name:             "circuit_breaker_enhancement",
		Enabled:          true,
		Description:      "Enhanced circuit breaker with header-based rate limiting integration",
		RolloutPercentage: 100.0,
		LastModified:     time.Now(),
	}

	ffm.flags["metrics_collection"] = &FeatureFlag{
		Name:             "metrics_collection",
		Enabled:          true,
		Description:      "Collect detailed metrics for header-based rate limiting",
		RolloutPercentage: 100.0,
		LastModified:     time.Now(),
	}

	ffm.flags["dynamic_queue_prioritization"] = &FeatureFlag{
		Name:             "dynamic_queue_prioritization",
		Enabled:          true,
		Description:      "Dynamic queue prioritization based on token usage and wait times",
		RolloutPercentage: 100.0,
		LastModified:     time.Now(),
	}

	ffm.flags["rate_limit_buffer_adjustment"] = &FeatureFlag{
		Name:             "rate_limit_buffer_adjustment",
		Enabled:          true,
		Description:      "Automatically adjust reset buffer based on observed clock skew",
		RolloutPercentage: 50.0, // Gradual rollout for experimental feature
		LastModified:     time.Now(),
	}

	ffm.flags["header_validation_strict"] = &FeatureFlag{
		Name:             "header_validation_strict",
		Enabled:          false, // Disabled by default to avoid breaking changes
		Description:      "Strict validation of rate limit headers (reject malformed headers)",
		RolloutPercentage: 0.0,
		LastModified:     time.Now(),
	}
}

// loadFromEnvironment loads feature flag values from environment variables
func (ffm *FeatureFlagManager) loadFromEnvironment() {
	ffm.mu.Lock()
	defer ffm.mu.Unlock()

	// Check environment variables for flag overrides
	if val := os.Getenv("COOLDOWN_HEADER_BASED_RATE_LIMITING"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			ffm.flags["header_based_rate_limiting"].Enabled = enabled
			ffm.flags["header_based_rate_limiting"].LastModified = time.Now()
		}
	}

	if val := os.Getenv("COOLDOWN_HEADER_FALLBACK"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			ffm.flags["header_fallback"].Enabled = enabled
			ffm.flags["header_fallback"].LastModified = time.Now()
		}
	}

	if val := os.Getenv("COOLDOWN_CIRCUIT_BREAKER_ENHANCEMENT"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			ffm.flags["circuit_breaker_enhancement"].Enabled = enabled
			ffm.flags["circuit_breaker_enhancement"].LastModified = time.Now()
		}
	}

	if val := os.Getenv("COOLDOWN_METRICS_COLLECTION"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			ffm.flags["metrics_collection"].Enabled = enabled
			ffm.flags["metrics_collection"].LastModified = time.Now()
		}
	}

	if val := os.Getenv("COOLDOWN_DYNAMIC_QUEUE_PRIORITIZATION"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			ffm.flags["dynamic_queue_prioritization"].Enabled = enabled
			ffm.flags["dynamic_queue_prioritization"].LastModified = time.Now()
		}
	}

	if val := os.Getenv("COOLDOWN_RATE_LIMIT_BUFFER_ADJUSTMENT"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			ffm.flags["rate_limit_buffer_adjustment"].Enabled = enabled
			ffm.flags["rate_limit_buffer_adjustment"].LastModified = time.Now()
		}
	}

	if val := os.Getenv("COOLDOWN_HEADER_VALIDATION_STRICT"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			ffm.flags["header_validation_strict"].Enabled = enabled
			ffm.flags["header_validation_strict"].LastModified = time.Now()
		}
	}

	// Load rollout percentages
	if val := os.Getenv("COOLDOWN_HEADER_BASED_RATE_LIMITING_ROLLOUT"); val != "" {
		if rollout, err := strconv.ParseFloat(val, 64); err == nil && rollout >= 0 && rollout <= 100 {
			ffm.flags["header_based_rate_limiting"].RolloutPercentage = rollout
			ffm.flags["header_based_rate_limiting"].LastModified = time.Now()
		}
	}
}

// IsEnabled checks if a feature flag is enabled (considering rollout percentage)
func (ffm *FeatureFlagManager) IsEnabled(featureName string, rolloutID string) bool {
	ffm.mu.RLock()
	flag, exists := ffm.flags[featureName]
	ffm.mu.RUnlock()

	if !exists {
		return false
	}

	// If flag is not enabled at all, return false
	if !flag.Enabled {
		return false
	}

	// If full rollout, return true
	if flag.RolloutPercentage >= 100.0 {
		return true
	}

	// For partial rollout, use consistent hashing based on rolloutID
	hash := ffm.consistentHash(rolloutID, 100.0)
	return hash < flag.RolloutPercentage
}

// IsEnabledSimple checks if a feature flag is enabled (without rollout consideration)
func (ffm *FeatureFlagManager) IsEnabledSimple(featureName string) bool {
	ffm.mu.RLock()
	defer ffm.mu.RUnlock()

	flag, exists := ffm.flags[featureName]
	return exists && flag.Enabled
}

// SetFlag sets a feature flag's enabled status
func (ffm *FeatureFlagManager) SetFlag(featureName string, enabled bool, rolloutPercentage float64) error {
	ffm.mu.Lock()
	defer ffm.mu.Unlock()

	flag, exists := ffm.flags[featureName]
	if !exists {
		return fmt.Errorf("feature flag '%s' does not exist", featureName)
	}

	flag.Enabled = enabled
	flag.RolloutPercentage = rolloutPercentage
	flag.LastModified = time.Now()

	return nil
}

// GetFlag returns a feature flag's current state
func (ffm *FeatureFlagManager) GetFlag(featureName string) (*FeatureFlag, error) {
	ffm.mu.RLock()
	defer ffm.mu.RUnlock()

	flag, exists := ffm.flags[featureName]
	if !exists {
		return nil, fmt.Errorf("feature flag '%s' does not exist", featureName)
	}

	// Return a copy to avoid concurrent modification
	return &FeatureFlag{
		Name:             flag.Name,
		Enabled:          flag.Enabled,
		Description:      flag.Description,
		LastModified:     flag.LastModified,
		RolloutPercentage: flag.RolloutPercentage,
	}, nil
}

// GetAllFlags returns all feature flags
func (ffm *FeatureFlagManager) GetAllFlags() map[string]*FeatureFlag {
	ffm.mu.RLock()
	defer ffm.mu.RUnlock()

	result := make(map[string]*FeatureFlag)
	for name, flag := range ffm.flags {
		result[name] = &FeatureFlag{
			Name:             flag.Name,
			Enabled:          flag.Enabled,
			Description:      flag.Description,
			LastModified:     flag.LastModified,
			RolloutPercentage: flag.RolloutPercentage,
		}
	}

	return result
}

// consistentHash provides simple consistent hashing for rollout decisions
func (ffm *FeatureFlagManager) consistentHash(rolloutID string, max float64) float64 {
	if rolloutID == "" {
		return 50.0 // Default middle value for no ID
	}

	// Simple hash function
	hash := uint32(0)
	for _, c := range rolloutID {
		hash = hash*31 + uint32(c)
	}

	// Convert to percentage
	return float64(hash%1000) / 10.0 // 0-99.9 in 0.1 increments
}

// Convenience methods for commonly used flags

// IsHeaderBasedRateLimitingEnabled checks if header-based rate limiting is enabled
func (ffm *FeatureFlagManager) IsHeaderBasedRateLimitingEnabled(rolloutID string) bool {
	return ffm.IsEnabled("header_based_rate_limiting", rolloutID)
}

// IsHeaderFallbackEnabled checks if header fallback is enabled
func (ffm *FeatureFlagManager) IsHeaderFallbackEnabled(rolloutID string) bool {
	return ffm.IsEnabled("header_fallback", rolloutID)
}

// IsCircuitBreakerEnhancementEnabled checks if enhanced circuit breaker is enabled
func (ffm *FeatureFlagManager) IsCircuitBreakerEnhancementEnabled(rolloutID string) bool {
	return ffm.IsEnabled("circuit_breaker_enhancement", rolloutID)
}

// IsMetricsCollectionEnabled checks if metrics collection is enabled
func (ffm *FeatureFlagManager) IsMetricsCollectionEnabled(rolloutID string) bool {
	return ffm.IsEnabled("metrics_collection", rolloutID)
}

// IsDynamicQueuePrioritizationEnabled checks if dynamic queue prioritization is enabled
func (ffm *FeatureFlagManager) IsDynamicQueuePrioritizationEnabled(rolloutID string) bool {
	return ffm.IsEnabled("dynamic_queue_prioritization", rolloutID)
}

// IsRateLimitBufferAdjustmentEnabled checks if automatic buffer adjustment is enabled
func (ffm *FeatureFlagManager) IsRateLimitBufferAdjustmentEnabled(rolloutID string) bool {
	return ffm.IsEnabled("rate_limit_buffer_adjustment", rolloutID)
}

// IsHeaderValidationStrictEnabled checks if strict header validation is enabled
func (ffm *FeatureFlagManager) IsHeaderValidationStrictEnabled(rolloutID string) bool {
	return ffm.IsEnabled("header_validation_strict", rolloutID)
}