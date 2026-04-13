#!/bin/bash
set -e

echo "========================================="
echo "[ENTRYPOINT] Starting Mega Agent"
echo "========================================="

# Запускаем Ollama в фоне
echo "[ENTRYPOINT] Starting Ollama server..."
ollama serve &
OLLAMA_PID=$!

# Ждём готовности Ollama API
echo "[ENTRYPOINT] Waiting for Ollama API to be ready..."
MAX_RETRIES=30
RETRY_COUNT=0
while ! curl -s http://localhost:11434/api/tags > /dev/null 2>&1; do
    sleep 1
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        echo "[ENTRYPOINT] ERROR: Ollama API failed to start"
        exit 1
    fi
    if [ $((RETRY_COUNT % 5)) -eq 0 ]; then
        echo "[ENTRYPOINT] Still waiting for Ollama... (${RETRY_COUNT}s)"
    fi
done
echo "[ENTRYPOINT] Ollama API is ready!"

# Проверяем, существует ли модель
echo "[ENTRYPOINT] Checking if model 'mws-agent' exists..."
if ollama list | grep -q "mws-agent"; then
    echo "[ENTRYPOINT] Model 'mws-agent' already exists. Skipping creation."
else
    echo "[ENTRYPOINT] Creating model 'mws-agent' from /app/ollama/Modelfile..."
    ollama create mws-agent -f /app/ollama/Modelfile
    echo "[ENTRYPOINT] Model created successfully!"
fi

echo "[ENTRYPOINT] Model 'mws-agent' is ready!"
echo "========================================="
echo "[ENTRYPOINT] Starting Go backend..."
echo "========================================="

# Запускаем Go-бэкенд (он сам будет ждать PostgreSQL через retry в коде)
exec /app/agent