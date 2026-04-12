package agent

import "context"

type Result struct {
    Code        string   `json:"code"`
    Explanation string   `json:"explanation"`
    Plan        []string `json:"plan"`
    Output      string   `json:"output"`
    Success     bool     `json:"success"`
    Error       string   `json:"error,omitempty"`
}

type Agent interface {
    Generate(ctx context.Context, prompt string) (*Result, error)
}