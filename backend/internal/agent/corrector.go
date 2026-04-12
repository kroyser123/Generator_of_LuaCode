package agent

// BuildCorrectionPrompt строит промпт для исправления ошибок
func BuildCorrectionPrompt(task, code, errorMsg string) string {
	return "The following Lua code failed.\n\nTask: " + task + "\n\nCode:\n" + code + "\n\nError:\n" + errorMsg + "\n\nPlease fix it. Return ONLY JSON with code, explanation, plan."
}
