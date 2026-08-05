# Epoch 15 — bot GTM copy + proxy IP (2.7.4)

## Done

- [x] MAX `/start` `/help` `/credits` — цена «от 99₽», free credits, 1 кредит = 1 песня
- [x] JobRecord JSON tags (`play_url`, `alt_play_url`, …)
- [x] Gin `SetTrustedProxies` for Amvera (YooKassa IP allowlist)
- [x] Dockerfile: no hardcoded `WEB_PUBLIC_URL` (must be Amvera env)
- [x] `/health` → `lyrics_assist` bool

## Owner

- Confirm `WEB_PUBLIC_URL` set in Amvera after deploy (removed from Dockerfile default)
- YOOKASSA_* / OPENAI / seed tracks
