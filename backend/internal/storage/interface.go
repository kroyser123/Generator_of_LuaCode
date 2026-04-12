package storage

import "context"

// HistoryEntry — запись истории (доменная модель)
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
}

// Stats — статистика
type Stats struct {
    TotalGenerations   int
    SuccessRate        float64
    AvgExecutionTimeMs int64
    TopErrors          []ErrorStat
}

// ErrorStat — статистика по ошибкам
type ErrorStat struct {
    Error string
    Count int
}

// Repository — интерфейс хранилища
type Repository interface {
    Save(ctx context.Context, entry *HistoryEntry) error
    GetRecentSuccess(ctx context.Context, sessionID string, limit int) ([]*HistoryEntry, error)
    GetSessionHistory(ctx context.Context, sessionID string, limit int) ([]*HistoryEntry, error)
    GetStats(ctx context.Context, sessionID string) (*Stats, error)
    Close() error
}