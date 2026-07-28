# Эпоха 11 — Quality & CI

**Цель:** регрессии ловятся автоматически; аудит не повторяется из-за забытых дыр.  
**Версия:** 2.4.0  
**Зависит от:** эпохи 7–10 (минимум 7–8 для meaningful tests)  
**База:** [AUDIT.md](AUDIT.md) §Средний, [FIXPLAN.md](FIXPLAN.md) P2 #18–21.

## Сделать

### Тесты
- [x] `middleware`: RequireAuth, OptionalAuth (JWT), CSRF, MaxWebhookAuth
- [x] `credits`: Spend atomic / Refund
- [x] `safe_http`: reject http / private IP / non-allowlist
- [x] `safe_files`: path outside root
- [x] API handlers: generate 401, topup guard (httptest)
- [x] `clients` ProviderError + redactBody

### Git / CI
- [x] CI: `go vet`, `go test`, `go build` (`.github/workflows/ci.yml`)
- [ ] Опционально: smoke против поднятого контейнера

### Логи и метрики
- [x] AceData body в логах — truncate/redact (`redactBody`)
- [x] Метрики: JSON `/metrics` (без Prometheus text — ок)

### Документация
- [x] [ACCEPTANCE.md](ACCEPTANCE.md) 2.x
- [x] README / ROADMAP / AUDIT «Закрыто»
- [x] Версия `/health` = 2.4.0

## Приёмка
- [x] `go test ./...` / `go vet ./...`
- [ ] `./scripts/smoke.sh` на локальном сервере (ручная)

## Статус
✅ Закрыта (2.4.0)

## После 11
Новые фичи — эпоха 12+ (отдельный документ). Не раздувать 7–11 scope.
