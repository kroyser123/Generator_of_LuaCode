package api

import (
	"encoding/json"
	"log"
	"mega-agent/internal/agent"
	"mega-agent/internal/storage"
	"net/http"
	"strconv"

	"github.com/google/uuid"
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

	// Используем session_id из запроса или генерируем новый
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// ВЫЗОВ С sessionID (исправлено!)
	result, err := h.agent.Generate(r.Context(), req.Prompt, sessionID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "generation failed", err.Error())
		return
	}

	resp := GenerateResponse{
		Code:               result.Code,
		Explanation:        result.Explanation,
		Plan:               result.Plan,
		Output:             result.Output,
		Success:            result.Success,
		NeedsClarification: result.NeedsClarification,
		Question:           result.Question,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// HistoryHandler — GET /history
func (h *Handler) Fantomas(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	limitStr := r.URL.Query().Get("limit")

	log.Printf("[DEBUG] History request: session_id=%s, limit=%s", sessionID, limitStr)

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	entries, err := h.storage.GetSessionHistory(r.Context(), sessionID, limit)
	if err != nil {
		log.Printf("[ERROR] Failed to get history: %v", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get history", err.Error())
		return
	}

	log.Printf("[DEBUG] Found %d entries for session %s", len(entries), sessionID)

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

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Строим промпт на основе фидбека
	prompt := req.Feedback
	if req.PreviousCode != "" {
		prompt = "На основе этого кода:\n" + req.PreviousCode + "\n\nИсправь по запросу: " + req.Feedback
	}

	// ВЫЗОВ С sessionID
	result, err := h.agent.Generate(r.Context(), prompt, sessionID)
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
	json.NewEncoder(w).Encode(resp)
}
