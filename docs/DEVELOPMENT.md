# План разработки: Параллелизация и этапы

## 1. Обзор

Документ описывает план параллельной разработки, зависимости между модулями и распределение задач между агентами/разработчиками.

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
| Создать Prisma schema | 1ч | backend/prisma/schema.prisma |
| Создать seed-данные | 30м | backend/prisma/seed.ts |
| Настроить Docker Compose | 30м | docker-compose.yml |

**Валидация**: `npx prisma validate` + `npx prisma migrate dev`

---

### Этап 1: Ядро (6 часов, параллельно)
**Параллельно**: ДА (3 агента одновременно)
**Агенты**: auth-agent, backend-agent, testing-agent

#### 1.1. Auth Module (2 часа)
| Задача | Время | Выход |
|--------|-------|-------|
| Auth service | 30м | auth.service.ts |
| JWT strategy + guards | 30м | jwt.strategy.ts, guards/ |
| DTO + валидация | 15м | dto/ |
| Контроллер | 15м | auth.controller.ts |
| Unit-тесты | 30м | auth.service.spec.ts |

#### 1.2. Backend API (2 часа)
| Задача | Время | Выход |
|--------|-------|-------|
| Модули (users, candidates, employers) | 1ч | modules/ |
| Invitations module | 30м | invitations/ |
| DTO + guards | 15м | dto/, guards/ |
| Swagger настройка | 15м | main.ts |

#### 1.3. Testing Engine (2 часа)
| Задача | Время | Выход |
|--------|-------|-------|
| Question pool service | 30м | question-pool.service.ts |
| CAT algorithm | 45м | cat.service.ts |
| Scoring service | 30м | scoring.service.ts |
| Anti-cheat | 15м | anti-cheat.service.ts |
| Seed вопросы | 30м | seed-data/ |

**Валидация**: Все unit-тесты проходят, API отвечает

---

### Этап 2: Интеллект (4 часа, параллельно)
**Параллельно**: ДА (2 агента)
**Агенты**: matching-agent, fsp-agent

#### 2.1. Matching Engine (3 часа)
| Задача | Время | Выход |
|--------|-------|-------|
| Embedding service | 45м | embedding.service.ts |
| Scoring service | 45м | scoring.service.ts |
| Explanation service | 30м | explanation.service.ts |
| Matching service | 30м | matching.service.ts |
| Кэширование Redis | 15м | cache integration |
| Unit-тесты | 15м | *.spec.ts |

#### 2.2. FSP Integration (1 час)
| Задача | Время | Выход |
|--------|-------|-------|
| FSP service | 30м | fsp.service.ts |
| Mock данные | 15м | mock-data.ts |
| Контроллер | 15м | fsp.controller.ts |

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
| E2E тесты | 1ч | test/ |
| Docker настройка | 30м | Dockerfile, docker-compose.yml |
| Nginx конфиг | 30м | docker/nginx/nginx.conf |
| Seed данные | 30м | seed scripts |
| Документация | 1ч | docs/ |
| Демо сценарии | 30м | demo scripts |

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
| Prisma schema не валидируется | Среднее | Высокое | Проверять на каждом этапе |
| CAT алгоритм сложен | Высокое | Среднее | Упрощённая версия для MVP |
| Embedding генерация медленная | Среднее | Среднее | Кэширование, async |
| Frontend интеграция ломается | Среднее | Высокое | Mock API для разработки |
| Docker не запускается | Низкое | Высокое | Тестировать на каждом этапе |

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