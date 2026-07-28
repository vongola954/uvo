# Roadmap UVO — эпохи

## Завершённые (история)

| Эпоха | Тема | Файл |
|-------|------|------|
| 1 | AceData only | [EPOCH1.md](EPOCH1.md) |
| 2 | Reliability | [EPOCH2.md](EPOCH2.md) |
| 3 | Security (частично) | [EPOCH3.md](EPOCH3.md) |
| 4 | MAX bot | [EPOCH4.md](EPOCH4.md) |
| 5 | Product UX | [EPOCH5.md](EPOCH5.md) |
| 6 | Полировка 1.9 | [EPOCH6.md](EPOCH6.md) |

Версия после эпохи 6: **1.9.0**.

## После аудита (обязательный трек)

Ориентиры: [AUDIT.md](AUDIT.md), [FIXPLAN.md](FIXPLAN.md).

| Эпоха | Тема | Цель | Версия (цель) |
|-------|------|------|----------------|
| **7** | Lockdown | ✅ 2.0.0 | [EPOCH7.md](EPOCH7.md) |
| **8** | Auth UX | ✅ 2.1.0 | [EPOCH8.md](EPOCH8.md) |
| **9** | Prod foundation | ✅ 2.2.0 | [EPOCH9.md](EPOCH9.md) |
| **10** | Product & pay | ⏳ next | [EPOCH10.md](EPOCH10.md) |
| **11** | Quality | Тесты, git/CI, логи, ACCEPTANCE 2.x | **2.4** |

```
1.9 ──аудит──► 7 Lockdown ──► 8 Auth UX ──► 9 Prod ──► 10 Product ──► 11 Quality
                 P0              P0/P1         P1           P2              P2
```

## Правило работы агента / разработчика

1. Перед фичей — прочитать актуальный `EPOCH*.md` и `FIXPLAN.md`.  
2. Не смешивать эпохи в одном PR/коммите без необходимости.  
3. После закрытия эпохи — отметить чеклист, обновить версию в `/health` и README.  
4. Секреты в чат/коммиты не писать; `.env` не коммитить.  

## Быстрый старт следующей работы

→ **[EPOCH10.md](EPOCH10.md)** — Product & pay (IsPublic, topup/ЮKassa).  
Эпохи 7–9 закрыты в коде (**2.2.0**). Владельцу: ротировать ключи из аудита.
