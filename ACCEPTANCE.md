# Чеклист приёмки UVO

Ориентиры: [AUDIT.md](AUDIT.md) · [FIXPLAN.md](FIXPLAN.md) · [ROADMAP.md](ROADMAP.md)

## 1.9 (эпохи 1–6) — baseline

### Обязательно
- [ ] `go build ./cmd/server`
- [ ] `go test ./internal/...`
- [ ] `./scripts/smoke.sh http://127.0.0.1:8010`
- [ ] AceData баланс > 0 на platform.acedata.cloud
- [ ] POST /api/generate → job → play_url
- [ ] used_up → refund кредита
- [ ] MAX: /me в логах при старте (токен валиден)

### Security (заявлено в EPOCH3; факт — см. аудит)
- [ ] ALLOW_ANON=false в prod
- [ ] DEV_AUTH=false в prod
- [ ] JWT_SECRET длинный

### MAX
- [ ] Long poll dev OK
- [ ] Webhook URL для prod: POST /api/max/webhook

### Не 10/10 без
- Живого баланса AceData
- HTTPS + webhook MAX в проде
- ЮKassa (опционально)

---

## 2.x (эпохи 7–11) — после аудита

### Эпоха 7 Lockdown
- [ ] `go vet` / `go test` зелёные
- [ ] RequireAuth на `/api/*`; simulate/topup под guard
- [ ] Job/playlist/TTS ownership; webhook secret

### Эпоха 8 Auth UX
- [ ] Фронт с Bearer + CSRF; XSS закрыт

### Эпоха 9 Prod foundation
- [ ] Atomic credits; Docker non-root; роуты вынесены

### Эпоха 10 Product & pay
- [ ] IsPublic работает; topup = DEMO или ЮKassa

### Эпоха 11 Quality
- [ ] CI + расширенные тесты; ACCEPTANCE 2.x полный проход

Детали чеклистов — в `EPOCH7.md` … `EPOCH11.md`.
