# Mega Agent — AI-генератор Lua кода для MWS Octapi (**Для большей наглядности смотрите презентацию localscript.pdf в корне проекта**)

AI-агент для генерации Lua кода под платформу MWS Octapi. Понимает запросы на русском языке, задаёт уточняющие вопросы, генерирует и валидирует код. Всё работает локально, без отправки данных наружу.

## Быстрый старт

**bash**

git clone <ваш-репозиторий>

cd task-repo

docker compose up -d

Первый запуск длится 10-20 минут (скачивается модель 4.7 GB).

Следите за прогрессом: docker logs -f mega-agent


Готово — открывайте http://localhost

# Требования
Docker Desktop
 
16+ GB RAM

GPU с 4+ GB VRAM (опционально)

# Технологии
LLM: Qwen2.5-Coder 7B через Ollama

Бэкенд: Go

Фронтенд: HTML/CSS/JS

База данных: PostgreSQL с pgvector

Контейнеризация: Docker Compose

# Параметры модели
num_ctx=4096

num_predict=256

temperature=0.1

batch=1

parallel=1

# Примеры запросов
Запрос-Ответ
"напиши функцию сложения двух чисел"	lua{function add(a,b) return a+b end}lua

"получить последний email"	lua{return wf.vars.emails[#wf.vars.emails]}lua

"удали дубликаты из массива"	lua{local r={};local s={};for _,v in ipairs(wf.vars.data) do if not s[v] then s[v]=true;table.insert(r,v) end end;return r}lua

"обработай данные"	

Clarify: какие данные обработать?

"Создай RSI индикатор"

Генерирует полную реализацию RSI

## API
POST /generate

json
{
  "prompt": "напиши функцию сложения",
  "session_id": "test"
}
GET /history?session_id=test — история генераций

GET /stats?session_id=test — статистика

## Остановка
bash

docker compose down
