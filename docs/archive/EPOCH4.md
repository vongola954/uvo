# Эпоха 4 — MAX-бот

## Сделано
- GET /updates long-poll (marker, timeout, types=message_created,bot_started)
- POST /messages?chat_id= (по доке, не /v1/messages)
- GET /me при старте
- /start /help /generate + credits + refund
- POST /api/max/webhook для production webhook
- POST /api/bot/simulate для локальных тестов

## Важно (MAX docs)
- Long poll только для dev; production → Webhook HTTPS
- Authorization: token без Bearer
- platform-api2.max.ru

## Проверка
1. MAX_BOT_TOKEN в .env
2. BOT_MODE=polling
3. Запуск сервера → лог "MAX bot online"
4. Написать боту /generate
