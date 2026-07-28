# Аудит UVO 2.6.2 — 2026-07-29

Жёсткий аудит после prod-hardening (2.6.1) + эпоха 12 (SSRF + idem). Предыдущий (2.4.0 ≈ 5.5/10): см. ниже «Закрыто».

**Метод:** обзор auth/CSRF/ownership/SSRF/credits/webhook/upload/Docker/karaoke/portrait + live `/health`.  
**Код:** `2.6.2` (эпоха 12: CheckRedirect + claim→spend).

---

## Вердикт

| Критерий | 2.4 | **2.6.1** | Комментарий |
|----------|-----|-----------|-------------|
| Архитектура | 7 | **7.5** | Слои ок; jobs без exclusive claim |
| Безопасность | 5.5 | **7.0** | Fail-closed; SSRF redirect остаётся |
| Надёжность | 6.5 | **6.5** | Atomic spend; idem↔credits race |
| Тесты | 6 | **6.5** | CI; нет redirect / race / media authz |
| Prod-ready | 5 | **7.0** | Guards + JWT + `/login`; root Docker |
| Cost control | 5 | **5.5** | Gen/cover/karaoke берут кредиты; TTS/clone — нет |

**Итог: 6.8/10** — shipable MVP на Amvera при включённых guards.  
До ~8.5: SSRF harden, claim→spend, cost controls, non-root, session cookie.  
До ~10: payments, CSP, signed media, dual JWT rotation.

```
Код + prod_guards ON ───► ~6.8–7
UVO_ALLOW_INSECURE=true ► ~3–4 (открытый demo снова)
```

---

## Закрыто в 2.6.1

| Было | Сейчас |
|------|--------|
| Нет fail-closed | `ApplyProductionGuards` — force `ALLOW_ANON`/`DEV_AUTH`/`DEMO_TOPUP=false` |
| Слабый JWT default | Durable `/data/jwt_secret`, reject weak |
| Нет публичного URL | `WEB_PUBLIC_URL` в Docker + fallback |
| Нет студийного входа без demo | MAX `/login` → JWT deep link 7d |

Live подтверждено: `prod_guards:true`, флаги off, `web_public_url` set.

---

## Критично (остаток)

### C1. Локальный `.env` toxic
`ALLOW_ANON`/`DEV_AUTH`/`DEMO_TOPUP=true` + живые ключи. Gitignored, но копипаст в Amvera или `UVO_ALLOW_INSECURE=true` снимает guards.

**Действие:** не копировать demo-флаги в prod; пароль DB уже сменён — ок; при сомнении ротировать AceData/MAX/JWT/webhook.

### C2. Escape hatch `UVO_ALLOW_INSECURE`
Полностью отключает fail-closed (`prod.go`).

**Фикс:** разрешать только non-HTTPS + loopback; иначе FATAL.

---

## Высокий (P1) — рекомендации по реализации

| ID | Находка | Где | Реализация |
|----|---------|-----|------------|
| ~~H1~~ | ~~SSRF через redirects~~ | **CLOSED 2.6.2** | `CheckRedirect` + dial reject private; тесты 302→link-local |
| ~~H2~~ | ~~Spend до exclusive job claim~~ | **CLOSED 2.6.2** | `CreateOrClaim` → Spend; `ClaimProcessing` CAS; Refund на lost claim |
| **H3** | TTS без кредитов / RL | `routes.go` `tts` | `Spend(1)` + `Limiter.Allow` (или daily cap) |
| **H4** | Voice clone soft quota TOCTOU | `voice_clone.go` | Atomic quota row; credits; magic-byte sniff (MIME no-op сейчас) |
| **H5** | Karaoke/portrait без gen RL | `routes.go` | `Limiter.Allow` на Hedra/Kling-пути |
| **H6** | Provider body клиенту | `err.Error()`, Hedra | Только `ProviderError` + `redactBody` |
| **H7** | Docker root | `Dockerfile` | `USER` non-root + `chown /data` |
| **H8** | Webhook `?secret=` | `webhook.go` | Только header; убрать query |

---

## Средний (P2)

| ID | Находка | Фикс |
|----|---------|------|
| M1 | `/media/assets/:name` без auth | Signed TTL или RequireAuth |
| M2 | `/uploads` без TTL cleanup | Cron delete 24–48h |
| M3 | JWT в `?token=` 7d | One-time code → HttpOnly cookie; link TTL ≤1h |
| M4 | `PGSSLMODE` default `disable` | `require` для postgres |
| M5 | CSRF `!=`, нет SameSite | Strict + constant-time |
| M6 | `deleteTrack` без `SafeMediaPath` | Resolve path перед `Remove` |
| M7 | Открытые `/health` details + `/metrics` | Урезать публичный health; auth metrics |
| M8 | Private play + Bearer из localStorage | Cookie / signed play URL |
| M9 | Rate limit count-then-insert | Atomic upsert |
| M10 | Нет CSP / security headers | Headers + self-host Tailwind |
| M11 | Caption без лимита | Cap ~500 |
| M12 | Hardcoded Amvera URL fallback | Fail если `WEB_PUBLIC_URL` пуст в prod |

---

## Низкий (P3)

- Sequential public track IDs (ожидаемо)
- Access log может держать `?token=` / `?secret=`
- JWT без `iss`/`aud` / dual-secret rotation
- README version drift
- `SpendTx` не используется; `Add` глотает ошибки
- Payments: `coming_soon` (ожидаемо)

---

## Что уже хорошо

1. `RequireAuth` + ownership на jobs/playlists/TTS voice  
2. Prod guards + durable JWT + `/login`  
3. CSRF на mutate studio API; XSS через `textContent`  
4. `SafeDownload` baseline (HTTPS, allowlist, private IP, size) + `SafeMediaPath` play  
5. Atomic `Spend` + Postgres; unique `IdemKey`  
6. AceData `redactBody` в логах  
7. Cover/karaoke/portrait списывают кредиты + ownership  
8. CI: vet / test / build  

---

## Соответствие prod checklist

| Пункт | Статус |
|-------|--------|
| `go build` / `test` / `vet` | OK |
| CI workflow | Есть |
| RequireAuth + ownership | OK |
| Prod danger-флаги (live) | **OK** (forced off) |
| Fail-closed boot | **OK** (2.6.1) |
| WEB_PUBLIC_URL | **OK** |
| MAX studio login | **OK** (`/login`) |
| Docker non-root | **FAIL** |
| SSRF redirects | **OK** (2.6.2) |
| TTS / clone cost | **FAIL** |
| Idem claim→spend | **OK** (2.6.2) |
| ЮKassa | нет |
| DB password rotated | владелец подтвердил |

---

## План реализации (эпохи)

| | Эпоха | Pri | Работа | Effort |
|---|-------|-----|--------|--------|
| **1** | **12** | P0 | SSRF: `CheckRedirect` + IP pin + тесты | 0.5–1d |
| **2** | **12** | P0 | Generate: claim → spend → CAS worker + refund | 1d |
| **3** | **13** | P0 | Credits+limits: TTS, clone, karaoke, portrait | 1d |
| **4** | **13** | P1 | Sanitize provider errors → `ProviderError` | 0.5d |
| **5** | **13** | P1 | Dockerfile `USER` non-root | 0.5d |
| **6** | **14** | P1 | One-time login → HttpOnly cookie | 1–2d |
| **7** | **14** | P2 | Signed/TTL media; header-only webhook | 1d |
| **8** | **15** | P2 | CI abuse tests; YooKassa; `PGSSLMODE=require` | 2d+ |

**Спринт:** эпоха 12 = integrity (SSRF + idem); 13 = money burn + container; 14–15 = session/ops/pay.

---

## Закрыто ранее (эпохи 7–11 + 2.5–2.6)

- **7–11:** auth lockdown, CSRF/XSS, Postgres, atomic credits, CI, redact  
- **2.5:** voice clone / cover upload  
- **2.6:** karaoke + singing portrait  
- **2.6.1:** fail-closed, durable JWT, `WEB_PUBLIC_URL`, MAX `/login`

**Правило:** закрывать находки этого документа чеклистами новых эпох; не раздувать scope без `EPOCH*.md`.
