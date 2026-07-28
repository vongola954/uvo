FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /uvo ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 uvo
WORKDIR /app
COPY --from=build /uvo .
COPY internal/api/web/static ./internal/api/web/static
# /data — persistent volume on Amvera (media + jwt_secret)
RUN mkdir -p /data/media /data/logs \
    && chown -R uvo:uvo /app /data
ENV WEB_HOST=0.0.0.0 \
    WEB_PORT=80 \
    PORT=80 \
    DB_DRIVER=postgres \
    MEDIA_ROOT=/data/media \
    WEB_PUBLIC_URL=https://uvo-baskakovanton.amvera.io \
    ALLOW_ANON=false \
    DEV_AUTH=false \
    DEMO_TOPUP=false \
    VOICE_CLONE_PROVIDER=acedata \
    BOT_MODE=polling
USER uvo
EXPOSE 80
CMD ["./uvo"]
