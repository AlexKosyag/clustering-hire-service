# 09. Известные проблемы, рассинхроны и заглушки

Содержание: [README](README.md) · [01](01-architecture.md) · [02](02-data-model.md) · [03](03-auth-and-security.md) · [04](04-api-reference.md) · [05](05-matching-engine.md) · [06](06-testing-engine.md) · [07](07-fsp-gamification-notifications.md) · [08](08-infrastructure.md) · **09**

Сводка найденных при разборе кода отклонений от заявленного поведения. Раздел пригодится при развитии сервиса, чтобы не размножать ошибки. Пути — относительно `backend/`.

---

## 1. Matching и эмбеддинги

### ⚠️ M1. Рассинхрон размерности эмбеддингов (768 vs 384) — влияет на качество V2
- **Где**: `ml-service/main.py` (`EMBEDDING_DIM = 768`, модель `paraphrase-multilingual-mpnet-base-v2`), `src/modules/matching/ml-client.service.ts` (`tfidfEmbed` — тоже 768), `src/modules/matching/vector.repository.ts` (`literal()` фиксирует 384), `prisma/schema.prisma` (`vector(384)`).
- **Что происходит**: вектор запроса из ML-сервиса **молча усекается** до 384 компонент (`literal()` берёт первые 384, остальное обнуляет) и сравнивается с векторами кандидатов, построенными **другой моделью** (`@xenova/transformers`, MiniLM, 384-dim). Косинусная схожесть между разными пространствами бессмысленна.
- **Эффект**: в V2 семантический компонент и retrieval-выборка при живом ML-сервисе некорректны. V1 внутренне консистентен (обе стороны через `EmbeddingService`).
- **Варианты исправления**: привести ml-service к 384-dim моделей MiniLM либо перевести pgvector-колонки на 768 и единый источник векторов.

### ⚠️ M2. `matchBreakdown.score` всегда 0
- **Где**: `src/modules/matching/matching-v2.service.ts` (сборка breakdown).
- **Что происходит**: при заполнении `matchBreakdown[key].score` используются ключи вкладов (`'specialization'`, `'stack'`, …), тогда как `score.components` содержит ключи вида `'specializationMatch'`, `'stackOverlap'` — чтение возвращает `undefined` → 0. `weight`/`contribution` считаются верно.

### ⚠️ M3. Fan-out rerank-запросов
- **Где**: `src/modules/matching/scoring-v2.service.ts` → `getRerankBoost`.
- **Что происходит**: HTTP-вызов `mlClient.rerank` выполняется **на каждого кандидата** внутри `calculateMatch` (пул до 3000) — потенциально тысячи запросов к ML-сервису на один поиск, **плюс** отдельный батчевый rerank в `matching-v2.service.ts`. Стоимость поиска резко возрастает при доступном ML.

### ⚠️ M4. Двойной счёт requiredSkills в скоринге V2
- **Где**: `matching-v2.service.ts` (передача `need.stack = parsedNeed.requiredSkills` и одновременно `requiredSkills` в `calculateMatch`).
- **Что происходит**: скиллы из `requiredSkills` учитываются и с весом ×1.5, и ×1.0 (в `totalWeight`/`matchedWeight`) — искажает долю стека.

### ⚠️ M5. `getWeight` показывает `rerank: 0.10`, которого нет в весах
`DEFAULT_WEIGHTS` не содержит `rerank`; фактически rerank — аддитивный буст до +10 баллов, а не взвешенная компонента. Отображение вводит в заблуждение.

### M6. Отсутствие ANN-индекса и префильтрации
- Нет `ivfflat`/`hnsw` на `profile_embedding`/`need_embedding` — KNN делает последовательный скан (`vector.repository.ts`).
- Векторная выборка top-3000 **глобальная**: жёсткие фильтры (специализация/грейд) применяются после, пересечение в JS — при узких фильтрах результат может быть меньше `limit`.

### M7. Мёртвое поле `CandidateProfile.lastMatchScore`
Объявлено («последний рассчитанный match score»), но matching-подсистема его никогда не пишет.

### M8. Общий неймспейс кэша
V1 и V2 используют `matching:cache:*` с префиксами `search`/`search-v2`; любая инвалидация `delByPattern('matching:cache:*')` стирает кэш обеих версий целиком.

---

## 2. Тестирование и анти-чит

### ⚠️ T1. Поведенческая телеметрия не персистится
- **Где**: `src/modules/testing/testing-v2.service.ts` (`complete()`, пересчёт behavioral с `copyPasteEvents: 0, tabSwitches: 0, keystrokeCount: 0, idleTimeSeconds: 0` — комментарий «Would come from client»).
- **Эффект**: флаги по вопросам возвращаются клиенту в `behavioralFlags`, но не сохраняются; сессионный поведенческий анализ на `complete()` видит только тайминги. Сигналы paste/tab-switch по факту не влияют на итоговый `isSuspicious`.

### T2. Скрытые тесты в песочнице — заглушка
- **Где**: `src/modules/testing/services/code-sandbox.service.ts` → `generateHiddenTests`.
- **Что происходит**: входы генерируются детерминированно (LCG), но `expectedOutput` не заполнен (`hidden_${i}`) — такие тесты практически всегда «падают», поэтому `score` по code_execution занижен и нестабилен. В коде прямой комментарий, что в проде ожидаемые выходы должны браться из БД.

### T3. Песочница не изолирована
`SANDBOX_DOCKER` и `SANDBOX_MEMORY_MB` читаются, но Docker-режим и лимит памяти **не реализованы**; код исполняется через `spawn` на хосте с таймаутом и лимитом вывода. `sandboxUsed` в отчёте лишь отражает флаг.

### T4. Анти-чит не влияет на результат
Score/грейд/рекомендация считаются независимо от `cheatFlags`/`isSuspicious` (см. [06-testing-engine.md](06-testing-engine.md) §6.3). Работодатели и matching эти поля не читают — «анти-чит» сегодня только информирует кандидата.

### T5. Расхождение источников конфигурации тестов
V1 берёт `testing.questionsPerTest` (20) и `testing.cooldownDays` (90) из `configuration.ts`, V2 — из env `TESTING_TASKS_PER_TEST` (15) и `TESTING_COOLDOWN_DAYS` (90) напрямую (`testing-v2.service.ts`). Единой конфигурации нет.

### T6. Распределение заданий V2 не учитывает реальную длину теста
`task-generator.service.ts` строит слоты от `totalTasks = 20`, тогда как фактический тест — 15 заданий; тип задания выбирается по позиции (`index % shuffled.length`), а не по `questionType` вопроса.

### T7. Кулдаун не ставится при истечении времени
`testCooldownUntil` пишется только в `complete()`. Сессия, истёкшая по времени и не завершённая явно, кулдауна не ставит (новый `start` лишь помечает её `TIMED_OUT`).

### T8. Мелочи
- `RetakeDto.targetGrade` принимается, но игнорируется (`testing.service.ts` → `retake`).
- `SessionStatus.ABANDONED` не используется нигде.
- `analyzeKeystrokePattern` (анализ ритма ввода) реализован и покрыт тестами, но не вызывается.
- `categoryForSpecialization` при отсутствии категории специализации берёт **любую** категорию — кандидат может получить тест по чужой специализации.

---

## 3. Геймификация и уведомления

### ⚠️ G1. XP за бейджи не начисляется
`awardBadge` вызывает `awardXp('BADGE_<code>')`, но таких ключей нет в `XP_REWARDS` → начисляется 0 XP. Поле `Badge.xpReward` декоративное.

### G2. `awardXp` не идемпотентен
Нет ключа дедупликации событий — повторный вызов начисляет XP повторно (в отличие от `awardBadge` и связывания достижений ФСП, защищённых unique-ограничениями).

### G3. `seedBadges()` без вызывающего кода
Определения 18 бейджей есть в `gamification.service.ts`, но их upsert не вызывается из сида/старта — таблица `badges` заполняется только миграцией или вручную.

### G4. Некорректный прогресс уровня
`toGamificationProfile` читает несуществующее поле `xpRequired`: `xpProgress` вырождается в 100, `xpToNextLevel` возвращает абсолютный порог, а не остаток.

### G5. Ежедневные задания — только схема
Модель `DailyTask`, enum `DailyTaskStatus` и XP-константа `DAILY_TASK_DONE` созданы, но сервисов/эндпоинтов нет (в сиде только очистка).

### G6. Большинство XP-действий не вызываются
Реально начисляются только `TEST_COMPLETED/TEST_PASSED/TEST_UPGRADED` (в `testing-v2.complete()`). `INVITATION_*`, `FSP_CONNECTED`, `PROFILE_COMPLETE`, `FIRST_LOGIN`, `STREAK_*` — без вызовов.

### G7. Уведомления: статус «прочитано» не хранится
Read-model выводит `read` по правилам (см. [07](07-fsp-gamification-notifications.md) §3); нет ни отметки прочтения, ни канала доставки (SMTP — заглушка).

---

## 4. Приватность и безопасность

### ⚠️ S1. Флаги приватности не проверяются при раскрытии контактов
`EmployersService.candidateDetail` и `InvitationsService.get` отдают email/телефон/ФИО при `Invitation.status === ACCEPTED`, **игнорируя** `showEmail`/`showPhone`/`showFullName`. Принятие приглашения открывает всё независимо от настроек.

### S2. Нет rate-limiting и защиты от brute-force
Redis используется только для кэша; попытки подбора пароля/регистрации ничем не ограничены.

### S3. Верификация e-mail не реализована
`isEmailVerified` всегда `false`, `emailVerifyToken` генерируется, но письма не отправляются и эндпоинта подтверждения нет. Восстановления пароля тоже нет.

### S4. Одна refresh-сессия на пользователя
Хранится один bcrypt-хеш `users.refreshToken`; список устройств/сессий невозможен, детекции повторного использования токена нет (replay → просто 401).

### S5. Проверка зарплаты приглашения/вакансии
`salaryFrom ≤ salaryTo` проверяется при создании вакансии и приглашения, но **не** при обновлении вакансии (`vacancies.service.ts` → `update`).

---

## 5. Файлы и прочее

### F1. MinIO: публичные URL без presigned
`storage.service.ts` конкатенирует `MINIO_PUBLIC_ENDPOINT` + публичный путь к объекту; скачивания/удаления нет — «удаление» резюме/аватара лишь перезаписывает поле профиля, файл остаётся в бакете.

### F2. Мёртвые поля и код (сводно)
| Элемент | Где | Статус |
|---------|-----|--------|
| `User.isEmailVerified`, `emailVerifyToken` | `schema.prisma` | не используются |
| `EmployerProfile.isVerified`, `verifiedAt` | `schema.prisma` | не используются |
| `CandidateProfile.lastMatchScore` | `schema.prisma` | не заполняется |
| `SessionStatus.ABANDONED` | `schema.prisma` | не выставляется |
| `DailyTask` (вся группа) | `schema.prisma` | только схема |
| `RetakeDto.targetGrade` | `dto/testing.dto.ts` | игнорируется |
| `CreateNeedDto` | `dto/employer.dto.ts` | импортируется в контроллере, но не используется (применяется `EmployerNeedDto` из matching) |
| `MatchingService.createNeed` (V1) | `matching.service.ts` | нет HTTP-маршрута |
| `behavioral-anti-cheat.analyzeKeystrokePattern` | services | не вызывается |
| `SPEC_OFFSET` (поправки θ по специализациям) | `services/scoring.service.ts` | пустой объект |
| `ml-client.tfidfVocab/tfidfIdf` | `ml-client.service.ts` | никогда не заполняются |
| `buildCookie` | `common/utils/cookie.util.ts` | не используется (контроллер использует `res.cookie`) |
| `Badge.xpReward` | `gamification.service.ts` | не начисляется (G1) |

### F3. Сид: недетерминированные timestamp-ы
`prisma/seed.ts` использует `Math.random()` для `createdAt`/`statusUpdatedAt` приглашений и потребностей — остальная часть сида детерминирована (mulberry32).

---

## 6. Приоритет для исправления (рекомендация)

| Приоритет | Проблемы | Почему |
|-----------|----------|--------|
| Высокий | M1 (768/384), M3 (fan-out rerank), S1 (приватность) | влияют на качество ядра сервиса (matching) и доверие пользователей |
| Средний | T1/T2 (анти-чит и скрытые тесты), M2/M4 (скоринг V2), G1 (XP за бейджи), S5 | искажают заявленную механику |
| Низкий | G4/G5/G6, F2, M7, T5/T6 | косметика и нереализованные концепции |
