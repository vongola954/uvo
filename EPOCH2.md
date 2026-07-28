# Эпоха 2 — Надёжность ядра

## Сделано
- SSRF-safe download (HTTPS + allowlist suno/acedata + private IP block)
- Jobs в SQLite (`JobRecord`), переживают рестарт
- Idempotency: `request_id` в POST /api/generate
- Generate/GetJob handlers → `internal/api/web`
- Тесты: provider errors, validate

## Проверка
```bash
go test ./internal/clients/ ./internal/services/
curl -X POST /api/generate -d '{"prompt":"test","request_id":"same-id"}'
# повтор с same request_id → idempotent job
```

## Не сделано (нужен Redis)
- Asynq workers — отложено; goroutine + DB jobs достаточно для single-node
