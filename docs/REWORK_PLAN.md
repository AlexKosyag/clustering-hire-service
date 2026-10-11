# План переработки: Matching Engine V2 + Testing Engine V2 + Frontend Redo

> Ветка: `feat/rework`
> Философия: максимальный акцент на семантическом подборе, полная переработка тестирования
> **Статус**: план выполнен. Бэкенд в этой ветке переписан на **Go** (точный порт
> NestJS-версии, `backend/`); актуальные схемы и команды — в `docs/ARCHITECTURE.md`
> и `docs/DEVELOPMENT.md`. Раздел 7 приведён к фактическим командам валидации.

---

## 1. Стратегия параллелизации

### 1.1. Фазы и зависимости

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        ФАЗА 0: FOUNDATION (3ч)                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                     │
│  │ Design       │  │ Shared       │  │ Python       │                     │
│  │ System       │  │ Types        │  │ ML Service   │                     │
│  │ (tokens,     │  │ (API         │  │ (FastAPI +   │                     │
│  │  components) │  │  contracts)  │  │  sentence-   │                     │
│  │              │  │              │  │  transformers│                     │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘                     │
└─────────┼──────────────────┼──────────────────┼─────────────────────────────┘
          │                  │                  │
          ▼                  ▼                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        ФАЗА 1: CORE ENGINES (8ч)                           │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    MATCHING ENGINE V2                                │   │
│  │  (максимум усилий — главный критерий 35%)                          │   │
│  │                                                                     │   │
│  │  Agent-M1: Query Understanding + NER        Agent-M2: Hybrid       │   │
│  │  (2ч)                                       Search (2ч)           │   │
│  │       │                                         │                  │   │
│  │       ▼                                         ▼                  │   │
│  │  Agent-M3: Cross-Encoder Re-Ranking         Agent-M4: Learning    │   │
│  │  (3ч)                                       from Feedback (2ч)   │   │
│  │       │                                         │                  │   │
│  │       ▼                                         ▼                  │   │
│  │  Agent-M5: Explainability + SHAP-like       Agent-M6: A/B +      │   │
│  │  (2ч)                                       Evaluation (2ч)     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    TESTING ENGINE V2                                 │   │
│  │  (второй по важности — критерий 25%)                               │   │
│  │                                                                     │   │
│  │  Agent-T1: Code Sandbox                   Agent-T2: LLM           │   │
│  │  (3ч)                                     Evaluation (2ч)        │   │
│  │       │                                         │                  │   │
│  │       ▼                                         ▼                  │   │
│  │  Agent-T3: Behavioral Anti-Cheat          Agent-T4: Multi-       │   │
│  │  (2ч)                                     Format Tasks (3ч)      │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        ФАЗА 2: FRONTEND (8ч)                               │
│                                                                             │
│  Agent-F1: Candidate UI                          Agent-F2: Employer UI    │
│  - Dashboard                                      - Dashboard             │
│  - Profile (edit + achievements)                  - Search (new UX)       │
│  - Testing (new flow)                             - Candidates (cards)    │
│  - Invitations                                    - Vacancies             │
│  - Settings                                       - Analytics             │
│  (4ч, параллельно)                                (4ч, параллельно)       │
└─────────────────────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        ФАЗА 3: INTEGRATION (4ч)                            │
│                                                                             │
│  Agent-I1: E2E + Performance               Agent-I2: Docs + Demo         │
│  - Playwright тесты                          - Обновить документацию     │
│  - Load testing                              - Demo сценарии             │
│  - Validation report                         - Presentation slides       │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2. Матрица параллелизации (Gantt)

```
Время →    0h   1h   2h   3h   4h   5h   6h   7h   8h   9h  10h  11h  12h  13h  14h  15h  16h  17h  18h  19h  20h
           │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │
F0: Design ████████████
F0: Types       ████████
F0: Python           ████████████
           │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │
F1: M1-QA                    ██████████
F1: M2-Hybrid                    ██████████
F1: M3-Rerank                         ████████████████
F1: M4-Learning                           ██████████████
F1: M5-Explain                                   ██████████
F1: M6-AB                                          ██████████
F1: T1-Sandbox                    ████████████████
F1: T2-LLM                             ██████████████
F1: T3-Behavioral                            ██████████████
F1: T4-MultiFormat                       ████████████████████
           │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │
F2: FE-Cand                                                              ████████████████
F2: FE-Emp                                                               ████████████████
           │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │    │
F3: Integ                                                                                   ████████████
```

**Общее время**: ~20 часов с полной параллелизацией
**Критический путь**: Python ML → Matching V2 → Frontend → Integration

### 1.3. Распределение агентов

| Агент | Зона ответственности | Фаза | Параллельно с |
|-------|---------------------|------|---------------|
| **Agent-DS** | Design System + Shared Components | 0 | Agent-Types, Agent-ML |
| **Agent-Types** | Shared TypeScript types + API contracts | 0 | Agent-DS, Agent-ML |
| **Agent-ML** | Python ML Service (FastAPI) | 0 | Agent-DS, Agent-Types |
| **Agent-M1** | Query Understanding + NER | 1 | Agent-M2, T1-T4 |
| **Agent-M2** | Hybrid Search (BM25 + Dense) | 1 | Agent-M1, T1-T4 |
| **Agent-M3** | Cross-Encoder Re-Ranking | 1 | Agent-M4, T1-T4 |
| **Agent-M4** | Learning from Feedback | 1 | Agent-M3, T1-T4 |
| **Agent-M5** | Explainability + Feature Attribution | 1 | Agent-M6, T1-T4 |
| **Agent-M6** | A/B Testing + Evaluation | 1 | Agent-M5, T1-T4 |
| **Agent-T1** | Code Sandbox (Docker) | 1 | Agent-M1-M6, T2-T4 |
| **Agent-T2** | LLM Evaluation | 1 | Agent-M1-M6, T1, T3-T4 |
| **Agent-T3** | Behavioral Anti-Cheat | 1 | Agent-M1-M6, T1-T2, T4 |
| **Agent-T4** | Multi-Format Tasks | 1 | Agent-M1-M6, T1-T3 |
| **Agent-F1** | Frontend Candidate | 2 | Agent-F2 |
| **Agent-F2** | Frontend Employer | 2 | Agent-F1 |
| **Agent-I1** | E2E + Performance | 3 | Agent-I2 |
| **Agent-I2** | Docs + Demo | 3 | Agent-I1 |

---

## 2. Matching Engine V2 — Философия

> **Цель**: Превратить подбор кандидатов из "фильтра по тегам" в "интеллектуального рекрутера"

### 2.1. Почему текущий matching недостаточен

```
Текущее состояние (V1):
├── TF-IDF fallback — bag-of-words, нет семантики
│   "Node.js разработчик" ≈ "JavaScript developer" (нет!)
│   "PostgreSQL DBA" ≈ "MySQL DBA" (частично)
│
├── Transformers могут не загрузиться (300MB модель)
│   В демо: медленно или TF-IDF fallback
│
├── Веса захардкожены: [0.3, 0.25, 0.2, 0.15, 0.1]
│   Каждый работодатель ценит разное — веса должны учиться
│
├── Нет re-ranking
│   Первый retrieval → сразу выдача
│   Нет cross-encoder для точного сравнения query ↔ candidate
│
├── Нет learning from feedback
│   Принял/отклонил приглашение → ничего не меняется
│
└── Объяснение — статичные правила
    Нет feature attribution (SHAP-like)
```

### 2.2. Целевая архитектура V2

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         MATCHING ENGINE V2                                  │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  LAYER 1: Query Understanding                                       │   │
│  │                                                                     │   │
│  │  Вход: "Ищем senior backend разработчика с опытом в микросервисах   │   │
│  │         и очередях, обязательно знание Go и Kubernetes"             │   │
│  │                                                                     │   │
│  │  Выход: {                                                            │   │
│  │    intent: "hire",                                                   │   │
│  │    specialization: "BACKEND",                                        │   │
│  │    seniority: "SENIOR",                                              │   │
│  │    required_skills: ["Go", "Kubernetes", "message-queues"],         │   │
│  │    preferred_skills: ["microservices", "gRPC"],                     │   │
│  │    context: "high-load backend systems",                            │   │
│  │    embedding: [0.12, -0.34, ...]  // 768-dim                       │   │
│  │  }                                                                  │   │
│  │                                                                     │   │
│  │  Компоненты:                                                         │   │
│  │  • NER (spaCy/HF) — извлечение сущностей                           │   │
│  │  • Intent classifier — классификация намерения                      │   │
│  │  • Skill extractor — извлечение навыков (включая синонимы)          │   │
│  │  • Embedding generation — 768-dim vector                            │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                   │                                         │
│                                   ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  LAYER 2: Hybrid Retrieval                                          │   │
│  │                                                                     │   │
│  │  Не просто "или" или "и" — комбинация обоих методов:               │   │
│  │                                                                     │   │
│  │  ┌──────────────┐     ┌──────────────┐                             │   │
│  │  │ Dense        │     │ Sparse       │                             │   │
│  │  │ (HNSW index) │     │ (BM25/TF-IDF)│                             │   │
│  │  │ 768-dim      │     │ vocabulary   │                             │   │
│  │  │ vectors      │     │              │                             │   │
│  │  └──────┬───────┘     └──────┬───────┘                             │   │
│  │         │                    │                                      │   │
│  │         ▼                    ▼                                      │   │
│  │  ┌─────────────────────────────────────────────────────────────┐   │   │
│  │  │  Reciprocal Rank Fusion (RRF)                               │   │   │
│  │  │  score = Σ 1/(k + rank_i)                                   │   │   │
│  │  │  k=60 (стандартное значение)                                 │   │   │
│  │  └─────────────────────────────────────────────────────────────┘   │   │
│  │                                                                     │   │
│  │  Плюс:                                                              │   │
│  │  • Hard filters (specialization, grade) — до retrieval             │   │
│  │  • Metadata boost (achievements, test score, recency)             │   │
│  │  • Diversity (MMR — Maximal Marginal Relevance)                   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                   │                                         │
│                                   ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  LAYER 3: Cross-Encoder Re-Ranking                                 │   │
│  │                                                                     │   │
│  │  Retrieval вернул 100 кандидатов → re-ranker сортирует топ-20     │   │
│  │                                                                     │   │
│  │  Модель: cross-encoder/ms-marco-MiniLM-L-6-v2                     │   │
│  │  (или paraphrase-multilingual-mpnet-base-v2 для русского)         │   │
│  │                                                                     │   │
│  │  Вход: (query_text, candidate_text) пары                           │   │
│  │  Выход: relevance score 0-1 для каждой пары                        │   │
│  │                                                                     │   │
│  │  Преимущество: cross-encoder видит query и документ вместе        │   │
│  │  (много лучше bi-encoder для точного ranking)                      │   │
│  │                                                                     │   │
│  │  Оптимизация:                                                       │   │
│  │  • Batch processing (GPU batching)                                 │   │
│  │  • Early exit (если score < порога — пропускаем)                  │   │
│  │  • Cache embeddings (TTL 1ч)                                       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                   │                                         │
│                                   ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  LAYER 4: Learning from Feedback                                   │   │
│  │                                                                     │   │
│  │  Каждое приглашение → фидбек:                                       │   │
│  │  • ACCEPTED → positive example                                      │   │
│  │  • REJECTED → negative example                                      │   │
│  │  • VIEWED (no action) → weak negative                               │   │
│  │                                                                     │   │
│  │  Обучение:                                                          │   │
│  │  • Per-employer Logistic Regression на features                    │   │
│  │  • Features: [category_match, stack_overlap, semantic, grade,      │   │
│  │              achievement, test_score, experience, recency]         │   │
│  │  • Обновление: online learning (каждые 10 фидбеков)               │   │
│  │                                                                     │   │
│  │  Результат: персонализированные веса для каждого работодателя      │   │
│  │  "Компания X ценит опыт > стек"                                    │   │
│  │  "Компания Y ценит достижения > опыт"                              │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                   │                                         │
│                                   ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  LAYER 5: Explainability                                            │   │
│  │                                                                     │   │
│  │  Не просто "score = 85" — а ПОЧЕМУ 85:                            │   │
│  │                                                                     │   │
│  │  SHAP-like feature attribution:                                     │   │
│  │  ┌──────────────────────────────────────────────────────────────┐  │   │
│  │  │  Итоговый score: 85/100                                     │  │   │
│  │  │                                                              │  │   │
│  │  │  +15.0  Специализация совпадает (BACKEND)                   │  │   │
│  │  │  +20.0  Стек: 4/5 совпадений (Go, K8s, gRPC, PostgreSQL)    │  │   │
│  │  │  +18.0  Семантика: описание близко (cos=0.82)               │  │   │
│  │  │  +12.0  Грейд: SENIOR ✓ (подтверждён тестом)                │  │   │
│  │  │  +10.0  Достижения ФСП: ICPC Finalist                       │  │   │
│  │  │  + 5.0  Активность: тест 5 дней назад                       │  │   │
│  │  │  - 5.0  Опыт: 3 года (просили 5+)                          │  │   │
│  │  │                                                              │  │   │
│  │  │  Top-3 совпадений: Go, Kubernetes, microservices            │  │   │
│  │  └──────────────────────────────────────────────────────────────┘  │   │
│  │                                                                     │   │
│  │  Метод: perturbation-based attribution                             │   │
│  │  1. Убираем каждый feature → measure score delta                   │   │
│  │  2. Δ = full_score - score_without_feature                         │   │
│  │  3. Нормализуем → проценты                                         │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.3. Python ML Service (ключевой компонент)

> **Зачем Python**: transformers, sentence-transformers, spaCy — всё в Python生态

```
┌─────────────────────────────────────────────────────────────────┐
│                    PYTHON ML SERVICE                            │
│                    (FastAPI + uvicorn)                          │
│                                                                 │
│  Endpoints:                                                     │
│  POST /embed          → generate embedding (768-dim)           │
│  POST /embed/batch    → batch embeddings                       │
│  POST /ner            → named entity recognition               │
│  POST /rerank         → cross-encoder re-ranking               │
│  POST /parse-query    → full query understanding               │
│  GET  /health         → model status                           │
│                                                                 │
│  Модели (загружаются при старте):                               │
│  • sentence-transformers/paraphrase-multilingual-mpnet-base-v2 │
│    (768-dim, русский + английский)                             │
│  • cross-encoder/ms-marco-MiniLM-L-6-v2                        │
│    (re-ranking)                                                │
│  • spaCy/ru_core_news_sm                                       │
│    (NER для русского)                                          │
│                                                                 │
│  Оптимизации:                                                   │
│  • ONNX runtime (быстрее на CPU)                               │
│  • Quantization (int8)                                         │
│  • Batch processing                                            │
│  • Model caching (LRU)                                         │
└─────────────────────────────────────────────────────────────────┘
```

### 2.4. HNSW Index для быстрого поиска

```
Текущее: pgvector с brute-force (O(n) по всем кандидатам)
Целевое: HNSW (Hierarchical Navigable Small World) — O(log n)

Реализация:
1. Встроенный HNSW index в pgvector (версия 0.5+)
   CREATE INDEX ON candidate_profiles 
   USING hnsw (profile_embedding vector_cosine_ops)
   WITH (m = 16, ef_construction = 200);

2. Или: hnswlib (Python) для in-memory поиска
   Быстрее, но нужно синхронизировать с БД
   
3. Стратегия: pgvector HNSW как основной
   + кэширование топ-K результатов в Redis
```

### 2.5. Формула scoring V2

```
V1: score = 0.3*category + 0.25*stack + 0.2*semantic + 0.15*grade + 0.1*achievement

V2: score = Σ w_i * f_i  (веса обучаются per-employer)

Features (расширенный набор):
f1: specialization_match      (exact / related / none)
f2: stack_jaccard             (Jaccard similarity стеков)
f3: stack_importance_weighted (важные навыки весят больше)
f4: semantic_cosine           (cosine similarity embeddings)
f5: semantic_cross_encoder    (re-ranker score)
f6: grade_match               (exact ±1 / far off)
f7: experience_relevance     (годы vs требование)
f8: achievement_score        (очки ФСП)
f9: test_score               (результат теста)
f10: recency                 (дней с последней активности)
f11: response_rate           (история ответов на приглашения)

Initial weights (без обучения):
w = [0.20, 0.15, 0.10, 0.15, 0.15, 0.10, 0.05, 0.05, 0.03, 0.02, 0.00]
```

### 2.6. Критерии успеха Matching V2

| Метрика | V1 (текущее) | V2 (цель) |
|---------|--------------|-----------|
| Precision@5 (простые запросы) | 100% | ≥ 95% |
| Precision@5 (сложные запросы) | ~70% | ≥ 85% |
| Recall@10 | ~80% | ≥ 90% |
| MRR | 1.00 | ≥ 0.90 |
| nDCG@10 | 1.000 | ≥ 0.95 |
| Query understanding accuracy | N/A | ≥ 90% |
| Re-ranking improvement | N/A | +5-10% precision |
| Learning convergence | N/A | < 100 фидбеков |
| Search latency (top-100) | 59ms | < 200ms |
| Re-ranking latency (top-20) | N/A | < 500ms |

---

## 3. Testing Engine V2 — Философия

> **Цель**: Перейти от "теста с 4 вариантами" к "настоящему измерению квалификации"

### 3.1. Почему текущий testing недостаточен

```
Текущее состояние (V1):
├── 80% вопросов — multiple choice (4 варианта)
│   ✓ Можно угадать (25% шанс)
│   ✓ Можно найти ответы в интернете
│   ✓ Не измеряет реальное умение писать код
│   ✗ Не измеряет понимание концепций
│
├── Anti-cheat слабый
│   • Только анализ времени
│   • Нет детекции copy-paste
│   • Нет детекции переключения вкладок
│   • Нет behavioral analysis
│
├── Оценка — только правильность
│   • Нет оценки качества кода
│   • Нет оценки альтернативных решений
│   • Нет оценки explanation
│
└── Нет реальных задач
    • Нет "напиши функцию"
    • Нет "исправь баг"
    • Нет "проведи code review"
    • Нет "спроектируй систему"
```

### 3.2. Целевая архитектура V2

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         TESTING ENGINE V2                                   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  TASK TYPE 1: Code Execution Sandbox                                 │   │
│  │                                                                     │   │
│  │  Задание: "Напишите функцию debounce(fn, delay)"                    │   │
│  │                                                                     │   │
│  │  Кандидат пишет код → выполняется в изолированной среде:           │   │
│  │                                                                     │   │
│  │  ┌──────────────────────────────────────────────────────────────┐  │   │
│  │  │  Docker Sandbox (изолированный)                               │  │   │
│  │  │  • Ограничение: CPU 0.5 core, RAM 256MB, Timeout 10s        │  │   │
│  │  │  • Нет сети                                                   │  │   │
│  │  │  • Read-only filesystem (кроме /tmp)                          │  │   │
│  │  │  • Случайные тесты (hidden tests)                             │  │   │
│  │  │                                                              │  │   │
│  │  │  Кандидат видит: 2 sample tests                              │  │   │
│  │  │  Система прогоняет: 10 hidden tests                          │  │   │
│  │  │                                                              │  │   │
│  │  │  Score = passed_tests / total_tests * 100                     │  │   │
│  │  └──────────────────────────────────────────────────────────────┘  │   │
│  │                                                                     │   │
│  │  Языки: JavaScript, Python, Go (для MVP)                           │   │
│  │  Метрики: pass rate, execution time, code complexity                │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  TASK TYPE 2: LLM-Evaluated Open Response                          │   │
│  │                                                                     │   │
│  │  Задание: "Объясните, как работает garbage collection в Go"        │   │
│  │                                                                     │   │
│  │  Кандидат пишет текст → LLM оценивает по критериям:               │   │
│  │                                                                     │   │
│  │  ┌──────────────────────────────────────────────────────────────┐  │   │
│  │  │  LLM Evaluation Prompt:                                       │  │   │
│  │  │                                                              │  │   │
│  │  │  "Оцени ответ кандидата по шкале 0-100.                     │  │   │
│  │  │   Критерии:                                                   │  │   │
│  │  │   - Точность технических утверждений (40%)                   │  │   │
│  │  │   - Полнота (30%)                                             │  │   │
│  │  │   - Структурированность (20%)                                 │  │   │
│  │  │   - Примеры кода (10%)                                        │  │   │
│  │  │                                                              │  │   │
│  │  │   Верни JSON: {score, breakdown, strengths, weaknesses}"      │  │   │
│  │  │                                                              │  │   │
│  │  │  Модель: GPT-4o-mini / Claude 3 Haiku (быстро + дёшево)    │  │   │
│  │  └──────────────────────────────────────────────────────────────┘  │   │
│  │                                                                     │   │
│  │  Fallback: если LLM недоступен → keyword matching                  │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  TASK TYPE 3: Code Review                                            │   │
│  │                                                                     │   │
│  │  Задание: "Найдите и исправьте баги в этом коде"                   │   │
│  │                                                                     │   │
│  │  Код с багами:                                                      │   │
│  │  ┌──────────────────────────────────────────────────────────────┐  │   │
│  │  │  async function getUser(id) {                                 │  │   │
│  │  │    const user = await db.query('SELECT * FROM users WHERE id = ?' │  │   │
│  │  │    id);  // BUG: SQL injection + нет await                    │  │   │
│  │  │    return user;                                                │  │   │
│  │  │  }                                                            │  │   │
│  │  └──────────────────────────────────────────────────────────────┘  │   │
│  │                                                                     │   │
│  │  Оценка:                                                            │   │
│  │  • Найдено багов: 2/2                                              │   │
│  │  • Качество исправлений (LLM)                                      │   │
│  │  • Объяснение (LLM)                                                │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  TASK TYPE 4: System Design                                         │   │
│  │                                                                     │   │
│  │  Задание: "Спроектируйте систему rate limiting для API"            │   │
│  │                                                                     │   │
│  │  Формат: текстовое описание + схема (ASCII/Mermaid)                 │   │
│  │                                                                     │   │
│  │  Оценка (LLM):                                                      │   │
│  │  • Выбранные алгоритмы (token bucket, sliding window, etc.)        │   │
│  │  • Учёт распределённости                                           │   │
│  │  • Хранение состояния (Redis, etc.)                                │   │
│  │  • Масштабируемость                                                │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  ANTI-CHEAT V2                                                      │   │
│  │                                                                     │   │
│  │  1. Behavioral Analysis:                                             │   │
│  │     • Keystroke dynamics (паттерн нажатий)                         │   │
│  │     • Copy-paste detection (onCopy, onPaste events)                │   │
│  │     • Tab visibility (visibilitychange API)                         │   │
│  │     • Idle time detection                                           │   │
│  │                                                                     │   │
│  │  2. Code Analysis:                                                   │   │
│  │     • Стиль кода vs прошлые работы (если есть)                     │   │
│  │     • Импорты не из задания (подозрительно)                        │   │
│  │     • Отладочные statements (console.log)                          │   │
│  │                                                                     │   │
│  │  3. Time Analysis (улучшенный):                                     │   │
│  │     • Per-question timing vs difficulty                            │   │
│  │     • Слишком быстрые ответы на сложных вопросах                  │   │
│  │     • Подозрительные паузы                                          │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.3. Распределение типов заданий

```
Тест для {SPECIALIZATION} {GRADE}:

Junior (JUNIOR):
├── 5x Multiple Choice (basics)           25%
├── 3x Code Execution (easy)              30%
├── 2x LLM-Open (concepts)                25%
└── 2x Code Review (find bugs)            20%

Middle (MIDDLE, MIDDLE_PLUS):
├── 3x Multiple Choice (edge cases)       15%
├── 4x Code Execution (medium)            35%
├── 3x LLM-Open (design patterns)         25%
└── 2x Code Review (refactoring)          25%

Senior (SENIOR, LEAD):
├── 2x Multiple Choice (architectural)    10%
├── 4x Code Execution (hard)              35%
├── 3x LLM-Open (system design)           30%
└── 3x Code Review (complex bugs)         25%
```

### 3.4. Критерии успеха Testing V2

| Метрика | V1 (текущее) | V2 (цель) |
|---------|--------------|-----------|
| Multiple choice % | 80% | < 20% |
| Code execution tasks | 0% | ≥ 30% |
| LLM-evaluated tasks | 0% | ≥ 25% |
| Anti-cheat detection | 3 признака | 8+ признаков |
| Cheating resistance | Низкая | Высокая |
| Time to complete | 60 мин | 90-120 мин |
| Discriminativeness (θ spread) | 8.0 | ≥ 6.0 |
| Test-retest reliability | Δ=0 | Δ ≤ 10 |
| Candidate satisfaction | — | ≥ 4/5 |

---

## 4. Frontend Redo

### 4.1. Design System

```typescript
// tokens/theme.ts
export const theme = {
  // Цвета ФСП
  colors: {
    primary: '#2563eb',      // Синий
    primaryLight: '#3b82f6',
    primaryDark: '#1d4ed8',
    
    success: '#10b981',      // Зелёный
    warning: '#f59e0b',      // Жёлтый
    error: '#ef4444',        // Красный
    
    // Neutrals
    bg: '#f8fafc',
    surface: '#ffffff',
    border: '#e2e8f0',
    
    text: '#0f172a',
    textSecondary: '#64748b',
    textMuted: '#94a3b8',
    
    // Grades
    gradeJunior: '#22c55e',
    gradeMiddle: '#3b82f6',
    gradeMiddlePlus: '#8b5cf6',
    gradeSenior: '#f59e0b',
    gradeLead: '#ef4444',
    gradePrincipal: '#ec4899',
  },
  
  // Тени
  shadows: {
    sm: '0 1px 2px rgba(0,0,0,0.05)',
    md: '0 4px 6px -1px rgba(0,0,0,0.1)',
    lg: '0 10px 15px -3px rgba(0,0,0,0.1)',
    xl: '0 20px 25px -5px rgba(0,0,0,0.1)',
  },
  
  // Скругления
  radius: {
    sm: '6px',
    md: '10px',
    lg: '16px',
    full: '9999px',
  },
  
  // Анимации
  transitions: {
    fast: '150ms ease',
    normal: '300ms ease',
    slow: '500ms ease',
  },
};
```

### 4.2. Ключевые компоненты

```
components/
├── ui/                          # Базовые
│   ├── GradientCard.tsx         # Карточка с градиентной шапкой
│   ├── AnimatedNumber.tsx       # Анимированное число
│   ├── ScoreRing.tsx            # Круговая диаграмма score
│   ├── EmptyState.tsx           # Красивое пустое состояние
│   └── LoadingSkeleton.tsx      # Скелетон
│
├── candidate/                   # Кандидат
│   ├── ProfileCard.tsx          # Карточка профиля
│   ├── AchievementGrid.tsx      # Сетка достижений
│   ├── TestProgress.tsx         # Прогресс теста
│   ├── InvitationCard.tsx       # Карточка приглашения
│   └── DailyTaskCard.tsx        # Ежедневное задание
│
├── employer/                    # Работодатель
│   ├── CandidateCard.tsx        # Карточата кандидата
│   ├── MatchScoreRing.tsx       # Кольцо совпадения
│   ├── MatchExplanation.tsx     # Объяснение совпадения
│   ├── SearchFilters.tsx        # Фильтры поиска
│   └── InvitationModal.tsx      # Модалка приглашения
│
├── testing/                     # Тестирование
│   ├── QuestionCard.tsx         # Карточка вопроса
│   ├── CodeEditor.tsx           # Редактор кода (Monaco)
│   ├── CodeOutput.tsx           # Вывод выполнения
│   └── TestResults.tsx          # Результаты
│
└── gamification/                # Геймификация
    ├── BadgeDisplay.tsx         # Бейдж
    ├── LevelProgress.tsx        # Прогресс уровня
    ├── StreakCounter.tsx        # Счётчик стрика
    └── Leaderboard.tsx          # Рейтинг
```

### 4.3. Страницы Candidate

```
candidate/
├── page.tsx                     # Dashboard
│   ├── WelcomeHeader            # Приветствие + уровень + стрик
│   ├── CategoryCard             # Текущая категория
│   ├── TestStatusCard           # Статус теста
│   ├── InvitationsPreview       # Последние приглашения
│   ├── AchievementsPreview      # Топ достижений
│   └── DailyTaskCard            # Ежедневное задание
│
├── profile/
│   ├── page.tsx                 # View mode
│   │   ├── ProfileHeader        # Аватар, ФИО, специализация
│   │   ├── ProfileDetails       # Стек, опыт, био
│   │   ├── AchievementsGrid     # Достижения ФСП
│   │   ├── TestHistory          # История тестов
│   │   └── PreviewToggle        # "Как видит работодатель"
│   │
│   └── edit/page.tsx            # Edit mode
│       ├── BasicInfoForm        # ФИО, контакты
│       ├── ProfessionalForm     # Специализация, грейд, стек
│       ├── ResumeUpload         # Загрузка PDF
│       ├── FSPConnect           # Подключение ФСП ID
│       └── PrivacySettings      # Настройки приватности
│
├── test/
│   ├── page.tsx                 # Тестирование
│   │   ├── TestIntro            # Введение + правила
│   │   ├── ActiveTest           # Текущий тест
│   │   │   ├── ProgressHeader   # Прогресс + таймер
│   │   │   ├── QuestionCard     # Текущий вопрос
│   │   │   ├── CodeEditor       # Для coding-задач
│   │   │   └── TestNavigation   # Навигация
│   │   └── TestResult           # Результаты
│   │       ├── ScoreDisplay     # Итоговый score
│   │       ├── GradeResult      # Грейд до/после
│   │       ├── TaskBreakdown    # По типам задач
│   │       └── Recommendations  # Рекомендации
│   └── history/page.tsx         # История тестов
│
├── invitations/
│   └── page.tsx                 # Приглашения
│       ├── InvitationList       # Список приглашений
│       └── InvitationDetail     # Детали приглашения
│
└── settings/
    └── page.tsx                 # Настройки
        ├── PrivacySettings      # Приватность
        ├── NotificationSettings # Уведомления
        └── AccountSettings      # Аккаунт
```

### 4.4. Страницы Employer

```
employer/
├── page.tsx                     # Dashboard
│   ├── StatsOverview            # Статистика
│   ├── RecentInvitations        # Последние приглашения
│   └── TopRecommendations       # Топ рекомендаций
│
├── search/
│   └── page.tsx                 # Поиск
│       ├── SearchBar            # Описание потребности
│       ├── SearchFilters        # Фильтры (sidebar)
│       ├── CandidateGrid        # Сетка кандидатов
│       │   └── CandidateCard    # Карточата
│       │       ├── Avatar
│       │       ├── MatchScoreRing
│       │       ├── StackTags
│       │       └── QuickInvite
│       └── ExplanationDrawer    # Детали совпадения
│
├── candidates/
│   ├── page.tsx                 # Список кандидатов
│   └── [id]/
│       └── page.tsx             # Профиль кандидата
│           ├── CandidateHeader  # Основная информация
│           ├── MatchDetails     # Детали совпадения
│           ├── Achievements     # Достижения
│           └── InviteButton     # Кнопка приглашения
│
├── invitations/
│   └── page.tsx                 # Отправленные приглашения
│       ├── InvitationTable      # Таблица приглашений
│       └── StatusTracker        # Трекер статусов
│
└── vacancies/
    ├── page.tsx                 # Список вакансий
    └── new/page.tsx             # Создать вакансию
```

---

## 5. API Contract (Shared Types)

### 5.1. Matching API V2

```typescript
// types/matching.ts

// Запрос на подбор
interface SearchCandidatesRequest {
  description: string;           // Текстовое описание потребности
  specialization?: Specialization;
  grade?: Grade;
  stack?: string[];
  salaryFrom?: number;
  salaryTo?: number;
  hasAchievements?: boolean;
  limit?: number;
  offset?: number;
  
  // V2: расширенные опции
  useReranker?: boolean;         // Включить cross-encoder re-ranking
  diversify?: boolean;           // Diversity (MMR)
  explainMode?: 'detailed' | 'summary' | 'none';
}

// Ответ
interface SearchCandidatesResponse {
  total: number;
  candidates: CandidateMatch[];
  queryUnderstanding: QueryUnderstanding;  // V2: что понял система
  timing: {
    retrieval: number;          // мс
    reranking: number;          // мс
    total: number;              // мс
  };
}

// Кандидат в выдаче
interface CandidateMatch {
  id: string;
  displayName: string;
  specialization: Specialization;
  verifiedGrade: Grade | null;
  stack: string[];
  
  matchScore: number;           // 0-100
  matchBreakdown: MatchBreakdown;
  matchExplanation: MatchExplanation;
  
  // V2: детали
  semanticSimilarity: number;   // 0-1
  rerankerScore: number | null; // 0-1, если re-ranking включён
  retrievalRank: number;        // позиция до re-ranking
  finalRank: number;            // позиция после re-ranking
}

// Понимание запроса
interface QueryUnderstanding {
  originalQuery: string;
  parsed: {
    intent: 'hire' | 'search' | 'evaluate';
    specialization: Specialization | null;
    seniority: Grade | null;
    requiredSkills: string[];
    preferredSkills: string[];
    context: string | null;
  };
  embedding: number[];          // 768-dim
  embeddingProvider: 'transformers' | 'tfidf';
}

// Breakdown score
interface MatchBreakdown {
  total: number;
  components: {
    specialization: { score: number; weight: number; contribution: number };
    stack: { score: number; weight: number; contribution: number };
    semantic: { score: number; weight: number; contribution: number };
    grade: { score: number; weight: number; contribution: number };
    achievements: { score: number; weight: number; contribution: number };
    experience: { score: number; weight: number; contribution: number };
    testScore: { score: number; weight: number; contribution: number };
  };
  // V2: персональные веса (если есть learning)
  weightsSource: 'default' | 'learned' | 'hybrid';
  confidence: number;           // 0-1, уверенность в весах
}

// Объяснение
interface MatchExplanation {
  summary: string;              // "Сильное совпадение"
  highlights: string[];         // Топ совпадений
  concerns: string[];           // Потенциальные проблемы
  detailedFactors: ExplanationFactor[];
}

interface ExplanationFactor {
  factor: string;
  impact: 'positive' | 'negative' | 'neutral';
  contribution: number;         // -100..+100
  explanation: string;
  evidence: string[];           // Конкретные доказательства
}
```

### 5.2. Testing API V2

```typescript
// types/testing.ts

// Типы заданий
type TaskType = 
  | 'multiple_choice'
  | 'code_execution'      // V2: новый
  | 'llm_open'            // V2: новый
  | 'code_review'         // V2: новый
  | 'system_design';      // V2: новый

// Задание
interface TestTask {
  id: string;
  type: TaskType;
  difficulty: number;           // 0-1
  points: number;               // Баллы за задание
  
  // Общие поля
  title: string;
  description: string;
  timeLimitSeconds: number;
  
  // type-specific поля
  multipleChoice?: {
    options: Array<{ id: string; text: string; isCorrect: boolean }>;
  };
  
  codeExecution?: {
    starterCode: string;
    language: 'javascript' | 'python' | 'go';
    sampleTests: Array<{ input: string; expectedOutput: string }>;
    hiddenTestsCount: number;   // Сколько скрытых тестов
  };
  
  llmOpen?: {
    evaluationCriteria: Array<{ name: string; weight: number }>;
    referenceAnswer?: string;
    keyConcepts: string[];      // Ключевые концепции для оценки
  };
  
  codeReview?: {
    buggyCode: string;
    language: string;
    bugCount: number;
    expectedBugs: string[];     // Описание ожидаемых багов
  };
  
  systemDesign?: {
    constraints: string[];
    evaluationCriteria: Array<{ name: string; weight: number }>;
  };
}

// Ответ кандидата
interface TaskAnswer {
  taskId: string;
  
  multipleChoice?: { selectedOptionId: string };
  
  codeExecution?: { code: string };
  
  llmOpen?: { answerText: string };
  
  codeReview?: { 
    fixedCode: string;
    bugsFound: string[];
    explanation: string;
  };
  
  systemDesign?: { 
    description: string;
    diagram?: string;           // Mermaid/ASCII
  };
  
  // Behavioral data (anti-cheat)
  behavioral?: {
    timeSpentSeconds: number;
    copyPasteEvents: number;
    tabSwitches: number;
    keystrokeCount: number;
    idleTimeSeconds: number;
  };
}

// Результат оценки
interface TaskResult {
  taskId: string;
  score: number;                // 0-100
  maxScore: number;
  
  // Для code_execution
  codeExecution?: {
    passedTests: number;
    totalTests: number;
    testResults: Array<{ passed: boolean; hidden: boolean }>;
    executionTimeMs: number;
  };
  
  // Для llm_open
  llmEvaluation?: {
    score: number;
    breakdown: Record<string, number>;  // По критериям
    strengths: string[];
    weaknesses: string[];
    feedback: string;
  };
  
  // Для code_review
  codeReview?: {
    bugsFound: number;
    bugsTotal: number;
    fixQuality: number;         // 0-100 (LLM)
    explanationQuality: number; // 0-100 (LLM)
  };
  
  // Anti-cheat
  antiCheat?: {
    flags: string[];
    confidence: number;
    behavioralScore: number;    // 0-100, насколько "человечески"
  };
}
```

---

## 6. Инфраструктура

### 6.1. Docker Compose V2

```yaml
# Добавляется к существующему docker-compose.yml
services:
  # Python ML Service (новый)
  ml-service:
    build: ./ml-service
    container_name: fsp-ml-service
    restart: unless-stopped
    ports:
      - "8000:8000"
    environment:
      - MODEL_NAME=sentence-transformers/paraphrase-multilingual-mpnet-base-v2
      - RERANKER_NAME=cross-encoder/ms-marco-MiniLM-L-6-v2
      - CACHE_SIZE=1000
    volumes:
      - ml_models:/root/.cache
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8000/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    deploy:
      resources:
        limits:
          memory: 2G

  # Code Sandbox (новый)
  sandbox:
    build: ./sandbox
    container_name: fsp-sandbox
    restart: unless-stopped
    # Без портов — только внутренняя сеть
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp:size=256m
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '0.5'

volumes:
  ml_models:
```

### 6.2. ML Service Dockerfile

```dockerfile
# ml-service/Dockerfile
FROM python:3.11-slim

WORKDIR /app

# Системные зависимости
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Python зависимости
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Скачивание моделей заранее (для быстрого старта)
RUN python -c "from sentence_transformers import SentenceTransformer; \
    SentenceTransformer('sentence-transformers/paraphrase-multilingual-mpnet-base-v2'); \
    SentenceTransformer('cross-encoder/ms-marco-MiniLM-L-6-v2')"

COPY . .

EXPOSE 8000

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000", "--workers", "2"]
```

---

## 7. Метрики и валидация

### 7.1. Matching V2 метрики

```bash
# Запуск метрик matching (нужны работающий сервер и БД)
cd backend
API_URL=http://localhost:3000/api FRONTEND_URL=http://localhost:8080 go run ./cmd/metrics

# Вывод (docs/VALIDATION_REPORT.md):
# - precision@5, @10 (per query type)
# - recall@10
# - MRR, nDCG
# - Среднее время ответа API
```

### 7.2. Testing V2 метрики

```bash
# Запуск метрик testing (тот же прогон)
cd backend
API_URL=http://localhost:3000/api go run ./cmd/metrics

# Вывод (docs/VALIDATION_REPORT.md):
# - Discriminativeness (θ spread strong vs weak)
# - Test-retest reliability
# - Анти-чит: нет ложных срабатываний на честном прогоне

# Unit-тесты алгоритмов (CAT, скоринг, анти-чит):
go test ./internal/modules/testing/...
```

---

## 8. Риски и митигация

| Риск | Вероятность | Влияние | Митигация |
|------|-------------|---------|-----------|
| Python ML service медленный | Среднее | Высокое | ONNX, quantization, batching |
| Transformers не скачиваются | Низкое | Высокое | Pre-download в Docker, кэш |
| Code sandbox vulnerable | Низкое | Критическое | Docker isolation, resource limits |
| LLM API дорогой/медленный | Высокое | Среднее | Fallback: keyword matching |
| Frontend regression | Среднее | Высокое | E2E тесты, постепенная миграция |
| Время хакатона | Высокое | Высокое | Приоритизация P0 |
