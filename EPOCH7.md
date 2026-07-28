# Эпоха 7 — Lockdown (P0)

**Цель:** закрыть критические дыры аудита. Без этой эпохи публичный деплой запрещён.  
**Версия:** 2.0.0  
**База:** [AUDIT.md](AUDIT.md) §Критично, [FIXPLAN.md](FIXPLAN.md) P0 #1–6, #20 (vet).

## Сделать

### Конфиг / владелец
- [x] Prod defaults в `.env.example`: `ALLOW_ANON=false`, `DEV_AUTH=false`, `DEMO_TOPUP=false`, `JWT_SECRET=` + комментарий
- [x] Документировать: локальный demo только при явном `ALLOW_ANON=true` (README + .env.example)
- [ ] Ротировать `SUNO_API_KEY`, `MAX_BOT_TOKEN`, `SILICONFLOW_API_KEY` — **действие владельца** (ключи светились в аудите)

### Auth
- [x] `RequireAuth()` на группу `/api/*`
- [x] Исключения: health, metrics, static, `/api/auth/token` (DEV_AUTH), webhook
- [x] Webhook MAX — отдельный `MaxWebhookAuth`
- [x] Пустой `user_id` → 401; `middleware.UserID`; JWT alg = HS256 only

### Закрыть demo-дыры
- [x] `POST /api/bot/simulate` — только `DEV_AUTH=true` + RequireAuth
- [x] `POST /api/credits/topup` — только `DEMO_TOPUP=true` (+ cap 1000)

### Ownership
- [x] `GET /api/jobs/:id` — только владелец (иначе 404)
- [x] `GET /api/playlists/:id/tracks` — `GetTracksForUser`
- [x] `POST /api/tts` — `OwnsVoice`
- [x] voice/clone и elevenlabs/voices — под RequireAuth

### MAX webhook
- [x] `MAX_WEBHOOK_SECRET` + header `X-Max-Bot-Api-Secret` / `?secret=`
- [x] Без секрета → 403/401
- [x] `deploy/HTTPS_WEBHOOK.md` обновлён

### Мелкий техдолг эпохи
- [x] `bot.go` redundant `/help` — vet
- [x] Удалён `downloadFile`
- [x] `/health` version `2.0.0`
- [x] Тесты `middleware` RequireAuth + MaxWebhookAuth

## Не входит
- JWT во фронте (эпоха 8)
- Postgres / Redis (эпоха 9)
- ЮKassa (эпоха 10)

## Приёмка
- [x] `go vet ./...` OK
- [x] `go test ./...` OK
- [x] `go build ./cmd/server` OK
- [ ] Ручная: `ALLOW_ANON=false` без Bearer → 401 (прогнать после рестарта)
- [ ] Ручная: чужой job → 404
- [ ] Ручная: topup без `DEMO_TOPUP` → 403
- [ ] Ручная: webhook без секрета → 401/403

## Статус
✅ Код эпохи 7 закрыт (2026-07-28). Осталось: ротация ключей владельцем + ручная smoke-проверка.
