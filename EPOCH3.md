# Эпоха 3 — Security

## Сделано
- JWT: пустой user_id → 401 на generate (ALLOW_ANON=true для локального demo)
- CSRF cookie + header на feed/playlists/delete/edit
- DEV_AUTH для /api/auth/token
- .env.example без секретов
- Play: private tracks только owner

## Prod checklist
ALLOW_ANON=false
DEV_AUTH=false
JWT_SECRET=<long random>
HTTPS + Secure cookies later
