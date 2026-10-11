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

### 3.4. Маршруты V2 (все — POST, роль EMPLOYER)

| Маршрут | Назначение |
|---------|-----------|
| POST /api/matching/v2/search | ML query understanding + расширенный scoring (7 компонент) + опциональный re-ranking |
| POST /api/matching/v2/semantic | Семантический поиск с разбором запроса |
| POST /api/matching/v2/need | Создание потребности + подборка (query understanding + SearchV2) |

Ответ V2-поиска дополнительно содержит `queryUnderstanding` (parsedSpecialization,
parsedSeniority, parsedSkills, embeddingProvider) и `timing` (retrieval/scoring/reranking/total, мс).
POST /api/employers/need (модуль employers) делегирует тот же use-case — matching.CreateNeed.

## 4. Алгоритмы

### 4.1. Scoring Formula

```go
// backend/internal/modules/matching/scoring.go (V1)

// MatchScore — итоговый счёт V1.
type MatchScore struct {
	Total      float64         `json:"total"`      // 0-100
	Components MatchComponents `json:"components"`
}

// MatchComponents — компоненты счёта V1 (веса: category 0.30, stack 0.25,
// semantic 0.20, grade 0.15, achievement 0.10).
type MatchComponents struct {
	CategoryMatch      float64 `json:"categoryMatch"`
	StackOverlap       float64 `json:"stackOverlap"`
	SemanticSimilarity float64 `json:"semanticSimilarity"`
	GradeMatch         float64 `json:"gradeMatch"`
	AchievementBoost   float64 `json:"achievementBoost"`
}

// scoringService — расчёт match score V1.
type scoringService struct{}

// calculateMatch — итоговый счёт: Σ component × weight, округление до 0.1.
func (s scoringService) calculateMatch(candidate ScoreableCandidate, need Need, semanticSimilarity float64) MatchScore {
	components := MatchComponents{
		CategoryMatch:      s.categoryScore(candidate, need),
		StackOverlap:       s.stackScore(candidate.Stack, need.Stack),
		SemanticSimilarity: s.clamp100(semanticSimilarity * 100),
		GradeMatch:         s.gradeScore(candidate, need),
		AchievementBoost:   s.achievementScore(candidate.AchievementPoints),
	}

	total := components.CategoryMatch*WEIGHTS.Category +
		components.StackOverlap*WEIGHTS.Stack +
		components.SemanticSimilarity*WEIGHTS.Semantic +
		components.GradeMatch*WEIGHTS.Grade +
		components.AchievementBoost*WEIGHTS.Achievement

	return MatchScore{Total: jsRound(total*10) / 10, Components: components}
}

// categoryScore — совпадение специализации: 50 (не указана) / 100 / 60 (смежная) / 30 / 0.
func (s scoringService) categoryScore(candidate ScoreableCandidate, need Need) float64 {
	if need.Specialization == "" {
		return 50 // Если не указано — нейтрально
	}
	if candidate.Specialization == "" {
		return 30
	}
	if candidate.Specialization == need.Specialization {
		return 100
	}
	// Близкие специализации (RELATED)
	if containsSpec(RELATED[string(need.Specialization)], candidate.Specialization) {
		return 60
	}
	return 0
}

// stackScore — доля пересечения стеков после нормализации названий
// («Node.js» = «Node JS» = «node-js» = «nodejs», «C++» ≠ «C»).
func (s scoringService) stackScore(candidateStack, needStack []string) float64 {
	if len(needStack) == 0 {
		return 50
	}
	if len(candidateStack) == 0 {
		return 0
	}
	need := make([]string, 0, len(needStack))
	for _, s := range needStack {
		need = append(need, normalizeTech(s))
	}
	have := map[string]bool{}
	for _, s := range candidateStack {
		have[normalizeTech(s)] = true
	}
	overlap := 0
	for _, s := range need {
		if have[s] {
			overlap++
		}
	}
	return s.clamp100(jsRound(float64(overlap) / float64(len(need)) * 100))
}

// gradeScore — ±1 грейд (100/80/60), дальше 40/20.
func (s scoringService) gradeScore(candidate ScoreableCandidate, need Need) float64 {
	if need.Grade == "" {
		return 50
	}
	candidateGrade := candidate.VerifiedGrade
	if candidateGrade == "" {
		candidateGrade = candidate.DeclaredGrade
	}
	if candidateGrade == "" {
		return 30
	}

	candidateIdx := gradeIndexOf(candidateGrade)
	needIdx := gradeIndexOf(need.Grade)

	switch {
	case candidateIdx == needIdx:
		return 100
	case candidateIdx == needIdx+1:
		return 80 // На уровень выше — хорошо
	case candidateIdx == needIdx-1:
		return 60 // На уровень ниже — допустимо
	case candidateIdx > needIdx+1:
		return 40 // Слишком senior
	default:
		return 20 // Слишком junior
	}
}

// achievementScore — очки достижений ×10, cap 100.
func (scoringService) achievementScore(totalPoints int) float64 {
	if totalPoints <= 0 {
		return 0
	}
	return math.Min(100, float64(totalPoints)*10)
}

// cosineSimilarity — косинусная схожесть (tfidf.go), с защитой от NaN.
func cosineSimilarity(a, b []float64) float64 { /* ... */ }
```

> V2-скоринг (`scoring-v2.go`): 7 компонент (специализация 0.18, стек 0.15, семантика 0.20,
> грейд 0.12, достижения 0.08, опыт 0.12, тест 0.15) + аддитивный rerank-буст до +10 баллов
> (M3: fan-out HTTP-вызовов при доступном ML — воспроизведён намеренно, `// BUG-REF: M3`).

### 4.2. Semantic Embedding

```go
// backend/internal/modules/matching/embedding.go
// Провайдер выбирается по EMBEDDING_PROVIDER (auto | tfidf | transformers).
// ONNX/transformers в Go-сборке недоступны → немедленная ПОСТОЯННАЯ
// деградация на TF-IDF; провайдер всегда "tfidf" (CurrentProvider()).

func newEmbeddingService() *embeddingService {
	return &embeddingService{
		tfidf:    newTfidfVectorizer(),
		provider: providerTfidf,
	}
}

// initTransformers — паритет с TS, но импорт пакета всегда неуспешен.
func (e *embeddingService) initTransformers(required bool) {
	// try { await import('@xenova/transformers') } catch → warn + return
	log.Printf("[Embedding] Пакет @xenova/transformers недоступен: ONNX-рантайм отсутствует в Go-сборке. Работаем на TF-IDF.")
	if required {
		log.Printf("[Embedding] Ни одна модель не загрузилась — включён лексический fallback (TF-IDF).")
	}
}

// embed — 384 finite-числа, никогда не бросает исключение.
func (e *embeddingService) embed(text string) []float64 {
	return e.tfidf.embed(text)
}
```

```go
// backend/internal/modules/matching/tfidf.go — детерминированный TF-IDF 384-dim.
// fnv1a — 32-битный FNV-1a по UTF-16 code units (паритет с JS charCodeAt + Math.imul).
// tokenize — lowercase, ё→е, символы [a-zа-я0-9+#], длина > 1, + биграммы w1_w2.
// embed — sublinear TF (1 + ln count) × IDF → signed feature hashing
// (index = hash % 384, знак — старший бит) → L2-нормализация.

func (v *tfidfVectorizer) embed(text string) []float64 {
	tokens := tokenize(text)
	// ... TF по токенам, вес (1 + ln count) × idfOf(token)
	hash := fnv1a(token)
	index := int(hash % EMBEDDING_DIM) // 384
	sign := 1.0
	if hash&0x80000000 != 0 {
		sign = -1
	}
	vector[index] += sign * weight
	return normalize(vector) // L2; при нулевой норме — unitVector()
}
```

**Роль ml-service (Python/FastAPI, не менялся)**: для V2-пайплайна клиент
`ml-client.go` ходит в ml-service по HTTP (`ML_SERVICE_URL`, по умолчанию
`http://ml-service:8000`): `/embed`, `/embed/batch`, `/parse-query`, `/rerank`, `/ner`.
При недоступности ML — локальные фолбэки (TF-IDF `tfidf-local`, `localParseQuery`, `localNer`).
Размерность векторов — 384 (`internal/store/vector.go`, `EmbeddingDim`), запись в pgvector
через `VectorRepository`; офлайн-предзапись — `cmd/seed-embeddings` (порт prisma/embeddings.ts).

**Генерация текстовых представлений** (`indexing.go → buildText`): специализация +
грейд (verified || declared) + stack + roles + softSkills + город + «N лет опыта» + bio +
названия достижений. Для потребности — `description + stack` (fallback: спец+грейд+стек).

### 4.3. Explainable Results

```go
// backend/internal/modules/matching/explanation.go (RU-строки дословно из TS)

// ExplanationDetail — деталь объяснения счёта.
type ExplanationDetail struct {
	Factor      string    `json:"factor"`
	Match       any       `json:"match"` // boolean | string
	Weight      float64   `json:"weight"`
	Explanation string    `json:"explanation"`
	Overlap     *[]string `json:"overlap,omitempty"` // всегда у детали «Стек» (в т.ч. [])
	Detail      *string   `json:"detail,omitempty"`
}

// MatchExplanation — объяснение счёта {overall, details}.
type MatchExplanation struct {
	Overall string              `json:"overall"`
	Details []ExplanationDetail `json:"details"`
}

// generate — детали по всем весомым факторам (порядок как в TS):
// «Специализация» (не указана / Совпадает / Кандидат — X, требуется Y),
// «Стек» (Совпадение k/n: ...), «Грейд» (подтверждён тестом / заявлен кандатом /
// допустимо / Избыточно высокий / Грейд ниже запросленного),
// «Семантическое совпадение» (match = score ≥ 50, detail = "N%"),
// «Достижения ФСП» (до 2 названий), «Активность» (тест пройден N дн. назад).
func (s explanationService) generate(candidate ExplanationCandidate, need Need, score MatchScore) MatchExplanation {
	details := []ExplanationDetail{}
	// ... (как в коде: Специализация → Стек → Грейд → Семантика → Достижения ФСП → Активность)
	return MatchExplanation{Overall: s.summary(score.Total), Details: details}
}

// summary — итоговая строка по суммарному счёту.
func (explanationService) summary(total float64) string {
	if total >= 80 {
		return "Сильное совпадение по ключевым факторам"
	}
	if total >= 60 {
		return "Хорошее совпадение с некоторыми расхождениями"
	}
	if total >= 40 {
		return "Частичное совпадение"
	}
	return "Слабое совпадение"
}
```

## 5. Индексация и кэширование

### 5.1. Индексация кандидатов

```go
// backend/internal/modules/matching/indexing.go

// indexCandidate — полная индексация одного кандидата
// (вызывается при обновлении профиля, завершении теста, подключении ФСП).
func (i *indexingService) indexCandidate(ctx context.Context, candidateID string) error {
	if err := i.ensureCorpus(ctx); err != nil {
		return err
	}
	candidates, err := loadCandidatesByID(ctx, i.db, []string{candidateID})
	if err != nil || len(candidates) == 0 {
		return nil // findUnique → null
	}
	profile := candidates[0]
	text := i.buildText(profile, achievementNames(profile))
	vector := i.embedding.embed(text) // 384-dim TF-IDF
	return i.vectors.SetCandidateVector(ctx, profile.ID, vector) // pgvector
}
```

- `indexBatch(ids)` — пакетная индексация; из поиска лениво индексируются кандидаты
  без вектора, батчами **по 25** за запрос (не блокирует ответ).
- `reindexAll()` — полная переиндексация (до **5000** профилей) с переобучением TF-IDF корпуса;
  стартовая — фоном через `time.AfterFunc(2 * time.Second)` после запуска сервера.
- Инвалидация кэша подборок — `InvalidateCache()` = `delByPattern("matching:cache:*")`
  (`// BUG-REF: M8` — общий неймспейс: инвалидация стирает кэш обеих версий).

### 5.2. Кэширование результатов

```go
// backend/internal/modules/matching/service.go

// Кэш — поверх Redis (go-redis); nil → nopCache (молча без кэша).
// Ключ: matching:cache:{search|search-v2}:{sha1(payload + [limit, offset])}
// TTL: matching.cacheTtlSeconds || 900 (15 минут).

func (s *Service) Search(ctx context.Context, dto SearchCandidatesDTO) (map[string]any, error) {
	cacheKey := s.cacheKey("search", dto, limit, offset)

	// Проверяем кэш
	if cached := s.cache.Get(ctx, cacheKey); cached != nil {
		/* ... распаковка JSON ... */
	}

	/* // Выполняем поиск */

	// Кэшируем на 15 минут
	ttl := s.cacheTTL()
	s.cache.Set(ctx, cacheKey, jsJSON(result), ttl)

	return result, nil
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
- docs/modules/database.md — схема данных (candidate_profiles, employer_needs, etc.)

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать backend/internal/modules/matching/ со всеми файлами
2. Реализовать scoring.go / scoring-v2.go — расчёт match score
3. Реализовать embedding.go + tfidf.go — детерминированные embeddings
4. Реализовать explanation.go — объяснение результатов
5. Реализовать service.go / service-v2.go — оркестраторы поиска
6. Создать dto.go с валидацией
7. Написать unit-тесты (tfidf_test.go, scoring-v2_test.go, explanation_test.go)
8. Настроить кэширование в Redis (go-redis)

АЛГОРИТМЫ:
- Scoring: взвешенная сумма (category 30%, stack 25%, semantic 20%, grade 15%, achievements 10%)
- Embedding: детерминированный TF-IDF 384-dim (fnv1a по UTF-16, биграммы);
  ONNX/transformers в Go-сборке недоступны → провайдер всегда "tfidf",
  ml-service (FastAPI) используется по HTTP для V2 с локальными фолбэками
- Cosine similarity для сравнения векторов

СТЕК:
- Go 1.25
- gin
- pgx/v5 (сырой SQL, pgvector)

ВАЛИДАЦИЯ:
- Создай 10 тестовых кандидатов с разными стеками
- Создай 3 тестовых потребности работодателя
- Проверь что релевантные кандидаты в топе
- Проверь что explanation генерируется корректно
- Проверь что кэш работает
- Прогон: go test ./internal/modules/matching/...
  (toBeCloseTo(x, 1) из TS-спеков → допуск 0.05)

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Примеры подборки для 3 разных потребностей
- Метрики качества (precision@5, recall@10)
- Замечания по алгоритмам
```