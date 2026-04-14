package main

import (
	"context"
	"log"
	"mega-agent/internal/agent"
	"mega-agent/internal/api"
	"mega-agent/internal/llm"
	"mega-agent/internal/storage"
	"mega-agent/internal/validator"
	"net/http"
	"os"
	"os/signal"

	"syscall"
	"time"
)

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Устанавливаем заголовки ДО обработки запроса
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Логируем для отладки
		log.Printf("[CORS] Method=%s, Path=%s", r.Method, r.URL.Path)

		// Обрабатываем preflight (OPTIONS) запрос
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Вызываем следующий handler
		next(w, r)
	}
}
func main() {
	// Конфигурация из переменных окружения
	llmEndpoint := getEnv("OLLAMA_ENDPOINT", "http://localhost:11434")
	llmModel := getEnv("MODEL_NAME", "mws-agent:latest")
	dbConnString := getEnv("DB_CONN_STRING", "postgres://postgres:postgres@postgres:5432/mega_agent?sslmode=disable")
	sessionID := getEnv("SESSION_ID", "default-session")
	serverPort := getEnv("SERVER_PORT", ":8080")

	log.Printf("[INIT] LLM Endpoint: %s", llmEndpoint)
	log.Printf("[INIT] Model: %s", llmModel)
	log.Printf("[INIT] DB Connection: postgres://postgres:***@postgres:5432/mega_agent")
	log.Printf("[INIT] Server port: %s", serverPort)

	// 1. Storage с retry
	log.Println("[INIT] Connecting to storage...")
	var repo storage.Repository
	var err error

	for i := 1; i <= 10; i++ {
		repo, err = storage.NewPostgresRepository(dbConnString)
		if err == nil {
			break
		}
		log.Printf("[INIT] Waiting for PostgreSQL... (attempt %d/10): %v", i, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to storage after 10 attempts: %v", err)
	}
	defer repo.Close()
	log.Println("[INIT] Storage connected")

	// 2. LLM Client
	log.Println("[INIT] Initializing LLM client...")
	llmClient := llm.NewService(llmEndpoint, llmModel)
	log.Println("[INIT] LLM client ready")

	// 3. Validator
	log.Println("[INIT] Initializing validator...")
	val := validator.NewPipeline(5 * time.Second)
	log.Println("[INIT] Validator ready")

	// 4. Agent
	log.Println("[INIT] Initializing agent...")
	ag := agent.NewAgent(llmClient, val, repo, sessionID)
	log.Println("[INIT] Agent ready")

	// 5. API Handler
	handler := api.NewHandler(ag, repo)

	// 6. HTTP Server
	mux := http.NewServeMux()

	// Убираем метод из пути (разрешаем любые методы)
	mux.HandleFunc("/generate", handler.GenerateHandler)
	mux.HandleFunc("/history", handler.HistoryHandler)
	mux.HandleFunc("/stats", handler.StatsHandler)
	mux.HandleFunc("/feedback", handler.FeedbackHandler)

	// CORS обёртка
	corsHandler := func(w http.ResponseWriter, r *http.Request) {
		// Устанавливаем CORS заголовки для ВСЕХ ответов
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Логируем
		log.Printf("[CORS] Method=%s, Path=%s", r.Method, r.URL.Path)

		// Для OPTIONS сразу возвращаем OK
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		mux.ServeHTTP(w, r)
	}

	server := &http.Server{
		Addr:    serverPort,
		Handler: http.HandlerFunc(corsHandler),
	}

	go func() {
		log.Printf("[INIT] Server starting on %s", serverPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[SHUTDOWN] Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("[SHUTDOWN] Server stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
