# 02. Модель данных

Содержание: [README](README.md) · [01](01-architecture.md) · **02** · [03](03-auth-and-security.md) · [04](04-api-reference.md) · [05](05-matching-engine.md) · [06](06-testing-engine.md) · [07](07-fsp-gamification-notifications.md) · [08](08-infrastructure.md) · [09](09-known-issues.md)

Источник истины: `backend/prisma/schema.prisma` (PostgreSQL, Prisma 5).
Все имена таблиц snake_case (`@@map`), полей БД — `@map`, в TypeScript используются camelCase-имена.

---

## 1. Общая схема (ER-диаграмма текстом)

```
User 1───1 CandidateProfile 1───* TestSession 1───* TestAnswer *───1 TestQuestion *───1 TestCategory
  │              │  │  │                │
  │              │  │  │                └── cheatFlags/isSuspicious, θ, score, gradeBefore/After
  │              │  │  └──* CandidateAchievement *───1 FSPAchievement
  │              │  └──* DailyTask (только схема)
  │              └──1 GamificationProfile 1───* CandidateBadge *───1 Badge
  │                          └──* XpEvent
  │
  └───1 EmployerProfile 1───* Vacancy 1───(0..1)── Invitation
                  │                              ▲
                  ├──* Invitation ───────────────┘
                  └──* EmployerNeed   (needEmbedding vector(384))

CandidateProfile 1───* Invitation (relation "CandidateInvitations")
CandidateProfile.profileEmbedding vector(384)  — raw SQL
```

---

## 2. Enum

### `UserRole`
`CANDIDATE` | `EMPLOYER` | `ADMIN`
Регистрацией доступны только CANDIDATE и EMPLOYER; ADMIN создаётся вручную в БД. ⚠️ Явных ADMIN-маршрутов в коде нет.

### `Specialization` (13 значений)
`BACKEND`, `FRONTEND`, `FULLSTACK`, `MOBILE`, `DEVOPS`, `DATA_SCIENCE`, `QA`, `SECURITY`, `EMBEDDED`, `GAME_DEV`, `ML_AI`, `DESIGN_UX`, `MANAGEMENT`

### `Grade` (6 значений, порядок значим)
`JUNIOR` (0) < `MIDDLE` (1) < `MIDDLE_PLUS` (2) < `SENIOR` (3) < `LEAD` (4) < `PRINCIPAL` (5)
Порядок используется в matching («±1 грейд») и в тестировании (маппинг θ → грейд).

### `TestStatus` (статус тестирования кандидата)
`NOT_STARTED` | `IN_PROGRESS` | `COMPLETED` | `FAILED` | `PASSED` | `UPGRADED`
Маппинг из рекомендации теста: `FAIL → FAILED`, `UPGRADE → UPGRADED`, иначе `PASSED`.

### `InvitationStatus`
`SENT` | `VIEWED` | `ACCEPTED` | `REJECTED` | `WITHDRAWN` — см. §6.

### `SessionStatus` (сессия тестирования)
`IN_PROGRESS` | `COMPLETED` | `TIMED_OUT` | `ABANDONED`
⚠️ `ABANDONED` нигде не выставляется кодом.

### `DailyTaskStatus`
`PENDING` | `IN_PROGRESS` | `COMPLETED` | `SKIPPED` | `EXPIRED` — для нереализованной функции ежедневных заданий.

---

## 3. Модели по группам

### 3.1 Пользователи

#### `User` → таблица `users`
| Поле | Тип | Описание |
|------|-----|----------|
| `id` | uuid PK | |
| `email` | text unique | логин |
| `passwordHash` | text | bcrypt (cost 12) |
| `role` | `UserRole` | |
| `isEmailVerified` | bool = false | ⚠️ всегда false — верификация не реализована |
| `emailVerifyToken` | text? | ⚠️ генерируется, не используется |
| `refreshToken` | text? | **bcrypt-хеш** текущего refresh-токена (ротация при refresh) |
| `createdAt` / `updatedAt` | timestamp | |

Связи 1:1 с `CandidateProfile` и `EmployerProfile` (каскадное удаление).

### 3.2 Кандидат

#### `CandidateProfile` → `candidate_profiles`
| Группа полей | Поля | Примечание |
|--------------|------|-----------|
| Личные | `firstName`, `lastName`, `middleName`, `phone`, `city`, `bio` (text), `avatarUrl` | заполняются вручную или парсером резюме |
| Профессиональные | `specialization`, `declaredGrade` (самооценка), `verifiedGrade` (подтверждён тестом), `stack[]`, `roles[]`, `experienceYears` (float), `softSkills[]` | `verifiedGrade` пишет только testing-модуль |
| ФСП | `fspId` (unique), `fspConnectedAt` | привязка аккаунта ФСП |
| Тестирование | `testStatus`, `lastTestDate`, `testCooldownUntil`, `lastTestScore` | кулдаун пересдачи 90 дней |
| Резюме | `resumePdfUrl`, `resumeParsed` (Json) | результат парсинга PDF |
| Приватность | `showEmail` (false), `showPhone` (false), `showFullName` (true), `consentToProcessing` (false) | анонимизация `displayName` |
| Matching | `lastMatchScore` (float?) | ⚠️ никогда не заполняется |
| Вектор | `profileEmbedding Unsupported("vector(384)")` → `profile_embedding` | пишется raw SQL, см. §5 |

Индексы: `[specialization, verifiedGrade]`, `[testStatus]`, `[fspId]`.
Связи: `user`, `testSessions`, `achievements` (`CandidateAchievement`), `invitations`, `dailyTasks`, `gamification`.

### 3.3 Работодатель и вакансии

#### `EmployerProfile` → `employer_profiles`
`companyName`, `companyDescription`, `industry`, `website`, `logoUrl`, `contactPerson`, `contactEmail`, `contactPhone`, `isVerified` (⚠️ не используется), `verifiedAt`, timestamps. Связи: `vacancies`, `invitations`, `needs`.

#### `Vacancy` → `vacancies`
`title`, `description`, `specialization`, `grade`, `stack[]`, `salaryFrom`/`salaryTo` (int), `isActive` (true). Индексы `[specialization, grade]`, `[isActive]`. При удалении вакансии связанные приглашения получают `vacancyId = null` (`onDelete: SetNull`).

#### `EmployerNeed` → `employer_needs`
Потребность работодателя (текстовый запрос для matching): `description`, `specialization?`, `grade?`, `stack[]`, `needEmbedding vector(384)` → `need_embedding`. Индекс `[employerId, createdAt]`.

### 3.4 Приглашения

#### `Invitation` → `invitations`
| Поле | Тип | Описание |
|------|-----|----------|
| `employerId`, `candidateId` | FK | каскадное удаление |
| `vacancyId` | FK? | `onDelete: SetNull` |
| `message` | text? | сопроводительный текст |
| `salaryFrom`, `salaryTo` | int | предложение по зарплате |
| `status` | `InvitationStatus` = `SENT` | статусная машина, см. §6 |
| `statusUpdatedAt` | timestamp | обновляется при каждом переходе |

Индексы `[candidateId, status]`, `[employerId, status]`.
**Дубли**: ограничение на уровне бизнес-логики (не БД) — повторное приглашение пары employer↔candidate в статусе SENT/VIEWED/ACCEPTED запрещено (409).

### 3.5 Тестирование

#### `TestCategory` → `test_categories`
`name` (unique), `description?`, `specialization`. В сиде — 13 категорий (по специализациям).

#### `TestQuestion` → `test_questions`
| Поле | Описание |
|------|----------|
| `categoryId` | FK, каскадное удаление |
| `questionType` | строка: `multiple_choice`, `coding`, `open_text`, `scenario` |
| `text`, `codeSnippet?`, `options?` (Json), `correctAnswer?`, `explanation?` | содержимое вопроса; `options` — массив `{id, text, isCorrect}` |
| `difficulty` (0.5), `discrimination` (1.0), `guessing` (0.25) | параметры IRT-3PL |
| `isUnique` (false), `poolTag?` | анти-чит: уникальные вопросы, группировка пулов |

Индекс `[categoryId, difficulty]`. В сиде ≈858 вопросов (13 категорий × 66).

#### `TestSession` → `test_sessions`
| Поле | Описание |
|------|----------|
| `candidateId`, `categoryId` | FK (кандидат — каскад) |
| `status` (`IN_PROGRESS`), `startedAt`, `completedAt?`, `timeLimitMinutes` (60) | V1: 60 мин, V2: 90 мин |
| `score`, `rawScore`, `maxScore` | итоговые баллы (0–100) |
| `gradeBefore`, `gradeAfter` | грейд до/после теста |
| `recommendation` | `PASS` \| `FAIL` \| `UPGRADE` |
| `cheatFlags String[]`, `isSuspicious Boolean` | результат анти-чита |
| `currentDifficulty` (0.5), `thetaEstimate Float?` | состояние CAT (θ ∈ [−4, 4]) |

Индекс `[candidateId, status]`.

#### `TestAnswer` → `test_answers`
`sessionId`, `questionId`, `answerText?`, `isCorrect?`, `score?`, `timeSpentSeconds?`, `answeredAt`.
**`@@unique([sessionId, questionId])`** — защита от повторных ответов на уровне БД.

### 3.6 ФСП

#### `FSPAchievement` → `fsp_achievements`
Каталог достижений: `name`, `description?`, `category` (`competition` | `event` | `certification`), `year?`, `place?`, `points` (10–90). Индекс `[category, year]`. В сиде 39 записей.

#### `CandidateAchievement` → `candidate_achievements`
Связь «кандидат ↔ достижение»: `candidateId`, `achievementId`, `verifiedAt`. **`@@unique([candidateId, achievementId])`** — идемпотентное связывание (upsert).

### 3.7 Геймификация

#### `GamificationProfile` → `gamification_profiles`
`candidateId` (unique), `xp` (0), `level` (1), `streak` (0), `longestStreak` (0), `lastActiveDate?`, `multiplier` (1.0), timestamps.

#### `Badge` → `badges`
`code` (unique), `name`, `description`, `icon`, `category` (`testing|profile|activity|fsp`), `rarity` (`common|rare|epic|legendary`), `xpReward`. 18 бейджей в коде (`gamification.service.ts` → `BADGE_DEFINITIONS`).

#### `CandidateBadge` → `candidate_badges`
`candidateId`, `badgeId`, `earnedAt`; `@@unique([candidateId, badgeId])` — бейдж выдаётся один раз.

#### `XpEvent` → `xp_events`
Журнал начислений: `candidateId`, `action`, `xp`, `metadata?` (Json), `createdAt`. Индекс `[candidateId, createdAt]`.

### 3.8 Ежедневные задания (концепция)

#### `DailyTask` → `daily_tasks`
`candidateId`, `title`, `description`, `difficulty` (`easy|medium|hard`), `expectedTimeMinutes`, `status` (`DailyTaskStatus`), `candidateAnswer?`, `score?`, `assignedAt`, `dueAt`, `completedAt?`.
⚠️ Таблица создаётся миграцией, но **сервисной логики нет** (только очистка в сиде).

---

## 4. Хранение векторов (pgvector)

- Расширение: `CREATE EXTENSION IF NOT EXISTS vector` (в `0_init/migration.sql`).
- Колонки: `candidate_profiles.profile_embedding vector(384)`, `employer_needs.need_embedding vector(384)`.
- Prisma объявляет их как `Unsupported("vector(384)")` — **чтение/запись только raw SQL** через `VectorRepository` (`src/modules/matching/vector.repository.ts`):
  - `setCandidateVector(id, vec)` / `setNeedVector(id, vec)` — `UPDATE ... SET ... = '[...]'::vector`;
  - `topBySimilarity(vec, limit)` — KNN: `ORDER BY profile_embedding <=> '[...]'::vector LIMIT n`, similarity = `1 - cosine_distance`;
  - fallback `topBySimilarityInMemory` — при ошибке SQL читает векторы текстом и сортирует в памяти (dot-product, векторы L2-нормализованы).
- ⚠️ **ANN-индекс (ivfflat/hnsw) не создаётся** — поиск идёт последовательным сканом; выборка ограничена (до 3000 кандидатов).

## 5. Индексы и ограничения (сводно)

| Таблица | Индексы / constraints |
|---------|----------------------|
| `users` | unique `email` |
| `candidate_profiles` | unique `user_id`, `fsp_id`; idx `[specialization, verified_grade]`, `[test_status]`, `[fsp_id]` |
| `employer_profiles` | unique `user_id` |
| `vacancies` | idx `[specialization, grade]`, `[is_active]` |
| `employer_needs` | idx `[employer_id, created_at]` |
| `invitations` | idx `[candidate_id, status]`, `[employer_id, status]` |
| `test_questions` | idx `[category_id, difficulty]` |
| `test_sessions` | idx `[candidate_id, status]` |
| `test_answers` | **unique `[session_id, question_id]`**, idx `[session_id]` |
| `candidate_achievements` | **unique `[candidate_id, achievement_id]`** |
| `candidate_badges` | **unique `[candidate_id, badge_id]`** |
| `badges` | unique `code` |
| `xp_events` | idx `[candidate_id, created_at]` |
| `daily_tasks` | idx `[candidate_id, status]` |

## 6. Статусная машина приглашений

```
                     ┌──────────────────────────────────────────┐
                     │  (просмотр кандидатом GET /invitations/:id)│
 create ──► SENT ────┴──► VIEWED                                │
   │                   │                                        │
   │                   ├── POST /accept (кандидат) ──► ACCEPTED  │  терминальный
   │                   │                          (контакты открыты; отозвать нельзя)
   │                   └── POST /reject (кандидат) ──► REJECTED ─┐
   │                                                             │
   └── POST /withdraw (работодатель) ──► WITHDRAWN ◄─────────────┘
        (доступно из SENT/VIEWED/REJECTED, из ACCEPTED — нельзя)
```

Правила:
- обрабатываются (accept/reject) только статусы `SENT` и `VIEWED`; иначе `409 Приглашение уже обработано`;
- `ACCEPTED` терминальный: нельзя отозвать (`409 Принятое приглашение нельзя отозвать`);
- **истечения срока нет** (нет статуса EXPIRED, нет cron/таймеров);
- контакты кандидата раскрываются работодателю **только в статусе ACCEPTED**.

## 7. Миграции

| Миграция | Содержание |
|----------|-----------|
| `0_init` | полная схема + `CREATE EXTENSION vector`, колонки `vector(384)` |
| `20261008000000_remove_applications` | удаление сущности откликов (Applications) — взаимодействие кандидат и работодателя только через `Invitation` |
| `20261008214509_add_gamification` | таблицы геймификации (`gamification_profiles`, `badges`, `candidate_badges`, `xp_events`) |

Применение: `npx prisma migrate deploy` (в Docker — автоматически при старте контейнера, см. [08-infrastructure.md](08-infrastructure.md)).

## 8. Seed-данные (`prisma/seed.ts` + `prisma/seed/*`)

Сид **детерминированный** (mulberry32 PRNG, `prisma/seed/rng.ts`), фиксированные сиды: кандидаты `20261005`, вакансии `424242`, приглашения `151515`.

| Данные | Объём | Комментарий |
|--------|-------|-------------|
| Категории + вопросы | 13 категорий × 66 ≈ **858 вопросов** | 60 MCQ (12 концепций × 5 шаблонов, difficulty 0.2–0.8), 3 `coding`, 2 `open_text`, 1 `scenario`; IRT-параметры `discrimination` 0.9–1.7, `guessing` 0.25 |
| Кандидаты | **204** (3 демо + 201 сгенерированный) | полный спектр грейдов, 70% с тестом, 25% с ФСП, 35% с достижениями |
| Работодатели | **8** | вкл. демо `test@employer.com` (ООО «ТехноСтар») |
| Вакансии | **30** | вилки зарплат по грейдам (JUNIOR 120–180k … PRINCIPAL 600–900k ₽) |
| Приглашения | **15** | по всем статусам, без дублей пар |
| Потребности (needs) | **6** | BACKEND, DEVOPS, FRONTEND, ML_AI, QA, DATA_SCIENCE |
| Каталог ФСП | **39** достижений | категории competition/event/certification |
| Сессии тестов | синтетические завершённые для протестированных кандидатов | θ по грейдам: JUNIOR −1.6 … PRINCIPAL 1.8 |

Пароль всех демо-аккаунтов — `Test123!` (bcrypt cost 10). `cleanup()` удаляет данные в безопасном порядке (ответы → сессии → вопросы → … → пользователи).
Команды: `npm run db:seed`, `npm run db:reset` (migrate reset --force), `npm run seed:embeddings` (предзаполнение векторов TF-IDF).
