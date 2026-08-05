# Эпоха 1 — AceData only (вариант A)

## Сделано
- ProviderError + used_up → 503 + понятный текст
- Refund кредитов при ошибке generate/edit/rate-limit
- DEV_AUTH на /api/auth/token
- GET /health → acedata status (key_ok / auth / balance)

## Действие владельца (обязательно)
1. Открыть https://platform.acedata.cloud
2. Пополнить баланс Suno/AceData
3. Перезапустить сервер
4. curl http://localhost:8010/health — смотреть acedata.ok

## Не входит в эпоху A
- MiniMax / ElevenLabs Music / Apiframe fallback

## Приёмка
- [ ] health возвращает acedata
- [ ] при used_up кредит UVO возвращается
- [ ] после пополнения generate создаёт трек
