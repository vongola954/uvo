# План исправлений (после аудита 2026-07-28)

Источник: [AUDIT.md](AUDIT.md). Исполнение — эпохи [EPOCH7](EPOCH7.md)–[EPOCH11](EPOCH11.md).  
Порядок строгий: **не начинать P1/P2 до закрытия P0**, если нет явного исключения.

---

## P0 — до любого публичного деплоя

| # | Задача | Где | Эпоха |
|---|--------|-----|-------|
| 1 | Ротация ключей; `ALLOW_ANON=false`, `DEV_AUTH=false`, сильный JWT | `.env` (владелец) | 7 |
| 2 | `RequireAuth` на все `/api/*` кроме health, metrics, static, auth (ограниченно), webhook | `main.go`, middleware | 7 |
| 3 | Закрыть / удалить `POST /api/bot/simulate` (или `DEV_AUTH` + admin) | `main.go` | 7 |
| 4 | `POST /api/credits/topup` — только demo под guard или выключить | `main.go` | 7 |
| 5 | Ownership: jobs, playlist tracks, TTS voice | web/jobs, playlist, voice | 7 |
| 6 | Проверка webhook MAX (secret / signature / shared token) | bot webhook | 7 |
| 7 | XSS: без `innerHTML` с пользовательскими строками | static HTML | 8 |
| 8 | CSRF на generate / voice / topup (если cookie-сессии) | csrf.go + фронт | 8 |

## P1 — prod foundation

| # | Задача | Эпоха |
|---|--------|-------|
| 9 | Разбить роуты из `main.go` | 9 |
| 10 | Postgres (опционально dual-driver) | 9 |
| 11 | Redis / DB rate limit + durable quotas | 9 |
| 12 | Атомарный Spend/ensure кредитов | 9 |
| 13 | Non-root Docker, выровнять Go version | 9 |
| 14 | JWT + CSRF во фронте студии | 8 |

## P2 — продукт и качество

| # | Задача | Эпоха |
|---|--------|-------|
| 15 | IsPublic API + фикс ленты/поиска | 10 |
| 16 | Likes/comments или убрать из моделей/UI | 10 |
| 17 | ЮKassa (или явная «оплата позже») | 10 |
| 18 | Тесты auth/credits/SafeDownload/API | 11 |
| 19 | Git + CI (`test`, `vet`, smoke) | 11 |
| 20 | Удалить `downloadFile`, починить vet bot | 7/11 |
| 21 | Structured logs без тел провайдера | 11 |

---

## Definition of Done (глобально)

- [ ] `go vet ./...` и `go test ./...` зелёные  
- [ ] `ALLOW_ANON=false` + `DEV_AUTH=false` в prod-примере  
- [ ] Нет открытого topup / simulate без явного demo-флага  
- [ ] Job/playlist/play — только владелец (или public)  
- [ ] Фронт работает с Bearer (и CSRF где нужно)  
- [ ] [ACCEPTANCE.md](ACCEPTANCE.md) обновлён под 2.x  

---

## Вне кода (владелец)

1. Пополнить AceData: https://platform.acedata.cloud  
2. Ротировать ключи после любого копирования `.env`  
3. HTTPS + `BOT_MODE=webhook` для MAX в проде (`deploy/`)  
