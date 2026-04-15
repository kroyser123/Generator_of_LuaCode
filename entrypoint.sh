#!/bin/bash
set -e

echo "========================================="
echo "[ENTRYPOINT] Starting Mega Agent"
echo "========================================="

# Запускаем Ollama в фоне
ollama serve &
OLLAMA_PID=$!

# Ждём готовности Ollama API
echo "[ENTRYPOINT] Waiting for Ollama API to be ready..."
MAX_RETRIES=60
RETRY_COUNT=0
while ! curl -s http://localhost:11434/api/tags > /dev/null 2>&1; do
    sleep 2
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        echo "[ENTRYPOINT] ERROR: Ollama API failed to start"
        exit 1
    fi
    if [ $((RETRY_COUNT % 10)) -eq 0 ]; then
        echo "[ENTRYPOINT] Still waiting for Ollama... (${RETRY_COUNT*2}s)"
    fi
done
echo "[ENTRYPOINT] Ollama API is ready!"

# Создаём модель
echo "[ENTRYPOINT] Creating model 'mws-agent'..."
ollama rm mws-agent 2>/dev/null || true
ollama create mws-agent -f /app/ollama/Modelfile
echo "[ENTRYPOINT] Model created successfully!"

echo "========================================="
echo "[ENTRYPOINT] Starting Go backend..."
echo "========================================="

exec /app/agent
