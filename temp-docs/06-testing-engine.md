# 06. Testing engine — тестирование кандидатов

Содержание: [README](README.md) · [01](01-architecture.md) · [02](02-data-model.md) · [03](03-auth-and-security.md) · [04](04-api-reference.md) · [05](05-matching-engine.md) · **06** · [07](07-fsp-gamification-notifications.md) · [08](08-infrastructure.md) · [09](09-known-issues.md)

Каталог исходников: `backend/src/modules/testing/`.

| Файл | Роль |
|------|------|
| `testing.controller.ts` | HTTP-слой (V1 `/testing/*` + V2 `/testing/v2/*`, роль CANDIDATE) |
| `testing.service.ts` | движок V1 (legacy): 20 вопросов, 60 минут |
| `testing-v2.service.ts` | движок V2 (основной): 15 мультиформатных заданий, 90 минут |
| `services/cat.service.ts` | адаптивный подбор вопросов (3PL IRT) |
| `services/question-pool.service.ts` | банк вопросов / категории |
| `services/scoring.service.ts` | баллы, грейд из θ, рекомендация |
| `services/anti-cheat.service.ts` | статистический анти-чит (V1) |
| `services/behavioral-anti-cheat.service.ts` | поведенческий анти-чит (V2, телеметрия клиента) |
| `services/task-generator.service.ts` | генерация мультиформатных заданий (V2) |
| `services/code-sandbox.service.ts` | исполнение кода кандидата |
| `services/llm-evaluation.service.ts` | LLM-оценка развёрнутых ответов |

---

## 1. Жизненный цикл сессии

```
POST /start ──► TestSession(IN_PROGRESS), profile.testStatus = IN_PROGRESS
   │            (409, если уже есть активная; 400 при активном кулдауне)
   ▼
GET .../current ──► текущий вопрос/задание (восстановление после перезагрузки страницы)
   ▼
POST .../answer (×N) ──► TestAnswer (unique sessionId+questionId), обновление θ и сложности
   │                     (истечение времени проверяется лениво на каждом вызове)
   ▼
POST .../complete ──► status=COMPLETED, score/gradeAfter/recommendation/анти-чит
                      profile: verifiedGrade, testStatus (FAILED|PASSED|UPGRADED),
                      lastTestDate, testCooldownUntil = now + 90 дней, lastTestScore
                      сайд-эффекты: matching.invalidateCache(), indexing.indexCandidate()
                      [V2] геймификация: XP, бейджи, стрик
```

### Статусы сессии (`SessionStatus`)
`IN_PROGRESS → COMPLETED | TIMED_OUT` (`ABANDONED` не используется).
Истечение: `now > startedAt + timeLimitMinutes × 60_000` — проверяется лениво (в `active`, `current`, `start`), просроченная сессия помечается `TIMED_OUT`; ответы после истечения → `400 Время тестирования истекло`.

### Восстановление (`resumability`)
`GET /api/testing/active` возвращает активную сессию и `canRetakeAt`. Номер вопроса восстанавливается из числа записанных ответов (`questionNumber = answered.length + 1`); CAT-выбор **детерминирован** для сессии (ключ `sessionOrderKey(sessionId, questionId)`), поэтому после перезагрузки показывается то же следующее задание.

### Повторная сдача (cooldown)
`complete()` ставит `testCooldownUntil = now + 90 × 86_400_000 мс`. `start()` при активном кулдауне → `400 Пересдача возможна не раньше <ISO>`. Для кандидата это отражается как `canRetakeAt` в `GET /api/candidates/category`.
⚠️ Кулдаун ставится только при явном `complete()` — истёкшая без завершения сессия его не ставит (новый `start` просто помечает её `TIMED_OUT`).

### Различия V1 / V2

| Параметр | V1 | V2 |
|----------|----|----|
| Маршруты | `/api/testing/*` | `/api/testing/v2/*` |
| Объём | 20 вопросов (`testing.questionsPerTest`) | 15 заданий (`TESTING_TASKS_PER_TEST`) |
| Лимит времени | 60 мин | 90 мин |
| Минимум банка | 20 вопросов | 10 вопросов |
| Форматы | MC + текстовые | MC, code_execution, llm_open, code_review, system_design |
| Оценка | MC + `referenceCoverage` | sandbox, LLM, эвристики |
| Повторный ответ | `409 Ответ на этот вопрос уже записан` | идемпотентно: `duplicate: true` |
| Анти-чит | статистический | статистический + поведенческий |
| Результат | + `details` по сложности | + `taskBreakdown`, `gamification`, `behavioralFlags` |
| Источник кулдауна | конфиг (90 дней) | env `TESTING_COOLDOWN_DAYS` (90) |

Фронтенд по умолчанию работает в режиме V2 (`v2Mode = true`), но для восстановления сессии использует V1 `GET /api/testing/active`.

---

## 2. Банк вопросов (`question-pool.service.ts`)

Намеренно простой слой:
- `poolFor(categoryId)` — вся категория (`orderBy: id asc`) с IRT-параметрами, `options` (с `isCorrect`), `correctAnswer`, `explanation`, `poolTag`, `isUnique`;
- `categoryForSpecialization(spec)` — категория по специализации, **fallback — любая категория** (`orderBy: name asc`); если нет ни одной → `400 Для этой специализации нет теста`;
- порог полноты пула: V1 `>= 20`, V2 `>= 10`, иначе `400 Банк вопросов неполный — тест временно недоступен`.

Случайности/фильтрации по сложности здесь нет — адаптивность целиком в `CATService`. Эталонные ответы клиенту не отдаются: `viewQuestion()` (V1) вырезает `isCorrect`/`correctAnswer`/`explanation` и **переставляет варианты** детерминированно (`AntiCheatService.shuffleOptions`).

В сиде: 13 категорий × 66 вопросов ≈ 858 (см. [02-data-model.md](02-data-model.md) §8).

---

## 3. CAT — адаптивный подбор (`cat.service.ts`)

**Модель: 3PL IRT** (трёхпараметрическая логистическая модель).

### Вероятность правильного ответа
```
P(correct | θ) = c + (1 − c) / (1 + e^(−a(θ − b)))
```
где `a = discrimination` (>0, default 1), `b = difficulty` (default 0.5), `c = guessing` ∈ [0,1), default 0.25. Результат зажимается в `[1e-6, 1 − 1e-6]`.

### Информация Фишера
```
p' = a(1 − c)e^(−a(θ−b)) / (1 + e^(−a(θ−b)))²
I(θ) = (p')² / (p(1 − p))
```

### Оценка способности θ — MLE (Newton–Raphson / Fisher scoring)
Старт `θ = 0`, допуск `0.001`, максимум 50 итераций. На каждой итерации по ответам (`u` ∈ {0,1}, `pc = p − c`):
```
first += a(u − p)(p − c) / (p(1 − c))        // score-функция
info  += a²(p − c)²(1 − p) / ((1 − c) p)     // ожидаемая информация
θ     += clamp(first / info, ±0.5)           // демпфирование шага
θ      = clamp(θ, −4, 4)
```
Остановка: `info < 1e-9` или `|step| < 0.001`. Пустой список ответов → `θ = 0`.
В докстринге пояснено, почему используется ожидаемая информация вместо наблюдаемой гессианы: у 3PL гессиана меняет знак при `p < c`, что ломало оценку (все неверные ответы давали θ = 0 вместо нижней границы).

### Выбор следующего вопроса (`selectNext`)
1. Исключение уже отвеченных (`answeredIds`);
2. **первый вопрос** — с `difficulty`, ближайшей к 0.5;
3. далее — **максимум информации Фишера** при текущем θ; ничьи (в пределах `1e-12`) разрешаются меньшим `sessionOrderKey(sessionId, questionId)` — чтобы `/current` и `/answer` выбирали одинаково;
4. пул исчерпан → `null`.

### Детерминизм (`sessionOrderKey`)
FNV-1a 32-бит от `"${sessionId}:${questionId}"` (`hash = 0x811c9dc5`, `Math.imul(hash, 0x01000193)`, `>>> 0`) — уникальный стабильный порядок на сессию (используется и для перестановки вариантов ответа).

### Критерий остановки
Фиксированная длина теста (20/15 ответов) или истечение времени; критериев по стандартной ошибке измерения (SE) нет.

---

## 4. Оценка ответов и итоговый скоринг

### 4.1 Баллы за тест (`services/scoring.service.ts`)
```
w = 0.5 + difficulty                       // 0.5 … 1.5
earned = Σ w × clamp(score ?? (isCorrect ? 1 : 0), 0, 1)
score  = round(earned / Σw × 1000) / 10    // 0–100, один знак
```
Сложные вопросы весят больше.

### 4.2 Грейд из θ (`GRADE_THRESHOLDS`)
| Грейд | Порог θ |
|-------|---------|
| JUNIOR | ≥ −∞ (всегда) |
| MIDDLE | ≥ −1.0 |
| MIDDLE_PLUS | ≥ 0.0 |
| SENIOR | ≥ 0.5 |
| LEAD | ≥ 1.0 |
| PRINCIPAL | ≥ 1.5 |

`calculateGrade` — верхний достигнутый порог. Таблица поправок `SPEC_OFFSET` по специализациям существует, но пуста (`{}`) — `normalizeTheta` ничего не делает.

### 4.3 Рекомендация (`getRecommendation`)
- грейд не заявлен (`declaredGrade == null`): `score ≥ 50 ? PASS : FAIL`;
- `расчётный ≥ заявленного + 1` **и** `score ≥ 85` → **`UPGRADE`**;
- `расчётный ≥ заявленного` → `PASS`; иначе → `FAIL`.

Итог при `complete()`: `FAIL → TestStatus.FAILED`, `UPGRADE → UPGRADED`, иначе `PASSED`; `verifiedGrade = gradeAfter`, `gradeBefore = declaredGrade`.

### 4.4 Оценка отдельных ответов

**V1 (`gradeAnswer`)**:
- `multiple_choice`: `dto.answer` = ID варианта → `isCorrect = selected.isCorrect`, `score = 1|0`;
- текстовые: пустой → `400 Ответ пустой`; без эталона → `score = min(0.6, len/500)`, `isCorrect = false`; с эталоном → `referenceCoverage(эталон, ответ)`, `isCorrect = score ≥ 0.6`;
- `referenceCoverage`: эталон разбивается по `'|'` на ключевые фразы; для каждой — покрытие токенов и попадание фразы, итог — среднее по частям, зажатое в [0,1].

**V2 (`gradeAnswerV2`)** — грейдер выбирается **по присланным полям ответа** (fallback — `question.questionType`):

| Поля ответа | Грейдер | Логика |
|-------------|---------|--------|
| `code` | `CodeSandboxService` | исполнение + тесты, `score = passed/total × 100`; порог `isCorrect: score ≥ 60`; ошибка → 0 |
| `designDescription`/`diagram` | `LLMEvaluationService` | LLM-оценка по критериям (архитектура/масштабируемость/компромиссы/ясность) |
| `fixedCode`/`bugsFound` | code-review эвристика | сравнение с `expectedBugCount: 2` |
| `selectedOptionId` | MC | как в V1 |
| `answerText` | `LLMEvaluationService` | критерии accuracy 0.4 / completeness 0.3 / structure 0.2 / examples 0.1 |

### 4.5 `CodeSandboxService` (реальная, MVP-уровня)
- `executeWithTests(code, language, sampleTests, hiddenTestsCount = 5)`; `SANDBOX_TIMEOUT_MS` (10000), SIGKILL по таймауту, лимит вывода 1 МБ;
- языки: `node -e <harness>` (вызов `solution(input)`/`solve(input)`), `python3 -c <harness>`, `go run -` (через stdin); сравнение `stdout.trim() === expected.trim()`;
- ⚠️ скрытые тесты — **заглушка**: детерминированные входы (LCG), но `expectedOutput` не заполнен (`hidden_${i}`) — фактически всегда провал; комментарий в коде: «For hidden tests, we don't know the expected output in advance»;
- ⚠️ `SANDBOX_DOCKER`/`SANDBOX_MEMORY_MB` читаются, но Docker-режим и лимит памяти не реализованы (`sandboxUsed` только отражается в отчёте).

### 4.6 `LLMEvaluationService` (реальная, с fallback)
- используется OpenAI-совместимый API: `OPENAI_API_URL` (default `https://api.openai.com/v1/chat/completions`), `OPENAI_MODEL` (default `gpt-4o-mini`), `temperature 0.3`, `max_tokens ≤ 1000`, таймаут 30 с; строгий JSON-ответ `{score, breakdown, strengths, weaknesses, feedback}`;
- при отсутствии `OPENAI_API_KEY` или ошибке — **детерминированная keyword-эвристика**: покрытие концепций (`foundConcepts/totalConcepts × 100`), длина (`min(100, words × 2)`), структура (70 при наличии маркеров списков, иначе 40), взвешенное среднее по критериям (`provider: 'keyword-fallback'`);
- ⚠️ этот сервис **не** использует `ml-service` (он работает только для matching).

---

## 5. Генератор заданий V2 (`task-generator.service.ts`)

**Типы заданий** (`TaskType`): `multiple_choice`, `code_execution`, `llm_open`, `code_review`, `system_design`.

**Распределение по грейду** (проценты `{MC, code, llmOpen, review, design}`):
| Грейд | MC | code | llm_open | review | design |
|-------|----|------|----------|--------|--------|
| JUNIOR | 40 | 30 | 20 | 10 | 0 |
| MIDDLE | 20 | 35 | 25 | 15 | 5 |
| MIDDLE_PLUS | 15 | 35 | 25 | 15 | 10 |
| SENIOR | 10 | 35 | 25 | 15 | 15 |
| LEAD | 5 | 30 | 30 | 15 | 20 |
| PRINCIPAL | 5 | 25 | 30 | 20 | 20 |

Механика `createTaskFromQuestion(question, grade, index)`:
- строится список слотов типов по процентам от `totalTasks = 20` (⚠️ фактический тест — 15 заданий; распределение не балансируется под реальную длину);
- список детерминированно перемешивается LCG-генератором с сидом `index`, тип для позиции — `shuffled[index % shuffled.length]`;
- шаблоны заданий: `timeLimitSeconds` (MC 120, code_execution 300, llm_open 300, code_review 240, system_design 600 — только подсказка клиенту), `points` (MC 10, code 25, llm_open 20, review 30, design 40) × множитель `0.5 + difficulty` (⚠️ `points` информационные: итоговый score считает `ScoringService`, не сумма points);
- `detectLanguage(codeSnippet)` — эвристика (`def `/`print(` → python; `func `/`fmt.` → go; иначе javascript);
- `extractKeyConcepts` — список ключевых слов для LLM-критериев;
- устойчивость: MC-вопрос без `options` деградирует в `llm_open` (иначе задание стало бы `null` и тест завис бы).

⚠️ Тип задания выбирается распределением, **независимо** от исходного `questionType` вопроса — поэтому `gradeAnswerV2` определяет грейдер по фактическим полям ответа.

---

## 6. Анти-чит

### 6.1 Статистический (`anti-cheat.service.ts`, V1)
`analyze(answers, thetaHistory)` — сигналы:
| Сигнал | Условие | Флаг |
|--------|---------|------|
| Слишком быстрые «трудные» ответы | `timeSpent < 5с` при `difficulty > 0.5`, более 3 раз | `MULTIPLE_TOO_FAST_ANSWERS` |
| Инверсия сложности | ≥3 лёгких и ≥3 сложных, `hardCorrect > easyCorrect` и `easyCorrect < 2` (утечка ответов) | `SUSPICIOUS_DIFFICULTY_PATTERN` |
| Идеальный пул | пул `poolTag` с >5 ответов, все верные | `PERFECT_SCORE_POOL_<TAG>` |
| Волатильность θ | >2 скачков `|Δθ| > 2` | `THETA_VOLATILITY` |

`isSuspicious = flags.length ≥ 2`; `confidence = min(1, flags × 0.35)`.
`shuffleOptions(sessionId, questionId, options)` — детерминированная ротация вариантов (`shift = sessionOrderKey % options.length`) — уникальный порядок на сессию; `isCorrect` клиенту не отдаётся.

### 6.2 Поведенческий (`behavioral-anti-cheat.service.ts`, V2)
Вход (`BehavioralData` из фронтенда): `timeSpentSeconds`, `copyPasteEvents`, `tabSwitches`, `keystrokeCount`, `idleTimeSeconds`, `mouseMovementDistance?`, `focusLossEvents?`, `pasteSources?`.

Пороги: `minTimePerDifficulty {easy 10с, medium 20с, hard 40с}`, `maxCopyPastePerTest 3`, `maxTabSwitchesPerTest 5`, `minKeystrokesPerChar 0.3`, `maxIdleRatio 0.3`, `suspiciousFastAnswer 5с`.

Оценка подозрительности вопроса (аддитивно, 0–100):
| Сигнал | Условие | Баллы |
|--------|---------|-------|
| `TOO_FAST_FOR_HARD_QUESTION` | < 5 с при difficulty > 0.5 | +30 |
| `SUSPICIOUSLY_QUICK` | timeRatio < 0.3 и < 10 с | +20 |
| `MULTIPLE_COPY_PASTE` | copyPasteEvents ≥ 3 | +25 |
| `LARGE_PASTE_DETECTED` | длина > 100 при 1 paste (> 50 символов на событие) | +35 |
| `EXTERNAL_SOURCE_PASTE` | `pasteSources` совпадает с `/stackoverflow\|github\|chatgpt\|claude\|bard\|google/i` | +40 |
| `EXCESSIVE_TAB_SWITCHING` | tabSwitches > 5 | +20 |
| `FREQUENT_FOCUS_LOSS` | focusLossEvents > 3 | +15 |
| `EXCESSIVE_IDLE_TIME` | idle/time > 0.3 | +15 |
| `LOW_KEYSTROKE_DENSITY` | keystrokes/length < 0.3 при длине > 50 | +20 |

Сессионный уровень (`analyzeSession`): объединение флагов, `THETA_VOLATILITY`, `MULTIPLE_SUSPICIOUS_QUESTIONS` (≥3 вопросов с score > 50); `finalSuspicion = min(100, avgSuspicion × 1.5 + suspiciousQuestions × 10)`.
Также реализован `analyzeKeystrokePattern` (анализ ритма ввода, ≥10 интервалов) — ⚠️ нигде не вызывается (заготовка верификации личности).

### 6.3 Как телеметрия попадает на сервер и что с ней делают
- Отдельного эндпоинта событий **нет**: фронтенд (`frontend/app/candidate/test/page.tsx`) собирает `visibilitychange`→`tabSwitches`, `paste`→`copyPasteEvents`, `keydown`→`keystrokeCount`, `timeSpentSeconds` и передаёт объектом `behavioral` в `POST /api/testing/v2/session/:id/answer`;
- флаги по вопросу возвращаются в ответе (`behavioralFlags`), но **не персистятся** в `TestAnswer`;
- при `complete()` флаги статистического и поведенческого анализаторов объединяются и пишутся в **`TestSession.cheatFlags[]` и `TestSession.isSuspicious`** (`isSuspicious = v1.isSuspicious || behavioral.suspicionScore > 50`);
- ⚠️ на `complete()` поведенческие данные пересчитываются из сохранённых ответов с **нулевой** телеметрией (`copyPasteEvents: 0, tabSwitches: 0, ...` — комментарий «Would come from client»), т.е. сессионный анализ фактически видит только тайминги.

**Влияние на результат: отсутствует.** Score, грейд и рекомендация считаются независимо от анти-чита; флаги — только индикация (фронтенд показывает «Подозрительно»/«Честный проход»). Matching и работодатели эти поля не читают.

---

## 7. Побочные эффекты завершения теста

| Что | Эффект |
|-----|--------|
| `TestSession` | score, rawScore, maxScore=100, gradeBefore/After, recommendation, thetaEstimate, cheatFlags, isSuspicious |
| `CandidateProfile` | `verifiedGrade = gradeAfter`, `testStatus`, `lastTestDate`, `testCooldownUntil`, `lastTestScore` |
| Matching | `matching.invalidateCache()` + `indexing.indexCandidate(profileId)` (рейтинг пересчитывается с учётом `verifiedGrade` и `lastTestScore`) |
| Геймификация (только V2) | `updateStreak` → XP `TEST_COMPLETED` (100), при PASS/UPGRADE — `TEST_PASSED` (200) / `TEST_UPGRADED` (300); бейджи `FIRST_TEST`, `TEST_PASSED`, `GRADE_<gradeAfter>` |

---

## 8. Проверки качества тестирования

`scripts/metrics.ts` (раздел Testing quality) прогоняет 3 полные сессии через API и проверяет:
- **дискриминативность**: сильный профиль ≥ 80 баллов, слабый ≤ 30, разница θ ≥ 2, порядок грейдов корректен;
- **test-retest**: |Δscore| ≤ 10, грейд стабилен;
- **согласованность рекомендации**: слабый профиль → FAIL;
- **отсутствие ложных срабатываний анти-чита** на честном прохождении.

Unit-тесты: `cat.spec.ts` (IRT-математика, детерминизм выбора), `scoring.spec.ts`, `anti-cheat.spec.ts`, `behavioral-anti-cheat.spec.ts`, `testing-v2.service.spec.ts` (регрессия: повторный ответ возвращает `duplicate`, CAT не выдаёт один и тот же вопрос без учёта ответов).
