package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func main() {
	// Оборачиваем все обработчики в middleware для CORS
	http.HandleFunc("/generate", enableCORS(handleGenerate))
	http.HandleFunc("/feedback", enableCORS(handleFeedback))
	http.HandleFunc("/history", enableCORS(handleHistory))
	http.HandleFunc("/stats", enableCORS(handleStats))

	log.Println("Mock server starting on :8080")
	log.Println("CORS enabled - accepting requests from any origin")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

// Middleware для добавления CORS заголовков
func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Добавляем CORS заголовки
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400") // 24 часа кеширования preflight запросов

		// Обрабатываем preflight запрос (OPTIONS)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Вызываем оригинальный обработчик
		next(w, r)
	}
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt    string `json:"prompt"`
		SessionID string `json:"session_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Имитация задержки
	time.Sleep(500 * time.Millisecond)

	resp := map[string]interface{}{
		"code":                "-- Сгенерированный код для: " + req.Prompt + "\nfunction main()\n    print('hello world')\nend",
		"explanation":         "Тестовая генерация (мок)",
		"plan":                []string{"Анализ задачи", "Генерация кода", "Валидация"},
		"success":             true,
		"execution_time_ms":   523,
		"needs_clarification": false,
		"question":            nil,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleFeedback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID    string `json:"session_id"`
		Feedback     string `json:"feedback"`
		PreviousCode string `json:"previous_code"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	resp := map[string]interface{}{
		"code":              "-- Исправленный код\nfunction main()\n    print('fixed')\nend",
		"explanation":       "Код исправлен на основе фидбека",
		"success":           true,
		"execution_time_ms": 234,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"entries": []map[string]interface{}{
			{"id": "1", "prompt": "Функция сложения", "code": "function add(a,b) return a+b end", "success": true, "execution_time_ms": 123, "created_at": time.Now().Add(-2 * time.Hour).Format(time.RFC3339)},
			{"id": "2", "prompt": "RSI индикатор", "code": "function rsi() end", "success": true, "execution_time_ms": 456, "created_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339)},
			{"id": "3", "prompt": "Очистка данных", "code": "-- ошибка", "success": false, "execution_time_ms": 789, "created_at": time.Now().Format(time.RFC3339)},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"total_generations":     42,
		"success_rate":          0.85,
		"avg_execution_time_ms": 2345,
		"top_errors": []map[string]interface{}{
			{"error": "syntax error", "count": 5},
			{"error": "security violation", "count": 2},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
