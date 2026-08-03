# Деплой UVO на Amvera + PostgreSQL

Официально: [Docker](https://docs.amvera.ru/applications/configuration/docker.html) · [PostgreSQL](https://docs.amvera.ru/databases/postgreSQL.html)

В репозитории: `Dockerfile`, `amvera.yml` (порт **8080**, volume **/data**).

---

## 1. PostgreSQL

1. [cloud.amvera.ru](https://cloud.amvera.ru/projects) → **PostgreSQL**
2. Внутренний хост вида `amvera-<login>-cnpg-<project>-rw`, порт `5432`
3. Из приложений Amvera — **без SSL** (`sslmode=disable`) — так устроен CNPG

---

## 2. Приложение

1. Проект **Приложение** + git push на remote `amvera`
2. Нужны `Dockerfile`, `amvera.yml`, `go.mod`, исходники
3. `containerPort: "8080"` (non-root user не слушает 80)

---

## 3. Переменные

| Переменная | Значение | Секрет? |
|------------|----------|---------|
| `DB_DRIVER` | `postgres` | нет |
| `DATABASE_URL` или `PG*` | см. ниже | да |
| `PGSSLMODE` | `disable` (Amvera internal) | нет |
| `SUNO_API_KEY` | AceData | да |
| `MAX_BOT_TOKEN` | MAX | да |
| `JWT_SECRET` | длинная строка | да |
| `MAX_WEBHOOK_SECRET` | для webhook mode | да |
| `WEB_PUBLIC_URL` | `https://uvo-….amvera.io` | да |
| `ALLOW_ANON` / `DEV_AUTH` / `DEMO_TOPUP` | `false` | нет |
| `BOT_MODE` | `polling` или `webhook` | нет |
| `MEDIA_ROOT` | `/data/media` | нет |
| `WEB_HOST` | `0.0.0.0` | нет |
| `YOOKASSA_SHOP_ID` / `YOOKASSA_SECRET_KEY` | оплата | да |
| `YOOKASSA_WEBHOOK_IPS` | опц. CIDR/IP | нет |
| `OPENAI_API_KEY` | lyrics assist | да |
| `METRICS_TOKEN` | `/metrics` + full `/health` | да |
| `DUAL_OUTPUT` | `true` только после COST.md | нет |

### DATABASE_URL

```text
postgres://USER:PASSWORD@amvera-LOGIN-cnpg-DBPROJECT-rw:5432/DBNAME?sslmode=disable
```

Спецсимволы в пароле — URL-encode.

Webhook ЮKassa: `https://YOUR_DOMAIN/api/payments/yookassa`  
Проверка: `GET /health` → `"version":"2.7.5"`, `"yookassa":true` после ключей.

### MAX мини-приложение (кнопка «Запуск» в чате)

1. [Платформа партнёров](https://business.max.ru/) → Чат-боты → ваш бот → **Расширенные настройки**
2. URL мини-приложения: `https://uvo-baskakovanton.amvera.io/` (тот же `WEB_PUBLIC_URL`)
3. Вид кнопки: **Старт** / **Открыть**
4. Amvera env: `BOT_MODE=polling`, `MAX_BOT_TOKEN`, `WEB_PUBLIC_URL`
5. В чате с ботом: `/start` → кнопка **Запуск** открывает студию внутри MAX

**Важно:** `WEB_PUBLIC_URL` задаётся только в переменных Amvera (в Docker image больше не зашит).

---

## 4. Чеклист

- [ ] Порт контейнера **8080**
- [ ] Postgres запущен, хост `-rw`
- [ ] `SUNO_API_KEY`, `JWT_SECRET`, `WEB_PUBLIC_URL`
- [ ] `prod_guards: true` на `/health`
- [ ] ЮKassa ключи + webhook (когда готовы платить)

---

## Локально vs Amvera

| | Локально | Amvera |
|--|----------|--------|
| БД | sqlite | managed Postgres |
| Порт | 8010 | 8080 |
| Media | `./data/media` | `/data/media` |
