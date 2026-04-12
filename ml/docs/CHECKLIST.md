> Ручная валидация промптов 

# Prompt Validation Checklist (Manual)

## Compliance Verification
- [x] `system.txt` содержит запрет: markdown, JSON, natural language, comments
- [x] `system.txt` требует: ONLY valid Lua 5.1/5.4 code, compact output
- [x] `security.txt` запрещает: os.execute, io.popen, loadstring, debug.*, socket.*
- [x] `clarify.txt` требует формат: `Clarify: [question]?`
- [x] `few_shot.json` содержит 25 примеров, все без запрещённых паттернов
- [x] Все файлы: UTF-8 encoding, LF line endings, no BOM

## Runtime Verification (executed 2026-04-11)
- [x] Model `mws-agent:latest` responds to `/api/chat` in < 3s
- [x] Output starts with valid Lua keyword: `function|local|return|if|for`
- [x] Output contains no markdown fences (```), no USER:/ASSISTANT: prefixes
- [x] Generated code passes `luac -p` syntax check (verified manually)
- [x] Peak VRAM usage: ~1.9–3.3 GB (≤ 8 GB limit) — verified via `nvidia-smi`

## Integration Contract
- Backend loads `system.txt` once at startup, injects via `/api/chat` messages
- Stop tokens configured: `["```", "USER:", "ASSISTANT:", "\n\n"]`
- Fallback: if validation fails, retry with `correction.txt` (max 2 attempts)
