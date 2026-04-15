package llm

import "context"

type Client interface {
    Generate(ctx context.Context, prompt string, sessionID string) (string, error)
    GetEmbedding(ctx context.Context, text string) ([]float32, error)
}