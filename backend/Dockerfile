# syntax=docker/dockerfile:1
# =====================================================================
# Backend (NestJS + Prisma) — multi-stage: base → build → prod / dev
# =====================================================================

FROM node:20-bookworm-slim AS base
# openssl + libstdc++/libgomp — требования Prisma-движка и ONNX-рантайма (@xenova/onnxruntime-node)
RUN apt-get update \
  && apt-get install -y --no-install-recommends openssl ca-certificates curl libstdc++6 libgomp1 \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
# Сначала схема Prisma, чтобы postinstall @prisma/client нашёл её
COPY package.json package-lock.json ./
COPY prisma ./prisma
RUN npm ci --no-audit --no-fund
RUN npx prisma generate

FROM base AS build
COPY . .
RUN npm run build

# ---------- PRODUCTION ----------
FROM base AS prod
ENV NODE_ENV=production
COPY --from=build /app/dist ./dist
COPY prisma ./prisma
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=10s --start-period=90s --retries=5 \
  CMD curl -sf http://localhost:3000/api/health || exit 1
# Миграции применяются при старте, затем приложение
CMD ["sh", "-c", "npx prisma migrate deploy && node dist/main.js"]

# ---------- DEVELOPMENT (hot-reload, монтируется через docker-compose.dev.yml) ----------
FROM base AS dev
COPY . .
CMD ["npm", "run", "dev"]
