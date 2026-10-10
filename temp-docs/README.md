# temp-docs — Документация бэкенда платформы ФСП

Полное техническое описание бэкенда (`backend/`) — цифровой платформы-агрегатора ИТ-вакансий ФСП.
Документация отражает **фактическое состояние кода** (включая известные проблемы и заглушки),
а не только заявленный дизайн.

## Состав документации

| Файл | Содержание |
|------|-----------|
| [01-architecture.md](01-architecture.md) | Стек, схема сервисов, bootstrap приложения, DI-слои, конфигурация и переменные окружения |
| [02-data-model.md](02-data-model.md) | Схема БД (Prisma): 20 моделей, enum, связи, индексы, pgvector, миграции, seed |
| [03-auth-and-security.md](03-auth-and-security.md) | Аутентификация, JWT/refresh-токены, RBAC, формат ошибок, приватность контактов |
| [04-api-reference.md](04-api-reference.md) | Каталог всех HTTP-эндпоинтов `/api/*` с ролями, DTO, ответами и ошибками |
| [05-matching-engine.md](05-matching-engine.md) | Механика подбора кандидатов: пайплайн V1/V2, скоринг, эмбеддинги, pgvector, ML, кэш |
| [06-testing-engine.md](06-testing-engine.md) | Механика тестирования: CAT/IRT-3PL, скоринг, генератор заданий, анти-чит |
| [07-fsp-gamification-notifications.md](07-fsp-gamification-notifications.md) | Интеграция с ФСП, геймификация (XP/бейджи/стрики), уведомления |
| [08-infrastructure.md](08-infrastructure.md) | Docker-стек, миграции и seed, тесты, валидация и целевые метрики |
| [09-known-issues.md](09-known-issues.md) | Известные проблемы, рассинхроны и мёртвый код с путями к исходникам |

## Быстрая карта бэкенда

```
backend/
├── src/
│   ├── main.ts                  # bootstrap: префикс /api, CORS, ValidationPipe, Swagger
│   ├── app.module.ts            # корневой модуль (12 feature-модулей + infra)
│   ├── config/configuration.ts  # типизированная конфигурация (ConfigModule)
│   ├── common/                  # guards, decorators, filters, interceptor, Redis, cookie util
│   ├── prisma/                  # PrismaService (глобальный модуль)
│   └── modules/
│       ├── auth/                # аутентификация, JWT, refresh-ротация
│       ├── candidates/          # профиль кандидата, резюме (PDF), аватар
│       ├── employers/           # профиль работодателя, поиск кандидатов, дашборд
│       ├── vacancies/           # CRUD вакансий
│       ├── invitations/         # приглашения (статусная машина) — ядро найма
│       ├── matching/            # matching engine V1/V2, эмбеддинги, pgvector, ML-клиент
│       ├── testing/             # тестирование кандидатов: CAT/IRT, анти-чит, sandbox, LLM
│       ├── fsp/                 # интеграция с ФСП (достижения)
│       ├── gamification/        # XP, уровни, бейджи, стрики
│       ├── notifications/       # лента уведомлений (read-model)
│       ├── storage/             # MinIO (резюме, аватары, логотипы)
│       └── health/              # GET /api/health
├── prisma/
│   ├── schema.prisma            # схема данных
│   ├── migrations/              # 0_init, remove_applications, add_gamification
│   ├── seed.ts + seed/          # детерминированный сид демо-данных
│   └── embeddings.ts            # предзаполнение векторов (npm run seed:embeddings)
├── scripts/metrics.ts           # автоматическая валидация (npm run validate)
└── test/app.e2e.spec.ts         # e2e-тесты на реальной БД
```

## Ключевые факты

- **Язык/фреймворк:** TypeScript, NestJS 10 (Express), Prisma 5.
- **БД:** PostgreSQL 16 + pgvector (векторы 384-мерные); **кэш:** Redis 7; **файлы:** MinIO.
- **Все маршруты** живут под префиксом **`/api`**; Swagger — `/api/docs`, OpenAPI JSON — `/api/docs-json`.
- **Безопасность по умолчанию:** каждый маршрут требует JWT, если не помечен `@Public()`; роль проверяется `@Roles(...)`.
- **Ключевой сценарий сервиса:** работодатель описывает потребность → matching engine ранжирует кандидатов с объяснением → приглашение → кандидат принимает → контакты раскрываются. Параллельно кандидат проходит адаптивное тестирование (CAT + IRT), подтверждающее грейд.
- **Порт:** 3000 (в Docker — `backend:3000`); health-check: `GET /api/health`.

## Запуск и проверка (кратко)

```bash
# Весь стек
cp .env.example .env
docker compose up -d --build

# Демо-данные (858 вопросов, 204 кандидата, 30 вакансий)
docker compose exec backend npm run db:seed

# Локальная разработка бэкенда
cd backend && npm install && npx prisma migrate deploy && npm run start:dev

# Проверки
cd backend && npm test          # unit-тесты
npm run test:e2e                # e2e (реальная БД)
npm run validate                # метрики → docs/VALIDATION_REPORT.md
```

### Демо-аккаунты

| Роль | Email | Пароль |
|------|-------|--------|
| Кандидат | `test@candidate.com`, `demo2@candidate.com`, `demo3@candidate.com` | `Test123!` |
| Работодатель | `test@employer.com`, `hr@company.com`, `hr3@example.com` | `Test123!` |

## Условные обозначения

- Пути вида `src/modules/matching/scoring-v2.service.ts` — относительно `backend/`.
- Все HTTP-пути в документации указаны **с префиксом `/api`**.
- Разделы «Известная проблема» помечены ⚠️ и сведены в [09-known-issues.md](09-known-issues.md).
