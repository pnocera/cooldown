package token

import (
	"testing"
)

func TestTokenEstimator_EstimateInputTokens(t *testing.T) {
	estimator := NewTokenEstimator()

	// Test with simple text
	text := "Hello world"
	tokens, err := estimator.EstimateInputTokens("llama3.1-70b", text)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if tokens <= 0 {
		t.Errorf("Expected positive token count, got %d", tokens)
	}
}
