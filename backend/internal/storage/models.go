package storage

// DBHistory — модель для PostgreSQL
type DBHistory struct {
    ID              string   `db:"id"`
    SessionID       string   `db:"session_id"`
    Prompt          string   `db:"prompt"`
    Code            string   `db:"code"`
    Explanation     string   `db:"explanation"`
    Plan            []string `db:"plan"`
    Success         bool     `db:"success"`
    ErrorMessage    string   `db:"error_message"`
    ExecutionTimeMs int64    `db:"execution_time_ms"`
    CreatedAt       int64    `db:"created_at"`
}