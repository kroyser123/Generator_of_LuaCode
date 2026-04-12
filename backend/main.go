package main

import (
	"context"
	"log"
	"mega-agent/backend/internal/agent"
	"mega-agent/backend/internal/api"
	"mega-agent/backend/internal/llm"
	"mega-agent/backend/internal/storage"
	"mega-agent/backend/internal/validator"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	llmEndpoint := "http://localhost:11434"
	llmModel := "llama3.1:8b"
	dbConnString := "postgres://postgres:postgres@localhost:5432/mega_agent?sslmode=disable"
	sessionID := "test-session-001"
	serverPort := ":8080"

	log.Println("INIT Connecting to storage...")
	repo, err := storage.NewPostgresRepository(dbConnString)
	if err != nil {
		log.Fatalf("Failed to connect to storage: %v", err)
	}
	defer repo.Close()
	log.Println("INIT Storage connected")

	log.Println("INIT Initializing LLM client")
	embedder := llm.NewMockEmbedder(384)
	rag := llm.NewMockRAG()
	llmClient := llm.NewService(llmEndpoint, llmModel, embedder, rag)
	log.Println("INIT LLM client ready")

	log.Println("INIT Initializing validator...")
	val := validator.NewPipeline(5 * time.Second)
	log.Println("INIT Validator ready")

	log.Println("[INIT] Initializing agent...")
	ag := agent.NewAgent(llmClient, val, repo, sessionID)
	log.Println("[INIT] Agent ready")

	handler := api.NewHandler(ag, repo)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /generate", handler.GenerateHandler)
	mux.HandleFunc("POST /feedback", handler.FeedbackHandler)
	mux.HandleFunc("GET /history", handler.HistoryHandler)
	mux.HandleFunc("GET /stats", handler.StatsHandler)

	server := &http.Server{
		Addr:    serverPort,
		Handler: mux,
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
