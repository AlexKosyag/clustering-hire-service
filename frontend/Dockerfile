# syntax=docker/dockerfile:1
# =====================================================================
# Frontend (Next.js 14 + AntD) — multi-stage: base → deps → build → prod / dev
# =====================================================================

FROM node:20-bookworm-slim AS base
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1

FROM base AS deps
COPY package.json package-lock.json ./
# npm 10 в контейнере не блокирует install-scripts (sharp и пр.)
RUN npm ci --no-audit --no-fund

# ---------- BUILD ----------
FROM deps AS build
# rewrites(next.config.js) вычисляются на этапе build → адрес backend нужен заранее
ARG BACKEND_URL=http://backend:3000
ENV BACKEND_URL=${BACKEND_URL}
COPY . .
RUN npm run build

# ---------- PRODUCTION ----------
FROM node:20-bookworm-slim AS prod
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0
# standalone-сборка: минимальный node_modules + server.js
COPY --from=build /app/package.json ./package.json
COPY --from=build /app/next.config.js ./next.config.js
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=5 \
  CMD node -e "require('http').get('http://127.0.0.1:3000/',(r)=>process.exit(r.statusCode<500?0:1)).on('error',()=>process.exit(1))"
CMD ["node", "server.js"]

# ---------- DEVELOPMENT (hot-reload через bind-mount в docker-compose) ----------
FROM deps AS dev
COPY . .
EXPOSE 3000
CMD ["npx", "next", "dev", "-p", "3000"]
