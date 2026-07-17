# Build frontend
FROM node:22-alpine AS web-builder
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Build backend
FROM golang:1.24-alpine AS go-builder
WORKDIR /src
RUN apk add --no-cache gcc musl-dev
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /out/faka-server ./cmd/server

# Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
  && adduser -D -H -u 10001 app
WORKDIR /app
COPY --from=go-builder /out/faka-server /app/faka-server
COPY --from=web-builder /web/dist /app/web/dist
RUN mkdir -p /app/data /app/storage/uploads /app/storage/downloads \
  && chown -R app:app /app
USER app
ENV TIKAWANG_BASE_DIR=/app \
    TIKAWANG_ADDR=0.0.0.0:18743 \
    TIKAWANG_STATIC_DIR=/app/web/dist \
    TZ=UTC
EXPOSE 18743
VOLUME ["/app/data", "/app/storage"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:18743/api/inventory >/dev/null || exit 1
ENTRYPOINT ["/app/faka-server"]
