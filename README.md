# UVO — AI Music Platform

Go · Gin · AceData Suno · MAX bot · Web studio

**Version: 2.7.8**

## Quick start

```bash
cp .env.example .env   # fill keys
go mod tidy
go run ./cmd/server
```

Open http://localhost:8010/

## Pages
- `/` — студия (режимы Идея / Свой текст / Instrumental)
- `/tracks.html` — треки
- `/feed.html` — лента
- `/playlists.html` — плейлисты
- `/health` — slim status (full с `METRICS_TOKEN`)
- `/metrics` — счётчики (токен или localhost)

## Docker

```bash
docker compose up --build
```

## Amvera + PostgreSQL

См. [deploy/AMVERA.md](deploy/AMVERA.md): `amvera.yml` + managed Postgres, `containerPort: 8080`.

## Env (prod)
```
ALLOW_ANON=false
DEV_AUTH=false
DEMO_TOPUP=false
JWT_SECRET=<random>          # или авто из /data/jwt_secret
WEB_PUBLIC_URL=https://your-host
MAX_WEBHOOK_SECRET=<random>  # for webhook mode
BOT_MODE=polling
YOOKASSA_SHOP_ID=
YOOKASSA_SECRET_KEY=
METRICS_TOKEN=<random>       # для /metrics и /health?full=1
# DUAL_OUTPUT=true           # 2 клипа (только после COST.md margin)
# OPENAI_API_KEY=            # lyrics assist
```

`UVO_ALLOW_INSECURE=true` **игнорируется**, если `WEB_PUBLIC_URL` — публичный HTTPS.

Вход в веб на проде: кнопка **«Запуск»** в MAX (mini-app) или **`/login`** → one-time code → cookie.

## Оплата
- Пакеты: `GET /api/credits` (входной `pack5` @ 99₽).
- Checkout: `POST /api/credits/checkout` при заданных `YOOKASSA_*`.
- Webhook: `POST /api/payments/yookassa` — **верификация через GetPayment API** до начисления.
- Демо: `DEMO_TOPUP=true` + `POST /api/credits/topup`.

## Studio
- Генерация (−1) · клон (−2) · кавер (−2) · караоке (−2) · portrait (−2/−3)
- Черновик текста: `POST /api/lyrics/assist` (нужен `OPENAI_API_KEY`, без списания кредита музыки)

## Docs
- **[DEVELOPMENT.md](DEVELOPMENT.md)** — ключевые моменты, версии, MAX, деплой, открытые задачи
- [COST.md](COST.md) — unit economics / dual gate
- [deploy/AMVERA.md](deploy/AMVERA.md) — прод на Amvera
- [docs/archive/](docs/archive/) — эпохи 1–15, старый аудит и планы

Remote: https://github.com/vongola954/uvo · deploy: Amvera `amvera` remote
