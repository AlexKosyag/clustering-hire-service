# syntax=docker/dockerfile:1
# =====================================================================
# Backend (Go 1.25 + gin + pgx) — multi-stage: build → prod / dev
# =====================================================================

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
  && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed \
  && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed-embeddings ./cmd/seed-embeddings \
  && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/metrics ./cmd/metrics

# ---------- PRODUCTION ----------
FROM debian:bookworm-slim AS prod
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/server /out/seed /out/seed-embeddings /out/metrics /app/
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=5 \
  CMD curl -sf http://localhost:3000/api/health || exit 1
# Миграции применяются при старте сервера, затем сам сервер
# (эквивалент `npx prisma migrate deploy && node dist/main.js`).
# Сид и векторы: docker compose exec backend /app/seed && /app/seed-embeddings
CMD ["/app/server"]

# ---------- DEVELOPMENT (hot-reload через go run, монтируется через docker-compose.dev.yml) ----------
FROM build AS dev
ENV GOCACHE=/tmp/go-build GOMODCACHE=/go/pkg/mod
EXPOSE 3000
# Миграции тоже выполняются при старте; сид: docker compose exec backend go run ./cmd/seed
CMD ["sh", "-c", "go run ./cmd/server"]
