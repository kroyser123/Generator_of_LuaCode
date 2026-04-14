package api

// GenerateRequest — запрос на генерацию
type GenerateRequest struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id,omitempty"`
}

// GenerateResponse — ответ на генерацию
type GenerateResponse struct {
	Code               string   `json:"code"`
	Explanation        string   `json:"explanation"`
	Plan               []string `json:"plan"`
	Output             string   `json:"output"`
	Success            bool     `json:"success"`
	NeedsClarification bool     `json:"needs_clarification,omitempty"`
	Question           string   `json:"question,omitempty"`
}

// ErrorResponse — ответ с ошибкой
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// HistoryResponse — ответ с историей
type HistoryResponse struct {
	Entries []HistoryEntry `json:"entries"`
}

// HistoryEntry — запись в истории
type HistoryEntry struct {
	ID        string `json:"id"`
	Prompt    string `json:"prompt"`
	Code      string `json:"code"`
	Success   bool   `json:"success"`
	CreatedAt int64  `json:"created_at"`
}

// StatsResponse — ответ со статистикой
type StatsResponse struct {
	TotalGenerations   int         `json:"total_generations"`
	SuccessRate        float64     `json:"success_rate"`
	AvgExecutionTimeMs int64       `json:"avg_execution_time_ms"`
	TopErrors          []ErrorStat `json:"top_errors"`
}

// ErrorStat — статистика по ошибкам
type ErrorStat struct {
	Error string `json:"error"`
	Count int    `json:"count"`
}
