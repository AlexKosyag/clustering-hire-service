# Модуль: Testing Engine (Движок тестирования)

## 1. Назначение

Ключевой модуль платформы — отвечает за:
- Генерацию персонализированных тестов для кандидатов
- Адаптивное тестирование (CAT — Computer Adaptive Testing)
- Оценку ответов и расчёт грейда
- Anti-cheat механизмы
- Регулярные задания (daily tasks)

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
src/modules/testing/
├── testing.module.ts
├── testing.controller.ts
├── services/
│   ├── testing.service.ts          # Оркестратор тестирования
│   ├── question-pool.service.ts    # Управление банком вопросов
│   ├── cat.service.ts              # Adaptive testing algorithm
│   ├── scoring.service.ts          # Оценка и расчёт грейда
│   └── anti-cheat.service.ts       # Anti-cheat механизмы
├── dto/
│   ├── start-test.dto.ts
│   ├── submit-answer.dto.ts
│   └── test-result.dto.ts
├── entities/
│   ├── test-session.entity.ts
│   ├── test-question.entity.ts
│   └── test-answer.entity.ts
└── strategies/
    ├── grading-strategy.ts         # Стратегия определения грейда
    └── question-selection.ts       # Стратегия выбора вопросов
```

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

## 5. Алгоритмы

### 5.1. CAT (Computer Adaptive Testing)

```typescript
// services/cat.service.ts

@Injectable()
export class CATService {
  /**
   * Выбирает следующий вопрос на основе истории ответов.
   * 
   * Алгоритм:
   * 1. Оцениваем текущий theta (уровень способности) через IRT
   * 2. Выбираем вопрос с максимальной информацией при текущем theta
   * 3. Учитываем чтобы вопросы не повторялись
   * 4. Учитываем баланс сложности (не только hard)
   */
  selectNextQuestion(
    answeredQuestions: AnsweredQuestion[],
    availableQuestions: Question[],
    currentTheta: number,
  ): Question {
    if (answeredQuestions.length === 0) {
      // Первый вопрос — средней сложности
      return this.findByDifficulty(availableQuestions, 0.5);
    }

    // Обновляем theta через Maximum Likelihood Estimation
    const newTheta = this.estimateTheta(answeredQuestions);

    // Выбираем вопрос с максимальной Fisher Information
    let bestQuestion: Question | null = null;
    let maxInformation = -Infinity;

    for (const q of availableQuestions) {
      if (answeredQuestions.some((a) => a.questionId === q.id)) continue;

      const info = this.fisherInformation(newTheta, q);
      if (info > maxInformation) {
        maxInformation = info;
        bestQuestion = q;
      }
    }

    return bestQuestion!;
  }

  /**
   * Fisher Information для 3PL IRT модели
   * I(θ) = (P'(θ))² / (P(θ) * (1 - P(θ)))
   * 
   * где P(θ) = c + (1-c) / (1 + e^(-a(θ-b)))
   * a = discrimination, b = difficulty, c = guessing
   */
  private fisherInformation(theta: number, question: Question): number {
    const { difficulty: b, discrimination: a, guessing: c } = question;
    const p = this.probability(theta, a, b, c);
    const pItem = (a * (1 - c) * Math.exp(-a * (theta - b))) /
      Math.pow(1 + Math.exp(-a * (theta - b)), 2);
    return (pItem * pItem) / (p * (1 - p));
  }

  /**
   * Вероятность правильного ответа (3PL model)
   */
  private probability(
    theta: number,
    a: number,
    b: number,
    c: number,
  ): number {
    return c + (1 - c) / (1 + Math.exp(-a * (theta - b)));
  }

  /**
   * Оценка theta через MLE
   */
  private estimateTheta(answers: AnsweredQuestion[]): number {
    let theta = 0; // Начальное приближение
    const maxIter = 50;
    const tolerance = 0.001;

    for (let i = 0; i < maxIter; i++) {
      let firstDeriv = 0;
      let secondDeriv = 0;

      for (const answer of answers) {
        const { discrimination: a, difficulty: b, guessing: c } = answer.question;
        const p = this.probability(theta, a, b, c);
        const q = 1 - p;
        const u = answer.isCorrect ? 1 : 0;

        firstDeriv += a * (u - p) * (p - c) / (p * (1 - c));
        secondDeriv -= a * a * (p - c) * (q * (p - c) + c * (u - p)) /
          (p * p * (1 - c) * (1 - c));
      }

      const delta = firstDeriv / secondDeriv;
      theta -= delta;

      if (Math.abs(delta) < tolerance) break;
    }

    return theta;
  }
}
```

### 5.2. Scoring — определение грейда

```typescript
// services/scoring.service.ts

@Injectable()
export class ScoringService {
  // Пороговые значения theta для каждого грейда
  private readonly GRADE_THRESHOLDS: Record<Grade, number> = {
    JUNIOR: -1.0,      // theta < -1.0
    MIDDLE: 0.0,       // -1.0 <= theta < 0.0
    MIDDLE_PLUS: 0.5,  // 0.0 <= theta < 0.5
    SENIOR: 1.0,       // 0.5 <= theta < 1.0
    LEAD: 1.5,         // 1.0 <= theta < 1.5
    PRINCIPAL: 2.0,    // theta >= 1.5
  };

  /**
   * Определяет грейд на основе theta и порогов
   */
  calculateGrade(theta: number, specialization: Specialization): Grade {
    // Нормализуем theta по специализации (разные специальности — разная сложность)
    const normalizedTheta = this.normalizeTheta(theta, specialization);

    if (normalizedTheta >= this.GRADE_THRESHOLDS.PRINCIPAL) return Grade.PRINCIPAL;
    if (normalizedTheta >= this.GRADE_THRESHOLDS.LEAD) return Grade.LEAD;
    if (normalizedTheta >= this.GRADE_THRESHOLDS.SENIOR) return Grade.SENIOR;
    if (normalizedTheta >= this.GRADE_THRESHOLDS.MIDDLE_PLUS) return Grade.MIDDLE_PLUS;
    if (normalizedTheta >= this.GRADE_THRESHOLDS.MIDDLE) return Grade.MIDDLE;
    return Grade.JUNIOR;
  }

  /**
   * Определяет рекомендацию: PASS, FAIL, UPGRADE
   */
  getRecommendation(
    declaredGrade: Grade,
    calculatedGrade: Grade,
    score: number,
  ): 'PASS' | 'FAIL' | 'UPGRADE' {
    const declaredIndex = this.gradeIndex(declaredGrade);
    const calculatedIndex = this.gradeIndex(calculatedGrade);

    if (calculatedIndex >= declaredIndex + 1 && score >= 85) {
      return 'UPGRADE'; // Уверенно справился — можно повысить
    }
    if (calculatedIndex >= declaredIndex) {
      return 'PASS'; // Подтвердил грейд
    }
    return 'FAIL'; // Не дотянул
  }

  private gradeIndex(grade: Grade): number {
    const order = [
      Grade.JUNIOR,
      Grade.MIDDLE,
      Grade.MIDDLE_PLUS,
      Grade.SENIOR,
      Grade.LEAD,
      Grade.PRINCIPAL,
    ];
    return order.indexOf(grade);
  }
}
```

### 5.3. Anti-Cheat

```typescript
// services/anti-cheat.service.ts

@Injectable()
export class AntiCheatService {
  /**
   * Проверяет подозрительную активность
   */
  analyzeSession(session: TestSession): CheatAnalysis {
    const flags: string[] = [];

    // 1. Анализ времени ответа
    const avgTime = this.averageTime(session.answers);
    const tooFastAnswers = session.answers.filter(
      (a) => a.timeSpentSeconds < 5 && a.question.difficulty > 0.5,
    );
    if (tooFastAnswers.length > 3) {
      flags.push('MULTIPLE_TOO_FAST_ANSWERS');
    }

    // 2. Паттерн "всё правильно на сложных, всё неправильно на лёгких"
    const easyCorrect = session.answers.filter(
      (a) => a.question.difficulty < 0.3 && a.isCorrect,
    ).length;
    const hardCorrect = session.answers.filter(
      (a) => a.question.difficulty > 0.7 && a.isCorrect,
    ).length;
    if (hardCorrect > easyCorrect && easyCorrect < 2) {
      flags.push('SUSPICIOUS_DIFFICULTY_PATTERN');
    }

    // 3. Идеальные ответы на вопросах из одного пула
    const poolGroups = this.groupByPool(session.answers);
    for (const [pool, answers] of Object.entries(poolGroups)) {
      const allCorrect = answers.every((a) => a.isCorrect);
      if (allCorrect && answers.length > 5) {
        flags.push(`PERFECT_SCORE_POOL_${pool}`);
      }
    }

    // 4. Резкое изменение theta
    const thetaHistory = this.calculateThetaHistory(session.answers);
    const thetaJumps = this.detectThetaJumps(thetaHistory);
    if (thetaJumps > 2) {
      flags.push('THETA_VOLATILITY');
    }

    return {
      isSuspicious: flags.length >= 2,
      flags,
      confidence: this.calculateConfidence(flags),
    };
  }

  /**
   * Рандомизирует формулировки вопросов
   */
  randomizeQuestion(question: Question): Question {
    if (question.questionType === 'multiple_choice') {
      // Перемешиваем варианты ответов
      const shuffled = this.shuffleArray([...question.options]);
      return { ...question, options: shuffled };
    }
    return question;
  }

  /**
   * Рандомизирует порядок вопросов (в пределах одного уровня сложности)
   */
  randomizeOrder(questions: Question[]): Question[] {
    const byDifficulty = this.groupByDifficulty(questions);
    const result: Question[] = [];

    for (const group of Object.values(byDifficulty)) {
      result.push(...this.shuffleArray(group));
    }

    return result;
  }
}
```

## 6. Банк вопросов (Question Pool)

### 6.1. Структура

```typescript
interface QuestionPool {
  specialization: Specialization;
  questions: {
    easy: Question[];      // difficulty 0.1-0.3
    medium: Question[];    // difficulty 0.4-0.6
    hard: Question[];      // difficulty 0.7-0.9
  };
  minPerTest: number;      // Минимум вопросов в тесте
  maxPerTest: number;      // Максимум вопросов в тесте
}
```

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

## 7. Ежедневные задания (Daily Tasks)

```typescript
// services/daily-task.service.ts

@Injectable()
export class DailyTaskService {
  /**
   * Генерирует ежедневное задание для кандидата.
   * Задание приходит от "работодателя" (из пула реальных или синтетических).
   */
  async generateDailyTask(candidateId: string): Promise<DailyTask> {
    const candidate = await this.prisma.candidateProfile.findUnique({
      where: { id: candidateId },
    });

    // Выбираем задание по специализации и грейду
    const task = await this.selectTask(
      candidate.specialization,
      candidate.verifiedGrade || candidate.declaredGrade,
    );

    return this.prisma.dailyTask.create({
      data: {
        candidateId,
        title: task.title,
        description: task.description,
        difficulty: task.difficulty,
        expectedTimeMinutes: task.expectedTime,
        dueAt: new Date(Date.now() + 24 * 60 * 60 * 1000), // 24 часа
      },
    });
  }

  /**
   * Оценивает ответ на ежедневное задание.
   * Использует простую эвристику + опционально LLM для оценки.
   */
  async evaluateAnswer(taskId: string, answer: string): Promise<number> {
    const task = await this.prisma.dailyTask.findUnique({
      where: { id: taskId },
    });

    // Простая оценка по ключевым словам (MVP)
    // В продакшене — LLM-based оценка
    const score = this.keywordScoring(task.description, answer);

    return score;
  }
}
```

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
- [ ] Ежедневные задания генерируются

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

## 9. Промпт для агента

```
Ты — Backend-разработчик с опытом в разработке систем тестирования и психометрии.

ТВОЯ ЗАДАЧА: Реализовать модуль testing-engine для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/ARCHITECTURE.md — общая архитектура проекта
- docs/modules/testing-engine.md — ТЗ на модуль (этот файл)
- docs/modules/database.md — схема данных (TestSession, TestQuestion, TestAnswer, etc.)

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать backend/src/modules/testing/ со всеми файлами
2. Реализовать QuestionPoolService — управление банком вопросов
3. Реализовать CATService — адаптивный алгоритм (CAT + IRT)
4. Реализовать ScoringService — расчёт грейда
5. Реализовать AntiCheatService — anti-cheat механизмы
6. Реализовать DailyTaskService — ежедневные задания
7. Создать DTO с валидацией
8. Написать seed-данные: 60+ вопросов для специализации BACKEND
9. Написать unit-тесты для каждого сервиса
10. Написать e2e тест для полного флоу тестирования

АЛГОРИТМЫ:
- CAT: используй 3PL IRT модель для выбора вопросов
- Scoring: пороговые значения theta для определения грейда
- Anti-cheat: анализ времени, рандомизация, детекция паттернов

СТЕК:
- NestJS 10
- Prisma 5
- class-validator

ВАЛИДАЦИЯ:
- Запусти полный флоу: start → answer 20 questions → complete
- Проверь что грейд рассчитывается корректно
- Проверь что cooldown работает
- Проверь anti-cheat на подозрительных паттернах

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Результат тестов
- Пример вывода CAT для 5 ответов
- Замечания по алгоритмам
```