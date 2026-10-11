# Процедуры валидации решения

## 1. Обзор

Документ описывает процедуры валидации решения на соответствие ТЗ и критериям оценки.

## 2. Критерии оценки (из ТЗ)

| Критерий | Вес | Что проверяем |
|----------|-----|---------------|
| 1. Новая механика подбора | 35% | Employer-first модель, semantic matching, объяснимость |
| 2. Механика тестирования | 25% | CAT, anti-cheat, стабильность грейда |
| 3. Удобство интерфейса | 10% | UX, навигация, читаемость |
| 4. Техническая проработка | 7% | Код, архитектура, производительность |
| 5. Полнота реализации | 6% | Обязательные функции, документация |
| 6. Дополнительный функционал | 7% | Отклики, вакансии, daily tasks, геймификация |
| 7. Презентация | 10% | Питч, демонстрация, ответы на вопросы |

## 3. Процедура валидации

### 3.1. Smoke Test (быстрая проверка)

**Цель**: Убедиться что система запускается и базовые функции работают.

```bash
# 1. Запуск через Docker
docker compose up -d

# 2. Проверка что все сервисы запустились
docker compose ps

# 3. Проверка API
curl http://localhost:3000/api/health

# 4. Проверка Frontend
curl http://localhost:8080

# 5. Проверка Swagger
curl http://localhost:3000/api/docs
curl http://localhost:3000/api/docs-json   # OpenAPI 3.0, > 20 путей
```

**Чеклист**:
- [ ] Docker-compose запускается без ошибок
- [ ] Все контейнеры в статусе "Up"
- [ ] API отвечает на /api/health
- [ ] Frontend загружается
- [ ] Swagger UI доступен

### 3.2. E2E Test (сквозной сценарий)

**Цель**: Проверить полный пользовательский сценарий.

#### Сценарий 1: Кандидат
```bash
# 1. Регистрация
curl -X POST http://localhost:3000/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@candidate.com","password":"Test123!","role":"CANDIDATE"}'

# 2. Заполнение профиля
curl -X PUT http://localhost:3000/api/candidates/profile \
  -H "Authorization: Bearer {token}" \
  -d '{"firstName":"Иван","lastName":"Иванов","specialization":"BACKEND","declaredGrade":"MIDDLE","stack":["Node.js","PostgreSQL"]}'

# 3. Начать тестирование
curl -X POST http://localhost:3000/api/testing/start \
  -H "Authorization: Bearer {token}" \
  -d '{"specialization":"BACKEND","declaredGrade":"MIDDLE"}'

# 4. Ответить на 20 вопросов
for i in {1..20}; do
  curl -X POST http://localhost:3000/api/testing/session/{sessionId}/answer \
    -H "Authorization: Bearer {token}" \
    -d "{\"questionId\":\"q$i\",\"answer\":\"b\"}"
done

# 5. Завершить тест
curl -X POST http://localhost:3000/api/testing/session/{sessionId}/complete \
  -H "Authorization: Bearer {token}"
```

#### Сценарий 2: Работодатель
```bash
# 1. Регистрация
curl -X POST http://localhost:3000/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@employer.com","password":"Test123!","role":"EMPLOYER"}'

# 2. Профиль компании
curl -X PUT http://localhost:3000/api/employers/profile \
  -H "Authorization: Bearer {token}" \
  -d '{"companyName":"ТестКомпания","industry":"IT"}'

# 3. Поиск кандидатов
curl -X POST http://localhost:3000/api/matching/search \
  -H "Authorization: Bearer {token}" \
  -d '{"specialization":"BACKEND","grade":"MIDDLE","stack":["Node.js"]}'

# 4. Отправка приглашения
curl -X POST http://localhost:3000/api/invitations \
  -H "Authorization: Bearer {token}" \
  -d '{"candidateId":"{candidateId}","message":"Приглашаем","salaryFrom":150000,"salaryTo":250000}'
```

**Чеклист**:
- [ ] Регистрация кандидата проходит
- [ ] Профиль сохраняется
- [ ] Тест стартует и завершается
- [ ] Грейд рассчитывается
- [ ] Регистрация работодателя проходит
- [ ] Поиск возвращает кандидатов
- [ ] Приглашение отправляется
- [ ] Приглашение принимается

### 3.3. Matching Validation (качество подбора)

**Цель**: Проверить что релевантные кандидаты в топе выдачи.

#### Тестовые данные
```json
{
  "testCandidates": [
    { "name": "Иван",    "spec": "BACKEND",   "grade": "MIDDLE",     "stack": ["Node.js", "PostgreSQL"] },
    { "name": "Анна",    "spec": "FRONTEND",  "grade": "JUNIOR",     "stack": ["React", "TypeScript"] },
    { "name": "Дмитрий", "spec": "BACKEND",   "grade": "SENIOR",     "stack": ["Python", "Docker", "PostgreSQL"] },
    { "name": "Мария",   "spec": "BACKEND",   "grade": "MIDDLE",     "stack": ["Node.js", "Redis", "Docker"] },
    { "name": "Алексей", "spec": "FULLSTACK", "grade": "MIDDLE_PLUS","stack": ["Node.js", "React", "PostgreSQL"] }
  ],
  "testNeed": {
    "specialization": "BACKEND",
    "grade": "MIDDLE",
    "stack": ["Node.js", "PostgreSQL"]
  }
}
```

#### Метрики
| Метрика | Что измеряем |
|---------|--------------|
| precision@5 | Релевантные в топ-5 |
| precision@10 | Релевантные в топ-10 |
| recall@10 | Найденные релевантные из всех |
| MRR | Mean Reciprocal Rank |
| nDCG | Normalized Discounted Cumulative Gain |

**Чеклист**:
- [ ] Релевантные кандидаты в топ-5 (>70%)
- [ ] Объяснение понятно и точно
- [ ] Фильтры работают корректно
- [ ] Пагинация работает

### 3.4. Testing Validation (качество тестирования)

**Цель**: Проверить стабильность и дискриминативность тестов.

#### Test-Retest Reliability
```text
# Один и тот же кандидат проходит тест дважды
# Результат должен быть стабильным (±1 грейд)

1. POST /api/testing/start { specialization: "BACKEND", declaredGrade: "MIDDLE" }
   → пройти 20 вопросов → POST .../complete → result1
2. После cooldown (POST /api/testing/retake) повторить то же самое → result2

Проверка: result1.gradeAfter == result2.gradeAfter   # тот же грейд
```

#### Discriminativeness
```text
# Слабый кандидат (JUNIOR) должен получить низкий score
# Сильный кандидат (SENIOR) должен получить высокий score

weakResult.score       < strongResult.score
weakResult.gradeAfter  < strongResult.gradeAfter
```

**Чеклист**:
- [ ] Тест даёт стабильный грейд при повторе
- [ ] Слабые кандидаты получают низкий score
- [ ] Сильные кандидаты получают высокий score
- [ ] Cooldown работает
- [ ] Вопросы перемешиваются

### 3.5. Security Validation

**Цель**: Проверить защиту от несанкционированного доступа.

```bash
# 1. Попытка доступа без токена
curl http://localhost:3000/api/candidates/profile
# Ожидание: 401 Unauthorized

# 2. Попытка доступа с невалидным токеном
curl http://localhost:3000/api/candidates/profile \
  -H "Authorization: Bearer invalid-token"
# Ожидание: 401 Unauthorized

# 3. Попытка доступа к чужим данным
curl http://localhost:3000/api/candidates/{otherUserId} \
  -H "Authorization: Bearer {myToken}"
# Ожидание: 403 Forbidden

# 4. Попытка SQL injection
curl -X POST http://localhost:3000/api/auth/login \
  -d '{"email":"admin@example.com\" OR 1=1--","password":"test"}'
# Ожидание: 401 Unauthorized (не 500)
```

**Чеклист**:
- [ ] Неавторизованный доступ блокируется (401)
- [ ] Невалидные токены отклоняются (401)
- [ ] Доступ к чужим данным запрещён (403)
- [ ] SQL injection не работает (401, не 500)

### 3.6. Performance Validation

**Цель**: Проверить производительность.

```bash
# Нагрузочное тестирование (Apache Bench)
# 100 запросов, 10 параллельных
ab -n 100 -c 10 http://localhost:3000/api/candidates/profile

# Ожидание:
# - Среднее время ответа < 200ms
# - 95-й перцентиль < 500ms
# - 0 ошибок
```

**Чеклист**:
- [ ] API отвечает < 200ms
- [ ] Поиск < 500ms
- [ ] Frontend загружается < 3s
- [ ] Нет утечек памяти

## 4. Отчёт о валидации

### Шаблон отчёта

```markdown
# Отчёт о валидации решения ФСП

## 1. Smoke Test
- [ ] Docker запускается
- [ ] API работает
- [ ] Frontend работает

## 2. E2E Test
- [ ] Регистрация кандидата
- [ ] Профиль кандидата
- [ ] Тестирование
- [ ] Регистрация работодателя
- [ ] Поиск кандидатов
- [ ] Приглашение

## 3. Matching Quality
- Precision@5: 80%
- Precision@10: 75%
- MRR: 0.85

## 4. Testing Quality
- Test-Retest: PASS (одинаковый грейд)
- Discriminativeness: PASS (разные scores)

## 5. Security
- [ ] Auth работает
- [ ] RBAC работает
- [ ] SQL injection защищён

## 6. Performance
- API response: 150ms (avg)
- Search: 300ms (avg)
- Frontend: 2.5s (load)

## 7. Дополнительный функционал
- [x] Отклик на вакансии
- [x] Публикация вакансий
- [ ] Daily tasks
- [ ] Геймификация
- [ ] PDF парсинг
```

## 5. Автоматизированная валидация

### 5.1. CI/CD Pipeline

```yaml
# .github/workflows/validate.yml
name: Validate Solution

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: Build
        run: go build ./...
        working-directory: backend

      - name: Vet + форматирование
        run: |
          go vet ./...
          test -z "$(gofmt -l .)"
        working-directory: backend

      - name: Unit tests
        run: go test ./internal/...
        working-directory: backend

      - name: Start dependencies (postgres, redis, ml-service)
        run: docker compose up -d postgres redis ml-service

      - name: Wait for services
        run: sleep 20

      - name: E2E tests
        run: go test ./test/e2e/ -count=1 -timeout 5m
        working-directory: backend
        env:
          DATABASE_URL: postgresql://fsp:changeme@localhost:5433/fsp_platform
          REDIS_URL: redis://localhost:6379
          ML_SERVICE_URL: http://localhost:8000

  docker:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Build Docker images
        run: docker compose build

      - name: Start services
        run: docker compose up -d

      - name: Wait for services
        run: sleep 30

      - name: Smoke test
        run: |
          curl -f http://localhost:3000/api/health
          curl -f http://localhost:8080

      - name: Stop services
        run: docker compose down
```

### 5.2. Test Scripts

```bash
#!/bin/bash
# scripts/validate.sh

echo "=== FSP Platform Validation ==="

# 1. Smoke test
echo "1. Smoke test..."
docker compose up -d
sleep 30

# 2. Health check
echo "2. Health check..."
curl -f http://localhost:3000/api/health || exit 1
curl -f http://localhost:8080 || exit 1

# 3. Unit tests
echo "3. Unit tests..."
(cd backend && go test ./internal/...) || exit 1

# 4. E2E tests
echo "4. E2E tests..."
(cd backend && go test ./test/e2e/ -count=1 -timeout 5m) || exit 1

# 5. Performance test
echo "5. Performance test..."
ab -n 100 -c 10 http://localhost:3000/api/health

echo "=== Validation Complete ==="
```

### 5.3. Автоматический отчёт

Полная проверка контракта (41 проверка: health, auth, matching, testing,
gamification, invitations, vacancies, fsp, notifications, Swagger) выполняется
одной командой с работающим сервером и БД:

```bash
cd backend
API_URL=http://localhost:3000/api FRONTEND_URL=http://localhost:8080 go run ./cmd/metrics
```

Скрипт пишет отчёт в `docs/VALIDATION_REPORT.md` (итог последнего прогона:
41/41 PASS).

## 6. Промпт для агента валидации

```
Ты — QA-инженер, отвечающий за валидацию платформы ФСП.

ТВОЯ ЗАДАЧА: Провести полную валидацию решения.

ВХОДНЫЕ ДАННЫЕ:
- docs/VALIDATION.md — процедуры валидации (этот файл)
- docs/ARCHITECTURE.md — архитектура проекта

ЧТО НУЖНО СДЕЛАТЬ:
1. Провести smoke test
2. Провести e2e test
3. Проверить matching quality
4. Проверить testing quality
5. Проверить security
6. Проверить performance
7. Составить отчёт о валидации

СТЕК:
- Docker
- curl
- Apache Bench

ВАЛИДАЦИЯ:
- Все тесты проходят
- Метрики соответствуют ожиданиям
- Отчёт составлен

ФОРМАТ ОТВЕТА:
- Отчёт о валидации (по шаблону)
- Список найденных проблем
- Рекомендации по улучшению
```