FROM ollama/ollama:0.5.7

# Установка curl для healthcheck
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

# Копируем скрипт инициализации
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Точка входа - наш скрипт
ENTRYPOINT ["/entrypoint.sh"]
# Необходим нам для того чтобы не логировать каждую сикунду 5s оптимален не перегружает диск
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD curl -f http://localhost:11434/api/tags || exit 1