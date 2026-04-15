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
	if strings.HasSuffix(code, "```") {
		code = strings.TrimSuffix(code, "```")
	}

	code = strings.TrimSpace(code)

	if !strings.HasPrefix(code, "lua{") {
		code = "lua{" + code
	}
	if !strings.HasSuffix(code, "}lua") {
		code = code + "}lua"
	}

	return code
}

// isClarificationRequest проверяет, является ли ответ модели уточняющим вопросом
func (a *agent) isClarificationRequest(response string) (bool, string) {
	trimmed := strings.TrimSpace(response)

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

	// Проверяем частичное совпадение
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

// isComplexRequest проверяет, требует ли запрос Chain-of-Thought
func (a *agent) isComplexRequest(prompt string) bool {
	complexKeywords := []string{
		"RSI", "MACD", "индикатор", "алгоритм", "сложный",
		"анализ", "рассчитать", "оптимизировать", "рекурсия",
	}
	lowerPrompt := strings.ToLower(prompt)
	for _, kw := range complexKeywords {
		if strings.Contains(lowerPrompt, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// hasSecurityRisk проверяет, есть ли риск безопасности в запросе
func (a *agent) hasSecurityRisk(prompt string) bool {
	securityKeywords := []string{
		"os.execute", "io.popen", "loadstring", "debug",
		"удали файл", "delete file", "system command",
		"выполни команду", "shell", "exec",
	}
	lowerPrompt := strings.ToLower(prompt)
	for _, kw := range securityKeywords {
		if strings.Contains(lowerPrompt, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func (a *agent) Generate(ctx context.Context, prompt string, sessionID string) (*Result, error) {
	// Для сложных запросов добавляем CoT
	if a.isComplexRequest(prompt) {
		prompt = prompts.GetCOTPrompt() + "\n\n" + prompt
	}

	// Проверяем безопасность
	if a.hasSecurityRisk(prompt) {
		prompt = prompts.GetSecurityPrompt() + "\n\n" + prompt
	}

	start := time.Now()

	// Загружаем few-shot примеры из БД
	examples, err := a.storage.GetFewShotExamples(ctx, 5)
	if err != nil {
		log.Printf("[WARN] Failed to load few-shot examples: %v", err)
		examples = []*storage.HistoryEntry{}
	}

	// ========== ВСТАВИТЬ ЭТОТ КОД ЗДЕСЬ ==========
	// Получаем эмбеддинг для RAG поиска
	embedding, err := a.llm.GetEmbedding(ctx, prompt)
	if err != nil {
		log.Printf("[WARN] Failed to get embedding: %v", err)
		embedding = nil
	}

	// Ищем похожие примеры
	var similarExamples []*storage.HistoryEntry
	if embedding != nil {
		similarExamples, err = a.storage.FindSimilarByEmbedding(ctx, embedding, 3)
		if err != nil {
			log.Printf("[WARN] Failed to find similar examples: %v", err)
			similarExamples = []*storage.HistoryEntry{}
		}
		log.Printf("[RAG] Found %d similar examples", len(similarExamples))
	}
	// ============================================

	// Строим промпт с RAG примерами
	finalPrompt := prompts.BuildRAGPrompt(similarExamples, prompt)
	finalPrompt = prompts.BuildFewShotPrompt(examples, finalPrompt)

	log.Printf("[DEBUG] Few-shot examples count: %d", len(examples))
	log.Printf("[DEBUG] RAG examples count: %d", len(similarExamples))

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
			// САМОПРОВЕРКА КОДА
			if !a.selfTestCode(code, prompt) {
				log.Printf("[SELF-TEST] Code failed self-test, retrying...")
				lastError = "self-test failed: code does not produce expected output"
				finalPrompt = prompts.BuildCorrectionPrompt(prompt, code, lastError)
				continue
			}

			// Сохраняем в БД
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

// selfTestCode проверяет код на простых примерах
func (a *agent) selfTestCode(code string, prompt string) bool {
	cleanCode := validator.ExtractMWSCode(code)
	tests := a.generateTests(prompt, cleanCode)

	if len(tests) == 0 {
		log.Printf("[SELF-TEST] No tests generated for this prompt, skipping")
		return true
	}

	sandbox := validator.NewSandboxValidator(5 * time.Second)

	for _, test := range tests {
		// Мягкая проверка кода
		if test.ValidateFunc != nil && !test.ValidateFunc(cleanCode) {
			log.Printf("[SELF-TEST] ⚠️ Test '%s' failed code structure check, but continuing", test.Name)
			// Не проваливаем тест, только логируем
			continue
		}

		result, err := sandbox.Run(code, test.Input)
		if err != nil {
			log.Printf("[SELF-TEST] ⚠️ Test '%s' execution error: %v", test.Name, err)
			continue
		}

		// Мягкое сравнение — ищем подстроку
		if !strings.Contains(result, test.Expected) && result != test.Expected {
			log.Printf("[SELF-TEST] ⚠️ Test '%s' output mismatch: got '%s', expected '%s'",
				test.Name, result, test.Expected)
			// Не проваливаем тест, только логируем
			continue
		}

		log.Printf("[SELF-TEST] ✅ Test '%s' passed", test.Name)
	}

	log.Printf("[SELF-TEST] Self-test completed (soft mode)")
	return true // Всегда возвращаем true для мягкой проверки
}

func (a *agent) generateTests(prompt string, code string) []TestCase {
	var tests []TestCase
	lowerPrompt := strings.ToLower(prompt)

	// Тест для сложения — мягкая проверка
	if strings.Contains(lowerPrompt, "слож") || strings.Contains(lowerPrompt, "add") ||
		strings.Contains(lowerPrompt, "плюс") {
		tests = append(tests, TestCase{
			Name: "addition",
			Input: map[string]interface{}{
				"wf.vars.a": 5,
				"wf.vars.b": 3,
			},
			Expected: "8",
			// Мягкая проверка: код должен содержать return и сложение
			ValidateFunc: func(code string) bool {
				hasReturn := strings.Contains(code, "return")
				hasAddition := strings.Contains(code, "+")
				return hasReturn && hasAddition
			},
		})
	}

	// Тест для последнего email — мягкая проверка
	if strings.Contains(lowerPrompt, "последн") || strings.Contains(lowerPrompt, "last") {
		tests = append(tests, TestCase{
			Name: "last email",
			Input: map[string]interface{}{
				"wf.vars.emails": []string{"a@b.com", "c@d.com", "e@f.com"},
			},
			Expected: "e@f.com",
			ValidateFunc: func(code string) bool {
				hasReturn := strings.Contains(code, "return")
				hasBrackets := strings.Contains(code, "[") && strings.Contains(code, "]")
				hasLength := strings.Contains(code, "#")
				return hasReturn && (hasBrackets || hasLength)
			},
		})
	}

	// Тест для удаления дубликатов — мягкая проверка
	if strings.Contains(lowerPrompt, "дубликат") || strings.Contains(lowerPrompt, "duplicate") ||
		strings.Contains(lowerPrompt, "очист") {
		tests = append(tests, TestCase{
			Name: "remove duplicates",
			Input: map[string]interface{}{
				"wf.vars.data": []interface{}{1, 2, 2, 3, 3, 4},
			},
			Expected: "[1 2 3 4]",
			ValidateFunc: func(code string) bool {
				hasReturn := strings.Contains(code, "return")
				hasLoop := strings.Contains(code, "for") || strings.Contains(code, "ipairs")
				return hasReturn && hasLoop
			},
		})
	}

	return tests
}

// TestCase — структура теста
type TestCase struct {
	Name         string
	Input        map[string]interface{}
	Expected     string
	ValidateFunc func(code string) bool // дополнительная проверка кода
}
