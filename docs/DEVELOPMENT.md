# План разработки: Параллелизация и этапы

## 1. Обзор

Документ описывает план параллельной разработки, зависимости между модулями и распределение задач между агентами/разработчиками.

> **Статус**: все этапы выполнены. Бэкенд работает на **Go** (точный порт NestJS-версии,
> `backend/`); фронтенд — Next.js/TypeScript. Пути в таблицах — актуальные Go-файлы.

## 2. Карта зависимостей

```
ЭТАП 0 (база):
  └── database ← Фундамент, 0 зависимостей

ЭТАП 1 (ядро, параллельно):
  ├── auth ← зависит от database
  ├── backend-api ← зависит от database + auth
  └── testing-engine ← зависит от database + auth

ЭТАП 2 (интеллект, параллельно):
  ├── matching-engine ← зависит от backend-api + testing-engine
  └── fsp-integration ← зависит от database + auth

ЭТАП 3 (интерфейс, параллельно):
  ├── frontend-candidate ← зависит от всех API
  └── frontend-employer ← зависит от всех API

ЭТАП 4 (интеграция):
  └── integration + testing + documentation
```

## 3. Детальный план

### Этап 0: База данных (2 часа)
**Параллельно**: НЕТ (базовый модуль)
**Агент**: database-agent

| Задача | Время | Выход |
|--------|-------|-------|
| SQL-схема БД | 1ч | backend/migrations/000001_init.up.sql (+ 000002, 000003) |
| Seed-данные (JSON-дамп) | 30м | backend/internal/seed/data/*.json, backend/cmd/seed |
| Настроить Docker Compose | 30м | docker-compose.yml |

**Валидация**: `go build ./...` + `go run ./cmd/server` (миграции применяются автоматически, отметки переносятся из `_prisma_migrations`)

---

### Этап 1: Ядро (6 часов, параллельно)
**Параллельно**: ДА (3 агента одновременно)
**Агенты**: auth-agent, backend-agent, testing-agent

#### 1.1. Auth Module (2 часа)
| Задача | Время | Выход |
|--------|-------|-------|
| Auth service | 30м | internal/modules/auth/service.go |
| JWT + guard | 30м | internal/common/jwt.go, NewAuthGuard в auth/service.go |
| DTO + валидация | 15м | internal/modules/auth/dto.go |
| Хендлер | 15м | internal/modules/auth/handler.go |
| Покрытие | 30м | e2e-сценарии в backend/test/e2e/e2e_test.go |

#### 1.2. Backend API (2 часа)
| Задача | Время | Выход |
|--------|-------|-------|
| Модули (candidates, employers, vacancies) | 1ч | internal/modules/{candidates,employers,vacancies}/ |
| Invitations module | 30м | internal/modules/invitations/ |
| DTO + guard-маршруты | 15м | internal/modules/*/dto.go, internal/common/router.go (RouteOpts) |
| Swagger | 15м | cmd/server/main.go + swag init → backend/docs/ |

#### 1.3. Testing Engine (2 часа)
| Задача | Время | Выход |
|--------|-------|-------|
| Question pool service | 30м | internal/modules/testing/services/question-pool.go |
| CAT algorithm | 45м | internal/modules/testing/services/cat.go |
| Scoring service | 30м | internal/modules/testing/services/scoring.go |
| Anti-cheat | 15м | services/anti-cheat.go, behavioral-anti-cheat.go |
| Seed вопросы | 30м | internal/seed/data/questions.json (858 вопросов) |

**Валидация**: Все unit-тесты проходят, API отвечает

---

### Этап 2: Интеллект (4 часа, параллельно)
**Параллельно**: ДА (2 агента)
**Агенты**: matching-agent, fsp-agent

#### 2.1. Matching Engine (3 часа)
| Задача | Время | Выход |
|--------|-------|-------|
| Embedding service | 45м | internal/modules/matching/embedding.go, ml-client.go |
| Scoring service | 45м | matching/scoring.go, scoring-v2.go |
| Explanation service | 30м | matching/explanation.go |
| Matching service | 30м | matching/service.go, service-v2.go |
| Кэширование Redis | 15м | internal/cache/redis.go + Cache в matching/service.go |
| Unit-тесты | 15м | tfidf_test.go, scoring-v2_test.go, explanation_test.go |

#### 2.2. FSP Integration (1 час)
| Задача | Время | Выход |
|--------|-------|-------|
| FSP service | 30м | internal/modules/fsp/service.go |
| Mock-каталог достижений | 15м | internal/modules/fsp/dto.go, service.go |
| Хендлер | 15м | internal/modules/fsp/handler.go |

**Валидация**: Matching возвращает релевантных кандидатов, FSP достижения загружаются

---

### Этап 3: Интерфейс (6 часов, параллельно)
**Параллельно**: ДА (2 агента)
**Агенты**: frontend-candidate-agent, frontend-employer-agent

#### 3.1. Frontend Candidate (3 часа)
| Задача | Время | Выход |
|--------|-------|-------|
| Next.js setup + layout | 30м | app/, components/layout/ |
| Auth pages (login, register) | 30м | app/(auth)/ |
| Dashboard | 30м | app/candidate/page.tsx |
| Profile pages | 30м | app/candidate/profile/ |
| Testing pages | 45м | app/candidate/testing/ |
| Invitations page | 15м | app/candidate/invitations/ |
| Stores + API client | 30м | stores/, lib/api.ts |

#### 3.2. Frontend Employer (3 часа)
| Задача | Время | Выход |
|--------|-------|-------|
| Layout + dashboard | 30м | app/employer/ |
| Profile page | 30м | app/employer/profile/ |
| Search page | 45м | app/employer/search/ |
| Candidates page | 30м | app/employer/candidates/ |
| Invitations page | 30м | app/employer/invitations/ |
| Vacancies page | 15м | app/employer/vacancies/ |

**Валидация**: Все страницы рендерятся, навигация работает

---

### Этап 4: Интеграция (4 часа)
**Параллельно**: НЕТ
**Агент**: integration-agent

| Задача | Время | Выход |
|--------|-------|-------|
| E2E тесты | 1ч | backend/test/e2e/e2e_test.go (30 сценариев) |
| Docker настройка | 30м | backend/Dockerfile (multi-stage), docker-compose.yml |
| Nginx конфиг | 30м | docker/nginx/nginx.conf |
| Seed данные | 30м | cmd/seed, cmd/seed-embeddings |
| Документация | 1ч | docs/ |
| Демо-сценарии / валидация | 30м | docs/VALIDATION.md + cmd/metrics → docs/VALIDATION_REPORT.md |

---

## 4. Матрица параллелизации

```
Время →  0h   1h   2h   3h   4h   5h   6h   7h   8h   9h  10h  11h  12h  13h  14h  15h  16h  17h  18h
         │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │
DB       ████████
AUTH          ████████
BACKEND       ████████
TESTING       ████████
MATCHING           ████████████
FSP                ████
FE-CAND                 ████████████
FE-EMPL                 ████████████
INTEGRATION                       ████████
```

**Общее время**: ~18 часов (с параллелизацией)
**Без параллелизации**: ~30 часов

## 5. Распределение агентов

### Agent 1: Database + Backend
- Этап 0: database
- Этап 1: backend-api
- Этап 4: integration

### Agent 2: Auth + Testing
- Этап 1: auth
- Этап 1: testing-engine
- Этап 2: fsp-integration

### Agent 3: Matching + Frontend Candidate
- Этап 2: matching-engine
- Этап 3: frontend-candidate

### Agent 4: Frontend Employer + Optional
- Этап 3: frontend-employer
- Этап 4: optional features

## 6. Критический путь

```
database → backend-api → matching-engine → frontend → integration
```

**Минимальное время**: 14 часов (если всё параллелится идеально)
**Реалистичное время**: 18-20 часов

## 7. Риски и митигация

| Риск | Вероятность | Влияние | Митигация |
|------|-------------|---------|-----------|
| SQL-миграция не применяется | Среднее | Высокое | Раннер `store.DB.Migrate` при старте + бутстрап `_prisma_migrations`; e2e на реальной БД |
| CAT алгоритм сложен | Высокое | Среднее | Упрощённая версия для MVP |
| Embedding генерация медленная | Среднее | Среднее | Кэширование Redis, фоновая индексация батчами по 25 |
| Frontend интеграция ломается | Среднее | Высокое | Mock API для разработки |
| Docker не запускается | Низкое | Высокое | Тестировать на каждом этапе (`docker compose build backend`) |

## 8. Чеклист готовности к демонстрации

### Обязательный сценарий (показать в демо):
- [ ] Регистрация кандидата
- [ ] Заполнение профиля
- [ ] Прохождение тестирования
- [ ] Присвоение категории
- [ ] Регистрация работодателя
- [ ] Поиск кандидатов
- [ ] Отправка приглашения
- [ ] Принятие приглашения

### Опциональный сценарий:
- [ ] Отклик на вакансию
- [ ] Публикация вакансии
- [ ] Ежедневное задание
- [ ] Достижения ФСП
- [ ] Геймификация

## 9. Промпт для оркestrатора

```
Ты — Technical Lead, координирующий параллельную разработку платформы ФСП.

ТВОЯ ЗАДАЧА: Организовать параллельную разработку всех модулей.

ВХОДНЫЕ ДАННЫЕ:
- docs/DEVELOPMENT.md — план разработки (этот файл)
- docs/modules/*.md — ТЗ на каждый модуль
- docs/concepts/*.md — концепции

ПРИНЦИПЫ РАБОТЫ:
1. Сначала database, потом всё остальное параллельно
2. Каждый агент работает независимо в своей директории
3. Интеграция только после завершения всех модулей
4. Каждый модуль имеет свой чеклист валидации

ОРГАНИЗАЦИЯ:
1. Запусти database-agent (этап 0)
2. После завершения database — запусти 3 агента параллельно (этап 1)
3. После этапа 1 — запусти 2 агента параллельно (этап 2)
4. После этапа 2 — запусти 2 агента параллельно (этап 3)
5. Этап 4 — интеграция и тестирование

ВАЛИДАЦИЯ:
- После каждого этапа проверяй что модули работают
- После интеграции — запусти полный e2e тест
- Проверь что Docker запускается
```