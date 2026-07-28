# Эпоха 10 — Product & pay

**Цель:** убрать «декор» тарифов/моделей или сделать его рабочим; честная монетизация.  
**Версия:** 2.3.0  
**Зависит от:** эпохи 7–9  
**База:** [AUDIT.md](AUDIT.md) §9–10, [FIXPLAN.md](FIXPLAN.md) P2 #15–17.

## Сделать

### Публичность треков
- [ ] `PATCH/POST /api/tracks/:id/visibility` — `is_public` только owner
- [ ] Поиск/лента опираются на реально публичные треки
- [ ] Play: public без auth; private — owner

### Соц. минимум (выбрать один путь)
**Вариант A (реализовать):**
- [ ] Like / Comment API + UI на feed  
**Вариант B (упростить):**
- [ ] Убрать неиспользуемые модели из AutoMigrate / UI-обещаний Premier «лицензии»

### Плейлисты
- [ ] `is_public` на плейлист + GetTracks с проверкой
- [ ] Не отдавать чужие private треки через чужой playlist

### Оплата
- [ ] ЮKassa: create payment → webhook → `credits.Add`
- [ ] Или явно: topup только `DEMO_TOPUP=true`, в UI «оплата скоро», убрать ложные цены как «купить сейчас»
- [ ] Пакеты `CreditPacks` связаны с реальным checkout

### Тарифы на лендинге
- [ ] Согласовать Free/Pro/Premier с реальными лимитами плана в User/Subscription **или** упростить блок pricing

## Не входит
- Полноценный маркетплейс лицензий (можно stub)

## Приёмка
- [ ] Публичный трек виден в search/feed без owner token
- [ ] Private не утекает через playlist/play
- [ ] Topup без DEMO/ЮKassa → 403
- [ ] README: раздел «Оплата» актуален

## Статус
⏳ Ждёт эпоху 9
