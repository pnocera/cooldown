package ratelimit

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseRateLimitHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-tokens-minute", "1000")
	headers.Set("x-ratelimit-remaining-tokens-minute", "800")
	headers.Set("x-ratelimit-reset-tokens-minute", "45.5")

	parsed, err := ParseRateLimitHeaders(headers)

	assert.NoError(t, err)
	assert.Equal(t, 1000, parsed.TPMLimit)
	assert.Equal(t, 800, parsed.TPMRemaining)
	assert.Equal(t, time.Duration(45.5*float64(time.Second)), parsed.TPMReset)
}

func TestParseRateLimitHeadersMissingRequired(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-ratelimit-remaining-tokens-minute", "800")

	_, err := ParseRateLimitHeaders(headers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required rate limit headers")
}

func TestParseRateLimitHeadersInvalidValues(t *testing.T) {
	tests := []struct {
		name            string
		setupHeaders    func() http.Header
		expected        *RateLimitHeaders
		hasError        bool
	}{
		{
			name: "invalid tpm limit",
			setupHeaders: func() http.Header {
				headers := http.Header{}
				headers.Set("x-ratelimit-limit-tokens-minute", "invalid")
				headers.Set("x-ratelimit-reset-tokens-minute", "60")
				return headers
			},
			hasError: true,
		},
		{
			name: "negative tpm remaining",
			setupHeaders: func() http.Header {
				headers := http.Header{}
				headers.Set("x-ratelimit-limit-tokens-minute", "1000")
				headers.Set("x-ratelimit-remaining-tokens-minute", "-100")
				headers.Set("x-ratelimit-reset-tokens-minute", "60")
				return headers
			},
			expected: &RateLimitHeaders{TPMLimit: 1000, TPMRemaining: 0, TPMReset: 60 * time.Second},
		},
		{
			name: "fractional reset time",
			setupHeaders: func() http.Header {
				headers := http.Header{}
				headers.Set("x-ratelimit-limit-tokens-minute", "1000")
				headers.Set("x-ratelimit-reset-tokens-minute", "45.5")
				return headers
			},
			expected: &RateLimitHeaders{TPMLimit: 1000, TPMReset: time.Duration(45.5 * float64(time.Second))},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := tt.setupHeaders()
			result, err := ParseRateLimitHeaders(headers)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.TPMLimit, result.TPMLimit)
				assert.Equal(t, tt.expected.TPMRemaining, result.TPMRemaining)
				assert.Equal(t, tt.expected.TPMReset, result.TPMReset)
			}
		})
	}
}