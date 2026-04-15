package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"mega-agent/internal/llm"
	"mega-agent/internal/llm/prompts"
	"mega-agent/internal/storage"
	"mega-agent/internal/validator"

	"github.com/google/uuid"
)

type agent struct {
	llm        llm.Client
	validator  validator.Validator
	storage    storage.Repository
	sessionID  string
	maxRetries int
}

type Result struct {
	Code               string   `json:"code"`
	Explanation        string   `json:"explanation"`
	Plan               []string `json:"plan"`
	Output             string   `json:"output"`
	Success            bool     `json:"success"`
	NeedsClarification bool     `json:"needs_clarification,omitempty"`
	Question           string   `json:"question,omitempty"`
}

type Agent interface {
	Generate(ctx context.Context, prompt string, id string) (*Result, error)
}

func NewAgent(llmClient llm.Client, val validator.Validator, repo storage.Repository, sessionID string) Agent {
	return &agent{
		llm:        llmClient,
		validator:  val,
		storage:    repo,
		sessionID:  sessionID,
		maxRetries: 3,
	}
}

func (a *agent) cleanCode(raw string) string {
	code := strings.TrimSpace(raw)

	// Удаляем markdown fences
	if strings.HasPrefix(code, "```lua") {
		code = strings.TrimPrefix(code, "```lua")
	} else if strings.HasPrefix(code, "```") {
		code = strings.TrimPrefix(code, "```")
	}

	// Удаляем закрывающие fences (только если они есть в конце)
	if strings.HasSuffix(code, "```") {
		code = strings.TrimSuffix(code, "```")
	}

	// Проверяем формат lua{...}lua
	if strings.HasPrefix(code, "lua{") && strings.Contains(code, "}lua") {
		// Извлекаем содержимое между lua{ и }lua
		start := strings.Index(code, "lua{") + 4
		end := strings.LastIndex(code, "}lua")
		if end > start {
			code = code[start:end]
		}
	}

	return strings.TrimSpace(code)
}

// isClarificationRequest проверяет, является ли ответ модели уточняющим вопросом
func (a *agent) isClarificationRequest(response string) (bool, string) {
	trimmed := strings.TrimSpace(response)

	// Логируем сырой ответ
	log.Printf("[DEBUG] isClarificationRequest raw: %q", response)
	log.Printf("[DEBUG] isClarificationRequest trimmed: %q", trimmed)

	// Удаляем возможную обёртку lua{...}lua
	if strings.HasPrefix(trimmed, "lua{") && strings.HasSuffix(trimmed, "}lua") {
		trimmed = trimmed[4 : len(trimmed)-4]
		trimmed = strings.TrimSpace(trimmed)
		log.Printf("[DEBUG] After removing lua wrapper: %q", trimmed)
	}

	// Проверяем на Clarify:
	if strings.HasPrefix(trimmed, "Clarify:") {
		question := strings.TrimPrefix(trimmed, "Clarify:")
		log.Printf("[DEBUG] Found Clarify, question: %q", question)
		return true, strings.TrimSpace(question)
	}

	// Проверяем частичное совпадение (на случай обрезания)
	if strings.Contains(trimmed, "larify:") {
		parts := strings.SplitN(trimmed, "larify:", 2)
		if len(parts) == 2 {
			return true, strings.TrimSpace(parts[1])
		}
	}

	if strings.Contains(trimmed, "?") && !strings.Contains(trimmed, "lua{") {
		log.Printf("[DEBUG] Found question mark, returning as clarification")
		return true, trimmed
	}

	log.Printf("[DEBUG] Not a clarification request")
	return false, ""
}

func (a *agent) Generate(ctx context.Context, prompt string, sessionID string) (*Result, error) {
	start := time.Now()

	// Загружаем few-shot примеры из БД
	examples, err := a.storage.GetFewShotExamples(ctx, 5)
	if err != nil {
		log.Printf("[WARN] Failed to load few-shot examples: %v", err)
		examples = []*storage.HistoryEntry{}
	}

	// Строим промпт с примерами
	finalPrompt := prompts.BuildFewShotPrompt(examples, prompt)
	log.Printf("[DEBUG] Few-shot examples count: %d", len(examples))

	var lastCode string
	var lastError string
	var lastResponse string

	for attempt := 1; attempt <= a.maxRetries; attempt++ {
		response, err := a.llm.Generate(ctx, finalPrompt, sessionID)
		if err != nil {
			lastError = err.Error()
			log.Printf("[DEBUG] LLM generate error (attempt %d): %v", attempt, err)
			continue
		}

		lastResponse = response
		log.Printf("[DEBUG] LLM response (attempt %d): %s", attempt, response)

		// Проверяем на Clarify
		if isClarification, question := a.isClarificationRequest(response); isClarification {
			log.Printf("[AGENT] LLM requests clarification: %s", question)
			return &Result{
				Success:            true,
				NeedsClarification: true,
				Question:           question,
			}, nil
		}

		// Очищаем код
		code := a.cleanCode(response)
		lastCode = code

		// Валидируем
		validationResult := a.validator.Validate(code)
		if validationResult.Valid {
			entry := &storage.HistoryEntry{
				ID:              uuid.New().String(),
				SessionID:       sessionID,
				Prompt:          prompt,
				Code:            lastCode,
				Success:         true,
				ExecutionTimeMs: time.Since(start).Milliseconds(),
				CreatedAt:       time.Now().Unix(),
			}
			_ = a.storage.Save(ctx, entry)

			return &Result{
				Code:    code,
				Output:  validationResult.Output,
				Success: true,
			}, nil
		}

		if len(validationResult.Errors) > 0 {
			lastError = validationResult.Errors[0]
		} else {
			lastError = "validation failed"
		}

		log.Printf("[DEBUG] Validation failed (attempt %d): %s", attempt, lastError)

		// Для исправления используем исходный промпт (без few-shot)
		finalPrompt = prompts.BuildCorrectionPrompt(prompt, code, lastError)
	}

	return nil, fmt.Errorf("failed after %d attempts. Last response: %s, Last error: %s",
		a.maxRetries, lastResponse, lastError)
}
