package prompts

import (
	_ "embed"
	"fmt"
	"mega-agent/internal/storage"
	"strings"
)

//go:embed system.txt
var systemPrompt string

//go:embed correction.txt
var correctionPrompt string

//go:embed clarify.txt
var clarifyPrompt string

//go:embed cot.txt
var cotPrompt string

//go:embed optimization.txt
var optimizationPrompt string

//go:embed security.txt
var securityPrompt string

//go:embed validation.txt
var validationPrompt string

// BuildFewShotPrompt строит промпт с примерами из БД
func BuildFewShotPrompt(examples []*storage.HistoryEntry, userPrompt string) string {
	if len(examples) == 0 {
		return userPrompt
	}

	var sb strings.Builder
	sb.WriteString("Вот примеры правильных ответов:\n\n")

	for i, ex := range examples {
		if i >= 5 { // ограничиваем 5 примерами
			break
		}
		sb.WriteString(fmt.Sprintf("Пример %d:\n", i+1))
		sb.WriteString(fmt.Sprintf("Запрос: %s\n", ex.Prompt))
		sb.WriteString(fmt.Sprintf("Ответ: %s\n\n", ex.Code))
	}

	sb.WriteString(fmt.Sprintf("Теперь ответь на запрос: %s", userPrompt))
	return sb.String()
}

func BuildCorrectionPrompt(originalPrompt, badCode, errorMsg string) string {
	result := correctionPrompt
	result = strings.ReplaceAll(result, "{original_request}", originalPrompt)
	result = strings.ReplaceAll(result, "{previous_code_snippet}", badCode)
	result = strings.ReplaceAll(result, "{error_message}", errorMsg)

	errorType := "UNKNOWN_ERROR"
	lowerMsg := strings.ToLower(errorMsg)
	if strings.Contains(lowerMsg, "syntax") || strings.Contains(lowerMsg, "unexpected") {
		errorType = "SYNTAX_ERROR"
	} else if strings.Contains(lowerMsg, "security") || strings.Contains(lowerMsg, "forbidden") {
		errorType = "SECURITY_VIOLATION"
	} else if strings.Contains(lowerMsg, "mws") || strings.Contains(lowerMsg, "jsonpath") {
		errorType = "MWS_VIOLATION"
	} else if strings.Contains(lowerMsg, "timeout") {
		errorType = "TIMEOUT"
	}
	result = strings.ReplaceAll(result, "{error_type}", errorType)

	return result
}

func BuildClarifyPrompt(request string) string {
	return strings.ReplaceAll(clarifyPrompt, "{request}", request)
}

func BuildOptimizationPrompt(code string) string {
	return strings.ReplaceAll(optimizationPrompt, "{code}", code)
}

func BuildSecurityPrompt(request string) string {
	return strings.ReplaceAll(securityPrompt, "{request}", request)
}
func GetSystemPrompt() string {
	return systemPrompt
}

func GetCOTPrompt() string {
	return cotPrompt
}

func GetClarifyPrompt() string {
	return clarifyPrompt
}

func GetOptimizationPrompt() string {
	return optimizationPrompt
}

func GetSecurityPrompt() string {
	return securityPrompt
}

func GetValidationPrompt() string {
	return validationPrompt
}
