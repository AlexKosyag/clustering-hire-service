# Цифровая платформа-агрегатор ИТ-вакансий ФСП

## Быстрый старт

### Требования
- Docker и Docker Compose
- Go 1.25+ (для локальной разработки)

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
# backend: go run ./cmd/server на :3000 (hot-reload через bind-mount), frontend: next dev на :8080
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build

# один раз — сид демо-данных (858 вопросов, 204 кандидата, 30 вакансий)
docker compose exec backend go run ./cmd/seed
# предзаполнение TF-IDF-векторов (semantic-поиск работает сразу после сида)
docker compose exec backend go run ./cmd/seed-embeddings
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
docker compose up -d postgres redis minio ml-service

# 2. Backend (Go) — конфиг читается из окружения (DATABASE_URL и пр., см. .env.example)
cd backend
go run ./cmd/server          # миграции применяются при старте, http://localhost:3000/api
go run ./cmd/seed            # + go run ./cmd/seed-embeddings, если БД пуста

# 3. Frontend (в другом терминале) — /api проксируется на localhost:3000
cd frontend
npm install
npm run dev                    # http://localhost:8080
```

## Архитектура

Подробнее: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)

```
Frontend (Next.js) → API Gateway → Backend (Go) → PostgreSQL + Redis + MinIO
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
# 1. Smoke test API (23–25 проверок в зависимости от состояния данных)
pwsh scripts/smoke.ps1

# 2. Unit-тесты (CAT/IRT, скоринг, анти-чит, TF-IDF, объяснимость)
cd backend && go test ./internal/...

# 3. E2E-тесты API (реальная БД; нужен DATABASE_URL)
go test ./test/e2e/ -count=1 -timeout 5m

# 4. Метрики: security, precision@5, test-retest, производительность
go run ./cmd/metrics         # пишет docs/VALIDATION_REPORT.md

# 5. Сборка и vet
go build ./... && go vet ./...
cd ../frontend && npm run typecheck && npm run build
```

**Целевые метрики**: precision@5 ≥ 70% · поиск < 500 мс · API < 200 мс (avg) · frontend < 3 с.

## Технологический стек

- **Backend**: Go 1.25, gin, pgx/v5 (+ pgvector), Redis 7, MinIO, swaggo (OpenAPI 3.0)
- **Frontend**: Next.js 14 (App Router), Ant Design 5, axios
- **ML**: ml-service (sentence-transformers, 384-dim) + локальный TF-IDF fallback (`EMBEDDING_PROVIDER=auto`)
- **Testing**: 3PL IRT, Newton-Raphson/Fisher scoring, анти-чит, Go test (unit + e2e)
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