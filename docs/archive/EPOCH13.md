# Epoch 13 — engineering (2.7.2)

Agent-owned code sprint (no YooKassa/AceData ₽ keys required from owner).

## Done

- [x] **P0a** YooKassa webhook: `GetPayment` + amount/user/payment-id match → `SettlePaymentCAS`
- [x] **P0b** `UVO_ALLOW_INSECURE` ignored on public HTTPS
- [x] **P0c** `Credits.Add` returns error; transactional settle
- [x] **P1a** Slim `/health`; `/metrics` behind `METRICS_TOKEN` (404 on https without token)
- [x] **P1b** Uploads GC (7d) + rate-limit insert-then-check
- [x] **P1c** robots/sitemap/favicon/OG + README 2.7.2
- [x] **P1d** `POST /api/lyrics/assist` (OpenAI-compatible, optional)
- [x] **P2a** `DUAL_OUTPUT` env — keep up to 2 AceData clips + job AltPlayURL

## Owner still needed

- YOOKASSA_* on Amvera + webhook URL in cabinet
- AceData ₽ → fill COST.md → decide dual on/off in prod
- OPENAI_API_KEY for lyrics assist
- METRICS_TOKEN for ops
