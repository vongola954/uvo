# Эпоха 9 — Prod foundation

**Цель:** single-node → устойчивый прод-скелет (без смены продукта).  
**Версия:** 2.2.0  
**Зависит от:** эпохи 7–8  
**База:** [AUDIT.md](AUDIT.md) §Высокий #8–12, [FIXPLAN.md](FIXPLAN.md) P1 #9–13.

## Сделать

### Структура кода
- [x] Роуты в `internal/api/web/routes.go`
- [x] Группы: public / webhook / auth (`RequireAuth`)
- [x] Ошибки через `AbortJSON` где применимо

### Кредиты и лимиты
- [x] Атомарный `Spend` (`UPDATE … WHERE balance >= n`)
- [x] `FirstOrCreate` без дублей user_id
- [x] Rate limit в таблице `rate_events` (переживает рестарт; multi-instance — позже Redis)
- [x] Voice clone quota в `voice_clone_events`

### Данные
- [x] `DB_DRIVER=sqlite|postgres` + `DATABASE_URL`
- [x] AutoMigrate в `internal/db`
- [x] SQLite default

### Docker / ops
- [x] Go 1.22 в Dockerfile (= go.mod)
- [x] Non-root user `uvo`
- [x] `go mod download` без `|| true`
- [x] `COPY go.mod go.sum`

### Jobs
- [x] Уникальный `idem_key` (user|requestID)
- [x] Cleanup старых done/failed при старте (7 дней)

## Не входит
- ЮKassa
- Полный соц. продукт

## Приёмка
- [x] `go vet` / `go test` / `go build` OK
- [x] `TestCreditSpendAtomic` — параллельный Spend не уводит в минус
- [x] `/health` → 2.2.0 + `db_driver`
- [ ] `docker compose up --build` — прогнать локально при наличии Docker

## Статус
✅ Код эпохи 9 закрыт (2026-07-28). Дальше — [EPOCH10.md](EPOCH10.md).
