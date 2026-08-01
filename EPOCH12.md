# Epoch 12 — plan 10/10 without YooKassa keys (2.7.1)

## Done

- [x] **P0b** `COST.md` — unit economics + dual gate (dual=off until AceData ₽ filled, margin ≥40%)
- [x] **P0c** entry `pack5` @ 99₽, `rub_per_song`, free grant **2**, pricing copy «1 кредит = 1 песня»
- [x] **P1a** create modes: Idea / Свой текст / Instrumental + `mode` on `POST /api/generate`
- [x] **P1c** stale job sweeper 15m → fail + refund; signed `/tracks/:id/download` (mp3 attachment, TTL 1h)
- [x] Минусовка CTA → karaoke (−2) from studio result + tracks list

## Skipped (blocked)

- [ ] **P0a** YooKassa live — wait for `YOOKASSA_SHOP_ID` / `YOOKASSA_SECRET_KEY` on Amvera
- [ ] **P1b** dual output — blocked by COST.md margin gate

## Next

1. Fill AceData ₽ in `COST.md`
2. Wire YooKassa keys → one e2e payment
3. Dual only if margin OK
