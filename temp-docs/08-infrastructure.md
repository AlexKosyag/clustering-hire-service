# 08. Инфраструктура, данные и качество

Содержание: [README](README.md) · [01](01-architecture.md) · [02](02-data-model.md) · [03](03-auth-and-security.md) · [04](04-api-reference.md) · [05](05-matching-engine.md) · [06](06-testing-engine.md) · [07](07-fsp-gamification-notifications.md) · **08** · [09](09-known-issues.md)

---

## 1. Docker-стек (`docker-compose.yml`)

| Сервис | Образ/сборка | Порты (хост:контейнер) | Назначение |
|--------|-------------|------------------------|-----------|
| `postgres` | `pgvector/pgvector:pg16` | `5433:5432` | БД + pgvector; healthcheck `pg_isready`; монтирует `database/init.sql` |
| `redis` | `redis:7-alpine` | `6379:6379` | кэш matching (`--appendonly yes`) |
| `minio` | `bitnamilegacy/minio` | `9000:9000`, `9001:9001` | файлы + консоль |
| `ml-service` | `./ml-service` | `8000:8000` | FastAPI: эмбеддинги (`paraphrase-multilingual-mpnet-base-v2`, 768-dim), reranker (`ms-marco-MiniLM-L-6-v2`), NER; лимит памяти 2G, start_period 120с |
| `backend` | `./backend` (target `prod`) | `3000:3000` | NestJS API; depends_on все вышеперечисленные healthy |
| `frontend` | `./frontend` (target `prod`) | `8080:3000` | Next.js (аргумент `BACKEND_URL=http://backend:3000`) |
| `nginx` | `nginx:alpine` | `80:80` | единая точка входа (`docker/nginx/nginx.conf`) |

Вolumes: `postgres_data`, `redis_data`, `minio_data`, `ml_models`.

## 2. Сборка бэкенда (`backend/Dockerfile`)

Многостадийная: `base → build → prod | dev`.
- **base**: `node:20-bookworm-slim` + `openssl, ca-certificates, curl, libstdc++6, libgomp1` (нужны Prisma-движку и `@xenova/onnxruntime-node`); `prisma/` копируется **до** `npm ci`, чтобы postinstall `@prisma/client` нашёл схему; `npx prisma generate`.
- **prod**: `NODE_ENV=production`, копируются `dist/` и `prisma/`, `EXPOSE 3000`, HEALTHCHECK `curl -sf http://localhost:3000/api/health` (interval 30s, timeout 10s, start-period 90s, 5 retries); стартовая команда:
  ```sh
  npx prisma migrate deploy && node dist/main.js
  ```
  Миграции применяются автоматически, **сид — нет** (выполняется вручную).
- **dev**: `npm run dev` (`nest start --watch`), исходники монтируются bind-mount.

## 3. Dev-overlay (`docker-compose.dev.yml`)

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build
```
- `backend` → target `dev`, `NODE_ENV=development`, `EMBEDDING_PROVIDER` по умолчанию **`tfidf`** (мгновенный старт без загрузки ONNX-моделей), bind-mount `./backend:/app` + анонимные volumes для `node_modules`/`dist` + кэш моделей `backend_models:/app/.cache`; команда `sh -c "npx prisma migrate deploy && npx nest start --watch"`;
- `frontend` → target `dev`, `NEXT_PUBLIC_API_URL=/api` (Next-rewrites проксируют `/api` на backend — CORS не нужен);
- сид: `docker compose exec backend npm run db:seed`.

## 4. Переменные окружения

### Корневой `.env.example` (для docker-compose)
| Переменная | Назначение |
|-----------|-----------|
| `DB_PASSWORD`, `POSTGRES_DB`, `POSTGRES_USER` | учётные данные PostgreSQL (передаются в `DATABASE_URL` и контейнер postgres) |
| `REDIS_URL` | подключение к Redis |
| `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD` | учётные данные MinIO (маппятся в `MINIO_ACCESS_KEY`/`MINIO_SECRET_KEY` бэкенда) |
| `JWT_SECRET` / `JWT_REFRESH_SECRET` | подписи access (15 мин) / refresh (7 дней) токенов |
| `NODE_ENV`, `PORT`, `FRONTEND_URL` | режим, порт API, CORS-origin |
| `SMTP_HOST/PORT/USER/PASS/FROM` | резерв под e-mail (⚠️ в коде не используется) |
| `EMBEDDING_PROVIDER` | `auto` (каскад MiniLM → TF-IDF) / `tfidf` / `transformers` |
| `NEXT_PUBLIC_API_URL` | базовый URL API для фронтенда |

### `backend/.env` (локальная разработка)
`DATABASE_URL` (Postgres на хост-порту **5433**), `REDIS_URL`, `JWT_SECRET`, `JWT_REFRESH_SECRET`, `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`, `EMBEDDING_PROVIDER`, `PORT`. Переменные docker-compose имеют приоритет (dotenv не перезаписывает заданные).

### Переменные, читаемые только из кода
`MINIO_PUBLIC_ENDPOINT`, `ML_SERVICE_URL`, `ML_SERVICE_TIMEOUT`, `TESTING_TASKS_PER_TEST`, `TESTING_COOLDOWN_DAYS`, `OPENAI_API_KEY/URL/MODEL`, `SANDBOX_TIMEOUT_MS/MEMORY_MB/DOCKER`, `API_URL` (метрики). Полная сводка — [01-architecture.md](01-architecture.md) §7.

## 5. Миграции и работа с БД

| Команда | Действие |
|---------|----------|
| `npm run prisma:generate` | генерация Prisma Client |
| `npm run prisma:migrate` | `prisma migrate dev` (создание новой миграции в разработке) |
| `npm run prisma:deploy` | `prisma migrate deploy` (применение в проде; также в Docker-старте) |
| `npm run db:seed` | демо-данные (`prisma/seed.ts`) |
| `npm run db:reset` | `prisma migrate reset --force` (полный сброс + сид) |
| `npm run seed:embeddings` | предзаполнение векторов TF-IDF (`prisma/embeddings.ts`) |

Состав сида — [02-data-model.md](02-data-model.md) §8 (858 вопросов, 204 кандидата, 8 работодателей, 30 вакансий, 15 приглашений, 39 достижений).

## 6. Тесты

### Unit (`npm test`, Jest, `*.spec.ts` в `src/`)
Области: IRT/CAT-математика (`cat.spec.ts`), скоринг тестов (`scoring.spec.ts`), анти-чит (оба, `anti-cheat.spec.ts`, `behavioral-anti-cheat.spec.ts`), движок V2 (`testing-v2.service.spec.ts` — регрессия `duplicate`-ответов и детерминизма CAT), скоринг matching (`scoring-v2.service.spec.ts`), объяснения (`explanation.service.spec.ts`), TF-IDF (`embedding/tfidf.spec.ts`).

### E2E (`npm run test:e2e`, `test/app.e2e.spec.ts`)
Запускает реальный `AppModule` против **реальной засеянной БД** (`--runInBand`, таймаут 180 с), регистрирует свежие аккаунты. Покрытие:
- health (`ok`, БД up, `dim: 384`), `401` без токена, `400` на невалидном register;
- auth: регистрация, дубль email → 409, login, refresh-ротация, `/auth/me`;
- профиль кандидата: автосоздание, `PUT` сохраняет stack/специализацию;
- RBAC: кросс-рольные доступы → 403;
- matching: жёсткий фильтр BACKEND, `matchScore ∈ [0,100]` и конечен, объяснения непустые, сортировка по убыванию, < 500 мс, отсутствие NaN;
- testing: старт (`totalQuestions: 20`), отсутствие утечек `correctAnswer`/`isCorrect`/`explanation` в `/current`, дубль старта → 409, полное прохождение → score 0–100, recommendation ∈ {PASS, FAIL, UPGRADE}, θ ∈ [−4, 4], antiCheat присутствует, cooldown установлен, повторный старт заблокирован;
- vacancies: список (≥ 30), детали;
- privacy: приглашение создаётся, контакты заблокированы (`contactsLocked: true`, `contacts: null`), поиск не утекает email, accept открывает контакты только пригласившему;
- FSP/notifications/docs: `/api/fsp/catalog`, `/api/notifications`, `/api/docs-json` (> 20 путей).

Не покрыто e2e: геймификация, загрузка файлов, ежедневные задания.

## 7. Автовалидация (`npm run validate` → `scripts/metrics.ts`)

Требует запущенные backend (`API_URL`, default `http://localhost:3000/api`) и frontend (`FRONTEND_URL`, default `http://localhost:8080`). Пишет `docs/VALIDATION_REPORT.md`; коды выхода: 0 — всё прошло, 1 — есть провалы, 2 — фатальная ошибка.

| Раздел | Проверки |
|--------|----------|
| Smoke | `/api/health` (status ok, dim 384), `/api/docs-json`, фронтенд (содержит «ФСП»), `/login`, прокси `/api` через фронтенд |
| Security | без токена → 401; невалидный JWT → 401; кросс-роль → 403; SQLi в логин → 4xx; слабый пароль → 400; отсутствующая вакансия → 404; кандидат создаёт приглашение → 403; matching без токена → 401 |
| Matching quality | 6 синтетических потребностей; relevance = та же специализация и ≥ 50% overlap стека; **precision@5 (цель ≥ 70%)**, precision@10, recall@10, **MRR**, **nDCG@10**, латентность, проверка жёстких фильтров |
| Testing quality | 3 полных прогона (сильный ×2, слабый): дискриминативность (score ≥ 80 / ≤ 30, Δθ ≥ 2), test-retest (|Δscore| ≤ 10), согласованность рекомендаций, отсутствие ложных срабатываний анти-чита |
| Performance | 30 итераций на эндпоинт (health, профиль, vacancies, notifications, dashboard, matching/search, matching/semantic); цели: avg < 200 мс, **p95 < 500 мс**, фронтенд-документ < 3000 мс |

Также в репозитории есть `scripts/smoke.ps1` (быстрый smoke API) — см. корневой `README.md`.

## 8. Целевые метрики проекта

| Метрика | Цель |
|---------|------|
| precision@5 matching | ≥ 70% |
| Латентность поиска кандидатов | < 500 мс (p95) |
| Средняя латентность API | < 200 мс |
| Загрузка фронтенда | < 3 с |
| Стабильность тестов | test-retest |Δscore| ≤ 10, грейд стабилен |

## 9. Логирование и диагностика

- `LoggingInterceptor` — строка `METHOD /url statusCode Xms` (Logger `HTTP`) на каждый запрос;
- `Logger('Bootstrap')` — адреса API и Swagger при старте;
- `Logger('HttpException')` — необработанные ошибки (фильтр);
- `GET /api/health` — состояние БД, Redis и embedding-провайдера (используется Docker HEALTHCHECK).
Структурированного логирования/трейсинга (OpenTelemetry и т.п.) нет.
