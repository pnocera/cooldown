package proxyerrors

import (
	"fmt"
	"net/http"
)

// ProxyError represents different types of proxy errors
type ProxyError struct {
	Type    ErrorType
	Message string
	Cause   error
}

type ErrorType int

const (
	ErrorTypeUpstreamConnection ErrorType = iota
	ErrorTypeUpstreamTimeout
	ErrorTypeUpstreamUnavailable
	ErrorTypeRateLimitExceeded
	ErrorTypeInvalidRequest
	ErrorTypeConfiguration
	ErrorTypeInternal
)

func (e *ProxyError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *ProxyError) Unwrap() error {
	return e.Cause
}

// HTTPStatus returns the appropriate HTTP status code for each error type
func (e *ProxyError) HTTPStatus() int {
	switch e.Type {
	case ErrorTypeUpstreamConnection, ErrorTypeUpstreamUnavailable:
		return http.StatusBadGateway
	case ErrorTypeUpstreamTimeout:
		return http.StatusGatewayTimeout
	case ErrorTypeRateLimitExceeded:
		return http.StatusTooManyRequests
	case ErrorTypeInvalidRequest:
		return http.StatusBadRequest
	case ErrorTypeConfiguration:
		return http.StatusInternalServerError
	case ErrorTypeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// NewProxyError creates a new ProxyError
func NewProxyError(errorType ErrorType, message string, cause error) *ProxyError {
	return &ProxyError{
		Type:    errorType,
		Message: message,
		Cause:   cause,
	}
}

// Common error constructors
func NewUpstreamConnectionError(target string, cause error) *ProxyError {
	return NewProxyError(ErrorTypeUpstreamConnection, fmt.Sprintf("Failed to connect to upstream server: %s", target), cause)
}

func NewUpstreamTimeoutError(target string, cause error) *ProxyError {
	return NewProxyError(ErrorTypeUpstreamTimeout, fmt.Sprintf("Upstream server timeout: %s", target), cause)
}

func NewUpstreamUnavailableError(target string, cause error) *ProxyError {
	return NewProxyError(ErrorTypeUpstreamUnavailable, fmt.Sprintf("Upstream server unavailable: %s", target), cause)
}

func NewRateLimitExceededError(domain string) *ProxyError {
	return NewProxyError(ErrorTypeRateLimitExceeded, fmt.Sprintf("Rate limit exceeded for domain: %s", domain), nil)
}

func NewInvalidRequestError(message string) *ProxyError {
	return NewProxyError(ErrorTypeInvalidRequest, fmt.Sprintf("Invalid request: %s", message), nil)
}

func NewConfigurationError(message string, cause error) *ProxyError {
	return NewProxyError(ErrorTypeConfiguration, fmt.Sprintf("Configuration error: %s", message), cause)
}

func NewInternalError(message string, cause error) *ProxyError {
	return NewProxyError(ErrorTypeInternal, fmt.Sprintf("Internal server error: %s", message), cause)
}