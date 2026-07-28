# Эпоха 11 — Quality & CI

**Цель:** регрессии ловятся автоматически; аудит не повторяется из-за забытых дыр.  
**Версия:** 2.4.0  
**Зависит от:** эпохи 7–10 (минимум 7–8 для meaningful tests)  
**База:** [AUDIT.md](AUDIT.md) §Средний, [FIXPLAN.md](FIXPLAN.md) P2 #18–21.

## Сделать

### Тесты
- [ ] `middleware`: RequireAuth, OptionalAuth, CSRF
- [ ] `credits`: Spend/Refund/atomic
- [ ] `safe_http`: reject http / private IP / non-allowlist
- [ ] `safe_files`: path outside root
- [ ] API handlers: generate 401, job ownership, topup guard (httptest)
- [ ] `clients` ProviderError (уже есть — расширить)

### Git / CI
- [ ] Инициализировать git (если ещё нет) — по запросу владельца
- [ ] CI: `go vet`, `go test`, `go build`
- [ ] Опционально: smoke против поднятого контейнера

### Логи и метрики
- [ ] Не логировать полные body AceData на info/debug в prod (redact / truncate)
- [ ] Метрики: оставить JSON `/metrics` или добавить Prometheus text (опционально)

### Документация
- [ ] Обновить [ACCEPTANCE.md](ACCEPTANCE.md) под 2.x
- [ ] README: ссылка на ROADMAP / AUDIT
- [ ] Отметить закрытые пункты в [AUDIT.md](AUDIT.md) секцией «Закрыто в эпохах»

### Чистота
- [ ] Нет мёртвого кода downloadFile / unused models (после выбора A/B в эпохе 10)
- [ ] Версия `/health` = 2.4.0

## Приёмка
- [ ] `go test ./...` зелёный в CI
- [ ] `go vet ./...` зелёный
- [ ] `./scripts/smoke.sh` на локальном сервере
- [ ] ACCEPTANCE 2.x чеклист пройден

## Статус
⏳ Ждёт предыдущие эпохи

## После 11
Новые фичи — эпоха 12+ (отдельный документ). Не раздувать 7–11 scope.
