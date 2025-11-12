package circuitbreaker

import (
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker protects against cascading failures
type CircuitBreaker struct {
	name             string
	maxFailures      int
	resetTimeout     time.Duration
	state            State
	failures         int
	lastFailureTime  time.Time
	mu               sync.RWMutex
	onStateChange    func(name string, from, to State)
	halfOpenMaxCalls int
	halfOpenCalls    int
}

// Config holds the circuit breaker configuration
type Config struct {
	Name             string
	MaxFailures      int           // Number of failures before opening circuit
	ResetTimeout     time.Duration // How long to wait before trying again
	HalfOpenMaxCalls int           // Number of calls allowed in half-open state
	OnStateChange    func(name string, from, to State)
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config Config) *CircuitBreaker {
	if config.MaxFailures <= 0 {
		config.MaxFailures = 5 // Default
	}
	if config.ResetTimeout <= 0 {
		config.ResetTimeout = 60 * time.Second // Default
	}
	if config.HalfOpenMaxCalls <= 0 {
		config.HalfOpenMaxCalls = 3 // Default
	}

	return &CircuitBreaker{
		name:             config.Name,
		maxFailures:      config.MaxFailures,
		resetTimeout:     config.ResetTimeout,
		state:            StateClosed,
		halfOpenMaxCalls: config.HalfOpenMaxCalls,
		onStateChange:    config.OnStateChange,
	}
}

// Call executes the given function if the circuit breaker allows it
func (cb *CircuitBreaker) Call(fn func() error) error {
	if !cb.canCall() {
		return ErrCircuitOpen
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// canCall determines if the call should be allowed
func (cb *CircuitBreaker) canCall() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if we should try half-open
		if time.Since(cb.lastFailureTime) >= cb.resetTimeout {
			cb.setState(StateHalfOpen)
			cb.halfOpenCalls = 0
			return true
		}
		return false
	case StateHalfOpen:
		return cb.halfOpenCalls < cb.halfOpenMaxCalls
	default:
		return false
	}
}

// recordResult records the result of a call
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen {
		cb.halfOpenCalls++

		if err == nil {
			// Success in half-open state, close the circuit
			if cb.halfOpenCalls >= cb.halfOpenMaxCalls {
				cb.setState(StateClosed)
				cb.failures = 0
			}
		} else {
			// Failure in half-open state, open the circuit again
			cb.setState(StateOpen)
			cb.lastFailureTime = time.Now()
		}
		return
	}

	// Normal state (Closed)
	if err != nil {
		cb.failures++
		cb.lastFailureTime = time.Now()

		if cb.failures >= cb.maxFailures {
			cb.setState(StateOpen)
		}
	} else {
		// Success in closed state, reset failure count
		cb.failures = 0
	}
}

// setState changes the circuit breaker state
func (cb *CircuitBreaker) setState(newState State) {
	if cb.state != newState {
		oldState := cb.state
		cb.state = newState

		if cb.onStateChange != nil {
			go cb.onStateChange(cb.name, oldState, newState)
		}
	}
}

// State returns the current state
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Failures returns the current failure count
func (cb *CircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Name returns the circuit breaker name
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

// Reset forces the circuit breaker into closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.halfOpenCalls = 0
}

// Stats provides circuit breaker statistics
type Stats struct {
	Name            string
	State           State
	Failures        int
	LastFailureTime time.Time
}

// Stats returns current circuit breaker statistics
func (cb *CircuitBreaker) Stats() Stats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return Stats{
		Name:            cb.name,
		State:           cb.state,
		Failures:        cb.failures,
		LastFailureTime: cb.lastFailureTime,
	}
}
