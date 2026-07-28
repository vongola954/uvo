# UVO — AI Music Platform

Go · Gin · AceData Suno · MAX bot · Web studio

## Quick start

```bash
cp .env.example .env   # fill keys
go mod tidy
go run ./cmd/server
```

Open http://localhost:8010/

## Pages
- `/` — студия
- `/tracks.html` — треки
- `/feed.html` — лента
- `/playlists.html` — плейлисты
- `/health` — статус AceData/MAX
- `/metrics` — счётчики

## Docker

```bash
docker compose up --build
```

## Amvera + PostgreSQL

См. [deploy/AMVERA.md](deploy/AMVERA.md): `amvera.yml` + managed Postgres, `DB_DRIVER=postgres`.

## Env (prod)
```
ALLOW_ANON=false
DEV_AUTH=false
DEMO_TOPUP=false
JWT_SECRET=<random>
MAX_WEBHOOK_SECRET=<random>   # required for POST /api/max/webhook
BOT_MODE=polling   # or webhook + POST /api/max/webhook
```

Локальный demo: `ALLOW_ANON=true` (и при необходимости `DEV_AUTH=true`, `DEMO_TOPUP=true`).

## Оплата
- Пакеты кредитов в `GET /api/credits` (`packs`, `payment: coming_soon`).
- Демо topup: `POST /api/credits/topup` **только** при `DEMO_TOPUP=true` (+ auth) — иначе 403.
- Реальный checkout (ЮKassa) — ещё не подключён; UI не обещает «купить сейчас».

## Docs
- [AUDIT.md](AUDIT.md) — жёсткий аудит 1.9 (2026-07-28)
- [FIXPLAN.md](FIXPLAN.md) — приоритеты P0→P2
- [ROADMAP.md](ROADMAP.md) — эпохи 1–11

Текущая версия: **2.6.0** (караоке + поющий портрет).

## Studio
- **Генерация** — промпт / стиль / голос (AceData persona)
- **Клон голоса** — `POST /api/voice/clone` → AceData `/suno/voices` (нужен `WEB_PUBLIC_URL`)
- **Кавер** — `POST /api/cover` (загрузка любого трека + свой голос, −2 кредита)
- **Правки** — на `/tracks.html` → «Правка (−1)»
- **Караоке** — `/tracks.html` → «Караоке» → stems + timing + mp4 (−2)
- **Поющий портрет** — фото + трек → Hedra lip-sync (`HEDRA_API_KEY`) или Kling-клип (−2/−3)

Для clone/cover AceData скачивает файлы с вашего домена: `GET /uploads/:name`. На Amvera задайте `WEB_PUBLIC_URL=https://uvo-….amvera.io`.

## Epochs
1 AceData · 2 Reliability · 3 Security · 4 MAX · 5 Product UX · 6 Polish  
**Done:** 7–11 · **2.5** voice/cover · **2.6** karaoke/portrait
