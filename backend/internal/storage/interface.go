package storage

import "context"

type HistoryEntry struct {
    ID              string
    SessionID       string
    Prompt          string
    Code            string
    Explanation     string
    Plan            []string
    Success         bool
    ErrorMessage    string
    ExecutionTimeMs int64
    CreatedAt       int64
    Embedding       []float32 `json:"-"`
}

type Stats struct {
    TotalGenerations   int
    SuccessRate        float64
    AvgExecutionTimeMs int64
    TopErrors          []ErrorStat
}

type ErrorStat struct {
    Error string
    Count int
}

type Repository interface {
    Save(ctx context.Context, entry *HistoryEntry) error
    GetRecentSuccess(ctx context.Context, sessionID string, limit int) ([]*HistoryEntry, error)
    GetSessionHistory(ctx context.Context, sessionID string, limit int) ([]*HistoryEntry, error)
    GetStats(ctx context.Context, sessionID string) (*Stats, error)
    GetFewShotExamples(ctx context.Context, limit int) ([]*HistoryEntry, error)
    FindSimilarByEmbedding(ctx context.Context, embedding []float32, limit int) ([]*HistoryEntry, error)
    UpdateEmbedding(ctx context.Context, id string, embedding []float32) error
    Close() error
}