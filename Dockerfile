# Этап 1: Бэкенд (Go)
FROM golang:1.26-alpine AS backend-builder

WORKDIR /app/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
RUN go build -o /app/bin/agent ./cmd/agent

# Этап 2: Финальный образ на базе Ollama
FROM ollama/ollama:0.5.7

# Устанавливаем дополнительные пакеты
RUN apt-get update && apt-get install -y \
    curl \
    procps \
    && rm -rf /var/lib/apt/lists/*

# Копируем Go-бэкенд
COPY --from=backend-builder /app/bin/agent /app/agent

# Копируем Modelfile
COPY ml/ollama/Modelfile /app/ollama/Modelfile

# Копируем entrypoint скрипт
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Проверяем, что всё на месте
RUN ls -la /app/ && ls -la /app/ollama/

EXPOSE 8080 11434

ENTRYPOINT ["/entrypoint.sh"]