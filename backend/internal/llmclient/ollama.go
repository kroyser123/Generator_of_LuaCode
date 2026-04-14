package llmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/text/encoding/unicode"
)

type OllamaClient struct {
	endpoint string
	model    string
	http     *http.Client
}

func NewOllamaClient(endpoint, model string) *OllamaClient {
	return &OllamaClient{
		endpoint: endpoint,
		model:    model,
		http:     &http.Client{Timeout: 60 * time.Second},
	}
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string                 `json:"model"`
	Messages []ChatMessage          `json:"messages"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
	// Для /api/generate (fallback)
	Prompt string `json:"prompt,omitempty"`
}

type ChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Response string `json:"response"`
}

func normalizeUTF8(s string) string {
	// Принудительно конвертируем в UTF-8
	utf8Bytes, err := unicode.UTF8.NewEncoder().Bytes([]byte(s))
	if err != nil {
		return s
	}
	return string(utf8Bytes)
}
func (c *OllamaClient) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	for i := range messages {
		messages[i].Content = normalizeUTF8(messages[i].Content)
	}
	req := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Options: map[string]interface{}{
			"temperature": 0.1,
			"num_predict": 256,
			"num_ctx":     4096,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// Лог для отладки (покажет, что отправляется)
	log.Printf("[DEBUG] Request body (first 500 chars): %s", string(body)[:min(500, len(body))])

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8") // ← добавить charset

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Если /api/chat не работает, пробуем /api/generate
	if resp.StatusCode != http.StatusOK {
		log.Printf("[WARN] /api/chat returned %d, falling back to /api/generate", resp.StatusCode)
		return c.generate(ctx, messages)
	}

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return result.Message.Content, nil
}

// fallback для /api/generate
func (c *OllamaClient) generate(ctx context.Context, messages []ChatMessage) (string, error) {
	// Берем последнее сообщение пользователя как prompt
	var prompt string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			prompt = messages[i].Content
			break
		}
	}

	req := ChatRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.1,
			"num_predict": 512,
			"num_ctx":     4096,
			"stop":        []string{"```", "USER:", "ASSISTANT:", "\n\n"},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal generate request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create generate request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("do generate request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama generate returned %d", resp.StatusCode)
	}

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode generate response: %w", err)
	}

	return result.Response, nil
}
