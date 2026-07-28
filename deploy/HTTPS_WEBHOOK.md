# HTTPS + MAX Webhook

По [docs MAX](https://dev.max.ru/docs-api): production = **только Webhook**, HTTPS, без self-signed (с 25.05.2026). Long poll — только dev.

## 1. Домен и сертификат

```bash
# Пример Let's Encrypt
sudo certbot certonly --nginx -d uvo.example.com
```

Или сертификат Минцифры / другого УЦ.

## 2. Nginx

Скопировать `deploy/nginx-uvo.conf`, подставить домен и пути к cert:

```bash
sudo cp deploy/nginx-uvo.conf /etc/nginx/sites-available/uvo
sudo ln -s /etc/nginx/sites-available/uvo /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

UVO слушает `127.0.0.1:8010` (или `WEB_HOST=127.0.0.1`).

## 3. Приложение

```env
WEB_HOST=127.0.0.1
WEB_PORT=8010
BOT_MODE=webhook
ALLOW_ANON=false
DEV_AUTH=false
DEMO_TOPUP=false
MAX_WEBHOOK_SECRET=<long-random-secret>
```

Перезапуск UVO. **Не** запускать long-poll одновременно с webhook (MAX запрещает оба).

Webhook UVO принимает только с секретом:

```http
POST /api/max/webhook
X-Max-Bot-Api-Secret: <MAX_WEBHOOK_SECRET>
```

или `?secret=<MAX_WEBHOOK_SECRET>`. Без секрета → 401/403.

> Если MAX шлёт webhook напрямую без вашего header, поставьте перед UVO nginx, который добавляет `X-Max-Bot-Api-Secret`, либо проксируйте через свой gateway. Для ручных тестов используйте curl с header.

## 4. Подписка webhook

```bash
cd /path/to/uvo
go run deploy/webhook_setup.go -url https://uvo.example.com/api/max/webhook
```

Эндпоинт: `POST /api/max/webhook` (с секретом выше).

Проверка списка:

```bash
curl -H "Authorization: $MAX_BOT_TOKEN" https://platform-api2.max.ru/subscriptions
```

## 5. Проверка

1. Написать боту в MAX `/start`
2. В логах UVO — update / ответ
3. `/generate` → промпт → трек

## Troubleshooting

| Симптом | Действие |
|---------|----------|
| 401 MAX | Токен без `Bearer`, свежий из business.max.ru |
| Webhook не приходит | Только HTTPS, не HTTP; cert доверенный |
| Webhook 401/403 UVO | Проверить `MAX_WEBHOOK_SECRET` и header/query |
| Конфликт с poll | `BOT_MODE=webhook`, убрать polling |
| 429 | ≤30 rps к platform-api2.max.ru |
