# Аудит UVO 2.7.2 / 2.7.3 — 2026-08-03

Актуальный срез после epoch 12–14. Старые таблицы 2.6.x ниже как история закрытий.

**Live prod:** `https://uvo-baskakovanton.amvera.io` · GitHub: `vongola954/uvo`

---

## Вердикт

| Ось | Score | Комментарий |
|-----|------:|-------------|
| Security (code) | **8.0** | GetPayment settle, HTTPS ignores `UVO_ALLOW_INSECURE`, slim health, metrics gated |
| Security (ops) | **6.5** | Нет YOOKASSA_* / METRICS_TOKEN на проде |
| Reliability | **7.5** | Stale refund, claim→spend, SettlePaymentCAS |
| Product | **6.0** | Студия сильная; money/GTM off |
| **Overall code** | **7.8–8.0** | Shipable; блокер — ключи и канал |

```
2.7.2+ на Amvera + prod_guards ──► ~7.8
Без YooKassa keys                 ► продукт без выручки
UVO_ALLOW_INSECURE на HTTPS       ► игнорируется (closed)
```

---

## Закрыто (2.6.2 → 2.7.3)

| ID | Тема | Версия |
|----|------|--------|
| H1 | SSRF redirects | 2.6.2 |
| H2 | Claim→spend | 2.6.2 |
| H3–H7 | Cost / non-root / safe errors | 2.6.3 |
| H8 | Webhook query secret | 2.6.4 |
| C2 | `UVO_ALLOW_INSECURE` на public HTTPS | 2.7.2 |
| Pay | YooKassa GetPayment + SettlePaymentCAS | 2.7.2 |
| Ops | Slim `/health`, `/metrics` auth, uploads GC | 2.7.2 |
| Pay+ | Optional `YOOKASSA_WEBHOOK_IPS` | 2.7.3 |

---

## Открыто (owner / ops)

| Sev | Finding | Action |
|-----|---------|--------|
| HIGH | `yookassa:false` на проде | `YOOKASSA_SHOP_ID` + `SECRET` + webhook URL |
| MED | COST.md AceData ₽ TBD | заполнить → dual go/no-go |
| MED | Discover пустой | seed public tracks |
| MED | PGSSLMODE=disable | OK для Amvera CNPG; `require` на публичном PG |
| LOW | OPENAI / METRICS_TOKEN | опционально для lyrics + ops |
| LOW | JWT iss/aud, CSP, TG-бот | backlog |

---

## Не регрессировать

Fail-closed prod · SSRF · CSRF · signed media · MAX header webhook · non-root :8080 · stale 15m refund · modes/presets · pack5

Подробный canvas: `uvo-audit-272.canvas.tsx` (Cursor).
