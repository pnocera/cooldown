package token

import (
	"testing"
)

func TestTokenRequest_Creation(t *testing.T) {
	req := TokenRequest{
		InputTokens:  100,
		OutputTokens: 200,
		Model:        "llama3.1-70b",
	}

	expectedTotal := 300
	if req.TotalTokens() != expectedTotal {
		t.Errorf("Expected total tokens %d, got %d", expectedTotal, req.TotalTokens())
	}
}
