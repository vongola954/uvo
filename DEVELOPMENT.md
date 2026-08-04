# UVO — ключевые моменты разработки

Единый living-документ. Исторические EPOCH / AUDIT / FIXPLAN лежат в [`docs/archive/`](docs/archive/).

| | |
|--|--|
| **Версия** | **2.7.6** |
| **Prod** | https://uvo-baskakovanton.amvera.io |
| **GitHub** | https://github.com/vongola954/uvo (`origin`) |
| **Deploy** | Amvera remote `amvera` → `git push amvera master` |
| **Стек** | Go · Gin · AceData/Suno · MAX bot · Web studio · Postgres (Amvera CNPG) / SQLite local |

---

## 1. Продукт

- **UVO** — AI-студия музыки: генерация, клон голоса, кавер, караоке, singing portrait.
- Вход: MAX-бот → мини-приложение **«Запуск»** (внутри MAX) или `/login` → one-time code → cookie.
- Режимы создания: **Идея** / **Свой текст** / **Instrumental** (`mode` в `POST /api/generate`).
- Страницы: `/`, `/tracks.html`, `/feed.html`, `/playlists.html`, `/karaoke.html`.

### Кредиты и цены

| Op | Кредиты |
|----|--------:|
| Generate / Edit / TTS | 1 |
| Cover / Karaoke / Voice clone | 2 |
| Portrait | 2–3 |

| Пакет | Кредиты | ₽ |
|-------|--------:|--:|
| free (грант) | 2 | 0 |
| pack5 (вход) | 5 | 99 |
| pack10 | 10 | 199 |
| pack30 | 30 | 499 |
| pack100 | 100 | 699 |
| pack500 | 500 | 1690 |
| pack2000 | 2000 | 5990 |

Правило копирайта: **1 кредит = 1 песня**, от **99 ₽**. Unit-economics и dual-gate → [`COST.md`](COST.md).

---

## 2. Архитектура (коротко)

```
cmd/server          → boot, polling MAX, Gin
internal/api/web    → routes, static studio, payments, auth
internal/api/bot    → MAX commands, SendStudio «Запуск»
internal/clients    → MAX, AceData, YooKassa, Eleven, Hedra…
internal/services   → credits, jobs, media_sign, max_webapp, validate
internal/middleware → JWT/cookie, CSRF, webhook auth, metrics
deploy/             → Amvera, nginx, HTTPS/webhook notes
```

- Media: `MEDIA_ROOT` (`/data/media` на Amvera), signed download TTL ~1h.
- Jobs: claim→spend, stale sweeper **15m** → fail + refund.
- Dual-output: флаг `DUAL_OUTPUT` — **выкл**, пока `COST.md` не заполнит AceData ₽ и margin ≥40%.

---

## 3. Хронология версий

### 1.x — фундамент (эпохи 1–6 → **1.9.0**)

| Эпоха | Тема |
|-------|------|
| 1 | AceData / Suno only |
| 2 | Reliability (jobs, refunds) |
| 3 | Security baseline |
| 4 | MAX bot (long poll / webhook) |
| 5 | Product UX (studio pages) |
| 6 | Полировка 1.9 |

### 2.0–2.4 — lockdown после аудита (июль 2026)

| Ver | Эпоха | Ключевое |
|-----|-------|----------|
| **2.0.0** | 7 Lockdown | RequireAuth, simulate/topup под guard, ownership, MAX webhook secret |
| **2.1.0** | 8 Auth UX | CSRF, XSS (без опасного innerHTML), фронт + Bearer |
| **2.2.0** | 9 Prod foundation | Роуты из main, Postgres dual-driver, atomic credits, non-root Docker :8080 |
| **2.3.0** | 10 Product & pay | IsPublic/discover, likes/comments, YooKassa checkout skeleton |
| **2.4.0** | 11 Quality | CI (vet/test/build), тесты auth/credits/safe download, log redact |

### 2.6.x–2.7.x — hardening + GTM (авг 2026)

| Ver | Ключевое |
|-----|----------|
| **2.6.2** | SSRF redirects closed; claim→spend |
| **2.6.3** | Cost guards, non-root, safe errors |
| **2.6.4** | Webhook query secret; login code → cookie |
| **2.7.1** | Modes Idea/Lyrics/Instrumental; pack5 99₽; free 2; stale refund; signed download |
| **2.7.2** | YooKassa **GetPayment** + SettlePaymentCAS; `UVO_ALLOW_INSECURE` игнор на HTTPS; slim `/health`; metrics auth; lyrics assist; `DUAL_OUTPUT` flag |
| **2.7.3** | PG SSL warn; optional `YOOKASSA_WEBHOOK_IPS`; docs |
| **2.7.4** | MAX pricing copy; JobRecord JSON tags; Gin trusted proxies; WEB_PUBLIC_URL из env (hotfix: fallback в Dockerfile) |
| **2.7.5** | Кнопка **«Запуск»** (`open_app`); MAX Bridge + `initData` → `/api/auth/max-webapp`; ответ бота по `user_id`/`chat_id` |
| **2.7.6** | Fallback если `open_app` 404 (мини-приложение не в кабинете) → link «Запуск»; slash-команды через `PATCH /me/commands` |

Подробные чеклисты эпох: [`docs/archive/`](docs/archive/).

---

## 4. Безопасность — не регрессировать

- Fail-closed prod: `ALLOW_ANON=false`, `DEV_AUTH=false`, `DEMO_TOPUP=false`
- `UVO_ALLOW_INSECURE` **игнорируется**, если `WEB_PUBLIC_URL` — публичный HTTPS
- SSRF: SafeDownload / redirect limits
- CSRF на cookie-сессии; signed media
- MAX webhook: secret header; optional IP allowlist для ЮKassa
- Non-root контейнер, порт **8080**
- Slim `/health`; полный срез и `/metrics` — только с `METRICS_TOKEN` (или localhost)

Оценка кода ~**7.8–8.0** (ауг 2026); ops ниже из‑за отсутствия ЮKassa-ключей на проде. Старый срез: `docs/archive/AUDIT.md`.

---

## 5. MAX: бот + мини-приложение

### Код
- Polling по умолчанию: `BOT_MODE=polling` (Dockerfile / Amvera).
- API: `platform-api2.max.ru`, auth header = token (без Bearer).
- `SendStudioTo`: кнопка `open_app` текст **«Запуск»** (+ fallback `link` в браузер).
- Автологин мини-приложения: `window.WebApp.initData` → HMAC `WebAppData` → session cookie + JWT.

### Кабинет партнёра (обязательно для окна внутри MAX)
1. business.max.ru → Чат-боты → бот → **Расширенные настройки**
2. URL: `https://uvo-baskakovanton.amvera.io/`
3. Вид кнопки: **Старт** / **Открыть**
4. Env: `MAX_BOT_TOKEN`, `WEB_PUBLIC_URL`, `BOT_MODE=polling`

`link` открывает внешний браузер; in-MAX окно = только mini-app URL + `open_app` / кнопка платформы.

---

## 6. Деплой Amvera

См. [`deploy/AMVERA.md`](deploy/AMVERA.md).

Критичные env:

```
DB_DRIVER=postgres
DATABASE_URL=…          # или PG*
PGSSLMODE=disable       # CNPG internal
SUNO_API_KEY=
MAX_BOT_TOKEN=
JWT_SECRET=             # или авто /data/jwt_secret
WEB_PUBLIC_URL=https://uvo-baskakovanton.amvera.io
BOT_MODE=polling
MEDIA_ROOT=/data/media
WEB_HOST=0.0.0.0
# YOOKASSA_SHOP_ID / YOOKASSA_SECRET_KEY
# METRICS_TOKEN / OPENAI_API_KEY
# DUAL_OUTPUT=true      # только после COST.md
```

Проверка: `GET /health` → `"version":"2.7.5"`, `"prod_guards":true`, `"max_bot":true`.

**Урок:** не убирать fallback `WEB_PUBLIC_URL` из Dockerfile без гарантии env на Amvera — иначе 503 на boot (hotfix 2.7.4 / `12aa304`).

Self-hosted HTTPS + webhook MAX: [`deploy/HTTPS_WEBHOOK.md`](deploy/HTTPS_WEBHOOK.md).

---

## 7. Оплата (код готов, ключи — owner)

- Пакеты: `GET /api/credits`
- Checkout: `POST /api/credits/checkout`
- Webhook: `POST /api/payments/yookassa` — **сначала GetPayment API**, потом SettlePaymentCAS
- Демо topup только при `DEMO_TOPUP=true`
- Прод сейчас: `"yookassa":false` пока нет ключей

---

## 8. Открыто (владелец / след. спринты)

| Приоритет | Задача |
|-----------|--------|
| HIGH | `YOOKASSA_SHOP_ID` + `SECRET` + webhook URL на Amvera |
| HIGH | URL мини-приложения + кнопка Старт в кабинете MAX |
| MED | Заполнить AceData ₽ в [`COST.md`](COST.md) → dual go/no-go |
| MED | Seed публичных треков в discover |
| LOW | `OPENAI_API_KEY` (lyrics assist), `METRICS_TOKEN` |
| LOW | JWT iss/aud, CSP, опц. Telegram-бот |

---

## 9. Правила работы

1. Перед фичей — этот файл + [`COST.md`](COST.md); детали эпохи — `docs/archive/EPOCH*.md`.
2. Не коммитить `.env` / секреты.
3. После релиза: версия в `cmd/server/main.go`, `/health`, README, этот файл.
4. Push: `git push origin master` и `git push amvera master` (деплой).
5. Не включать `DUAL_OUTPUT` без margin из COST.md.

---

## 10. Быстрый старт локально

```bash
cp .env.example .env
go mod tidy
go run ./cmd/server
# http://127.0.0.1:8010
go test ./...
```

Docker: `docker compose up --build` · порт контейнера Amvera: **8080**.
