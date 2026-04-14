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
	Code        string   `json:"code"`
	Explanation string   `json:"explanation"`
	Plan        []string `json:"plan"`
	Output      string   `json:"output"`
	Success     bool     `json:"success"`
}

type Agent interface {
	Generate(ctx context.Context, prompt string) (*Result, error)
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

func (a *agent) Generate(ctx context.Context, prompt string) (*Result, error) {
	start := time.Now()
	log.Printf("[DEBUG] Original prompt: %s", prompt)
	var lastCode string
	var lastError string

	for attempt := 1; attempt <= a.maxRetries; attempt++ {
		response, err := a.llm.Generate(ctx, prompt)
		if err != nil {
			lastError = err.Error()
			continue
		}

		code := a.cleanCode(response) // ← используем защищенную очистку

		lastCode = code

		validationResult := a.validator.Validate(code)
		if validationResult.Valid {
			entry := &storage.HistoryEntry{
				ID:              uuid.New().String(),
				SessionID:       a.sessionID,
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

		// Используем исправленный BuildCorrectionPrompt
		prompt = prompts.BuildCorrectionPrompt(prompt, code, lastError)
	}

	return nil, fmt.Errorf("failed after %d attempts: %s", a.maxRetries, lastError)
}
