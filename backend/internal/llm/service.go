package llm

import (
	"context"
	"log"

	"mega-agent/internal/llm/prompts"
	"mega-agent/internal/llmclient"
)

type llmService struct {
	ollama *llmclient.OllamaClient
}

func NewService(endpoint, model string) Client {
	return &llmService{
		ollama: llmclient.NewOllamaClient(endpoint, model),
	}
}

func (s *llmService) Generate(ctx context.Context, prompt string, sessionID string) (string, error) {
	messages := []llmclient.ChatMessage{
		{Role: "system", Content: prompts.GetSystemPrompt()},
		{Role: "user", Content: prompt},
	}

	log.Printf("[DEBUG] Sending request to Ollama with system prompt")
	log.Printf("[DEBUG] User prompt: %s", prompt)

	return s.ollama.Chat(ctx, messages)
}

// Добавить这个方法
func (s *llmService) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	return s.ollama.Embed(ctx, text)
}
