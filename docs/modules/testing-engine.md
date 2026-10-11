# Модуль: Testing Engine (Движок тестирования)

## 1. Назначение

Ключевой модуль платформы — отвечает за:
- Генерацию персонализированных тестов для кандидатов
- Адаптивное тестирование (CAT — Computer Adaptive Testing)
- Оценку ответов и расчёт грейда
- Anti-cheat механизмы
- Регулярные задания (daily tasks) — по ТЗ; фактически реализована только схема БД (см. §7)

**Реализация**: Go-порт NestJS/TypeScript-бэкенда — две версии API: V1 (legacy, вопрос-ориентированный) и V2 (мультиформатные задания, см. §4.6).

**Зависимости**: database, auth
**Блокирует**: matching-engine, frontend

## 2. Проблема и решение

### Проблема из ТЗ:
> "Единый набор заданий для всех кандидатов не подходит — задания и ответы быстро разойдутся между участниками рынка; генерация уникальных заданий нейросетью под каждого кандидата может решить задачу, но она не гарантирует сопоставимую сложность и объективность оценки между разными кандидатами."

### Наше решение: Гибридный подход

```
┌─────────────────────────────────────────────────────────────┐
│                    TESTING ENGINE                           │
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ Question    │  │ Adaptive    │  │ Anti-Cheat          │ │
│  │ Pool        │  │ Algorithm   │  │ Layer               │ │
│  │ (банк       │  │ (CAT + IRT) │  │ (randomization,    │ │
│  │  вопросов)  │  │             │  │  time analysis)     │ │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘ │
│         │                │                     │            │
│         ▼                ▼                     ▼            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Personalized Test Builder                │  │
│  │  - Выбирает вопросы из пула по специализации         │  │
│  │  - Адаптирует сложность на основе ответов            │  │
│  │  - Формирует уникальную последовательность           │  │
│  │  - Добавляет вариативные формулировки                │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Компоненты:

1. **Question Pool (банк вопросов)** — 50+ вопросов на каждую специализацию, разбитых по сложности
2. **CAT Algorithm** — адаптивный алгоритм, подбирающий следующий вопрос на основе ответов
3. **IRT Scoring** — Item Response Theory для объективной оценки способностей
4. **Anti-Cheat** — рандомизация, анализ времени, вариативные формулировки

## 3. Архитектура модуля

```
backend/internal/modules/testing/
├── testing.go                # V1 — оркестратор тестирования (Service)
├── testing-v2.go             # V2 — мультиформатные задания (ServiceV2)
├── handler.go                # HTTP-слой: 10 маршрутов /api/testing (V1 + V2)
├── dto.go                    # DTO + валидация (тексты как в class-validator)
├── store.go                  # интерфейс Store + PGStore (pgx/v5, raw SQL)
├── testing-v2_test.go        # unit-тесты V2 (порт testing-v2.service.spec.ts)
└── services/
    ├── question-pool.go      # банк вопросов: Category, PoolQuestion, PoolStore
    ├── cat.go                # CAT: 3PL IRT, Fisher Information, MLE-theta
    ├── scoring.go            # баллы, грейд из theta, рекомендация
    ├── anti-cheat.go         # статистический анти-чит + рандомизация
    ├── behavioral-anti-cheat.go  # V2: keystroke, copy-paste, tab-switch
    ├── task-generator.go     # V2: мультиформатные задания, распределение по грейдам
    ├── llm-evaluation.go     # LLM-оценка (OpenAI API или keyword-fallback)
    ├── code-sandbox.go       # запуск кода кандидата (тест-кейсы, лимиты)
    ├── js.go                 # JS-совместимые утилиты (Math.round, LCG, UTF-16)
    └── *_test.go             # порты anti_cheat_spec / behavioral / cat / scoring
```

Хранение — таблицы `test_categories` / `test_questions` / `test_sessions` / `test_answers` (сырой SQL через pgx/v5, без ORM). Сид банка вопросов — `backend/cmd/seed` + `backend/internal/seed/data/questions.json` (858 вопросов — JSON-дамп эталонного TS-сида).

## 4. API Endpoints

### 4.1. Начать тестирование

```http
POST /api/testing/start
Authorization: Bearer {token}
Content-Type: application/json

{
  "specialization": "BACKEND",
  "declaredGrade": "MIDDLE"
}

Response 201:
{
  "sessionId": "uuid",
  "totalQuestions": 20,
  "timeLimitMinutes": 60,
  "firstQuestion": {
    "id": "uuid",
    "text": "Что такое индекс в PostgreSQL?",
    "questionType": "multiple_choice",
    "options": [...],
    "difficulty": 0.3
  }
}

Errors:
- 400: Cooldown active (нельзя пересдавать чаще чем раз в 3 месяца)
- 409: Test already in progress
```

### 4.2. Получить текущий вопрос

```http
GET /api/testing/session/{sessionId}/current
Authorization: Bearer {token}

Response 200:
{
  "questionNumber": 5,
  "totalQuestions": 20,
  "question": {
    "id": "uuid",
    "text": "...",
    "questionType": "multiple_choice",
    "options": [...],
    "codeSnippet": "...",  // если есть
    "difficulty": 0.6
  },
  "timeRemainingSeconds": 2847
}
```

### 4.3. Отправить ответ

```http
POST /api/testing/session/{sessionId}/answer
Authorization: Bearer {token}
Content-Type: application/json

{
  "questionId": "uuid",
  "answer": "b",  // Для multiple_choice — id варианта
  "answerText": "...",  // Для coding/open_text
  "timeSpentSeconds": 45
}

Response 200:
{
  "correct": true,
  "explanation": "Индекс ускоряет поиск по столбцу...",
  "nextQuestion": {  // Если есть следующий
    "id": "uuid",
    "text": "...",
    ...
  },
  "currentScore": 72.5,
  "questionsRemaining": 15
}
```

### 4.4. Завершить тест

```http
POST /api/testing/session/{sessionId}/complete
Authorization: Bearer {token}

Response 200:
{
  "sessionId": "uuid",
  "status": "COMPLETED",
  "score": 78.5,
  "gradeBefore": "MIDDLE",
  "gradeAfter": "MIDDLE",  // Может измениться
  "recommendation": "PASS",  // PASS, FAIL, UPGRADE
  "details": {
    "easy": { "correct": 8, "total": 10 },
    "medium": { "correct": 6, "total": 7 },
    "hard": { "correct": 2, "total": 3 }
  },
  "cooldownUntil": "2026-01-15T00:00:00Z"
}
```

### 4.5. Пересдать тест

```http
POST /api/testing/retake
Authorization: Bearer {token}
Content-Type: application/json

{
  "specialization": "BACKEND",
  "targetGrade": "MIDDLE_PLUS"  // Хочу повысить грейд
}

Response 201: { ... same as start ... }

Errors:
- 400: Cooldown active until {date}
```

Также есть `GET /api/testing/active` — активная сессия кандидата (используется фронтендом для восстановления теста).

### 4.6. V2 — мультиформатные задания

```http
POST /api/testing/v2/start
GET  /api/testing/v2/session/{sessionId}/current
POST /api/testing/v2/session/{sessionId}/answer
POST /api/testing/v2/session/{sessionId}/complete
```

Статусы — как в V1 (start/answer/complete — 201, current — 200), DTO — `dto.go`, контракты — `StartResultV2` / `CurrentResultV2` / `AnswerResultV2` / `TestResultV2` (см. Swagger `/api/docs`).

- Типы заданий: `multiple_choice`, `code_execution`, `llm_open`, `code_review`, `system_design`; распределение по грейду — `services/task-generator.go`.
- Повторная отправка ответа идемпотентна (`duplicate: true`); в ответ передаётся behavioral-телеметрия (copy-paste, tab-switch, keystrokes).
- `complete` — task breakdown + поведенческий анти-чит (`behavioral-anti-cheat.go`) + геймификация (см. gamification).
- Оценка кода — встроенный JS-рантайм (`js.go`, `code-sandbox.go`), Docker-режим не подключён (см. temp-docs/09-known-issues.md, T3).
- LLM-оценка (`llm-evaluation.go`) — OpenAI-совместимый API; при отсутствии `OPENAI_API_KEY` работает детерминированная keyword-эвристика (лог «LLM Evaluation: OPENAI_API_KEY not set, using keyword fallback»). Embeddings/ONNX в Go-бэкенде нет.

## 5. Алгоритмы

### 5.1. CAT (Computer Adaptive Testing)

```go
// services/cat.go

// CAT — Computer Adaptive Testing (3PL IRT):
//   - следующий вопрос выбирается по максимальной Fisher Information при текущем theta,
//   - theta оценивается методом максимального правдоподобия (Newton-Raphson).
// Все вычисления детерминированы — повторный запрос даёт тот же вопрос.
type CAT struct{}

// Probability — P(правильный ответ) по 3PL: c + (1-c)/(1+e^{-a(theta-b)}).
// NaN/Inf санитизируются (a → 1, b → 0.5, c → 0.25), результат — в (0; 1).
func (c *CAT) Probability(theta, a, b, guessing float64) float64 {
	p := guessing + (1-guessing)/(1+math.Exp(-a*(theta-b)))
	return math.Min(1-1e-6, math.Max(1e-6, p))
}

// FisherInformation — Fisher Information для 3PL-модели.
// I(θ) = (P'(θ))² / (P(θ) · (1 − P(θ)))
func (c *CAT) FisherInformation(theta float64, question IrtQuestion) float64 {
	p := c.Probability(theta, question.Discrimination, question.Difficulty, question.Guessing)
	exp := math.Exp(-question.Discrimination * (theta - question.Difficulty))
	pPrime := (question.Discrimination * (1 - question.Guessing) * exp) / math.Pow(1+exp, 2)
	return (pPrime * pPrime) / (p * (1 - p))
}

// EstimateTheta — оценка theta (MLE, Newton-Raphson / Fisher scoring):
// 50 итераций, tolerance 0.001; используется ожидаемая информация I(theta) >= 0
// вместо наблюдаемого гессиана (3PL-гессиан меняет знак при p < c и ломал оценку),
// демпфер шага ±0.5, theta ограничен [-4; 4].
func (c *CAT) EstimateTheta(answers []IrtAnswer) float64 { ... }

// SelectNext — выбор следующего вопроса из пула:
// 1) повторы исключаются; 2) первый вопрос — ближайший к difficulty 0.5;
// 3) далее — max Fisher Information при текущем theta;
// 4) при равной информации побеждает детерминированный порядок сессии
//    (FNV-1a от "sessionId:questionId") — /current и /answer согласованы.
func SelectNext[T IrtCapable](available []T, answered []IrtAnswer, sessionID string) *T { ... }
```

### 5.2. Scoring — определение грейда

```go
// services/scoring.go

// GradeThresholds — пороговые значения theta для каждого грейда.
var GradeThresholds = []GradeThreshold{
	{Grade: model.GradeJunior, Min: math.Inf(-1)}, // theta < -1.0
	{Grade: model.GradeMiddle, Min: -1.0},         // -1.0 <= theta < 0.0
	{Grade: model.GradeMiddlePlus, Min: 0.0},      // 0.0 <= theta < 0.5
	{Grade: model.GradeSenior, Min: 0.5},          // 0.5 <= theta < 1.0
	{Grade: model.GradeLead, Min: 1.0},            // 1.0 <= theta < 1.5
	{Grade: model.GradePrincipal, Min: 1.5},       // theta >= 1.5
}

// CalculateGrade — грейд из theta (верхний достигнутый порог).
// Нормализация theta по специализации — таблица specOffset, но она пустая
// (заготовка — normalizeTheta ничего не делает).
func (s *Scoring) CalculateGrade(theta float64, specialization *model.Specialization) model.Grade {
	normalized := theta + specOffset[*specialization]
	result := model.GradeJunior
	for _, threshold := range GradeThresholds {
		if normalized >= threshold.Min {
			result = threshold.Grade
		}
	}
	return result
}

// GetRecommendation — PASS/FAIL/UPGRADE.
// Без заявленного грейда: score >= 50 → PASS, иначе FAIL.
func (s *Scoring) GetRecommendation(declared *model.Grade, calculated model.Grade, score float64) Recommendation {
	declaredIndex := s.GradeIndex(*declared)
	calculatedIndex := s.GradeIndex(calculated)

	if calculatedIndex >= declaredIndex+1 && score >= 85 {
		return RecUpgrade // Уверенно справился — можно повысить
	}
	if calculatedIndex >= declaredIndex {
		return RecPass // Подтвердил грейд
	}
	return RecFail // Не дотянул
}

// ScoreFromAnswers — взвешенная оценка 0-100: сложные вопросы весят больше.
// DetailsByDifficulty — распределение верных ответов по корзинам easy/medium/hard.
// GradeIndex — строгий порядок грейдов (0..5).
```

### 5.3. Anti-Cheat

```go
// services/anti-cheat.go

// Analyze — статистический анализ сессии.
func (a *AntiCheat) Analyze(answers []TimedAnswer, thetaHistory []float64) CheatAnalysis {
	flags := []string{}

	// 1. Слишком быстрые ответы на сложных вопросах (< 5 сек, difficulty > 0.5):
	//    больше 3 → MULTIPLE_TOO_FAST_ANSWERS
	// 2. Паттерн «всё сложно — идеально, всё лёгкое — провал» → SUSPICIOUS_DIFFICULTY_PATTERN
	// 3. Идеальный результат по всему пулу (poolTag, больше 5 ответов) → PERFECT_SCORE_POOL_{POOL}
	// 4. Резкие скачки theta (дельта > 2 между ответами, больше 2 раз) → THETA_VOLATILITY

	return CheatAnalysis{
		IsSuspicious: len(flags) >= 2,
		Flags:        flags,
		Confidence:   min1(JSRound(float64(len(flags))*0.35*100) / 100),
	}
}

// ThetaHistory — история theta после каждого ответов — вход для детекции волатильности.
func (a *AntiCheat) ThetaHistory(answers []IrtAnswer, estimate func([]IrtAnswer) float64) []float64 { ... }

// ShuffleOptions — порядок вариантов ответа, уникальный для каждой сессии
// (клиент не видит isCorrect). Детерминированная ротация:
// shift = SessionOrderKey(sessionID, questionID) % len(options).
func ShuffleOptions[T any](sessionID, questionID string, options []T) []T { ... }
```

Поведенческий анти-чит V2 (`behavioral-anti-cheat.go`) дополнительно анализирует keystroke-ритм, copy-paste и tab-switch — аддитивный скор 0-100 по сигналам.

## 6. Банк вопросов (Question Pool)

### 6.1. Структура

Фактические модели (`services/question-pool.go`, хранение — raw SQL через `store.go`):

```go
// Category — строка test_categories.
type Category struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Description    *string              `json:"description"`
	Specialization model.Specialization `json:"specialization"`
}

// PoolQuestion — вопрос банка (вся строка test_questions с IRT-параметрами).
type PoolQuestion struct {
	ID            string
	QuestionType  string
	Text          string
	CodeSnippet   *string
	Options       json.RawMessage // JSON из БД: null/[]/[...] (nil = null)
	CorrectAnswer *string
	Explanation   *string

	Difficulty     float64
	Discrimination float64
	Guessing       float64

	PoolTag  *string
	IsUnique bool
}
```

Сид — `backend/internal/seed/data/questions.json` (858 вопросов, JSON-дамп из эталонного TS-сида), загружается командой `backend/cmd/seed` в таблицы `test_categories` / `test_questions`.

### 6.2. Примеры вопросов для BACKEND

#### Easy (difficulty: 0.1-0.3)

```json
{
  "questionType": "multiple_choice",
  "text": "Какой HTTP-статус код возвращается при успешном создании ресурса?",
  "options": [
    { "id": "a", "text": "200 OK", "isCorrect": false },
    { "id": "b", "text": "201 Created", "isCorrect": true },
    { "id": "c", "text": "204 No Content", "isCorrect": false },
    { "id": "d", "text": "301 Moved Permanently", "isCorrect": false }
  ],
  "difficulty": 0.15,
  "discrimination": 0.8,
  "guessing": 0.25,
  "poolTag": "http_basics"
}
```

#### Medium (difficulty: 0.4-0.6)

```json
{
  "questionType": "coding",
  "text": "Реализуйте middleware для rate limiting в Express.js. Ограничение: 100 запросов в минуту на IP.",
  "codeSnippet": "const rateLimit = (options) => {\n  // Ваш код здесь\n};",
  "correctAnswer": "const rateLimit = (options) => {\n  const requests = new Map();\n  \n  return (req, res, next) => {\n    const ip = req.ip;\n    const now = Date.now();\n    const windowMs = options.windowMs || 60000;\n    const max = options.max || 100;\n    \n    if (!requests.has(ip)) {\n      requests.set(ip, []);\n    }\n    \n    const timestamps = requests.get(ip).filter(t => now - t < windowMs);\n    timestamps.push(now);\n    requests.set(ip, timestamps);\n    \n    if (timestamps.length > max) {\n      return res.status(429).json({ error: 'Too many requests' });\n    }\n    \n    next();\n  };\n};",
  "difficulty": 0.5,
  "discrimination": 1.2,
  "guessing": 0.1,
  "poolTag": "express_middleware"
}
```

#### Hard (difficulty: 0.7-0.9)

```json
{
  "questionType": "scenario",
  "text": "Ваше микросервисная архитектура имеет 15 сервисов. Один из сервисов начал отвечать с задержкой 5 секунд вместо 100мс. Опишите пошаговый план диагностики и решения проблемы.",
  "difficulty": 0.8,
  "discrimination": 1.5,
  "guessing": 0.05,
  "poolTag": "distributed_systems"
}
```

### 6.3. Правила генерации пула

Для MVP достаточно:
- **Каждая специализация**: 60+ вопросов
- **Распределение**: 25 easy + 25 medium + 15 hard (включая 5 уникальных)
- **В тесте**: 20 вопросов (8 easy + 8 medium + 4 hard)
- **Уникальные вопросы**: 5 вопросов с `isUnique: true` — генерируются под конкретного кандидата (контекстные задачи)

Фактически сид содержит 858 вопросов по всем специализациям (`internal/seed/data/questions.json`); размер теста и кулдаун V1 — конфигурация (`TestingQuestionsPerTest=20`, `TestingCooldownDays=90`), V2 — env (`TESTING_TASKS_PER_TEST` / `TESTING_COOLDOWN_DAYS`) — см. temp-docs/09-known-issues.md, T5.

## 7. Ежедневные задания (Daily Tasks)

По ТЗ — сервис генерации ежедневного задания для кандидата (по специализации и грейду, срок 24 часа) и оценки ответа (простая keyword-эвристика, в продакшене — LLM-based оценка).

**Фактическое состояние**: сервиса и эндпоинтов daily tasks в бэкенде нет — сохранены только:
- модель `DailyTask` / `DailyTaskStatus` — `backend/internal/model/model.go`
- таблица `daily_tasks` — `backend/migrations/000001_init.up.sql`

См. temp-docs/09-known-issues.md (G5: DAILY_TASK_DONE — модель есть, сервиса/эндпоинтов нет). Keyword-эвристика и LLM-оценка из этого ТЗ реализованы для V2-заданий в `services/llm-evaluation.go` (см. §4.6).

## 8. Чеклист валидации

### Функциональность
- [ ] Тест стартует и возвращает первый вопрос
- [ ] Вопросы приходят в адаптивном порядке (CAT)
- [ ] Ответы оцениваются корректно
- [ ] Theta обновляется после каждого ответа
- [ ] Тест завершается после N вопросов или по таймауту
- [ ] Грейд рассчитывается корректно
- [ ] Рекомендация (PASS/FAIL/UPGRADE) определяется верно
- [ ] Cooldown работает (нельзя пересдавать чаще 1 раза в 3 месяца)
- [ ] Ежедневные задания генерируются (не реализовано — см. §7)

### Anti-Cheat
- [ ] Вопросы перемешиваются
- [ ] Варианты ответов перемешиваются
- [ ] Уникальные вопросы генерируются
- [ ] Анализ времени работает
- [ ] Подозрительные паттерны детектируются

### Качество тестов
- [ ] Тест даёт стабильный грейд при повторном прохождении (test-retest reliability)
- [ ] Тест разделяет слабых и сильных кандидатов (discriminativeness)
- [ ] Время прохождения адекватное (30-60 минут)

### Тесты модуля
- Unit: `go test ./internal/modules/testing/...` — V1/V2-флоу (`testing-v2_test.go`), CAT, scoring, статистический и поведенческий анти-чит (порты *.spec.ts → *_test.go)
- E2E: `backend/test/e2e/e2e_test.go` — полный флоу через HTTP; нужны переменные `DATABASE_URL`, `REDIS_URL`, `ML_SERVICE_URL`

## 9. Промпт для агента

```
Ты — Backend-разработчик с опытом в разработке систем тестирования и психометрии.

ТВОЯ ЗАДАЧА: Реализовать модуль testing-engine для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/ARCHITECTURE.md — общая архитектура проекта
- docs/modules/testing-engine.md — ТЗ на модуль (этот файл)
- docs/modules/database.md — схема данных (TestSession, TestQuestion, TestAnswer, etc.)

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать backend/internal/modules/testing/ (V1 + V2) со всеми файлами
2. Реализовать QuestionPool — банк вопросов (question-pool.go)
3. Реализовать CAT — адаптивный алгоритм (CAT + IRT, cat.go)
4. Реализовать Scoring — расчёт грейда (scoring.go)
5. Реализовать AntiCheat — anti-cheat механизмы (anti-cheat.go)
6. Реализовать TaskGenerator — V2-задания (task-generator.go); DailyTaskService — не реализован (см. §7)
7. Создать DTO с валидацией (dto.go)
8. Сид-данные: банк вопросов в JSON (backend/internal/seed/data/questions.json)
9. Написать unit-тесты для каждого сервиса (go test)
10. Написать e2e тест для полного флоу тестирования (backend/test/e2e)

АЛГОРИТМЫ:
- CAT: используй 3PL IRT модель для выбора вопросов
- Scoring: пороговые значения theta для определения грейда
- Anti-cheat: анализ времени, рандомизация, детекция паттернов

СТЕК:
- Go 1.25
- gin
- pgx/v5 (сырой SQL, без ORM)
- golang-jwt/v5, bcrypt (cost 12)
- валидация DTO (тексты как в class-validator)

ВАЛИДАЦИЯ:
- Запусти unit-тесты: go test ./internal/modules/testing/...
- Запусти e2e (DATABASE_URL, REDIS_URL, ML_SERVICE_URL)
- Проверь что грейд рассчитывается корректно
- Проверь что cooldown работает
- Проверь anti-cheat на подозрительных паттернах

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Результат тестов
- Пример вывода CAT для 5 ответов
- Замечания по алгоритмам
```
