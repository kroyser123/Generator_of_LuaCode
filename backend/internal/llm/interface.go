package llm

import "context"

// Client — интерфейс LLM клиента
type Client interface {
	Generate(ctx context.Context, prompt string) (string, error)
}
