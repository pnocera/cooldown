package token

import (
	"strings"
)

type TokenEstimator struct {
	// Simple word-based estimation for now
	wordToTokenRatio map[string]float64
}

func NewTokenEstimator() *TokenEstimator {
	return &TokenEstimator{
		wordToTokenRatio: map[string]float64{
			"llama3.1-70b": 1.3, // Approximate ratio
		},
	}
}

func (te *TokenEstimator) EstimateInputTokens(model, text string) (int, error) {
	ratio, exists := te.wordToTokenRatio[model]
	if !exists {
		ratio = 1.0 // Default ratio
	}

	words := strings.Fields(text)
	return int(float64(len(words)) * ratio), nil
}

func (te *TokenEstimator) EstimateOutputTokens(model, inputTokens int) int {
	// Conservative estimate: output tokens = 0.5 * input tokens
	return max(10, inputTokens/2)
}

func (te *TokenEstimator) EstimateTokens(req *TokenRequest) int {
	// For now, use a simple estimation based on the token request
	// In the future, this could be more sophisticated
	totalTokens := req.InputTokens + req.OutputTokens

	// If no tokens provided, use defaults
	if totalTokens == 0 {
		totalTokens = 1000 // Conservative default
	}

	return totalTokens
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
