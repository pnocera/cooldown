package circuitbreaker

import "errors"

// Circuit breaker specific errors
var (
	// ErrCircuitOpen is returned when the circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrMaxRetriesExceeded is returned when maximum retry attempts are exceeded
	ErrMaxRetriesExceeded = errors.New("maximum retry attempts exceeded")

	// ErrServiceUnavailable is returned when the service is temporarily unavailable
	ErrServiceUnavailable = errors.New("service temporarily unavailable")
)

// IsCircuitOpenError checks if the error is a circuit open error
func IsCircuitOpenError(err error) bool {
	return errors.Is(err, ErrCircuitOpen)
}

// IsRetryableError checks if the error is retryable
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Don't retry circuit open errors
	if IsCircuitOpenError(err) {
		return false
	}

	// Don't retry max retries exceeded errors
	if errors.Is(err, ErrMaxRetriesExceeded) {
		return false
	}

	// Other errors are generally retryable
	return true
}
