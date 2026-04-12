package llm

import (
	_ "embed"
	"strings"
)

var systemPromptRaw string

var correctionPromptRaw string

// GetSystemPrompt возвращает системный промпт
func GetSystemPrompt() string {
	return systemPromptRaw
}

// BuildCorrectionPrompt строит промпт для исправления ошибок
func BuildCorrectionPrompt(originalPrompt, badCode, errorMsg string) string {
	result := correctionPromptRaw
	result = strings.ReplaceAll(result, "{original_request}", originalPrompt)
	result = strings.ReplaceAll(result, "{previous_code_snippet}", badCode)
	result = strings.ReplaceAll(result, "{error_message}", errorMsg)
	return result
}
