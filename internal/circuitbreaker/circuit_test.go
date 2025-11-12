package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_Creation(t *testing.T) {
	config := Config{
		Name:         "test-circuit",
		MaxFailures:  3,
		ResetTimeout: 30 * time.Second,
	}

	cb := NewCircuitBreaker(config)

	if cb == nil {
		t.Fatal("Expected non-nil circuit breaker")
	}

	if cb.Name() != "test-circuit" {
		t.Errorf("Expected name 'test-circuit', got %s", cb.Name())
	}

	if cb.State() != StateClosed {
		t.Errorf("Expected initial state to be CLOSED, got %s", cb.State().String())
	}
}

func TestCircuitBreaker_SuccessfulCalls(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		Name:         "test-circuit",
		MaxFailures:  3,
		ResetTimeout: 30 * time.Second,
	})

	// Successful calls should not change state
	err := cb.Call(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if cb.State() != StateClosed {
		t.Errorf("Expected state to remain CLOSED, got %s", cb.State().String())
	}

	if cb.Failures() != 0 {
		t.Errorf("Expected 0 failures, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_FailureThreshold(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		Name:         "test-circuit",
		MaxFailures:  3,
		ResetTimeout: 1 * time.Second,
	})

	// Generate failures to open the circuit
	for i := 0; i < 3; i++ {
		cb.Call(func() error {
			return errors.New("test error")
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("Expected circuit to be OPEN after 3 failures, got %s", cb.State().String())
	}

	if cb.Failures() != 3 {
		t.Errorf("Expected 3 failures, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_OpenCircuitRejectsCalls(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		Name:         "test-circuit",
		MaxFailures:  2,
		ResetTimeout: 1 * time.Second,
	})

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Call(func() error {
			return errors.New("test error")
		})
	}

	// Calls should be rejected
	err := cb.Call(func() error {
		return nil
	})

	if !IsCircuitOpenError(err) {
		t.Errorf("Expected circuit open error, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpenState(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		Name:             "test-circuit",
		MaxFailures:      2,
		ResetTimeout:     100 * time.Millisecond,
		HalfOpenMaxCalls: 2,
	})

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Call(func() error {
			return errors.New("test error")
		})
	}

	// Wait for reset timeout
	time.Sleep(150 * time.Millisecond)

	// First call should be allowed (half-open state)
	err := cb.Call(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected successful call in half-open state, got %v", err)
	}

	if cb.State() != StateHalfOpen {
		t.Errorf("Expected state to be HALF_OPEN, got %s", cb.State().String())
	}

	// Second successful call should close the circuit
	err = cb.Call(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected successful call in half-open state, got %v", err)
	}

	if cb.State() != StateClosed {
		t.Errorf("Expected circuit to close after successful half-open calls, got %s", cb.State().String())
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		Name:             "test-circuit",
		MaxFailures:      2,
		ResetTimeout:     100 * time.Millisecond,
		HalfOpenMaxCalls: 2,
	})

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Call(func() error {
			return errors.New("test error")
		})
	}

	// Wait for reset timeout
	time.Sleep(150 * time.Millisecond)

	// First call fails in half-open state
	err := cb.Call(func() error {
		return errors.New("half-open failure")
	})

	if err == nil {
		t.Error("Expected error in half-open state")
	}

	// Circuit should open again
	if cb.State() != StateOpen {
		t.Errorf("Expected circuit to re-open after half-open failure, got %s", cb.State().String())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		Name:         "test-circuit",
		MaxFailures:  2,
		ResetTimeout: 1 * time.Second,
	})

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Call(func() error {
			return errors.New("test error")
		})
	}

	// Reset the circuit
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("Expected state to be CLOSED after reset, got %s", cb.State().String())
	}

	if cb.Failures() != 0 {
		t.Errorf("Expected 0 failures after reset, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := NewCircuitBreaker(Config{
		Name:         "test-circuit",
		MaxFailures:  2,
		ResetTimeout: 1 * time.Second,
	})

	// Generate a failure
	cb.Call(func() error {
		return errors.New("test error")
	})

	stats := cb.Stats()

	if stats.Name != "test-circuit" {
		t.Errorf("Expected name 'test-circuit', got %s", stats.Name)
	}

	if stats.State != StateClosed {
		t.Errorf("Expected state CLOSED, got %s", stats.State.String())
	}

	if stats.Failures != 1 {
		t.Errorf("Expected 1 failure, got %d", stats.Failures)
	}
}
