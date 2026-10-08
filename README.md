# Цифровая платформа-агрегатор ИТ-вакансий ФСП

## Быстрый старт

### Требования
- Docker и Docker Compose
- Node.js 20+ (для локальной разработки)
- npm или yarn

### Запуск через Docker (production-сборка)

```bash
# 1. Создать .env файл (пароли, секреты)
cp .env.example .env

# 2. Запустить весь стек: postgres, redis, minio, backend, frontend, nginx
docker compose up -d --build

# 3. Дождаться запуска (30-60 секунд)
docker compose ps

# 4. Открыть в браузере
# Frontend:      http://localhost:8080
# Через nginx:   http://localhost
# Backend API:   http://localhost:3000/api
# Swagger:       http://localhost:3000/api/docs
# MinIO Console: http://localhost:9001
```

### Запуск в dev-режиме (hot-reload, bind-mount)

```bash
# backend: nest start --watch на :3000, frontend: next dev на :8080
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build

# один раз — сид демо-данных (858 вопросов, 204 кандидата, 30 вакансий)
docker compose exec backend npm run db:seed
```

### Демо-аккаунты

| Роль | Email | Пароль |
|------|-------|--------|
| Кандидат | `test@candidate.com` | `Test123!` |
| Кандидат | `demo2@candidate.com`, `demo3@candidate.com` | `Test123!` |
| Работодатель | `test@employer.com` | `Test123!` |
| Работодатель | `hr@company.com`, `hr3@example.com` | `Test123!` |

### Запуск локально (для разработки)

```bash
# 1. Инфраструктура (PostgreSQL слушает 5433 на хосте)
docker compose up -d postgres redis minio

# 2. Backend — конфиг читается из backend/.env (localhost:5433)
cd backend
npm install
npx prisma migrate deploy      # + npx prisma db seed, если БД пуста
npm run start:dev              # nest start --watch, http://localhost:3000/api

# 3. Frontend (в другом терминале) — /api проксируется на localhost:3000
cd frontend
npm install
npm run dev                    # http://localhost:8080
```

## Архитектура

Подробнее: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)

```
Frontend (Next.js) → API Gateway → Backend (NestJS) → PostgreSQL + Redis + MinIO
                                         ↓
                                   Matching Engine
                                   Testing Engine
                                   FSP Integration
```

## Модули

| Модуль | Описание | Документация |
|--------|----------|--------------|
| Database | PostgreSQL + Redis + MinIO | [docs/modules/database.md](docs/modules/database.md) |
| Auth | Аутентификация и авторизация | [docs/modules/auth.md](docs/modules/auth.md) |
| Backend API | Основной API | [docs/modules/backend-api.md](docs/modules/backend-api.md) |
| Testing Engine | Движок тестирования (CAT + IRT) | [docs/modules/testing-engine.md](docs/modules/testing-engine.md) |
| Matching Engine | Подбор кандидатов | [docs/modules/matching-engine.md](docs/modules/matching-engine.md) |
| Frontend Candidate | Личный кабинет кандидата | [docs/modules/frontend-candidate.md](docs/modules/frontend-candidate.md) |
| Frontend Employer | Личный кабинет работодателя | [docs/modules/frontend-employer.md](docs/modules/frontend-employer.md) |
| FSP Integration | Интеграция с ФСП | [docs/modules/fsp-integration.md](docs/modules/fsp-integration.md) |

## Опциональные функции

| Функция | Описание | Документация |
|---------|----------|--------------|
| Gamification | Бейджи, уровни, стрики | [docs/concepts/gamification.md](docs/concepts/gamification.md) |
| Vacancy Apply | Отклик на вакансии | [docs/concepts/optional-features.md](docs/concepts/optional-features.md) |
| PDF Parsing | Парсинг резюме | [docs/concepts/optional-features.md](docs/concepts/optional-features.md) |

## План разработки

Подробнее: [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)

```
Этап 0: Database (2ч)
Этап 1: Auth + Backend + Testing (6ч, параллельно)
Этап 2: Matching + FSP (4ч, параллельно)
Этап 3: Frontend Candidate + Employer (6ч, параллельно)
Этап 4: Integration (4ч)
```

**Общее время**: ~18 часов (с параллелизацией)

## Валидация

Подробнее: [docs/VALIDATION.md](docs/VALIDATION.md) · результаты: [docs/VALIDATION_REPORT.md](docs/VALIDATION_REPORT.md)

```bash
# 1. Smoke test API (25–27 проверок в зависимости от состояния данных)
pwsh scripts/smoke.ps1

# 2. Unit-тесты (CAT/IRT, скоринг, анти-чит, TF-IDF, объяснимость)
cd backend && npm test

# 3. E2E-тесты API (31 проверка, реальная БД)
npm run test:e2e

# 4. Метрики: security, precision@5, test-retest, производительность
npm run validate               # пишет docs/VALIDATION_REPORT.md

# 5. Типы и сборка
npm run typecheck && npm run build
cd ../frontend && npm run typecheck && npm run build
```

**Целевые метрики**: precision@5 ≥ 70% · поиск < 500 мс · API < 200 мс (avg) · frontend < 3 с.

## Технологический стек

- **Backend**: NestJS 10, Prisma 5, PostgreSQL 16 + pgvector, Redis 7, MinIO
- **Frontend**: Next.js 14 (App Router), Ant Design 5, axios
- **ML**: @xenova/transformers (384-dim, каскад MiniLM → TF-IDF fallback)
- **Testing**: 3PL IRT, Newton-Raphson/Fisher scoring, анти-чит, Jest + supertest
- **Infrastructure**: Docker Compose (prod + dev-overlay), Nginx
- **Documentation**: Swagger (OpenAPI 3.0), docs/

## Критерии оценки

| Критерий | Вес | Наш подход |
|----------|-----|------------|
| Механика подбора | 35% | Employer-first + semantic matching + объяснимость |
| Механика тестирования | 25% | CAT + IRT + anti-cheat |
| Интерфейс | 10% | Ant Design, responsive, intuitive UX |
| Техническая проработка | 7% | Clean architecture, Docker, Swagger |
| Полнота реализации | 6% | Все обязательные функции |
| Доп. функционал | 7% | Отклики, вакансии, геймификация |
| Презентация | 10% | Сквозной сценарий, демо |

## Лицензия

MIT