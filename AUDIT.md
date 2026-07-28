# Аудит UVO 2.4.0 — 2026-07-28 (повторный)

Жёсткий аудит после эпох 7–11. Предыдущий (1.9 → 4.5/10): см. историю в git / раздел «Закрыто в эпохах».

**Метод:** полный обзор кода + целевые проверки auth/CSRF/ownership/SSRF/credits/webhook/upload/Docker/фронт.  
**Версия в коде:** 2.4.0 · live Amvera на момент аудита: `/health` 2.4.0.

---

## Вердикт

| Критерий | Было (1.9) | Сейчас (2.4) | Комментарий |
|----------|------------|--------------|-------------|
| Архитектура | 6/10 | **7/10** | Роуты вынесены; слои ок; jobs/idempotency сырые |
| Безопасность | 3/10 | **5.5/10** | Auth/CSRF/ownership есть; fail-open конфиг; SSRF redirects |
| Надёжность | 5/10 | **6.5/10** | Postgres + atomic spend; race idem↔credits; TTS без лимитов |
| Тесты | 2/10 | **6/10** | CI + auth/credits/safe_*/topup; нет SSRF-redirect / race tests |
| Prod-ready | 4/10 | **5/10** | Код лучше; без boot-guard и с demo-флагами — нет |
| UX/фронт | 7/10 | **7.5/10** | XSS закрыт; JWT в localStorage; private play + Bearer слабо |

**Итог: 5.5/10** — сильный lockdown MVP. **В интернет с текущим локальным `.env` (все danger-флаги ON) выставлять нельзя.**  
При корректных prod-флагах + ротации ключей ≈ **6.5–7**. До **10/10** — fail-closed, SSRF harden, cost controls, non-root, payments, session model.

```
Код после 7–11 ──────────► ~7   (если флаги выключены)
Мисконфиг / .env demo ───► ~3–4 (открытый demo)
Реальный риск сейчас ────► 5.5  (смесь хорошего кода и footgun'ов)
```

---

## Критично (P0)

### C1. Danger-флаги + живые ключи в workspace `.env`
Локальный `.env`: `ALLOW_ANON=true`, `DEV_AUTH=true`, `DEMO_TOPUP=true`, реальные provider keys, предсказуемый `JWT_SECRET`.  
**Не коммитится** (gitignore / dockerignore) — но любой запуск с этим env = shared demo + mint JWT + free topup + сжигание AceData.

**Действие:** ротация всех ключей (AceData, MAX, SiliconFlow, JWT, webhook, DB). Prod: все три флага `false`, длинный случайный `JWT_SECRET`.

### C2. `DEV_AUTH` выдаёт JWT на любой `user_id`
`POST /api/auth/token` (`routes.go`) при `DEV_AUTH=true` — захват любого user id (в т.ч. MAX).

**Фикс:** не стартовать в release/https при `DEV_AUTH=true`; либо только loopback; в prod-сборке вырезать эндпоинт.

### C3. Нет fail-closed bootstrap
`config.go`: default `JWT_SECRET=dev-secret-change-me`; нет отказа при danger-флагах. Забыли env → forgeable JWT / открытый demo.

**Фикс:** `config.Load()` падает, если secret короткий/default; danger-флаги только при `UVO_ALLOW_INSECURE=true`.

---

## Высокий (P1)

| ID | Находка | Где | Риск | Фикс |
|----|---------|-----|------|------|
| H1 | **SSRF через redirects** | `safe_http.go` — `http.Client` без `CheckRedirect`; private IP только на initial host | Allowlisted CDN → 302 на metadata/RFC1918 | CheckRedirect + re-validate; dial IP pin |
| H2 | **TTS без кредитов и rate limit** | `routes.go` `tts` — только OwnsVoice | Auth user жжёт ElevenLabs | Credits + limiter + daily cap |
| H3 | **Voice clone — мягкая quota** | check-then-record TOCTOU; MIME no-op | Параллельный clone / junk upload | Atomic quota; magic bytes; credits |
| H4 | **Idempotent generate ↔ double spend** | `generate.go`: Spend → Create; при race второй hit Processing/Done без Refund; оба Pending → два worker на один job | Потеря кредитов / двойная генерация | Spend после exclusive create; unique worker; refund на idempotent hit после spend |
| H5 | **`bot/simulate` принимает чужой `user_id`** | при `DEV_AUTH` | Трата чужих кредитов | Игнорировать body user_id = только `UserID(c)` |
| H6 | **Webhook `?secret=`** | `webhook.go` | Секрет в access/Referer/CDN логах | Только header; nginx inject |
| H7 | **Утечка provider body в клиент** | `err.Error()` в generate/voice/tts/eleven | Внутренности AceData | Только `ProviderError` коды |
| H8 | **Docker root** | `Dockerfile` — нет `USER` (ACCEPTANCE врёт про non-root) | RCE = root в контейнере | `USER` non-root + `/data` perms |

---

## Средний (P2)

| ID | Находка | Фикс |
|----|---------|------|
| M1 | `deleteTrack` — `os.Remove` без `SafeMediaPath` | Проверка пути |
| M2 | Upload `make([]byte, file.Size)` | `LimitReader` + MaxMultipartMemory |
| M3 | `ValidateVoiceUpload` MIME — пустой if | Magic-byte sniff |
| M4 | Rate limit count-then-insert | Atomic upsert |
| M5 | JWT в `localStorage` | HttpOnly cookie session |
| M6–M7 | CSRF без SameSite; `!=` compare | SameSite=Strict; constant-time |
| M8 | Webhook compare при разной длине | SHA-256 обоих, потом compare |
| M9 | Открытые `/health` + `/metrics` | Auth / network restrict |
| M10 | `PGSSLMODE` default `disable` | Default `require` |
| M11 | Caption без лимита длины | Cap ~500 |
| M12 | Private play + Bearer из localStorage не уходит в `<audio>` | Signed play URL / cookie |
| M13 | Нет CSP / security headers; Tailwind CDN | Headers + self-host |
| M14 | DNS rebinding на download | DialContext pin IP |

---

## Низкий (P3)

- Публичные sequential track IDs (ожидаемо для discover)
- Gin logger может засветить `?secret=` если query используется
- `Add` кредитов игнорирует ошибки; `SpendTx` не используется
- ACCEPTANCE prod-флаги всё ещё unchecked как release gate

---

## Что уже хорошо (эпохи 7–11)

1. `RequireAuth` на `/api/*` + `OptionalAuth`  
2. Guards: `DEV_AUTH` / `DEMO_TOPUP` / empty webhook secret → 403  
3. Job / playlist / TTS voice ownership; discover только public  
4. CSRF на mutate studio API  
5. XSS: `textContent` / `UVO.el` (не user `innerHTML`)  
6. `SafeDownload` baseline (HTTPS, allowlist, private IP, size) + `SafeMediaPath` на play  
7. Atomic `Spend` + Postgres quotas  
8. AceData log `redactBody`  
9. `.env` не в git / dockerignore  
10. CI: vet / test / build  
11. GORM parameterized; нет `exec.Command` / raw SQL concat  

---

## Соответствие prod checklist

| Пункт | Статус |
|-------|--------|
| `go build` / `go test` / `go vet` | OK (локально) |
| CI workflow | Есть (GitHub; Amvera remote ≠ GH) |
| RequireAuth + ownership | OK в коде |
| ALLOW_ANON / DEV_AUTH / DEMO_TOPUP в workspace `.env` | **FAIL** (все true) |
| Fail-closed boot | **FAIL** |
| Docker non-root | **FAIL** |
| SSRF redirects | **FAIL** |
| TTS cost control | **FAIL** |
| ЮKassa | нет (coming_soon) |
| Ротация ключей после чата | на владельце |

---

## Приоритеты следующих эпох

| | Эпоха | Работа |
|---|-------|--------|
| **P0** | **12 — Fail-closed** | Boot guards; ротация ключей; убрать/зажать DEV_AUTH; webhook без query secret |
| **P0** | **12b — SSRF** | CheckRedirect + IP pin + тесты 302→private |
| **P1** | **13 — Cost abuse** | Credits/limits TTS+clone; fix idem↔credits; atomic quotas |
| **P1** | **13b — Container & errors** | Non-root USER; sanitize 5xx; SafeMediaPath delete; LimitReader + magic |
| **P2** | **14 — Auth & play** | Cookie/signed play; SameSite CSRF; CSP |
| **P2** | **15 — Pay & ops** | YooKassa; lockdown metrics; PGSSLMODE=require |

---

## Закрыто в эпохах 7–11 (кратко)

- **7:** RequireAuth, ownership, webhook secret, topup/simulate guards  
- **8:** JWT+CSRF фронт, XSS  
- **9:** routes extract, atomic credits, Postgres, Docker image bump  
- **10:** visibility/discover/social, honest credits UI  
- **11:** тесты, CI, redact, 2.4.0  

**Правило:** закрывать находки этого документа чеклистами новых эпох; не раздувать scope без EPOCH*.md.
