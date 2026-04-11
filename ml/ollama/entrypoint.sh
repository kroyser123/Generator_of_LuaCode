#!/bin/bash
set -e

# === Конфигурация логирования ===
VRAM_LOG="/var/log/vram.log"
LOG_ROTATE_MAX_SIZE=10485760  # 10 MB
LOG_ROTATE_MAX_FILES=5

# === Функция ротации логов ===
rotate_log() {
    if [ -f "$VRAM_LOG" ] && [ $(stat -f%z "$VRAM_LOG" 2>/dev/null || stat -c%s "$VRAM_LOG" 2>/dev/null) -gt $LOG_ROTATE_MAX_SIZE ]; then
        for i in $(seq $((LOG_ROTATE_MAX_FILES-1)) -1 1); do
            [ -f "${VRAM_LOG}.$i" ] && mv "${VRAM_LOG}.$i" "${VRAM_LOG}.$((i+1))"
        done
        mv "$VRAM_LOG" "${VRAM_LOG}.1"
        touch "$VRAM_LOG"
    fi
}

# === Функция очистки при завершении ===
cleanup() {
    echo "[$(date -Iseconds)] Shutting down VRAM monitor..." >> "$VRAM_LOG"
    pkill -P $$ -f "nvidia-smi" 2>/dev/null || true
    exit 0
}
trap cleanup SIGTERM SIGINT EXIT

# === Запускаем Ollama в фоне ===
ollama serve &
SERVER_PID=$!

# === Ждём готовности сервера ===
echo "Waiting for Ollama server to be ready..."
until curl -s http://localhost:11434/api/tags > /dev/null 2>&1; do
    sleep 1
done
echo "[$(date -Iseconds)] Ollama server ready" >> "$VRAM_LOG"

# === Запускаем мониторинг VRAM в фоне ===
(
    echo "[$(date -Iseconds)] VRAM monitor started (PID: $BASHPID)" >> "$VRAM_LOG"
    while true; do
        TIMESTAMP=$(date -Iseconds)
        if command -v nvidia-smi &> /dev/null; then
            VRAM=$(nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits -i 0 2>/dev/null | tr -d ' ')
            GPU_UTIL=$(nvidia-smi --query-gpu=utilization.gpu --format=csv,noheader,nounits -i 0 2>/dev/null | tr -d ' ')
            echo "[$TIMESTAMP] VRAM: ${VRAM}MiB | GPU: ${GPU_UTIL}%" >> "$VRAM_LOG"
        fi
        rotate_log
        sleep 5
    done
) &
MONITOR_PID=$!

# === Создаём модель, если отсутствует ===
# ← ИСПРАВЛЕНО: новый путь после монтирования директории
MODELPATH="/app/ollama/Modelfile"

# ← ДОБАВЛЕНО: проверка существования файла перед созданием модели
if [ ! -f "$MODELPATH" ]; then
    echo "[$(date -Iseconds)] ERROR: Modelfile not found at $MODELPATH" >> "$VRAM_LOG"
    ls -la /app/ollama/ >> "$VRAM_LOG" 2>&1 || true
    exit 1
fi

if ! ollama list 2>/dev/null | grep -q "mws-agent"; then
    echo "[$(date -Iseconds)] Creating mws-agent from $MODELPATH..." >> "$VRAM_LOG"
    ollama create mws-agent -f "$MODELPATH"
else
    echo "[$(date -Iseconds)] Model mws-agent already exists." >> "$VRAM_LOG"
fi

# === Логирование пикового потребления ===
echo "[$(date -Iseconds)] === Generation session started ===" >> "$VRAM_LOG"

# === Возвращаем управление основному процессу ===
wait $SERVER_PID