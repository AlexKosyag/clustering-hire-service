# 05. Matching engine — подбор кандидатов

Содержание: [README](README.md) · [01](01-architecture.md) · [02](02-data-model.md) · [03](03-auth-and-security.md) · [04](04-api-reference.md) · **05** · [06](06-testing-engine.md) · [07](07-fsp-gamification-notifications.md) · [08](08-infrastructure.md) · [09](09-known-issues.md)

Каталог исходников: `backend/src/modules/matching/`.

| Файл | Роль |
|------|------|
| `matching.controller.ts` | HTTP-слой (6 маршрутов, V1 + V2) |
| `matching.service.ts` | оркестратор V1 (legacy) |
| `matching-v2.service.ts` | оркестратор V2 (ML-понимание запроса, 7-фичевый скоринг, re-rank) |
| `scoring.service.ts` / `scoring-v2.service.ts` | формулы скоринга V1 / V2 |
| `explanation.service.ts` | человекочитаемые объяснения совпадения |
| `query-understanding.service.ts` | текст запроса → структурированная потребность (V2) |
| `embedding.service.ts`, `embedding/tfidf.ts` | эмбеддинги (каскад transformers → TF-IDF) |
| `indexing.service.ts` | текст кандидата → вектор → pgvector |
| `vector.repository.ts` | raw-SQL доступ к pgvector |
| `ml-client.service.ts` | HTTP-клиент к ml-service (FastAPI) |
| `candidate-view.ts` | публичное представление кандидата (DTO/маппер) |
| `dto/matching.dto.ts` | DTO запросов |

---

## 1. Направление и общий пайплайн

Matching **однонаправленный**: потребность работодателя → ранжированные кандидаты. Обратного поиска вакансий для кандидата нет (вакансии — обычный CRUD).

Общая схема (V1 и V2):

```
Запрос (description/filters) 
   │ 1. кэш Redis? → вернуть
   ▼
2. Жёсткие фильтры в Prisma (specialization точный, grade ±1, hasAchievements), пул ≤ 3000
   ▼
3. Эмбеддинг запроса (384-dim) → KNN в pgvector (косинус), выборка ≤ 3000
   ▼
4. Скоринг каждого кандидата (5/7 компонент) + объяснение
   ▼
5. Сортировка по убыванию score → slice(offset, offset+limit) → кэш (TTL 900с)
   ▼
6. [V2] Batch cross-encoder re-rank топ-K с блендингом 70/30
```

### Жёсткие фильтры (`buildWhere`)
- `specialization` — точное совпадение;
- `grade` — **±1 уровень** по `GRADE_ORDER = [JUNIOR, MIDDLE, MIDDLE_PLUS, SENIOR, LEAD, PRINCIPAL]`: допускаются `verifiedGrade ∈ {g-1, g, g+1}` **или** (`verifiedGrade == null` и `declaredGrade ∈ {g-1, g, g+1}`);
- `hasAchievements: true` → `achievements: { some: {} }`;
- лимит пула `take: 3000`.

⚠️ Важно: KNN-выборка по вектору **не префильтруется** этими условиями — пересечение множеств происходит в приложении, поэтому при узких фильтрах итоговый список может быть меньше `limit`.

### Резюмирование / ленивая индексация
Кандидаты пула без вектора индексируются в фоне: `void indexing.indexBatch(unindexed.slice(0, 25))` — максимум 25 за запрос, чтобы не блокировать ответ.

---

## 2. V1 (`matching.service.ts`)

Шаги: кэш → `buildWhere` → текст запроса `description + stack.join(' ')` (fallback: спец+грейд+стек) → `EmbeddingService.embed()` → `VectorRepository.topBySimilarity(vec, 3000)` → скоринг `MatchingScoringService.calculateMatch(candidate, need, semantic)` + `ExplanationService.generate(...)` → сортировка → кэш.

Семантический компонент: `semantic = Math.max(0, cosineSimilarity)` (отрицательная косинусная схожесть обнуляется).

**Рекомендации** (`recommendations`) — только в V1:
1. кандидаты, приглашённые до 20 работодателями той же `industry`, исключая уже приглашённых этим работодателем (топ-5);
2. до 5 кандидатов, созданных за последние 14 дней, по специализации последнего `EmployerNeed` работодателя.

---

## 3. V2 (`matching-v2.service.ts`)

| Шаг | Что происходит |
|-----|----------------|
| 1. Query understanding | `QueryUnderstandingService.parseNeed(description, explicitSpecialization, explicitGrade, explicitStack)` → `{description, specialization, grade, requiredSkills, preferredSkills, context, embedding, embeddingProvider}`; явные фильтры DTO имеют приоритет над распознанными ML |
| 2. Retrieval | те же жёсткие фильтры + KNN по вектору потребности |
| 3. Scoring | `MatchingScoringServiceV2.calculateMatch(...)` — 7 компонент + rerank-буст, `matchBreakdown[key] = {score, weight, contribution}`; объяснение — тот же V1 `ExplanationService` с переименованием компонент (`categoryMatch ← specializationMatch`) |
| 4. Sort | по убыванию `matchScoreV2`, назначение `retrievalRank` |
| 5. Re-rank | батчевый cross-encoder для топ-`min(limit+20, 50)` (только при ML available и >1 кандидата) |
| 6. Result | `queryUnderstanding` + `timing {retrieval, scoring, reranking, total}` (мс) |

**Re-rank (шаг 5)**:
```ts
normalizedScore = clamp((crossEncoderScore + 10) * 5, 0, 100);  // типичный диапазон CE: -10..10
blendedScore    = matchScoreV2 * 0.7 + normalizedScore * 0.3;    // блендинг 70/30
```
После пересчёта топ-K пересортировывается, назначаются `finalRank`, хвост списка присоединяется без изменений.

**`createNeed` (V2)**: `parseNeed` → сохранение `EmployerNeed` (ML-поля, явные — как fallback) → `vectors.setNeedVector(need.id, embedding)` → `search(limit=10)` → ответ с `queryUnderstanding`, `candidatesCount`, `topCandidates`, `timing`.

⚠️ V1 `createNeed` существует, но **не имеет HTTP-маршрута**.

---

## 4. Формулы скоринга

Все компоненты нормализованы к шкале **0–100** (`clamp100`, нечисловые значения → 0).

### 4.1 V1 — `scoring.service.ts`

```ts
WEIGHTS = { category: 0.3, stack: 0.25, semantic: 0.2, grade: 0.15, achievement: 0.1 };
total = Σ component × weight   // округление до 1 знака
```

| Компонент | Формула |
|-----------|---------|
| `categoryMatch` | нет спец. у запроса → 50; нет у кандидата → 30; точное совпадение → 100; **родственная** (`RELATED`, напр. `BACKEND: [FULLSTACK, DEVOPS]`) → 60; иначе 0 |
| `stackOverlap` | `round(совпало / всего_в_запросе × 100)` после нормализации `normalizeTech` (lowercase, убрать `[.\s-]`; `C++ ≠ C`); пустой запрос → 50; пустой стек кандидата → 0 |
| `semanticSimilarity` | `clamp100(max(0, cosine) × 100)` |
| `gradeMatch` | нет грейда у запроса → 50; нет у кандидата → 30; равенство → 100; кандидат на уровень выше → 80; ниже → 60; переквалификация (>+1) → 40; недоквалификация (<−1) → 20 |
| `achievementBoost` | 0 очков → 0; иначе `min(100, points × 10)` |

### 4.2 V2 — `scoring-v2.service.ts`

```ts
DEFAULT_WEIGHTS = {
  specialization: 0.18,
  stack:          0.15,
  semantic:       0.20,
  grade:          0.12,
  achievement:    0.08,
  experience:     0.12,
  testScore:      0.15,
};  // сумма = 1.0
// + rerank-буст: до +10 баллов аддитивно
```
`contributions[key] = round(component × weight, 1 знак)`; `total = round(Σ contributions, 1 знак)`; `weightsSource = 'default'` (персональные веса под работодателя заявлены как возможное развитие, не реализованы).

| Компонент | Формула |
|-----------|---------|
| `specializationMatch` | как в V1 (50/30/100/60/0) |
| `stackOverlap` | **взвешенный по важности технологий**: каждый `requiredSkills`-скилл получает вес `(SKILL_IMPORTANCE[skill] ?? 0.7) × 1.5`, каждый из `needStack` — `× 1.0`; совпал, если нормализованный скилл есть в стеке кандидата; `100 × matchedWeight / totalWeight` |
| `semanticSimilarity` | `clamp100(max(0, cosine) × 100)` |
| `gradeMatch` | идентично V1 |
| `achievementBoost` | **логарифмическое затухание**: `min(100, round(log2(1 + points) × 15))` |
| `experienceRelevance` | ожидаемый опыт по грейду: JUNIOR [0,2], MIDDLE [2,5], MIDDLE_PLUS [3,7], SENIOR [5,10], LEAD [8,15], PRINCIPAL [10,20] лет. В диапазоне → 100; ниже, но ≥ min×0.5 → 60; ещё ниже → 20; выше max, но ≤ max×1.5 → 80; иначе 50; `experienceYears == null` → 40; грейд не задан → 50 |
| `testScore` | `null` → 40 (не тестирулся); иначе `clamp100(round(lastTestScore))` |

**`normalizeTech` (V2)** дополнен словарём алиасов: `node.js|node js|node → nodejs`, `postgres → postgresql`, `react.js → react`, `k8s → kubernetes`, `js → javascript`, `ts → typescript`, `py → python`, `golang → go`, `c#|c sharp → csharp`, `.net|dot net → dotnet`; далее удаляются `[.\s-]`.

**`SKILL_IMPORTANCE`** (фрагмент; полная карта — `scoring-v2.service.ts`): языки `nodejs/js/ts/python/java/go 1.0`, `c++/c#/rust/swift/kotlin 0.9`, `ruby/php 0.8`; фреймворки `react/django/spring 0.9`, `vue/angular/fastapi/nextjs 0.85`; БД `postgresql 1.0`, `mysql/redis 0.9`, `mongodb 0.85`, `elasticsearch 0.8`; инфра `kubernetes 0.9`, `docker/aws 0.85`, `gcp/azure 0.8`; `kafka 0.85`, `microservices 0.85`. Неизвестный скилл — **0.7**.

**Rerank-буст (покомпонентный)**: `getRerankBoost` вызывает `mlClient.rerank(query, [candidateText], 1)`, нормализация `(raw + 10) / 20` в [0,1], вклад `rerankBoost × 10` (до +10 баллов). ⚠️ Вызывается **на каждого кандидата** — потенциально тысячи HTTP-запросов на один поиск (см. [09](09-known-issues.md)).

⚠️ `matchBreakdown.score` в текущей версии всегда 0 (несовпадение имён ключей при чтении `score.components`); `weight`/`contribution` корректны.

### 4.3 Отсутствующие факторы
Ни в одной версии **нет** компонентов зарплаты и локации: `salaryFrom/To` существуют только у `Vacancy`/`Invitation` и в matching не участвуют; `city` попадает только в текст эмбеддинга.

---

## 5. Объяснения (`explanation.service.ts`)

Rule-based нарратив (не SHAP/вклады признаков) — `generate(candidate, need, score) → MatchExplanation {overall, details[]}`. Каждый `detail = {factor, match, weight, explanation, overlap?, detail?}`:

1. **Специализация** (0.3) — «не указана» / точное совпадение / несовпадение;
2. **Стек** (0.25) — «Совпадение k/n: ...» (нормализованный overlap);
3. **Грейд** (0.15) — «подтверждён тестом»/«заявлен кандидатом», «допустимо» (±1), «Избыточно высокий грейд», «Грейд ниже запросленного»;
4. **Семантическое совпадение** (0.2) — `match = semanticSimilarity ≥ 50`, `detail: "N%"`;
5. **Достижения ФСП** (0.1) — до 2 названий достижений (только при наличии);
6. **Активность** (0) — дней с `lastTestDate`, `match = days ≤ 90` (информационно).

Итоговая оценка (`summary`): `≥80` «Сильное совпадение», `≥60` «Хорошее», `≥40` «Частичное», иначе «Слабое совпадение».

---

## 6. Эмбеддинги

### 6.1 Провайдеры (`embedding.service.ts`)
Каскад выбирается `EMBEDDING_PROVIDER = auto | tfidf | transformers`:
1. `tfidf` — только лексическая модель (мгновенный старт, детерминированность);
2. иначе в фоне загружаются ONNX-модели `@xenova/transformers`: **`Xenova/paraphrase-multilingual-MiniLM-L12-v2`** (RU) → **`Xenova/all-MiniLM-L6-v2`** (EN), обе **384-dim** (mean-pooling + нормализация); пока модель грузится — работает TF-IDF;
3. гарантия: `embed()` никогда не бросает исключение и всегда возвращает 384 конечных чисел; при ошибке transformers — постоянная деградация на tfidf.

Событие `onProviderReady(cb)` запускает полную переиндексацию после загрузки модели. Также доступны `fitCorpus(texts)`, `embedLexicalSync`, `currentProvider/currentModel/dim` (dim всегда 384 — выводится в `/api/health`).

### 6.2 TF-IDF fallback (`embedding/tfidf.ts`)
- `EMBEDDING_DIM = 384`;
- токенизация: lowercase, `ё→е`, символы `[a-zа-я0-9+#]`, длина >1, **+ биграммы** `w1_w2`;
- `fit`: document frequency, сглаженный IDF `log((1+N)/(1+df)) + 1`;
- `embed`: sublinear TF `(1 + ln count) × idf` → **signed feature hashing** по FNV-1a (`index = hash % 384`, знак — старший бит) → L2-нормализация; при ошибке — `unitVector()` (базис e1);
- `cosineSimilarity(a,b)` — стандартная, с защитой от нулевых норм и NaN.

### 6.3 Текст кандидата для индексации (`indexing.service.ts → buildText`)
`специализация + грейд(verified||declared) + stack + roles + softSkills + город + "<N> лет опыта" + bio + названия достижений`, склеенные пробелом.
Для re-rank используется другой текст (`buildCandidateText`): маркированные англоязычные предложения `Specialization: ... . Grade: ... . Tech stack: ... . Bio: ...`.

### 6.4 Когда строятся векторы
- `onModuleInit`: подписка на `onProviderReady` → `reindexAll()`; плюс `setTimeout(2000)` → фоновая полная переиндексация (не блокирует старт);
- `indexCandidate(id)` / `indexBatch(ids)` — лениво из поиска (по 25 за запрос) и из `reindexCandidate` (вызывается из `fsp.service`, `candidates.service`, testing-сервисов);
- `reindexAll()`: до **5000** профилей, переобучение TF-IDF корпуса, запись векторов по одному `UPDATE` на кандидата.

Предзаполнение офлайн: `npm run seed:embeddings` (`prisma/embeddings.ts`) — общий корпус из профилей и потребностей, TF-IDF fit, запись векторов raw SQL.

---

## 7. Поиск по векторам (`vector.repository.ts`)

Все обращения к pgvector — raw SQL (Prisma не видит `Unsupported`-колонки):

```sql
-- запись
UPDATE candidate_profiles SET profile_embedding = '[...384...]::vector' WHERE id = $1;

-- KNN
SELECT id, (1 - (profile_embedding <=> '[...]::vector'))::float8 AS similarity
FROM candidate_profiles
WHERE profile_embedding IS NOT NULL
ORDER BY profile_embedding <=> '[...]::vector'
LIMIT $2;
```

- `similarity = 1 - косинусное расстояние`;
- `literal(vector)` — санация ровно 384 конечных значений с 6 знаками;
- **fallback**: при ошибке SQL выставляется sticky-флаг `sqlBroken` и далее сортировка в памяти (`dot` по нормализованным векторам, читаемым как текст);
- `indexedIds(ids)` — подмножество id с непустым вектором.
⚠️ ANN-индекс не создаётся (см. [02-data-model.md](02-data-model.md) §4).

---

## 8. Понимание запроса и ML-клиент

### `QueryUnderstandingService.parseNeed`
Делегирует в `MlServiceClient.parseQuery(description)`, затем мержит: **явные поля DTO приоритетнее** распознанных; `requiredSkills = union(mlSkills, explicitSkills)` (дедупликация).

### `MlServiceClient` (`ml-client.service.ts`)
Base URL `ML_SERVICE_URL` (в Docker `http://ml-service:8000`), таймаут `ML_SERVICE_TIMEOUT` (5000 мс), health-check с кэшем 30 с.

| Метод | HTTP | Fallback при недоступности ML |
|-------|------|------------------------------|
| `checkHealth` | `GET /health` | `_available = false` (кэш 30 с) |
| `embed` / `embedBatch` | `POST /embed`, `/embed/batch` | локальный TF-IDF (`provider: 'tfidf-local'`) |
| `parseQuery` | `POST /parse-query` | `localParseQuery` + локальный TF-IDF |
| `rerank` | `POST /rerank` (10 с) | identity-результат `{score: 0.5}`, `available: false` |
| `ner` | `POST /ner` | `localNer` (ключевые слова) |

**Локальный fallback `parseQuery`** (словари в `ml-client.service.ts`):
- специализации: `BACKEND: ['backend','бэкенд','server','api','микросервис']`, `FRONTEND: ['frontend','фронтенд','react','vue','angular']`, аналогично FULLSTACK/MOBILE/DEVOPS/DATA_SCIENCE/QA/SECURITY (первое совпадение побеждает);
- грейды: `JUNIOR/MIDDLE/SENIOR/LEAD` (+ русские варианты);
- ~45 технологий substring-поиском → `requiredSkills`; `intent = 'hire'`, `context` — первые 200 символов.

**Словари в Python-сервисе** (`ml-service/main.py`): `SKILL_SYNONYMS` (~40 канонических синонимов, напр. `javascript: [js, node.js, nodejs, node, ecmascript, es6]`), `SPECIALIZATION_KEYWORDS`, `GRADE_KEYWORDS`. Модели: `sentence-transformers/paraphrase-multilingual-mpnet-base-v2` (эмбеддинги), `cross-encoder/ms-marco-MiniLM-L-6-v2` (rerank).

⚠️ **Рассинхрон размерности**: ml-service отдаёт **768-мерные** векторы, а pgvector-колонки и `literal()` рассчитаны на **384** — вектор запроса молча усекается и сравнивается с векторами другой модели. Семантическая компонента V2 при живом ML некорректна; V1 внутренне консистентен (обе стороны через `EmbeddingService`). Подробнее — [09-known-issues.md](09-known-issues.md).

---

## 9. Кэширование

| Параметр | Значение |
|----------|----------|
| Что кэшируется | полный сериализованный `SearchResult` / `SearchResultV2` (кандидаты, объяснения, breakdown, timing) |
| Ключи | `matching:cache:search:<sha1>` / `matching:cache:search-v2:<sha1>`; hash = `sha1(JSON(payload со сортированными ключами) + JSON([limit, offset]))` |
| TTL | `matching.cacheTtlSeconds = 900` с (конфиг, без env-переопределения) |
| Инвалидация | `delByPattern('matching:cache:*')` — вызывается при: обновлении профиля кандидата, загрузке резюме, создании/обработке приглашения, обновлении профиля работодателя, завершении теста, подключении/отключении ФСП |
| Не кэшируется | `semanticSearch`, ответы ML, эмбеддинги (кроме внутреннего кэша ml-service) |

Поведение при сбое Redis: запросы работают без кэша (см. [01-architecture.md](01-architecture.md) §8).

---

## 10. Сравнение V1 и V2 (эксплуатационно)

| Характеристика | V1 | V2 |
|----------------|----|----|
| Маршруты | `/api/matching/search`, `/semantic`, `/recommendations` | `/api/matching/v2/search`, `/v2/semantic`, `/v2/need` |
| Понимание запроса | склейка `description + stack` | ML-parse + мерж с фильтрами |
| Скоринг | 5 компонент | 7 компонент + rerank-буст |
| Re-rank | нет | batch cross-encoder топ-K (70/30) |
| Ответ | `total, candidates` | + `matchBreakdown`, `queryUnderstanding`, `timing`, ранги |
| Кто использует | все модули (invalidateCache/reindex) | только testing-v2 + фронтенд |

Обе версии активны одновременно; V2 — «флагманская», V1 остаётся legacy и используется внутренними вызовами.
