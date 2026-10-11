# Архитектура платформы ФСП

## 1. Обзор системы

Цифровая платформа-агрегатор ИТ-вакансий с верифицированным профилем достижений участника Федерации спортивного программирования (ФСП).

### Ключевые принципы
- **Employer-first модель**: работодатель инициирует контакт, кандидат принимает/отклоняет
- **Категоризация по тестированию**: грейд определяется тестом, а не самооценкой
- **FSP Achievement Integration**: автоматическое обогащение профиля результатами соревнований
- **Anti-cheat устойчивость**: уникальные задания для каждого кандидата

## 2. Высокоуровневая архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                      FRONTEND LAYER                         │
│  ┌──────────────────┐  ┌──────────────────┐                │
│  │  Candidate SPA    │  │  Employer SPA     │                │
│  │  (React/Next.js)  │  │  (React/Next.js)  │                │
│  └────────┬─────────┘  └────────┬─────────┘                │
│           │                      │                          │
└───────────┼──────────────────────┼──────────────────────────┘
            │                      │
            ▼                      ▼
┌─────────────────────────────────────────────────────────────┐
│                      REVERSE PROXY                          │
│                       (Nginx)                               │
│         Rate Limiting, Routing, прокси /api → backend       │
└───────────────────────────┬─────────────────────────────────┘
                            │
            ┌───────────────┼───────────────┐
            ▼               ▼               ▼
┌────────────────┐ ┌────────────────┐ ┌────────────────┐
│   AUTH MODULE  │ │  CORE API      │ │ TESTING ENGINE │
│   (JWT +       │ │  (Go / gin)    │ │  (Go, V1+V2)   │
│    bcrypt)     │ │                │ │                │
└───────┬────────┘ └───────┬────────┘ └───────┬────────┘
        │                  │                  │
        ▼                  ▼                  ▼
┌─────────────────────────────────────────────────────────────┐
│                    DATA LAYER                               │
│  ┌────────────────┐  ┌────────────────┐  ┌──────────────┐  │
│  │   PostgreSQL    │  │     Redis      │  │  S3/MinIO    │  │
│  │   (основная БД) │  │   (кэш/сессии) │  │  (файлы)     │  │
│  └────────────────┘  └────────────────┘  └──────────────┘  │
└─────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────┐
│                 MATCHING ENGINE (Go)                        │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Semantic Matching (ml-service: sentence-transformers,│  │
│  │                     фолбэк — TF-IDF 384-dim)         │  │
│  │  Scoring V1/V2 + объяснения                          │  │
│  │  Category Ranking                                    │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘

Внешний ML-сервис (ml-service, Python/FastAPI) подключается по HTTP
(/embed, /rerank, /ner); при его недоступности matching деградирует
на локальный TF-IDF (детерминированный, 384 измерения).
```

## 3. Модули и их зависимости

### 3.1. Карта зависимостей

```
                    ┌─────────────┐
                    │  database   │  ← Фундамент, 0 зависимостей
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │   auth   │ │ backend  │ │ testing  │  ← Зависят от database
        └────┬─────┘ │   api    │ │  engine  │
             │       └────┬─────┘ └────┬─────┘
             │            │            │
             ▼            ▼            ▼
        ┌────────────────────────────────────┐
        │         matching engine            │  ← Зависит от backend + testing
        └────────────────┬───────────────────┘
                         │
              ┌──────────┴──────────┐
              ▼                     ▼
        ┌───────────┐        ┌───────────┐
        │ frontend  │        │ frontend  │
        │ candidate │        │ employer  │  ← Зависят от всех API
        └───────────┘        └───────────┘
```

### 3.2. Матрица параллелизации

| Этап | Модули | Параллельно? | Ожидаемое время |
|------|--------|--------------|-----------------|
| 1 | database, docker-compose | Нет (база) | 2ч |
| 2 | auth, backend-api, testing-engine | **ДА** | 6ч |
| 3 | matching-engine | Нет (depends on 2) | 4ч |
| 4 | frontend-candidate, frontend-employer | **ДА** (зависят от API) | 6ч |
| 5 | fsp-integration, gamification | **ДА** (опционально) | 3ч |
| 6 | интеграция, тестирование, документация | Нет | 4ч |

**Критический путь**: database → backend-api → matching-engine → frontend → integration

## 4. Технологический стек

### Backend (`backend/`, Go-порт NestJS-бэкенда)
- **Language**: Go 1.25
- **HTTP**: gin (маршрутизация, middleware: CORS, logging, recovery)
- **БД**: pgx/v5 (сырые SQL-запросы, пул соединений), без ORM
- **Миграции**: встроенный раннер `store.DB.Migrate` + `go:embed` (SQL-файлы `backend/migrations/*.up.sql`; бутстрап переносит отметки из `_prisma_migrations`)
- **Auth**: golang-jwt/v5 (access 15 мин / refresh 7 дней) + bcrypt (cost 12)
- **Валидация**: собственный слой в `internal/common` с текстами class-validator (русские сообщения)
- **Документация**: swaggo → `/api/docs` (Swagger UI) + `/api/docs-json` (OpenAPI 3.0)
- **Кэш/хранилище**: go-redis/v9, minio-go/v7, парсинг PDF — ledongthuc/pdf

### Frontend (без изменений при переносе)
- **Framework**: Next.js 14 (App Router, SSR)
- **UI Library**: Ant Design 5 (enterprise-grade, русская локализация)
- **State Management**: Zustand (лёгкий, типизированный)
- **HTTP Client**: Axios + React Query (кеширование, retry)
- **Forms**: React Hook Form + Zod (валидация)

### Database
- **Primary**: PostgreSQL 16 + pgvector (JSONB для гибких полей, векторы 384-dim)
- **Cache**: Redis 7 (кэш подборок matching, сессии)
- **Storage**: MinIO (S3-совместимый, для PDF и файлов)

### ML (без изменений)
- **ml-service**: FastAPI + sentence-transformers (embeddings, reranker, NER) для Matching V2

### Infrastructure
- **Containers**: Docker + docker-compose (backend — multi-stage Go-образ)
- **Reverse Proxy**: Nginx (прокси `/api` на backend, статика фронтенда)
- **Monitoring**: Prometheus + Grafana (опционально)

## 5. Структура проекта

```
LCTFSP/
├── docs/                          # Документация
│   ├── ARCHITECTURE.md            # Этот файл
│   ├── DEVELOPMENT.md             # План разработки и параллелизация
│   ├── VALIDATION.md              # Процедуры валидации
│   ├── modules/                   # Документация модулей
│   │   ├── database.md
│   │   ├── auth.md
│   │   ├── backend-api.md
│   │   ├── testing-engine.md
│   │   ├── matching-engine.md
│   │   ├── frontend-candidate.md
│   │   ├── frontend-employer.md
│   │   └── fsp-integration.md
│   └── concepts/                  # Концепции и опциональные фичи
│       ├── gamification.md
│       └── optional-features.md
│
├── backend/                       # Backend API (Go 1.25 / gin)
│   ├── cmd/
│   │   ├── server/                # Точка входа API (аналог src/main.ts)
│   │   ├── seed/                  # Сид демо-данных (JSON-дамп)
│   │   ├── seed-embeddings/       # Офлайн-запись эмбеддингов
│   │   └── metrics/               # Скрипт валидации → docs/VALIDATION_REPORT.md
│   ├── internal/
│   │   ├── app/                   # Сборка приложения + Swagger-роуты
│   │   ├── modules/               # Бизнес-модули (по одному каталогу)
│   │   │   ├── auth/              # Аутентификация (JWT, bcrypt)
│   │   │   ├── candidates/        # Профили кандидатов (+ PDF-резюме)
│   │   │   ├── candidateview/     # Вид кандидата для matching
│   │   │   ├── employers/         # Профили работодателей
│   │   │   ├── vacancies/         # Вакансии
│   │   │   ├── invitations/       # Приглашения
│   │   │   ├── testing/           # Тестирование V1/V2 (+ services/)
│   │   │   ├── matching/          # Подбор кандидатов V1/V2
│   │   │   ├── fsp/               # Интеграция с ФСП
│   │   │   ├── gamification/      # Геймификация
│   │   │   ├── notifications/     # Уведомления
│   │   │   ├── storage/           # Загрузка файлов (MinIO)
│   │   │   └── health/            # /api/health
│   │   ├── common/                # Router, ошибки, bind, JWT, валидация
│   │   ├── config/                # Конфигурация (env)
│   │   ├── store/                 # Подключение к БД, миграции, векторы
│   │   ├── seed/                  # Логика сида + JSON-данные
│   │   ├── cache/, model/         # Redis-клиент, модели БД
│   ├── migrations/                # SQL-миграции (go:embed)
│   ├── docs/                      # Сгенерированный Swagger (swaggo)
│   ├── test/e2e/                  # E2E тесты (in-process, реальная БД)
│   ├── Dockerfile                 # Multi-stage: golang → debian-slim
│   └── go.mod / go.sum
│
├── frontend/                      # Frontend (Next.js)
│   ├── src/
│   │   ├── app/                   # App Router
│   │   │   ├── (auth)/            # Страницы авторизации
│   │   │   ├── candidate/         # Личный кабинет кандидата
│   │   │   ├── employer/          # Личный кабинет работодателя
│   │   │   └── layout.tsx
│   │   ├── components/            # Переиспользуемые компоненты
│   │   ├── hooks/                 # Кастомные хуки
│   │   ├── lib/                   # Утилиты, API клиент
│   │   ├── stores/                # Zustand stores
│   │   └── types/                 # TypeScript типы
│   ├── public/
│   ├── Dockerfile
│   ├── package.json
│   └── tsconfig.json
│
├── database/                      # Скрипты и миграции БД
│   ├── init.sql
│   └── seed-data/                 # Тестовые данные
│
├── docker/                        # Docker конфигурация
│   ├── nginx/
│   │   └── nginx.conf
│   └── redis/
│       └── redis.conf
│
├── docker-compose.yml             # Основной compose файл
├── docker-compose.dev.yml         # Dev окружение
├── .env.example                   # Пример переменных окружения
├── .gitignore
└── README.md                      # Инструкция по запуску
```

## 6. Потоки данных

### 6.1. Регистрация и тестирование кандидата

```
Candidate → Frontend → POST /api/auth/register
         → Backend creates User (role=CANDIDATE)
         → Backend creates CandidateProfile (empty)
         → Return JWT

Candidate → Frontend → POST /api/candidates/profile
         → Backend updates profile (FIO, contacts, stack, etc.)

Candidate → Frontend → POST /api/testing/start
         → TestingEngine generates personalized test
         → Return test session with questions

Candidate → Frontend → POST /api/testing/session/:id/answer (на каждый вопрос)
         → POST /api/testing/session/:id/complete
         → TestingEngine evaluates answers
         → Calculates grade (PASS/FAIL/UPGRADE)
         → Updates CandidateProfile.category
         → MatchingEngine re-indexes candidate
```

### 6.2. Работодатель ищет кандидатов

```
Employer → Frontend → POST /api/employers/need
        → Backend stores employer need (stack, grade, description)
        → MatchingEngine computes ranked candidate list
        → Return categorized candidates with explanations

Employer → Frontend → POST /api/invitations
        → Backend creates Invitation (status=SENT)
        → Notification to candidate

Candidate → Frontend → GET /api/invitations (status=SENT)
        → Accepts/Rejects
        → If ACCEPTED: employer sees candidate contacts
```

## 7. Безопасность

### 7.1. Аутентификация
- JWT access token (15 min) + refresh token (7 days), HS256
- Refresh-токен: bcrypt-хеш в `users.refresh_token` + HttpOnly cookie и тело ответа
- Роуты помечаются явно: по умолчанию все закрыты (`RouteOpts{Public, Roles}`)

### 7.2. Авторизация
- Role-based access control (RBAC): CANDIDATE, EMPLOYER, ADMIN
- Guard на уровне маршрутов (аналог guard-ов NestJS): токен → пользователь в БД → роли
- Кандидат видит только свои данные
- Работодатель не видит контакты кандидата до ACCEPTED

### 7.3. Защита данных
- bcrypt для хеширования паролей (cost 12; паритет bcryptjs — вход > 72 байт усекается)
- CORS whitelist (FRONTEND_URL + localhost:8080/3000)
- Input validation с сообщениями class-validator (русские тексты)
- SQL injection protection: параметризованные запросы pgx (`$1…$n`), без ORM

## 8. API Endpoints (высокоуровнево)

### Auth
- `POST /api/auth/register` — регистрация
- `POST /api/auth/login` — вход
- `POST /api/auth/refresh` — обновление токена
- `POST /api/auth/logout` — выход

### Candidates
- `GET /api/candidates/profile` — профиль кандидата
- `PUT /api/candidates/profile` — обновление профиля
- `POST /api/candidates/profile/resume` — загрузка PDF резюме
- `GET /api/candidates/category` — текущая категория

### Employers
- `GET /api/employers/profile` — профиль компании
- `PUT /api/employers/profile` — обновление профиля
- `POST /api/employers/need` — описание потребности
- `GET /api/employers/candidates` — подборка кандидатов
- `GET /api/employers/candidates/search` — поиск по базе

### Testing
- `POST /api/testing/start` — начать тестирование
- `GET /api/testing/session/:id/current` — текущее задание (без утечки ответов)
- `POST /api/testing/session/:id/answer` — отправить ответ
- `POST /api/testing/session/:id/complete` — завершить тест
- `POST /api/testing/retake` — пересдать (с ограничением)
- V2: `/api/testing/v2/start|session/:id/current|answer|complete`

### Invitations
- `POST /api/invitations` — отправить приглашение
- `GET /api/invitations` — входящие приглашения
- `POST /api/invitations/:id/accept` — принять
- `POST /api/invitations/:id/reject` — отклонить
- `POST /api/invitations/:id/withdraw` — отозвать

### Vacancies (optional)
- `POST /api/vacancies` — создать вакансию
- `GET /api/vacancies` — список вакансий

### Matching
- `POST /api/matching/search` — semantic search
- `GET /api/matching/recommendations` — рекомендации

## 9. Развертывание

### Docker Compose (сокращённо; полный файл — docker-compose.yml)
```yaml
services:
  postgres:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_DB: fsp_platform
      POSTGRES_USER: fsp
      POSTGRES_PASSWORD: ${DB_PASSWORD:-changeme}
    ports:
      - "5433:5432"               # 5433 на хосте (5432 может быть занят)

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  minio:
    image: bitnamilegacy/minio:latest
    ports:
      - "9000:9000"

  ml-service:                      # Python/FastAPI, semantic matching V2
    build: ./ml-service
    ports:
      - "8000:8000"

  backend:                         # Go-образ (multi-stage Dockerfile)
    build:
      context: ./backend
      target: prod
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
      minio:    { condition: service_healthy }
      ml-service: { condition: service_healthy }
    ports:
      - "3000:3000"
    environment:
      DATABASE_URL: postgresql://fsp:${DB_PASSWORD:-changeme}@postgres:5432/fsp_platform
      REDIS_URL: redis://redis:6379
      JWT_SECRET: ${JWT_SECRET}
      ML_SERVICE_URL: http://ml-service:8000
    healthcheck:
      test: ["CMD-SHELL", "curl -sf http://localhost:3000/api/health || exit 1"]

  frontend:
    build: ./frontend
    depends_on:
      backend: { condition: service_healthy }
    ports:
      - "8080:3000"

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - ./docker/nginx/nginx.conf:/etc/nginx/nginx.conf
    depends_on:
      - frontend
      - backend

volumes:
  postgres_data:
```