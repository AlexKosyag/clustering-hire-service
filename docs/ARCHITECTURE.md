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
│                      API GATEWAY                            │
│              (Nginx / Traefik / Express)                    │
│         Rate Limiting, Auth, Routing, CORS                  │
└───────────────────────────┬─────────────────────────────────┘
                            │
            ┌───────────────┼───────────────┐
            ▼               ▼               ▼
┌────────────────┐ ┌────────────────┐ ┌────────────────┐
│   AUTH MODULE  │ │  CORE API      │ │ TESTING ENGINE │
│   (Keycloak    │ │  (NestJS)      │ │  (NestJS)      │
│    compatible) │ │                │ │                │
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
│                 MATCHING ENGINE                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Semantic Matching (sentence-transformers)            │  │
│  │  Collaborative Filtering                             │  │
│  │  Category Ranking                                    │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
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

### Backend
- **Runtime**: Node.js 20 LTS
- **Framework**: NestJS 10 (модульная архитектура, DI, guards)
- **ORM**: Prisma 5 (type-safe, миграции)
- **Auth**: Passport.js + JWT (совместимость с Keycloak через openid-client)
- **Validation**: class-validator + class-transformer
- **Documentation**: @nestjs/swagger (OpenAPI 3.0)

### Frontend
- **Framework**: Next.js 14 (App Router, SSR)
- **UI Library**: Ant Design 5 (enterprise-grade, русская локализация)
- **State Management**: Zustand (лёгкий, типизированный)
- **HTTP Client**: Axios + React Query (кеширование, retry)
- **Forms**: React Hook Form + Zod (валидация)

### Database
- **Primary**: PostgreSQL 16 (JSONB для гибких полей, полнотекстовый поиск)
- **Cache**: Redis 7 (сессии, rate limiting, кэш подборок)
- **Storage**: MinIO (S3-совместимый, для PDF и файлов)

### Infrastructure
- **Containers**: Docker + docker-compose
- **Reverse Proxy**: Nginx (rate limiting, SSL termination)
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
├── backend/                       # Backend API (NestJS)
│   ├── src/
│   │   ├── modules/
│   │   │   ├── auth/              # Аутентификация
│   │   │   ├── users/             # Пользователи (кандидаты + работодатели)
│   │   │   ├── candidates/        # Профили кандидатов
│   │   │   ├── employers/         # Профили работодателей
│   │   │   ├── vacancies/         # Вакансии
│   │   │   ├── invitations/       # Приглашения
│   │   │   ├── testing/           # Тестирование
│   │   │   ├── matching/          # Подбор кандидатов
│   │   │   ├── fsp/               # Интеграция с ФСП
│   │   │   └── notifications/     # Уведомления
│   │   ├── common/                # Shared: guards, decorators, pipes
│   │   ├── config/                # Конфигурация
│   │   └── main.ts
│   ├── prisma/
│   │   ├── schema.prisma          # Схема БД
│   │   ├── migrations/            # Миграции
│   │   └── seed.ts                # Сидирование данных
│   ├── test/                      # E2E тесты
│   ├── Dockerfile
│   ├── package.json
│   └── tsconfig.json
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

Candidate → Frontend → POST /api/testing/submit
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
- JWT access token (15 min) + refresh token (7 days)
- Хранение refresh token в HttpOnly cookie
- Совместимость с Keycloak через OIDC provider

### 7.2. Авторизация
- Role-based access control (RBAC): CANDIDATE, EMPLOYER, ADMIN
- Guard-based на уровне контроллеров NestJS
- Кандидат видит только свои данные
- Работодатель не видит контакты кандидата до ACCEPTED

### 7.3. Защита данных
- bcrypt для хеширования паролей
- Rate limiting на API (100 req/min)
- CORS whitelist
- Input validation через class-validator
- SQL injection protection через Prisma ORM

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
- `GET /api/testing/session/:id` — получить задания
- `POST /api/testing/session/:id/answer` — отправить ответ
- `POST /api/testing/session/:id/complete` — завершить тест
- `POST /api/testing/retake` — пересдать (с ограничением)

### Invitations
- `POST /api/invitations` — отправить приглашение
- `GET /api/invitations` — входящие приглашения
- `PUT /api/invitations/:id/accept` — принять
- `PUT /api/invitations/:id/reject` — отклонить

### Vacancies (optional)
- `POST /api/vacancies` — создать вакансию
- `GET /api/vacancies` — список вакансий
- `POST /api/vacancies/:id/apply` — откликнуться

### Matching
- `POST /api/matching/search` — semantic search
- `GET /api/matching/recommendations` — рекомендации

## 9. Развертывание

### Docker Compose
```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: fsp_platform
      POSTGRES_USER: fsp
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"

  backend:
    build: ./backend
    depends_on:
      - postgres
      - redis
      - minio
    ports:
      - "3000:3000"
    environment:
      DATABASE_URL: postgresql://fsp:${DB_PASSWORD}@postgres:5432/fsp_platform
      REDIS_URL: redis://redis:6379
      JWT_SECRET: ${JWT_SECRET}

  frontend:
    build: ./frontend
    depends_on:
      - backend
    ports:
      - "8080:3000"

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./docker/nginx/nginx.conf:/etc/nginx/nginx.conf
    depends_on:
      - frontend
      - backend

volumes:
  postgres_data:
```