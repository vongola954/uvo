# Эпоха 10 — Product & pay

**Цель:** убрать «декор» тарифов/моделей или сделать его рабочим; честная монетизация.  
**Версия:** 2.3.0  
**Зависит от:** эпохи 7–9  
**База:** [AUDIT.md](AUDIT.md) §9–10, [FIXPLAN.md](FIXPLAN.md) P2 #15–17.

## Сделать

### Публичность треков
- [x] `PATCH /api/tracks/:id/visibility` — `is_public` только owner
- [x] Поиск/лента опираются на реально публичные треки
- [x] Play: public без auth; private — owner
- [x] `GET /api/discover` — публичные треки без auth

### Соц. минимум (вариант A)
- [x] Like / Comment API + UI на feed
- [x] Пост в ленту делает трек публичным; feed фильтрует private

### Плейлисты
- [x] `is_public` на create + `PATCH /api/playlists/:id/visibility`
- [x] GetTracksForUser не отдаёт чужие private треки через публичный playlist

### Оплата
- [x] Topup только `DEMO_TOPUP=true` (уже было) + UI «оплата скоро» / демо-кнопка
- [x] Пакеты `CreditPacks` в `GET /api/credits` + `demo_topup` / `payment: coming_soon`
- [ ] ЮKassa create payment → webhook (отложено; честный stub вместо фейковых «купить»)

### Тарифы на лендинге
- [x] Убраны ложные Free/Pro/Premier «купить сейчас»; блок кредитов с честным текстом

## Не входит
- Полноценный маркетплейс лицензий (можно stub)
- Полная ЮKassa (следующая итерация)

## Приёмка
- [x] Публичный трек виден в discover/search/feed
- [x] Private не утекает через playlist/play
- [x] Topup без DEMO → 403
- [x] README: раздел «Оплата» актуален

## Статус
✅ Закрыта (2.3.0)
