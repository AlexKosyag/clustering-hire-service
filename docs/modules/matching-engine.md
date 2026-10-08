# Модуль: Matching Engine (Движок подбора кандидатов)

## 1. Назначение

Ключевой модуль для критерия №1 (35% оценки) — отвечает за:
- Semantic matching — векторный поиск кандидатов по описанию потребности
- Category-based ranking — ранжирование внутри категории
- Explainable results — объяснимость выдачи
- Collaborative filtering — рекомендации на основе поведения

**Зависимости**: database, backend-api, testing-engine
**Блокирует**: frontend

## 2. Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                    MATCHING ENGINE                          │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  1. QUERY UNDERSTANDING                               │  │
│  │  - Парсинг описания потребности работодателя         │  │
│  │  - Извлечение: stack, grade, specialization, skills  │  │
│  │  - Генерация embedding для текстового описания       │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                  │
│                          ▼                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  2. CANDIDATE FILTERING                               │  │
│  │  - Hard filters: specialization, grade, stack        │  │
│  │  - Soft filters: experience, achievements, activity  │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                  │
│                          ▼                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  3. SCORING & RANKING                                 │  │
│  │  - Category match score (0-100)                       │  │
│  │  - Stack overlap score (0-100)                        │  │
│  │  - Semantic similarity score (0-100)                  │  │
│  │  - Achievement boost                                  │  │
│  │  - Activity recency boost                             │  │
│  │  - Final weighted score                               │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                  │
│                          ▼                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  4. EXPLANATION                                       │  │
│  │  - Генерация текстового объяснения почему кандидат   │  │
│  │    попал в подборку                                   │  │
│  │  - Highlighting совпадений                            │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## 3. API Endpoints

### 3.1. Поиск кандидатов по потребности

```http
POST /api/matching/search
Authorization: Bearer {employer-token}
Content-Type: application/json

{
  "description": "Ищем backend-разработчика на Node.js с опытом от 3 лет, который знает PostgreSQL и умеет проектировать микросервисы",
  "specialization": "BACKEND",
  "grade": "MIDDLE_PLUS",
  "stack": ["Node.js", "PostgreSQL", "Docker"],
  "salaryFrom": 150000,
  "salaryTo": 250000,
  "limit": 20,
  "offset": 0
}

Response 200:
{
  "total": 45,
  "candidates": [
    {
      "id": "uuid",
      "displayName": "Иван И.",
      "specialization": "BACKEND",
      "verifiedGrade": "MIDDLE_PLUS",
      "stack": ["Node.js", "PostgreSQL", "Docker", "Redis"],
      "experienceYears": 4,
      "testScore": 82.5,
      "achievements": [
        { "name": "Финалист ICPC 2024", "category": "competition" }
      ],
      "matchScore": 92.3,
      "matchExplanation": {
        "overall": "Сильное совпадение по стеку и грейду",
        "details": [
          { "factor": "Специализация", "match": true, "weight": 0.3 },
          { "factor": "Грейд", "match": true, "weight": 0.25 },
          { "factor": "Стек", "match": "3/3", "weight": 0.25, "overlap": ["Node.js", "PostgreSQL", "Docker"] },
          { "factor": "Достижения ФСП", "match": true, "weight": 0.1, "detail": "Финалист ICPC 2024" },
          { "factor": "Активность", "match": true, "weight": 0.1, "detail": "Тест пройден 2 дня назад" }
        ]
      },
      "lastActive": "2026-01-10T12:00:00Z"
    }
  ]
}
```

### 3.2. Semantic Search

```http
POST /api/matching/semantic
Authorization: Bearer {employer-token}
Content-Type: application/json

{
  "query": "Нужен человек который умеет строить высоконагруженные системы и знает Go или Rust",
  "limit": 10
}

Response 200:
{
  "candidates": [
    {
      "id": "uuid",
      "displayName": "Дмитрий С.",
      "similarity": 0.87,
      "reason": "Опыт разработки высоконагруженных систем, стек включает Go"
    }
  ]
}
```

### 3.3. Рекомендации для работодателя

```http
GET /api/matching/recommendations
Authorization: Bearer {employer-token}

Response 200:
{
  "recommendations": [
    {
      "reason": "Похожие работодатели приглашали похожих кандидатов",
      "candidates": [...]
    },
    {
      "reason": "Новые кандидаты в вашей категории",
      "candidates": [...]
    }
  ]
}
```

## 4. Алгоритмы

### 4.1. Scoring Formula

```typescript
// services/scoring.service.ts

interface MatchScore {
  total: number;           // 0-100
  components: {
    categoryMatch: number;  // 0-100, вес 0.30
    stackOverlap: number;   // 0-100, вес 0.25
    semanticSimilarity: number; // 0-100, вес 0.20
    gradeMatch: number;     // 0-100, вес 0.15
    achievementBoost: number; // 0-100, вес 0.10
  };
}

@Injectable()
export class MatchingScoringService {
  calculateMatch(
    candidate: CandidateProfile,
    need: EmployerNeed,
  ): MatchScore {
    const categoryMatch = this.categoryScore(candidate, need);
    const stackOverlap = this.stackScore(candidate.stack, need.stack);
    const semanticSimilarity = this.semanticScore(
      candidate.profileEmbedding,
      need.needEmbedding,
    );
    const gradeMatch = this.gradeScore(candidate, need);
    const achievementBoost = this.achievementScore(candidate);

    const total =
      categoryMatch * 0.30 +
      stackOverlap * 0.25 +
      semanticSimilarity * 0.20 +
      gradeMatch * 0.15 +
      achievementBoost * 0.10;

    return {
      total: Math.round(total * 10) / 10,
      components: {
        categoryMatch,
        stackOverlap,
        semanticSimilarity,
        gradeMatch,
        achievementBoost,
      },
    };
  }

  private categoryScore(candidate: CandidateProfile, need: EmployerNeed): number {
    if (!need.specialization) return 50; // Если не указано — нейтрально
    if (candidate.specialization === need.specialization) return 100;
    // Близкие специализации
    const related = this.getRelatedSpecializations(need.specialization);
    if (related.includes(candidate.specialization)) return 60;
    return 0;
  }

  private stackScore(candidateStack: string[], needStack: string[]): number {
    if (needStack.length === 0) return 50;
    const overlap = candidateStack.filter((s) =>
      needStack.some((n) => n.toLowerCase() === s.toLowerCase()),
    );
    return Math.round((overlap.length / needStack.length) * 100);
  }

  private semanticScore(
    candidateEmbedding: number[],
    needEmbedding: number[],
  ): number {
    if (!candidateEmbedding || !needEmbedding) return 0;
    const similarity = this.cosineSimilarity(candidateEmbedding, needEmbedding);
    return Math.round(similarity * 100);
  }

  private gradeScore(candidate: CandidateProfile, need: EmployerNeed): number {
    if (!need.grade) return 50;
    const candidateGrade = candidate.verifiedGrade || candidate.declaredGrade;
    if (!candidateGrade) return 30;

    const gradeOrder = [
      Grade.JUNIOR, Grade.MIDDLE, Grade.MIDDLE_PLUS,
      Grade.SENIOR, Grade.LEAD, Grade.PRINCIPAL,
    ];
    const candidateIdx = gradeOrder.indexOf(candidateGrade);
    const needIdx = gradeOrder.indexOf(need.grade);

    if (candidateIdx === needIdx) return 100;
    if (candidateIdx === needIdx + 1) return 80; // На уровень выше — хорошо
    if (candidateIdx === needIdx - 1) return 60; // На уровень ниже — допустимо
    if (candidateIdx > needIdx + 1) return 40;   // Слишком senior
    return 20; // Слишком junior
  }

  private achievementScore(candidate: CandidateProfile): number {
    const achievements = candidate.achievements || [];
    if (achievements.length === 0) return 0;
    const totalPoints = achievements.reduce((sum, a) => sum + a.achievement.points, 0);
    return Math.min(100, totalPoints * 10);
  }

  private cosineSimilarity(a: number[], b: number[]): number {
    let dotProduct = 0;
    let normA = 0;
    let normB = 0;
    for (let i = 0; i < a.length; i++) {
      dotProduct += a[i] * b[i];
      normA += a[i] * a[i];
      normB += b[i] * b[i];
    }
    return dotProduct / (Math.sqrt(normA) * Math.sqrt(normB));
  }
}
```

### 4.2. Semantic Embedding

```typescript
// services/embedding.service.ts

@Injectable()
export class EmbeddingService {
  private model: any;

  async onModuleInit() {
    // Используем sentence-transformers через Python subprocess
    // Или @xenova/transformers для нативного JS
    // Модель: all-MiniLM-L6-v2 (384 dimensions, fast)
  }

  /**
   * Генерирует embedding для текста
   */
  async generateEmbedding(text: string): Promise<number[]> {
    // Вариант 1: @xenova/transformers (рекомендуется для MVP)
    const { pipeline } = await import('@xenova/transformers');
    const extractor = await pipeline('feature-extraction', 'Xenova/all-MiniLM-L6-v2');
    const result = await extractor(text, { pooling: 'mean', normalize: true });
    return Array.from(result.data);

    // Вариант 2: Python subprocess (если нужна более мощная модель)
    // const { execSync } = require('child_process');
    // const result = execSync(`python -c "from sentence_transformers import SentenceTransformer; m = SentenceTransformer('all-MiniLM-L6-v2'); print(m.encode('${text}').tolist())"`);
    // return JSON.parse(result.toString());
  }

  /**
   * Генерирует embedding для профиля кандидата
   */
  async generateProfileEmbedding(profile: CandidateProfile): Promise<number[]> {
    const text = [
      profile.specialization,
      profile.verifiedGrade || profile.declaredGrade,
      profile.stack.join(' '),
      profile.bio || '',
      profile.roles.join(' '),
      profile.softSkills.join(' '),
    ].filter(Boolean).join(' ');

    return this.generateEmbedding(text);
  }

  /**
   * Генерирует embedding для потребности работодателя
   */
  async generateNeedEmbedding(need: string, stack: string[]): Promise<number[]> {
    const text = [need, stack.join(' ')].join(' ');
    return this.generateEmbedding(text);
  }
}
```

### 4.3. Explainable Results

```typescript
// services/explanation.service.ts

@Injectable()
export class ExplanationService {
  /**
   * Генерирует текстовое объяснение почему кандидат подходит
   */
  generateExplanation(
    candidate: CandidateProfile,
    need: EmployerNeed,
    score: MatchScore,
  ): MatchExplanation {
    const details: ExplanationDetail[] = [];

    // Специализация
    if (candidate.specialization === need.specialization) {
      details.push({
        factor: 'Специализация',
        match: true,
        weight: 0.3,
        explanation: `Специализация совпадает: ${candidate.specialization}`,
      });
    }

    // Стек
    const stackOverlap = candidate.stack.filter((s) =>
      need.stack.some((n) => n.toLowerCase() === s.toLowerCase()),
    );
    details.push({
      factor: 'Стек',
      match: stackOverlap.length > 0,
      weight: 0.25,
      overlap: stackOverlap,
      explanation: `Совпадение ${stackOverlap.length}/${need.stack.length}: ${stackOverlap.join(', ')}`,
    });

    // Грейд
    const candidateGrade = candidate.verifiedGrade || candidate.declaredGrade;
    if (candidateGrade === need.grade) {
      details.push({
        factor: 'Грейд',
        match: true,
        weight: 0.15,
        explanation: `Грейд подтверждён тестом: ${candidateGrade}`,
      });
    }

    // Достижения ФСП
    if (candidate.achievements?.length > 0) {
      const top = candidate.achievements[0];
      details.push({
        factor: 'Достижения ФСП',
        match: true,
        weight: 0.1,
        explanation: `${top.achievement.name} (${top.achievement.year})`,
      });
    }

    return {
      overall: this.generateOverallSummary(details, score.total),
      details,
    };
  }

  private generateOverallSummary(
    details: ExplanationDetail[],
    totalScore: number,
  ): string {
    if (totalScore >= 80) return 'Сильное совпадение по ключевым факторам';
    if (totalScore >= 60) return 'Хорошее совпадение с некоторыми расхождениями';
    if (totalScore >= 40) return 'Частичное совпадение';
    return 'Слабое совпадение';
  }
}
```

## 5. Индексация и кэширование

### 5.1. Индексация кандидатов

```typescript
// При обновлении профиля кандидата или завершении теста:
async function indexCandidate(candidateId: string) {
  const candidate = await prisma.candidateProfile.findUnique({
    where: { id: candidateId },
    include: { achievements: true },
  });

  // Генерируем embedding
  const embedding = await embeddingService.generateProfileEmbedding(candidate);

  // Сохраняем в БД
  await prisma.candidateProfile.update({
    where: { id: candidateId },
    data: { profileEmbedding: embedding },
  });

  // Инвалидируем кэш подборок
  await redis.del(`matching:cache:*`);
}
```

### 5.2. Кэширование результатов

```typescript
async function searchCandidates(need: EmployerNeed): Promise<Candidate[]> {
  const cacheKey = `matching:cache:${hashNeed(need)}`;
  
  // Проверяем кэш
  const cached = await redis.get(cacheKey);
  if (cached) return JSON.parse(cached);

  // Выполняем поиск
  const results = await performSearch(need);

  // Кэшируем на 15 минут
  await redis.setex(cacheKey, 900, JSON.stringify(results));

  return results;
}
```

## 6. Чеклист валидации

### Функциональность
- [ ] Поиск по специализации и грейду работает
- [ ] Фильтрация по стеку работает
- [ ] Semantic search возвращает релевантных кандидатов
- [ ] Scoring рассчитывается корректно
- [ ] Explanation генерируется для каждого кандидата
- [ ] Кэширование работает (Redis)
- [ ] Результаты пагинируются

### Качество подбора
- [ ] Релевантные кандидаты в топе выдачи (>70% в топ-5)
- [ ] Объяснение понятно и точно
- [ ] Semantic search находит кандидатов с близким стеком
- [ ] Ранжирование учитывает достижения ФСП

### Производительность
- [ ] Поиск < 500ms для 1000 кандидатов
- [ ] Кэш работает корректно
- [ ] Embedding генерируется < 2s

## 7. Промпт для агента

```
Ты — Backend-разработчик с опытом в ML/NLP и поисковых системах.

ТВОЯ ЗАДАЧА: Реализовать модуль matching-engine для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/ARCHITECTURE.md — общая архитектура проекта
- docs/modules/matching-engine.md — ТЗ на модуль (этот файл)
- docs/modules/database.md — схема данных (CandidateProfile, EmployerNeed, etc.)

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать backend/src/modules/matching/ со всеми файлами
2. Реализовать MatchingScoringService — расчёт match score
3. Реализовать EmbeddingService — генерация embeddings
4. Реализовать ExplanationService — объяснение результатов
5. Реализовать MatchingService — оркестратор поиска
6. Создать DTO с валидацией
7. Написать unit-тесты
8. Настроить кэширование в Redis

АЛГОРИТМЫ:
- Scoring: взвешенная сумма (category 30%, stack 25%, semantic 20%, grade 15%, achievements 10%)
- Embedding: @xenova/transformers, модель all-MiniLM-L6-v2 (384 dim)
- Cosine similarity для сравнения векторов

СТЕК:
- NestJS 10
- Prisma 5
- @xenova/transformers
- Redis

ВАЛИДАЦИЯ:
- Создай 10 тестовых кандидатов с разными стеками
- Создай 3 тестовых потребности работодателя
- Проверь что релевантные кандидаты в топе
- Проверь что explanation генерируется корректно
- Проверь что кэш работает

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Примеры подборки для 3 разных потребностей
- Метрики качества (precision@5, recall@10)
- Замечания по алгоритмам
```