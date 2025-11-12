package token

type TokenRequest struct {
	InputTokens  int
	OutputTokens int
	Model        string
}

func (tr TokenRequest) TotalTokens() int {
	return tr.InputTokens + tr.OutputTokens
}
