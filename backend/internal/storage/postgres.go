package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// PostgresRepository — реализация Repository для PostgreSQL
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository создает новый репозиторий с реальным подключением
func NewPostgresRepository(connString string) (*PostgresRepository, error) {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	// Проверка подключения
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	// Создание таблиц
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	fmt.Println("[STORAGE] PostgreSQL connected")
	return &PostgresRepository{db: db}, nil
}

func createTables(db *sql.DB) error {
	query := `
    CREATE TABLE IF NOT EXISTS sessions (
        id TEXT PRIMARY KEY,
        created_at BIGINT NOT NULL
    );

    CREATE TABLE IF NOT EXISTS histories (
        id TEXT PRIMARY KEY,
        session_id TEXT NOT NULL,
        prompt TEXT NOT NULL,
        code TEXT NOT NULL,
        explanation TEXT,
        plan TEXT[],
        success BOOLEAN NOT NULL,
        error_message TEXT,
        execution_time_ms BIGINT NOT NULL,
        created_at BIGINT NOT NULL
    );

    CREATE INDEX IF NOT EXISTS idx_histories_session ON histories(session_id);
    CREATE INDEX IF NOT EXISTS idx_histories_success_created ON histories(success, created_at DESC);
    `
	_, err := db.Exec(query)
	return err
}

// Save сохраняет запись истории
// Save сохраняет запись истории
func (r *PostgresRepository) Save(ctx context.Context, entry *HistoryEntry) error {
	// Убедимся, что сессия существует
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO sessions (id, created_at) VALUES ($1, $2)
        ON CONFLICT (id) DO NOTHING
    `, entry.SessionID, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to ensure session: %w", err)
	}

	// Конвертируем Plan в JSON строку для PostgreSQL
	planJSON := "[]"
	if len(entry.Plan) > 0 {
		planBytes, err := json.Marshal(entry.Plan)
		if err == nil {
			planJSON = string(planBytes)
		}
	}

	query := `
    INSERT INTO histories (id, session_id, prompt, code, explanation, plan, success, error_message, execution_time_ms, created_at)
    VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10)
    `
	_, err = r.db.ExecContext(ctx, query,
		entry.ID, entry.SessionID, entry.Prompt, entry.Code,
		entry.Explanation, planJSON, entry.Success,
		entry.ErrorMessage, entry.ExecutionTimeMs, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}

	return nil
}

// GetRecentSuccess возвращает последние успешные генерации (для few-shot)
func (r *PostgresRepository) GetRecentSuccess(ctx context.Context, sessionID string, limit int) ([]*HistoryEntry, error) {
	query := `
    SELECT id, session_id, prompt, code, explanation, plan, success, error_message, execution_time_ms, created_at
    FROM histories
    WHERE session_id = $1 AND success = true
    ORDER BY created_at DESC
    LIMIT $2
    `
	rows, err := r.db.QueryContext(ctx, query, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent success: %w", err)
	}
	defer rows.Close()

	var entries []*HistoryEntry
	for rows.Next() {
		var entry HistoryEntry
		err := rows.Scan(
			&entry.ID, &entry.SessionID, &entry.Prompt, &entry.Code,
			&entry.Explanation, &entry.Plan, &entry.Success,
			&entry.ErrorMessage, &entry.ExecutionTimeMs, &entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

// GetSessionHistory возвращает историю сессии
func (r *PostgresRepository) GetSessionHistory(ctx context.Context, sessionID string, limit int) ([]*HistoryEntry, error) {
	query := `
    SELECT id, session_id, prompt, code, explanation, plan, success, error_message, execution_time_ms, created_at
    FROM histories
    WHERE session_id = $1
    ORDER BY created_at DESC
    LIMIT $2
    `
	rows, err := r.db.QueryContext(ctx, query, sessionID, limit)
	if err != nil {
		log.Printf("[ERROR] GetSessionHistory query failed: %v", err)
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer rows.Close()

	var entries []*HistoryEntry
	for rows.Next() {
		var entry HistoryEntry
		var plan sql.NullString // ← используем sql.NullString для NULL значений

		err := rows.Scan(
			&entry.ID,
			&entry.SessionID,
			&entry.Prompt,
			&entry.Code,
			&entry.Explanation,
			&plan,
			&entry.Success,
			&entry.ErrorMessage,
			&entry.ExecutionTimeMs,
			&entry.CreatedAt,
		)
		if err != nil {
			log.Printf("[ERROR] GetSessionHistory scan failed: %v", err)
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Конвертируем plan из JSON строки в []string
		if plan.Valid && plan.String != "" {
			var planSlice []string
			if err := json.Unmarshal([]byte(plan.String), &planSlice); err == nil {
				entry.Plan = planSlice
			}
		} else {
			entry.Plan = []string{} // пустой массив вместо NULL
		}

		entries = append(entries, &entry)
	}

	return entries, nil
}

// GetStats возвращает статистику
func (r *PostgresRepository) GetStats(ctx context.Context, sessionID string) (*Stats, error) {
	var total int
	var successCount int
	var avgTime float64

	query := `
SELECT 
    COUNT(*) as total,
    COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END), 0) as success_count,
    COALESCE(AVG(execution_time_ms), 0) as avg_time
FROM histories
WHERE session_id = $1
`
	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(&total, &successCount, &avgTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	successRate := 0.0
	if total > 0 {
		successRate = float64(successCount) / float64(total)
	}

	// Топ ошибок
	errorQuery := `
    SELECT error_message, COUNT(*) as count
    FROM histories
    WHERE session_id = $1 AND success = false AND error_message != ''
    GROUP BY error_message
    ORDER BY count DESC
    LIMIT 5
    `
	rows, err := r.db.QueryContext(ctx, errorQuery, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get top errors: %w", err)
	}
	defer rows.Close()

	var topErrors []ErrorStat
	for rows.Next() {
		var errStat ErrorStat
		if err := rows.Scan(&errStat.Error, &errStat.Count); err != nil {
			return nil, fmt.Errorf("failed to scan error row: %w", err)
		}
		topErrors = append(topErrors, errStat)
	}

	return &Stats{
		TotalGenerations:   total,
		SuccessRate:        successRate,
		AvgExecutionTimeMs: int64(avgTime),
		TopErrors:          topErrors,
	}, nil
}

// Close закрывает соединение с БД
func (r *PostgresRepository) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}
