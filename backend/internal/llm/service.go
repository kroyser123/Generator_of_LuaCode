package llm

import (
	"context"
	"log"

	"mega-agent/internal/llm/prompts" // ← добавляем
	"mega-agent/internal/llmclient"   // ← оставляем
)

type llmService struct {
	ollama *llmclient.OllamaClient
}

func NewService(endpoint, model string) Client {
	return &llmService{
		ollama: llmclient.NewOllamaClient(endpoint, model),
	}
}

func (s *llmService) Generate(ctx context.Context, prompt string) (string, error) {
	// Добавляем system prompt в сообщения
	messages := []llmclient.ChatMessage{
		{Role: "system", Content: prompts.GetSystemPrompt()},
		{Role: "user", Content: prompt},
	}

	log.Printf("[DEBUG] Sending request to Ollama with system prompt")
	log.Printf("[DEBUG] User prompt: %s", prompt)

	return s.ollama.Chat(ctx, messages)
}
