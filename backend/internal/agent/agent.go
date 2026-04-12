package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"mega-agent/backend/internal/llm"
	"mega-agent/backend/internal/storage"
	"mega-agent/backend/internal/validator"
	"time"

	"github.com/google/uuid"
)

type agent struct {
	llm        llm.Client
	validator  validator.Validator
	storage    storage.Repository
	sessionID  string
	maxRetries int
}

func NewAgent(
	llmClient llm.Client,
	val validator.Validator,
	repo storage.Repository,
	sessionID string,
) Agent {
	return &agent{
		llm:        llmClient,
		validator:  val,
		storage:    repo,
		sessionID:  sessionID,
		maxRetries: 3,
	}
}

func (a *agent) Generate(ctx context.Context, prompt string) (*Result, error) {
	start := time.Now()

	// 1. Получаем успешные примеры из истории (few-shot)
	examples, _ := a.storage.GetRecentSuccess(ctx, a.sessionID, 3)

	// 2. Строим промпт с примерами
	fullPrompt := a.buildPromptWithExamples(prompt, examples)

	var lastCode string
	var lastError string

	for attempt := 1; attempt <= a.maxRetries; attempt++ {
		// 3. Вызов LLM
		response, err := a.llm.Generate(ctx, fullPrompt)
		if err != nil {
			lastError = err.Error()
			continue
		}

		// 4. Парсим JSON ответ
		var llmOutput struct {
			Code        string   `json:"code"`
			Explanation string   `json:"explanation"`
			Plan        []string `json:"plan"`
		}
		if err := json.Unmarshal([]byte(response), &llmOutput); err != nil {
			lastError = fmt.Sprintf("parse error: %v", err)
			fullPrompt = a.buildFixPrompt(prompt, response, lastError)
			continue
		}
		lastCode = llmOutput.Code

		// 5. Валидация кода
		validationResult := a.validator.Validate(llmOutput.Code)
		if validationResult.Valid {
			// 6. Успех — сохраняем в историю
			entry := &storage.HistoryEntry{
				ID:              uuid.New().String(),
				SessionID:       a.sessionID,
				Prompt:          prompt,
				Code:            llmOutput.Code,
				Explanation:     llmOutput.Explanation,
				Plan:            llmOutput.Plan,
				Success:         true,
				ErrorMessage:    "",
				ExecutionTimeMs: time.Since(start).Milliseconds(),
				CreatedAt:       time.Now().Unix(),
			}
			_ = a.storage.Save(ctx, entry)

			return &Result{
				Code:        llmOutput.Code,
				Explanation: llmOutput.Explanation,
				Plan:        llmOutput.Plan,
				Output:      validationResult.Output,
				Success:     true,
			}, nil
		}

		// 7. Ошибка
		lastError = validationResult.Errors[0]
		fullPrompt = a.buildFixPrompt(prompt, llmOutput.Code, lastError)
	}

	// 8. Все попытки провалились — сохраняем неудачу
	entry := &storage.HistoryEntry{
		ID:              uuid.New().String(),
		SessionID:       a.sessionID,
		Prompt:          prompt,
		Code:            lastCode,
		Explanation:     "",
		Plan:            []string{},
		Success:         false,
		ErrorMessage:    lastError,
		ExecutionTimeMs: time.Since(start).Milliseconds(),
		CreatedAt:       time.Now().Unix(),
	}
	_ = a.storage.Save(ctx, entry)

	return nil, fmt.Errorf("failed after %d attempts: %s", a.maxRetries, lastError)
}

func (a *agent) buildPromptWithExamples(prompt string, examples []*storage.HistoryEntry) string {
	if len(examples) == 0 {
		return llm.SystemPrompt + "\n\nTask: " + prompt
	}

	var examplesText string
	for i, ex := range examples {
		examplesText += fmt.Sprintf("Example %d:\nTask: %s\nCode:\n%s\n\n", i+1, ex.Prompt, ex.Code)
	}
	return llm.SystemPrompt + "\n\n" + examplesText + "Task: " + prompt
}

func (a *agent) buildFixPrompt(originalPrompt, badCode, errorMsg string) string {
	return fmt.Sprintf(llm.CorrectionPrompt, originalPrompt, badCode, errorMsg)
}
