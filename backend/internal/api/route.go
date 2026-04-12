package api

import (
	"encoding/json"
	"mega-agent/backend/internal/agent"
	"mega-agent/backend/internal/storage"
	"net/http"
	"strconv"
)

type Handler struct {
	agent   agent.Agent
	storage storage.Repository
}

// NewHandler создает новый хендлер
func NewHandler(a agent.Agent, s storage.Repository) *Handler {
	return &Handler{
		agent:   a,
		storage: s,
	}
}

// GenerateHandler — POST /generate
func (h *Handler) GenerateHandler(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request", err.Error())
		return
	}

	if req.Prompt == "" {
		h.writeError(w, http.StatusBadRequest, "invalid request", "prompt is required")
		return
	}

	result, err := h.agent.Generate(r.Context(), req.Prompt)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "generation failed", err.Error())
		return
	}

	resp := GenerateResponse{
		Code:        result.Code,
		Explanation: result.Explanation,
		Plan:        result.Plan,
		Output:      result.Output,
		Success:     result.Success,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// HistoryHandler — GET /history
func (h *Handler) HistoryHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	limitStr := r.URL.Query().Get("limit")

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	entries, err := h.storage.GetSessionHistory(r.Context(), sessionID, limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to get history", err.Error())
		return
	}

	resp := HistoryResponse{
		Entries: make([]HistoryEntry, 0, len(entries)),
	}
	for _, e := range entries {
		resp.Entries = append(resp.Entries, HistoryEntry{
			ID:        e.ID,
			Prompt:    e.Prompt,
			Code:      e.Code,
			Success:   e.Success,
			CreatedAt: e.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// StatsHandler — GET /stats
func (h *Handler) StatsHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")

	stats, err := h.storage.GetStats(r.Context(), sessionID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to get stats", err.Error())
		return
	}

	resp := StatsResponse{
		TotalGenerations:   stats.TotalGenerations,
		SuccessRate:        stats.SuccessRate,
		AvgExecutionTimeMs: stats.AvgExecutionTimeMs,
		TopErrors:          make([]ErrorStat, 0, len(stats.TopErrors)),
	}
	for _, e := range stats.TopErrors {
		resp.TopErrors = append(resp.TopErrors, ErrorStat{
			Error: e.Error,
			Count: e.Count,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   message,
		Message: detail,
	})
}

// FeedbackHandler — POST /feedback
func (h *Handler) FeedbackHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID    string `json:"session_id"`
		Feedback     string `json:"feedback"`
		PreviousCode string `json:"previous_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request", err.Error())
		return
	}

	// TODO: реализовать логику исправления
	// Пока возвращаем заглушку
	resp := map[string]interface{}{
		"code":              "-- Исправленный код\nfunction main()\n    print('fixed')\nend",
		"explanation":       "Код исправлен на основе фидбека",
		"success":           true,
		"execution_time_ms": 500,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
