# Эпоха 8 — Auth UX + CSRF + XSS

**Цель:** фронт и cookie/Bearer-сессии совместимы с Lockdown; закрыть XSS и дыры CSRF.  
**Версия:** 2.1.0  
**Зависит от:** эпоха 7  
**База:** [AUDIT.md](AUDIT.md) §6 CSRF/XSS, [FIXPLAN.md](FIXPLAN.md) P0 #7–8, P1 #14.

## Сделать

### Фронт (static)
- [x] `static/auth.js` — localStorage JWT, Bearer, CSRF, `ensureDevToken`, `el()` без XSS
- [x] Все страницы: `index`, `tracks`, `feed`, `playlists` используют `UVO.api`
- [x] Кнопка «Demo token» + статус на страницах

### CSRF
- [x] CSRF на generate, voice, tts, topup, simulate, feed, playlists, delete/edit
- [x] Cookie `Secure` если `WEB_PUBLIC_URL` начинается с `https://`
- [x] Фронт шлёт `X-CSRF-Token`

### XSS
- [x] Title / Caption / Name через `textContent` / `UVO.el`

### UX при 401
- [x] Сообщение про авторизацию в `UVO.api`
- [x] Кредиты обновляются только после успешного ответа

## Не входит
- OAuth MAX / email login
- Postgres

## Приёмка
- [x] `go test` CSRF + auth
- [ ] Ручная: `ALLOW_ANON=false` + Demo token → generate
- [ ] Ручная: Title с `<script>` не исполняется

## Статус
✅ Код эпохи 8 закрыт (2026-07-28). Дальше — [EPOCH9.md](EPOCH9.md).
