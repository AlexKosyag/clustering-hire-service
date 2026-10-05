# Цифровая платформа-агрегатор ИТ-вакансий ФСП

## Быстрый старт

### Требования
- Docker и Docker Compose
- Node.js 20+ (для локальной разработки)
- npm или yarn

### Запуск через Docker

```bash
# 1. Клонировать репозиторий
git clone <repo-url>
cd LCTFSP

# 2. Создать .env файл
cp .env.example .env
# Отредактировать .env — задать пароли

# 3. Запустить все сервисы
docker-compose up -d

# 4. Дождаться запуска (30-60 секунд)
docker-compose ps

# 5. Открыть в браузере
# Frontend: http://localhost:8080
# Backend API: http://localhost:3000/api
# Swagger: http://localhost:3000/api/docs
# MinIO Console: http://localhost:9001
```

### Запуск локально (для разработки)

```bash
# 1. Запустить инфраструктуру
docker-compose up -d postgres redis minio

# 2. Backend
cd backend
npm install
npx prisma migrate dev
npx prisma db seed
npm run start:dev

# 3. Frontend (в другом терминале)
cd frontend
npm install
npm run dev
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

Подробнее: [docs/VALIDATION.md](docs/VALIDATION.md)

```bash
# Smoke test
docker-compose up -d
curl http://localhost:3000/api/health

# Unit tests
cd backend && npm test

# E2E tests
npm run test:e2e
```

## Технологический стек

- **Backend**: NestJS 10, Prisma 5, PostgreSQL 16, Redis 7
- **Frontend**: Next.js 14, Ant Design 5, Zustand
- **ML**: @xenova/transformers (sentence-transformers)
- **Infrastructure**: Docker, Nginx
- **Documentation**: Swagger (OpenAPI 3.0)

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