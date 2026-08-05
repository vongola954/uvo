FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /uvo ./cmd/server

FROM alpine:3.20
# Copy CA bundle from build image — avoid `apk` (Amvera builders often cannot reach Alpine CDN).
# Append Russian Trusted Root/Sub (Минцифры) — required for platform-api2.max.ru TLS.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY internal/clients/certs/*.pem /tmp/ru-ca/
RUN cat /tmp/ru-ca/*.pem >> /etc/ssl/certs/ca-certificates.crt && rm -rf /tmp/ru-ca
RUN adduser -D -H -u 10001 uvo
WORKDIR /app
COPY --from=build /uvo .
COPY internal/api/web/static ./internal/api/web/static
# /data — persistent volume on Amvera (media + jwt_secret)
RUN mkdir -p /data/media /data/logs \
    && chown -R uvo:uvo /app /data
# WEB_PUBLIC_URL: override in Amvera env if domain changes; fallback keeps prod bootable.
ENV WEB_HOST=0.0.0.0 \
    WEB_PORT=8080 \
    PORT=8080 \
    DB_DRIVER=postgres \
    MEDIA_ROOT=/data/media \
    WEB_PUBLIC_URL=https://uvo-baskakovanton.amvera.io \
    ALLOW_ANON=false \
    DEV_AUTH=false \
    DEMO_TOPUP=false \
    VOICE_CLONE_PROVIDER=acedata \
    BOT_MODE=polling \
    MAX_BOT_USERNAME=id262812853458_bot
USER uvo
EXPOSE 8080
CMD ["./uvo"]
