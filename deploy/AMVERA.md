# Деплой UVO на Amvera + PostgreSQL

Официально: [Docker](https://docs.amvera.ru/applications/configuration/docker.html) · [PostgreSQL](https://docs.amvera.ru/databases/postgreSQL.html) · [Go+Postgres пример](https://docs.amvera.ru/general/examples/go-postgresql.html)

В репозитории уже есть: `Dockerfile`, `amvera.yml` (порт **80**, volume **/data**).

---

## 1. Создать PostgreSQL (отдельный проект)

1. [cloud.amvera.ru](https://cloud.amvera.ru/projects) → **PostgreSQL** → создать БД  
2. Задать: имя БД, пользователя, пароль (нельзя `postgres` как имя БД/юзера приложения)  
3. Дождаться статуса «PostgreSQL запущен»  
4. На вкладке **Инфо** скопировать **внутренний хост** вида:

```text
amvera-<login>-cnpg-<project>-rw
```

Порт: `5432`. Из приложений Amvera — без SSL (`sslmode=disable`).

---

## 2. Создать приложение UVO

1. Создать проект типа **Приложение**  
2. Загрузить код (git push amvera / UI): нужен корень с `Dockerfile`, `amvera.yml`, `go.mod`, `go.sum`, исходники  
3. Дождаться успешной сборки  

`docker-compose.yml` Amvera **не** использует — только Dockerfile.

---

## 3. Переменные окружения (приложение UVO)

В кабинете → **Переменные** / **Секреты**:

| Переменная | Значение | Секрет? |
|------------|----------|---------|
| `DB_DRIVER` | `postgres` | нет |
| `DATABASE_URL` | см. ниже | **да** |
| или `PGHOST` + `PGUSER` + `PGPASSWORD` + `PGDATABASE` | вместо URL | пароль — секрет |
| `PGPORT` | `5432` (если не в URL) | нет |
| `PGSSLMODE` | `disable` | нет |
| `SUNO_API_KEY` | ключ AceData | да |
| `MAX_BOT_TOKEN` | токен MAX | да |
| `JWT_SECRET` | длинная случайная строка | да |
| `MAX_WEBHOOK_SECRET` | случайная строка | да |
| `SILICONFLOW_API_KEY` | опционально | да |
| `ELEVENLABS_API_KEY` | опционально | да |
| `ALLOW_ANON` | `false` | нет |
| `DEV_AUTH` | `false` | нет |
| `DEMO_TOPUP` | `false` | нет |
| `BOT_MODE` | `polling` или `webhook` | нет |
| `WEB_PUBLIC_URL` | `https://<ваш-домен-amvera>` | нет |
| `MEDIA_ROOT` | `/data/media` (уже в Dockerfile) | нет |
| `WEB_HOST` | `0.0.0.0` | нет |

### DATABASE_URL (рекомендуется)

```text
postgres://USER:PASSWORD@amvera-LOGIN-cnpg-DBPROJECT-rw:5432/DBNAME?sslmode=disable
```

Спецсимволы в пароле — URL-encode (`@` → `%40` и т.д.).

### Альтернатива без URL

```text
DB_DRIVER=postgres
PGHOST=amvera-LOGIN-cnpg-DBPROJECT-rw
PGPORT=5432
PGUSER=...
PGPASSWORD=...
PGDATABASE=...
PGSSLMODE=disable
```

При старте приложение само делает AutoMigrate таблиц.

---

## 4. Домен и проверка

1. Настройки приложения → активировать бесплатный домен Amvera  
2. `WEB_PUBLIC_URL=https://that-domain`  
3. Открыть `https://…/health` → `"version":"2.4.0"`, `"db_driver":"postgres"`, `acedata`  
4. Студия: при `ALLOW_ANON=false` — Demo token только если временно `DEV_AUTH=true` (только для отладки)

---

## 5. MAX bot

- **Dev:** `BOT_MODE=polling`  
- **Prod:** HTTPS домен + `BOT_MODE=webhook` + `MAX_WEBHOOK_SECRET`  
  - nginx/Amvera не добавляет header сам — для webhook нужен секрет в query при подписке **или** прокси; см. `deploy/HTTPS_WEBHOOK.md`  
  - Подписка: `go run deploy/webhook_setup.go -url https://YOUR/api/max/webhook`

---

## 6. Постоянное хранилище

`amvera.yml` → `persistenceMount: /data`  
Медиа: `/data/media` (не теряется при редеплое).  
Логи приложения — в stdout (кабинет Amvera → Логи).

---

## 7. Чеклист «не 502 / не падает»

- [ ] `WEB_HOST=0.0.0.0`, порт контейнера **80** (`amvera.yml` / Dockerfile)  
- [ ] Postgres **запущен**, хост `-rw`, приложение в том же облаке Amvera  
- [ ] `SUNO_API_KEY` задан (иначе конфиг не стартует)  
- [ ] `JWT_SECRET` не дефолтный  
- [ ] Ключи из аудита **ротированы**  

---

## Локально vs Amvera

| | Локально | Amvera |
|--|----------|--------|
| БД | sqlite / свой postgres | managed Postgres |
| Порт | 8010 (`docker compose`) | 80 |
| Media | `./data/media` | `/data/media` |
| Логи | опционально файл | stdout |

`docker compose` по-прежнему для локалки — переопределите `WEB_PORT=8010` и `DB_DRIVER=sqlite` в `docker-compose.yml`.
