-- Таблица сессий
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    created_at BIGINT NOT NULL
);

-- Таблица истории
CREATE TABLE IF NOT EXISTS histories (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    prompt TEXT NOT NULL,
    code TEXT NOT NULL,
    explanation TEXT,
    plan JSONB,  -- ← меняем с TEXT[] на JSONB
    success BOOLEAN NOT NULL,
    error_message TEXT,
    execution_time_ms BIGINT NOT NULL,
    created_at BIGINT NOT NULL
);

-- Индексы
CREATE INDEX IF NOT EXISTS idx_histories_session ON histories(session_id);
CREATE INDEX IF NOT EXISTS idx_histories_success_created ON histories(success, created_at DESC);