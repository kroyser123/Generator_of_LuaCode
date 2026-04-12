package llm

import (
	"context"
	"mega-agent/backend/internal/llm/client"
	"strings"
)

type llmService struct {
	ollama   *client.OllamaClient
	embedder Embedder
	rag      RAG
}

// NewService создает новый LLM сервис
func NewService(endpoint, model string, embedder Embedder, rag RAG) Client {
	return &llmService{
		ollama:   client.NewOllamaClient(endpoint, model),
		embedder: embedder,
		rag:      rag,
	}
}

// Generate отправляет промпт в LLM
func (s *llmService) Generate(ctx context.Context, prompt string) (string, error) {
	// Добавляем RAG если есть
	if s.rag != nil {
		examples, _ := s.rag.FindSimilar(prompt, 3)
		if len(examples) > 0 {
			prompt = s.buildPromptWithExamples(prompt, examples)
		}
	}

	return s.ollama.Generate(ctx, prompt)
}

func (s *llmService) buildPromptWithExamples(prompt string, examples []string) string {
	var sb strings.Builder
	sb.WriteString("Examples of good code:\n\n")
	for i, ex := range examples {
		sb.WriteString("Example ")
		sb.WriteString(string(rune('1' + i)))
		sb.WriteString(":\n")
		sb.WriteString(ex)
		sb.WriteString("\n\n")
	}
	sb.WriteString("Now solve this task:\n")
	sb.WriteString(prompt)
	return sb.String()
}
