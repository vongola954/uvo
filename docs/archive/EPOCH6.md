# Эпоха 6 — Полировка

- Edit без лишнего Track create/delete: обновляет тот же ряд + revision + удаляет старый файл
- Лимит горутин: MAX_WORKERS (default 4)
- scripts/smoke.sh
- ACCEPTANCE.md
- Версия 1.9.0

Postgres: опционально позже (gorm postgres driver); сейчас SQLite production-ready для single node.

---
Аудит после 1.9: [AUDIT.md](AUDIT.md). Дальше — [EPOCH7.md](EPOCH7.md) Lockdown (не считать SQLite «production-ready» для публичного мульти-инстанса без эпохи 9).
