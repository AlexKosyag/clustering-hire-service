# 01. Архитектура бэкенда

Содержание: [README](README.md) · **01** · [02](02-data-model.md) · [03](03-auth-and-security.md) · [04](04-api-reference.md) · [05](05-matching-engine.md) · [06](06-testing-engine.md) · [07](07-fsp-gamification-notifications.md) · [08](08-infrastructure.md) · [09](09-known-issues.md)

---

## 1. Роль бэкенда в системе

Бэкенд — монолитный NestJS-сервис, выполняющий роль API-слоя платформы-агрегатора ИТ-вакансий ФСП. Два типа пользователей:

- **Кандидат** — заполняет профиль, загружает резюме, проходит адаптивное тестирование (подтверждение грейда), подключает достижения ФСП, получает приглашения от работодателей.
- **Работодатель** — описывает потребность (текстом и/или фильтрами), получает ранжированный список кандидатов с объяснением совпадения, отправляет приглашения, публикует вакансии.

Принцип **employer-first**: поиск идёт только в направлении «потребность работодателя → кандидаты». Обратного matching («вакансии для кандидата») нет; вакансии — обычный CRUD-каталог, участвующий только как необязательная привязка к приглашению.

```
┌─────────────┐   /api/*    ┌──────────────────────────────────────────┐
│  Frontend   │ ──────────► │  NestJS backend (:3000)                  │
│  (Next.js)  │  (или nginx)│  ┌──────────┐ ┌────────────┐ ┌─────────┐ │
└─────────────┘             │  │ matching │ │  testing   │ │   fsp   │ │
                            │  └────┬─────┘ └─────┬──────┘ └────┬────┘ │
                            └───────┼─────────────┼─────────────┼──────┘
                                    ▼             ▼             ▼
                            PostgreSQL 16 + pgvector · Redis 7 · MinIO
                                    │
                                    ▼ HTTP (опционально)
                            ml-service (FastAPI: embed / parse-query / rerank / ner)
```

## 2. Технологический стек

| Слой | Технологии |
|------|-----------|
| Фреймворк | NestJS 10 (`@nestjs/platform-express`), RxJS 7 |
| Доступ к БД | Prisma 5 (`@prisma/client`), raw SQL для pgvector |
| БД | PostgreSQL 16 + расширение `pgvector` (колонки `vector(384)`) |
| Кэш | Redis 7 (клиент `ioredis`) |
| Файлы | MinIO (S3-совместимое хранилище) |
| Аутентификация | `@nestjs/jwt`, `passport-jwt`, `bcryptjs` (cost 12) |
| Валидация | `class-validator` + `class-transformer` (глобальный `ValidationPipe`) |
| ML локально | `@xenova/transformers` (ONNX, optionalDependency) |
| ML внешний | `ml-service` (FastAPI, Python: sentence-transformers, cross-encoder) |
| API-документация | `@nestjs/swagger` (OpenAPI 3.0) |
| PDF | `pdf-parse` (парсинг резюме) |
| Тесты | Jest, supertest |

## 3. Bootstrap приложения (`src/main.ts`)

Порядок инициализации:

1. `NestFactory.create(AppModule)` — Express-платформа.
2. `app.setGlobalPrefix('api')` — **все маршруты начинаются с `/api`**.
3. **CORS**: `origin: [FRONTEND_URL || 'http://localhost:8080', 'http://localhost:8080', 'http://localhost:3000', 'http://127.0.0.1:8080']`, `credentials: true` (нужно для refresh-cookie).
4. **Глобальный `ValidationPipe`**: `whitelist: true` (лишние поля DTO отбрасываются), `transform: true`, `enableImplicitConversion: true` (query-параметры неявно приводятся к типам).
5. **Глобальный `HttpExceptionFilter`** — единый формат ошибок (см. §6).
6. **Swagger** на `/api/docs` (JSON: `/api/docs-json`), схема авторизации `bearer` (JWT).
7. `app.listen(PORT || 3000)`.

## 4. Корневой модуль и провайдеры (`src/app.module.ts`)

`AppModule` импортирует:

| Модуль | Тип | Назначение |
|--------|-----|-----------|
| `ConfigModule` (isGlobal, `load: [configuration]`) | инфраструктура | типизированная конфигурация |
| `CommonModule` | инфраструктура | глобальные guard-ы и interceptor |
| `PrismaModule` (@Global) | инфраструктура | `PrismaService` — доступ к БД |
| `StorageModule` | инфраструктура | MinIO-загрузка файлов |
| `RedisModule` (@Global, в CommonModule) | инфраструктура | кэш matching-результатов |
| `AuthModule` | домен | аутентификация и авторизация |
| `CandidatesModule` | домен | профиль кандидата, резюме, аватар |
| `EmployersModule` | домен | профиль работодателя, поиск, дашборд |
| `InvitationsModule` | домен | приглашения (статусная машина) |
| `VacanciesModule` | домен | вакансии |
| `TestingModule` | домен | тестирование кандидатов (V1 + V2) |
| `FspModule` | домен | интеграция с ФСП |
| `MatchingModule` | домен | подбор кандидатов (V1 + V2) |
| `NotificationsModule` | домен | лента уведомлений |
| `HealthModule` | домен | `GET /api/health` |
| `GamificationModule` | домен | XP, уровни, бейджи |

### Глобальные провайдеры (`src/common/common.module.ts`)

```ts
{ provide: APP_GUARD,      useClass: JwtAuthGuard },       // JWT по умолчанию, отключается @Public()
{ provide: APP_GUARD,      useClass: RolesGuard },         // проверка @Roles(...)
{ provide: APP_INTERCEPTOR, useClass: LoggingInterceptor } // лог "METHOD /url status Xms"
```

**Следствие:** маршрут, не помеченный `@Public()`, требует валидный access-токен; роль проверяется только там, где задан `@Roles(...)`.

## 5. Слои и соглашения о коде

Типичный модуль содержит `*.controller.ts`, `*.service.ts`, `*.module.ts`, `dto/`. Контроллеры тонкие (приём DTO → вызов сервиса), вся бизнес-логика — в сервисах. Доступ к БД — через инжектируемый `PrismaService` (`src/prisma/prisma.module.ts`, `@Global`, `PrismaService extends PrismaClient` с `$connect`/`$disconnect`).

Декораторы (`src/common/decorators/auth.decorator.ts`):

| Декоратор | Назначение |
|-----------|-----------|
| `@Public()` | отключает глобальную JWT-проверку (метаданные `isPublic`) |
| `@Roles(...roles: UserRole[])` | ограничение по роли (`RolesGuard`) |
| `@CurrentUser(data?)` | `request.user` (`AuthUser {id, email, role}`) целиком или одно поле |

## 6. Формат ошибок (`src/common/filters/http-exception.filter.ts`)

Глобальный `@Catch()`-фильтр нормализует любые исключения:

```json
{
  "statusCode": 409,
  "timestamp": "2026-10-10T12:00:00.000Z",
  "path": "/api/invitations",
  "message": "Кандидат уже приглашён по этому приглашению"
}
```

- `message` — строка; для ошибок валидации `class-validator` (массив) — сообщения, склеенные через `', '`.
- Не-`HttpException` ошибки логируются (`Logger('HttpException')`) и отдаются как `500`.
- Стандартные коды: `400 BadRequest`, `401 Unauthorized` (`Требуется авторизация`), `403 Forbidden` (`Недостаточно прав для этого действия`), `404`, `409 Conflict`, `503 ServiceUnavailable` (файловое хранилище).

## 7. Конфигурация (`src/config/configuration.ts`)

`ConfigModule.forRoot({ isGlobal: true, load: [configuration] })` — доступ через `ConfigService`:

| Ключ | Источник | Значение по умолчанию | Назначение |
|------|----------|----------------------|-----------|
| `port` | `PORT` | 3000 | порт HTTP |
| `databaseUrl` | `DATABASE_URL` | — | строка подключения к PostgreSQL |
| `redisUrl` | `REDIS_URL` | `redis://redis:6379` | Redis |
| `jwt.secret` | `JWT_SECRET` | `dev-secret` | подпись access-токенов |
| `jwt.refreshSecret` | `JWT_REFRESH_SECRET` | `dev-refresh-secret` | подпись refresh-токенов |
| `jwt.accessTokenTtl` | — | `15m` | время жизни access-токена |
| `jwt.refreshTokenTtlDays` | — | 7 | время жизни refresh-токена |
| `minio.*` | `MINIO_ENDPOINT/ACCESS_KEY/SECRET_KEY/BUCKET` | `http://minio:9000`, `minioadmin`, `fsp-platform` | файловое хранилище |
| `embedding.provider` | `EMBEDDING_PROVIDER` | `auto` | `auto \| tfidf \| transformers` |
| `matching.cacheTtlSeconds` | — | 900 | TTL кэша результатов поиска |
| `testing.questionsPerTest` | — | 20 | число вопросов в тесте V1 |
| `testing.cooldownDays` | — | 90 | кулдаун пересдачи теста |

### Переменные окружения, читаемые напрямую из `process.env`

| Переменная | Где используется | Назначение |
|-----------|-----------------|-----------|
| `FRONTEND_URL` | `main.ts`, docker-compose | CORS-origin фронтенда |
| `MINIO_PUBLIC_ENDPOINT` | `storage.service.ts` | базовый URL публичных ссылок на файлы |
| `ML_SERVICE_URL` / `ML_SERVICE_TIMEOUT` | `ml-client.service.ts` | адрес ML-сервиса (`http://ml-service:8000`) и таймаут (5000 мс) |
| `TESTING_TASKS_PER_TEST` / `TESTING_COOLDOWN_DAYS` | `testing-v2.service.ts` | 15 заданий V2 / 90 дней кулдауна (⚠️ дублируют конфиг V1) |
| `OPENAI_API_KEY` / `OPENAI_API_URL` / `OPENAI_MODEL` | `llm-evaluation.service.ts` | LLM-оценка развёрнутых ответов (fallback — keyword-эвристика) |
| `SANDBOX_TIMEOUT_MS` / `SANDBOX_MEMORY_MB` / `SANDBOX_DOCKER` | `code-sandbox.service.ts` | песочница для кода (10000 мс / 256 МБ; Docker-режим не реализован) |
| `SMTP_*` | только `.env.example` | e-mail-канал (заглушка, в коде не используется) |
| `API_URL` / `FRONTEND_URL` | `scripts/metrics.ts` | адреса для автовалидации |

Полный список переменных окружения и их поведение в Docker — в [08-infrastructure.md](08-infrastructure.md).

## 8. Инфраструктурные сервисы

### PrismaService (`src/prisma/prisma.module.ts`)
`@Global`-модуль, `PrismaService extends PrismaClient` (`$connect()` при инициализации, `$disconnect()` при уничтожении). Инжектируется во все сервисы. ⚠️ Колонки `vector(384)` невидимы для Prisma (`Unsupported(...)`) — работа с векторами идёт raw SQL через `VectorRepository`.

### RedisService (`src/common/redis/redis.service.ts`)
`@Global`. **Полная деградация при недоступности**: операции молча пропускаются (`get` → `null`), на ошибке выставляется circuit-breaker `blockedUntil = now + 30s`. Методы: `get`, `set(key, value, ttlSeconds?)` (`SET ... EX`), `delByPattern(pattern)` (SCAN + DEL), `isAvailable`.
Назначение — **только кэширование результатов matching** (ключи `matching:cache:*`). Локов, rate-limit, сессий в Redis нет.

### StorageService (`src/modules/storage/storage.service.ts`)
MinIO-клиент; ленивое создание бакета (`ensureBucket`); единственный метод `upload(kind, owner, filename, buffer, contentType)` → `putObject` → публичный URL `${MINIO_PUBLIC_ENDPOINT}/${bucket}/${kind}/${owner}/${filename}`. При сбое — `503 ServiceUnavailableException`. Подробнее — [07-fsp-gamification-notifications.md](07-fsp-gamification-notifications.md) (раздел «Файловое хранилище»).

### MlServiceClient (`src/modules/matching/ml-client.service.ts`)
HTTP-клиент к `ml-service` (FastAPI): `/health`, `/embed`, `/embed/batch`, `/parse-query`, `/rerank`, `/ner`. Каждый метод имеет локальный fallback (TF-IDF / словари), поэтому недоступность ML **не роняет запросы**. Подробнее — [05-matching-engine.md](05-matching-engine.md).

## 9. Модульный обзор (что за что отвечает)

| Модуль | Ответственность | Ключевые сущности |
|--------|----------------|-------------------|
| `auth` | регистрация, вход, refresh-ротация, logout, текущий пользователь | `User` |
| `candidates` | профиль кандидата, парсинг PDF-резюме, аватар, статус тестирования | `CandidateProfile`, `TestSession` |
| `employers` | профиль компании, создание потребности, поиск, карточка кандидата, дашборд | `EmployerProfile`, `EmployerNeed`, `Invitation`, `Vacancy` |
| `vacancies` | каталог вакансий (публичное чтение + CRUD владельца) | `Vacancy` |
| `invitations` | статусная машина приглашений, раскрытие контактов | `Invitation` |
| `matching` | поиск/ранжирование кандидатов, эмбеддинги, индексация, объяснения | `EmployerNeed`, `CandidateProfile` (векторы) |
| `testing` | адаптивное тестирование (CAT/IRT), оценка ответов, анти-чит | `TestCategory`, `TestQuestion`, `TestSession`, `TestAnswer` |
| `fsp` | подключение аккаунта ФСП, достижения | `FSPAchievement`, `CandidateAchievement` |
| `gamification` | XP, уровни, бейджи, стрики, лидерборд | `GamificationProfile`, `Badge`, `XpEvent` |
| `notifications` | производная лента событий (без таблицы) | `Invitation`, `TestSession` |
| `storage` | загрузка файлов в MinIO | — |
| `health` | диагностика зависимостей | — |

## 10. Сквозные потоки данных (сайд-эффекты)

Важно понимать, какие действия инвалидируют кэш и переиндексируют векторы:

| Событие | Сайд-эффекты |
|---------|-------------|
| Обновление профиля кандидата (`PUT /api/candidates/profile`) | `matching.invalidateCache()` → `indexing.indexCandidate(id)` |
| Загрузка резюме | MinIO → парсинг → автозаполнение → `indexing.indexCandidate(id)` |
| Создание/обработка приглашения | `matching.invalidateCache()` |
| Обновление профиля работодателя | `matching.invalidateCache()` |
| Завершение теста | `matching.invalidateCache()` + `indexing.indexCandidate(id)` + геймификация (только V2) |
| Подключение/отключение ФСП | `matching.invalidateCache()` + `matching.reindexCandidate(id)` |
| Старт приложения | фоновая полная переиндексация (через 2 с), после загрузки ONNX-модели — повторная |

## 11. Тестирование и качество кода

- **Unit-тесты** (`*.spec.ts` рядом с кодом): `cat.service.spec.ts`, `scoring.spec.ts`, `anti-cheat.spec.ts`, `behavioral-anti-cheat.spec.ts`, `testing-v2.service.spec.ts`, `scoring-v2.service.spec.ts`, `explanation.service.spec.ts`, `embedding/tfidf.spec.ts`.
- **E2E** (`test/app.e2e.spec.ts`, `npm run test:e2e`, реальная БД, `--runInBand`): auth, RBAC, matching (precision, отсутствие NaN), тестирование (утечки `correctAnswer` нет), privacy контактов, vacancies, health, Swagger.
- **Автовалидация** (`npm run validate`, `scripts/metrics.ts`): smoke, security, метрики качества matching (precision@5 ≥ 70%, MRR, nDCG@10), дискриминативность тестов, производительность (p95 < 500 мс). Подробнее — [08-infrastructure.md](08-infrastructure.md).
