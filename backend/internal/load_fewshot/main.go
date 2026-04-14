package main

import (
	"context"
	"encoding/json"
	"log"
	"mega-agent/internal/storage"
	"os"
	"time"

	"github.com/google/uuid"
)

type FewShot struct {
	Examples []struct {
		Request string `json:"request"`
		Code    string `json:"code"`
	} `json:"examples"`
}

func main() {
	// Читаем файл
	data, err := os.ReadFile("ml/prompts/few_shot.json")
	if err != nil {
		log.Fatalf("Failed to read few_shot.json: %v", err)
	}

	var fs FewShot
	if err := json.Unmarshal(data, &fs); err != nil {
		log.Fatalf("Failed to parse few_shot.json: %v", err)
	}

	// Подключаемся к БД
	dbConnString := "postgres://postgres:postgres@localhost:5432/mega_agent?sslmode=disable"
	repo, err := storage.NewPostgresRepository(dbConnString)
	if err != nil {
		log.Fatalf("Failed to connect to storage: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Загружаем примеры
	for i, ex := range fs.Examples {
		entry := &storage.HistoryEntry{
			ID:              uuid.New().String(),
			SessionID:       "fewshot",
			Prompt:          ex.Request,
			Code:            ex.Code,
			Explanation:     "",
			Plan:            []string{},
			Success:         true,
			ErrorMessage:    "",
			ExecutionTimeMs: 0,
			CreatedAt:       time.Now().Unix(),
		}
		if err := repo.Save(ctx, entry); err != nil {
			log.Printf("Failed to save example %d: %v", i, err)
		} else {
			log.Printf("Saved example %d: %s", i+1, ex.Request)
		}
	}

	log.Println("Done! Loaded", len(fs.Examples), "examples")
}
