package ratelimit

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type RateLimitHeaders struct {
	TPMLimit             int
	TPMRemaining         int
	TPMReset             time.Duration
	RequestDayLimit      int
	RequestDayRemaining  int
	RequestDayReset      time.Duration
}

func ParseRateLimitHeaders(headers http.Header) (*RateLimitHeaders, error) {
	result := &RateLimitHeaders{}

	// Parse TPM limit
	if tpmLimitStr := headers.Get("x-ratelimit-limit-tokens-minute"); tpmLimitStr != "" {
		if tpmLimit, err := strconv.Atoi(tpmLimitStr); err == nil && tpmLimit > 0 {
			result.TPMLimit = tpmLimit
		}
	}

	// Parse TPM remaining
	if tpmRemainingStr := headers.Get("x-ratelimit-remaining-tokens-minute"); tpmRemainingStr != "" {
		if tpmRemaining, err := strconv.Atoi(tpmRemainingStr); err == nil {
			if tpmRemaining >= 0 {
				result.TPMRemaining = tpmRemaining
			} else {
				// Handle negative values by setting to 0
				result.TPMRemaining = 0
			}
		}
	}

	// Parse TPM reset (may be fractional)
	if tpmResetStr := headers.Get("x-ratelimit-reset-tokens-minute"); tpmResetStr != "" {
		if tpmReset, err := strconv.ParseFloat(tpmResetStr, 64); err == nil && tpmReset > 0 {
			result.TPMReset = time.Duration(tpmReset * float64(time.Second))
		}
	}

	// Validate required fields
	if result.TPMLimit == 0 || result.TPMReset == 0 {
		return nil, fmt.Errorf("missing required rate limit headers")
	}

	return result, nil
}