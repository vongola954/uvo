# Аудит UVO 1.9 — 2026-07-28

Жёсткий аудит кодовой базы. Ориентир для эпох 7+.  
Исторические эпохи 1–6: AceData → Reliability → Security (частично) → MAX → Product UX → Полировка.

## Вердикт

| Критерий | Оценка | Комментарий |
|----------|--------|-------------|
| Архитектура | 6/10 | Слои есть, `main.go` — монолит маршрутов |
| Безопасность | **3/10** | Auth optional, demo-дыры, секреты в `.env` |
| Надёжность | 5/10 | Refund/jobs/pool ок; SQLite + in-memory limits |
| Тесты | **2/10** | 2 пакета, почти нет покрытия |
| Prod-ready | **4/10** | `ALLOW_ANON`/`DEV_AUTH` в demo-режиме |
| UX/фронт | 7/10 | UI сильный, auth/CSRF не подключены |

**Итог: 4.5/10** — рабочий MVP для локалки. **Нельзя выставлять в интернет как есть.**

Сборка: `go build ./cmd/server` OK.  
`go test` / `go vet`: FAIL — redundant `or` в `internal/api/bot/bot.go:129`.

---

## Критично (P0)

### 1. Секреты и demo-конфиг
- В `.env` реальные ключи (AceData/Suno, MAX, SiliconFlow) и слабый `JWT_SECRET`.
- `DEV_AUTH=true`, `ALLOW_ANON=true` → JWT для любого `user_id` + все запросы как `demo_user`.
- **Действие:** ротация всех ключей; prod: `ALLOW_ANON=false`, `DEV_AUTH=false`, длинный `JWT_SECRET`.

### 2. `RequireAuth` не используется
- Объявлен в `middleware/auth.go`, нигде не повешен.
- При `ALLOW_ANON=false` `user_id=""` — запросы не блокируются, работают от пустого пользователя.

### 3. Бесплатный topup
- `POST /api/credits/topup` без оплаты, без жёсткой auth, без prod-guard.

### 4. Открытые опасные эндпоинты

| Эндпоинт | Риск |
|----------|------|
| `POST /api/bot/simulate` | Генерация от чужого `user_id`, трата AceData |
| `POST /api/max/webhook` | Нет проверки подписи MAX |
| `GET /api/jobs/:id` | Нет проверки владельца |
| `GET /api/playlists/:id/tracks` | Нет проверки владельца |
| `POST /api/voice/clone`, `POST /api/tts` | Слабая/нет auth; TTS без ownership voice |
| `GET /api/elevenlabs/voices` | Прокси без auth |

### 5. CSRF частичный
Защищены: feed, playlists, delete/edit tracks.  
Не защищены: generate, topup, voice, TTS, simulate, webhook.

### 6. XSS во фронте
`innerHTML` с `Title`/`Caption` в `tracks.html`, `feed.html`, `playlists.html`.

### 7. Мёртвый небезопасный код
`GenerationService.downloadFile` — голый `http.Get` без allowlist (не вызывается, но опасен).

---

## Высокий (P1)

8. Монолитный `main.go` (~430 строк роутинга).  
9. Мёртвые модели: `Like`, `Comment`, `Subscription`, `License`, `Referral`; `IsPublic` нигде не ставится в `true`.  
10. Rate limit / voice quota — in-memory, сброс при рестарте.  
11. SQLite — single-node only.  
12. Race в `CreditService.ensure` (дубликаты баланса).  
13. Job без ownership check.

---

## Средний (P2)

14. Почти нет тестов; vet fail.  
15. Нет git-репозитория / CI.  
16. Фронт без `Authorization` / CSRF (завязан на `ALLOW_ANON`).  
17. Docker: Go 1.23 vs go.mod 1.22; нет non-root; `go mod download || true`.  
18. Debug-логи тел AceData.  
19. Дубль условия `/help` в bot.go.

---

## Что уже хорошо

1. `SafeDownload` — HTTPS, allowlist, block private IP, max size  
2. `SafeMediaPath` — path traversal  
3. Refund кредитов при ошибке провайдера  
4. Idempotency `request_id` + jobs  
5. Worker pool (`MAX_WORKERS`)  
6. `ProviderError` с понятными кодами  
7. Валидация generate / voice / TTS  
8. `RecoveryJSON`  
9. Async UI + polling jobs  

---

## Соответствие ACCEPTANCE.md

| Пункт | Статус |
|-------|--------|
| `go build` | OK |
| `go test ./internal/...` | FAIL (vet bot) |
| smoke.sh | не прогнан в аудите |
| ALLOW_ANON / DEV_AUTH / JWT prod | FAIL в текущем `.env` |
| HTTPS + MAX webhook | только polling |
| ЮKassa | demo topup |

---

## Документы плана

- [ROADMAP.md](ROADMAP.md) — сводка эпох 7+  
- [FIXPLAN.md](FIXPLAN.md) — приоритеты P0→P2  
- [EPOCH7.md](EPOCH7.md) … [EPOCH11.md](EPOCH11.md) — исполнение по эпохам  

**Правило:** при изменениях кода ориентироваться на этот аудит и FIXPLAN; закрывать чеклисты эпох, обновлять ACCEPTANCE.

---

## Закрыто в эпохах

### Эпоха 7 (2.0.0) — частично закрыт P0
- RequireAuth на `/api/*`
- simulate / topup под флагами
- Job / playlist / TTS ownership
- MAX webhook secret
- vet bot + удалён downloadFile
- **Не закрыто владельцем:** ротация ключей в `.env`

### Эпоха 8 (2.1.0)
- JWT + CSRF во фронте (`auth.js`)
- XSS: textContent / UVO.el
- CSRF расширен на generate/voice/tts/topup

### Эпоха 9 (2.2.0)
- Роуты вынесены в `routes.go`
- Atomic credits + FirstOrCreate
- Rate/voice quotas в DB
- Optional Postgres (`DB_DRIVER`)
- Docker non-root, Go 1.22
- Job `idem_key` + cleanup

