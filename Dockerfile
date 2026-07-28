FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /uvo ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /uvo .
COPY internal/api/web/static ./internal/api/web/static
# /data — persistent volume on Amvera (media files)
RUN mkdir -p /data/media /data/logs
ENV WEB_HOST=0.0.0.0 \
    WEB_PORT=80 \
    PORT=80 \
    DB_DRIVER=postgres \
    MEDIA_ROOT=/data/media
EXPOSE 80
CMD ["./uvo"]
