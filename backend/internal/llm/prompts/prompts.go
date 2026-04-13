package prompts

import (
	_ "embed"
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

func GetSystemPrompt() string {
	return systemPrompt
}

func BuildCorrectionPrompt(originalPrompt, badCode, errorMsg string) string {
	result := correctionPrompt

	// Исправляем: заменяем все плейсхолдеры
	result = strings.ReplaceAll(result, "{original_request}", originalPrompt)
	result = strings.ReplaceAll(result, "{previous_code_snippet}", badCode)
	result = strings.ReplaceAll(result, "{error_message}", errorMsg)

	// Определяем тип ошибки
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
