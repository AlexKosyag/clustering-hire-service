# 04. Справочник API

Содержание: [README](README.md) · [01](01-architecture.md) · [02](02-data-model.md) · [03](03-auth-and-security.md) · **04** · [05](05-matching-engine.md) · [06](06-testing-engine.md) · [07](07-fsp-gamification-notifications.md) · [08](08-infrastructure.md) · [09](09-known-issues.md)

Все пути указаны **с глобальным префиксом `/api`**. Авторизация — `Authorization: Bearer <accessToken>`.
Обозначения: 🔓 — `@Public()` (без токена); 🧑 — только `CANDIDATE`; 🏢 — только `EMPLOYER`; 🔑 — любой авторизованный.
Интерактивный Swagger: `GET /api/docs` (JSON: `/api/docs-json`).

Сводка:

| Раздел | Маршруты | Документ-раздел |
|--------|----------|-----------------|
| Auth | 5 | §1 |
| Candidates | 5 | §2 |
| Employers | 7 | §3 |
| Vacancies | 5 | §4 |
| Invitations | 6 | §5 |
| Matching | 6 | §6 (+ [05](05-matching-engine.md)) |
| Testing | 10 | §7 (+ [06](06-testing-engine.md)) |
| FSP | 4 | §8 |
| Gamification | 4 | §9 |
| Notifications | 1 | §10 |
| Health | 1 | §11 |

---

## 1. Auth — `/api/auth`
Контроллер: `src/modules/auth/auth.controller.ts`.

| Метод | Путь | Доступ | Запрос | Ответ |
|-------|------|--------|--------|-------|
| POST | `/api/auth/register` | 🔓 | `{email, password, role: CANDIDATE\|EMPLOYER}` | `201 {accessToken, refreshToken, user}` + cookie `refreshToken` |
| POST | `/api/auth/login` | 🔓 | `{email, password}` | `200 {accessToken, refreshToken, user}` + cookie |
| POST | `/api/auth/refresh` | 🔓 | `{refreshToken?}` или cookie | `200 {accessToken, refreshToken}` + новая cookie |
| POST | `/api/auth/logout` | 🔑 | — | `200 {message: 'Выход выполнен'}`, cookie очищается |
| GET | `/api/auth/me` | 🔑 | — | `user + candidateProfile/employerProfile` (без секретов) |

Ошибки: `400` (валидация пароля/роли), `401` (неверный пароль, невалидный refresh), `409` (email занят).
Подробности — [03-auth-and-security.md](03-auth-and-security.md).

---

## 2. Кандидат — `/api/candidates`
Контроллер: `src/modules/candidates/candidates.controller.ts`, классовый `@Roles(CANDIDATE)`.

| Метод | Путь | Запрос | Ответ / ошибки |
|-------|------|--------|----------------|
| GET | `/api/candidates/profile` | — | расширенный профиль (см. ниже); `404 Профиль не найден` |
| PUT | `/api/candidates/profile` | `UpdateCandidateProfileDto` (все поля опциональны) | профиль после переиндексации; `404` |
| GET | `/api/candidates/category` | — | статус тестирования (см. ниже); `404` |
| POST | `/api/candidates/profile/resume` | multipart `file` (PDF ≤ 5 МБ) | `{resumePdfUrl, parsed, autoFilled}`; `400` (не PDF / >5 МБ / нет файла), `404`, `503` (MinIO) |
| POST | `/api/candidates/profile/avatar` | multipart `file` (image/* ≤ 2 МБ) | `{avatarUrl}`; `400/404/503` |

**`UpdateCandidateProfileDto`**: `firstName/lastName/middleName` (≤60), `phone` (≤40), `city` (≤60), `bio` (≤4000), `specialization` (enum), `declaredGrade` (enum), `stack[]/roles[]/softSkills[]` (≤60 каждый), `experienceYears` (число ≥0), `showEmail/showPhone/showFullName/consentToProcessing` (bool).
⚠️ `verifiedGrade`/`testStatus` через DTO не меняются — их пишет только testing-модуль.

**Ответ `GET /api/candidates/profile`**: публичная часть (`id`, анонимизированный `displayName`, specialization, оба грейда, stack/roles/softSkills, experienceYears, city, bio, testStatus, testScore, lastTestDate, achievements, achievementPoints, lastActive, indexed) **плюс приватные поля владельца**: `email, phone, middleName, avatarUrl, fspId, fspConnectedAt, resumePdfUrl, resumeParsed, showEmail, showPhone, showFullName, consentToProcessing, testCooldownUntil, createdAt`.

**Ответ `GET /api/candidates/category`**: `{specialization, declaredGrade, verifiedGrade, testStatus, lastTestDate, lastTestScore, canRetakeAt, completedTests, achievements[], achievementPoints, fspId}`.

**Резюме** (`uploadResume`): загрузка PDF в MinIO (`resumes/{profileId}/resume.pdf`) → парсинг `pdf-parse` (email/телефон/город/навыки/опыт/образование/имя/summary, см. [05](05-matching-engine.md) и `pdf-parsing.service.ts`) → автозаполнение **только пустых** полей (имя разбивается на first/last/middle), навыки **домеряются** в `stack` (без перезаписи) → реиндексация эмбеддинга кандидата. Парсинг необязателен: его сбой не роняет загрузку.

---

## 3. Работодатель — `/api/employers`
Контроллер: `src/modules/employers/employers.controller.ts`, классовый `@Roles(EMPLOYER)`.

| Метод | Путь | Запрос | Ответ / ошибки |
|-------|------|--------|----------------|
| GET | `/api/employers/profile` | — | профиль компании; `404 Профиль работодателя не найден` |
| PUT | `/api/employers/profile` | `UpdateEmployerProfileDto` | профиль (+ инвалидация кэша matching); `404` |
| POST | `/api/employers/profile/logo` | multipart `file` (image/* ≤ 2 МБ) | `{logoUrl}`; `400/404/503` |
| POST | `/api/employers/need` | `EmployerNeedDto` | `{id, description, specialization, grade, stack, candidatesCount, topCandidates}`; `404` |
| GET | `/api/employers/candidates/search` | query `EmployerSearchQueryDto` | `{total, candidates[]}` с `matchScore`/`matchExplanation`; `404` |
| GET | `/api/employers/candidates/:id` | param — id профиля кандидата | карточка + гейтинг контактов; `404 Кандидат не найден` |
| GET | `/api/employers/dashboard` | — | `{stats, recentInvitations, vacancies, topCandidates}`; `404` |

**`EmployerNeedDto`** (из `matching/dto/matching.dto.ts`): `description` (10–4000, обязательно), `specialization?`, `grade?`, `stack[]?`.
**`EmployerSearchQueryDto`** = `SearchCandidatesDto` + трансформация `stack` из строки `"Node.js,PostgreSQL"` в массив. Поля: `description?` (≤4000), `specialization?`, `grade?`, `stack[]`, `hasAchievements?` (bool), `limit` (1–100, default 20), `offset` (≥0).

**Карточка кандидата (`/employers/candidates/:id`)** — ключевой сценарий приватности:
```json
{
  "...": "поля toCandidatePublic",
  "invitationStatus": "ACCEPTED",
  "contactsLocked": false,
  "contacts": { "fullName": "Иван Иванов", "email": "...", "phone": "...", "city": "Москва" },
  "contactsHint": "Кандидат принял приглашение — контакты открыты"
}
```
До принятия приглашения `contacts: null`, `contactsLocked: true`.

**Дашборд**: статистика приглашений по статусам, последние 10 приглашений и вакансий, топ-5 кандидатов по последней потребности (если есть).

---

## 4. Вакансии — `/api/vacancies`
Контроллер: `src/modules/vacancies/vacancies.controller.ts`.

| Метод | Путь | Доступ | Запрос | Ответ / ошибки |
|-------|------|--------|--------|----------------|
| GET | `/api/vacancies` | 🔓 | query: `specialization`, `grade`, `employerId`, `search`, `onlyActive`, `limit`, `offset` | `{total, vacancies[]}` (+ карточка работодателя) |
| GET | `/api/vacancies/:id` | 🔓 | param `id` | вакансия + работодатель; `404 Вакансия не найдена` |
| POST | `/api/vacancies` | 🏢 | `CreateVacancyDto` | созданная вакансия; `400` (salaryFrom > salaryTo), `404` |
| PUT | `/api/vacancies/:id` | 🏢 | `UpdateVacancyDto` | обновлённая; `403 Это не ваша вакансия`, `404` |
| DELETE | `/api/vacancies/:id` | 🏢 | param `id` | `200 {deleted: true}`; `403/404` |

**`CreateVacancyDto`**: `title` (3–160), `description` (20–10000), `specialization`, `grade` (enum), `stack[]` (≤60), `salaryFrom/salaryTo` (int ≥0), `isActive?` (bool, default true). `UpdateVacancyDto` — те же поля опциональны.

Правила (`vacancies.service.ts`):
- список: `limit` clamp 1–100 (default 20), `offset` ≥0, по умолчанию только `isActive: true` (отключается `onlyActive=false`), `search` — case-insensitive `contains` по title/description, сортировка `createdAt desc`;
- владелец проверяется при изменении/удалении (`403 Это не ваша вакансия`);
- удаление жёсткое; приглашения с этой вакансией остаются с `vacancyId = null`.
⚠️ Проверка `salaryFrom ≤ salaryTo` есть только при создании, не при обновлении.

---

## 5. Приглашения — `/api/invitations`
Контроллер: `src/modules/invitations/invitations.controller.ts`. Роли — на уровне маршрута.

| Метод | Путь | Доступ | Запрос | Ответ / ошибки |
|-------|------|--------|--------|----------------|
| POST | `/api/invitations` | 🏢 | `CreateInvitationDto` | приглашение + `candidate`; `404`, `400`, `409` (дубль) |
| GET | `/api/invitations` | 🔑 | query `status?` | `{invitations[]}` (≤100, скоуп по роли) |
| GET | `/api/invitations/:id` | 🔑 | param `id` | детали; **сайд-эффект: SENT → VIEWED** для кандидата; `404` |
| POST | `/api/invitations/:id/accept` | 🧑 | param `id` | обновлённое приглашение + сообщение; `404`, `409` |
| POST | `/api/invitations/:id/reject` | 🧑 | param `id` | то же; `404`, `409` |
| POST | `/api/invitations/:id/withdraw` | 🏢 | param `id` | `{status: WITHDRAWN}`; `403`, `409` |

**`CreateInvitationDto`**: `candidateId` (обязательно), `vacancyId?` (должна принадлежать работодателю, иначе `400 Вакансия не найдена у этого работодателя`), `message?` (≤4000), `salaryFrom/salaryTo` (int 0–10 000 000).

Бизнес-правила (`invitations.service.ts`):
- **дубль**: существующее приглашение той же пары в статусе `SENT|VIEWED|ACCEPTED` → `409 Кандидат уже приглашён по этому приглашению` (REJECTED/WITHDRAWN разрешают повтор);
- **зарплата**: `salaryFrom > salaryTo` → `400`;
- **обработка**: только из `SENT|VIEWED`, иначе `409 Приглашение уже обработано: <статус>`; accept возвращает «Приглашение принято — работодатель получил ваши контакты»;
- **отзыв**: только владельцем (`403 Это не ваше приглашение`), из `ACCEPTED` нельзя (`409 Принятое приглашение нельзя отозвать`);
- каждый переход инвалидирует кэш matching.

Полная статусная машина — [02-data-model.md](02-data-model.md) §6.

---

## 6. Matching — `/api/matching` (все маршруты 🏢)
Контроллер: `src/modules/matching/matching.controller.ts`. Алгоритмы — [05-matching-engine.md](05-matching-engine.md).

| Метод | Путь | Запрос | Ответ |
|-------|------|--------|-------|
| POST | `/api/matching/search` | `SearchCandidatesDto` | `{total, candidates[]}` (V1) |
| POST | `/api/matching/semantic` | `SemanticQueryDto` | `{candidates: [... + similarity, reason]}` (V1) |
| GET | `/api/matching/recommendations` | — | `{recommendations: [{reason, candidates[]}]}` |
| POST | `/api/matching/v2/search` | `SearchCandidatesDto` | `SearchResultV2` (расширенный) |
| POST | `/api/matching/v2/semantic` | `SemanticQueryDto` | `{queryUnderstanding, candidates[]}` |
| POST | `/api/matching/v2/need` | `EmployerNeedDto` | `{id, ..., queryUnderstanding, candidatesCount, topCandidates, timing}` |

**`SearchCandidatesDto`**: `description?` (≤4000), `specialization?`, `grade?`, `stack[]?`, `hasAchievements?` (bool), `limit` (1–100, default 20), `offset` (≥0).
**`SemanticQueryDto`**: `query` (3–2000, обязательно), `limit` (1–50, default 10).

**`SearchResultV2`** дополнительно содержит `matchScoreV2`, `matchBreakdown` (`{score, weight, contribution}`), `weightsSource`, `semanticSimilarity`, `rerankerScore`, `retrievalRank`, `finalRank`, блок `queryUnderstanding` и `timing {retrieval, scoring, reranking, total}`.

Рекомендации (`GET /api/matching/recommendations`): 2 группы — кандидаты, приглашённые конкурентами той же отрасли (кроме уже приглашённых), и «свежие» кандидаты (≤14 дней) по специализации последней потребности работодателя.

⚠️ `POST /api/employers/need` (см. §3) и `POST /api/matching/v2/need` — разные маршруты одного сценария «создать потребность».

---

## 7. Тестирование — `/api/testing` (все маршруты 🧑)
Контроллер: `src/modules/testing/testing.controller.ts`, классовый `@Roles(CANDIDATE)`.
Детали механики — [06-testing-engine.md](06-testing-engine.md).

### V1 (legacy)
| Метод | Путь | Запрос | Ответ / ошибки |
|-------|------|--------|----------------|
| POST | `/api/testing/start` | `StartTestDto` | `{sessionId, totalQuestions: 20, timeLimitMinutes: 60, specialization, gradeBefore, firstQuestion}`; `400` (кулдаун, нет теста, неполный банк), `409 Тестирование уже начато`, `404` |
| POST | `/api/testing/retake` | `RetakeDto` | то же (делегирует в start); ⚠️ `targetGrade` игнорируется |
| GET | `/api/testing/active` | — | `{session: {id, startedAt}\|null, canRetakeAt}` (просроченная сессия помечается TIMED_OUT) |
| GET | `/api/testing/session/:id/current` | — | `{sessionId, status, questionNumber, totalQuestions, currentScore, timeRemainingSeconds, isFinished, question}`; `403 Это чужая сессия` |
| POST | `/api/testing/session/:id/answer` | `SubmitAnswerDto` | `{correct, score, explanation, currentScore, theta, questionsRemaining, isFinished, nextQuestion}`; `400` (сессия завершена / время истекло / пустой ответ), `409 Ответ на этот вопрос уже записан`, `404` |
| POST | `/api/testing/session/:id/complete` | — | результат + `details`, `cooldownUntil`, `theta`, `antiCheat`; `400` (нельзя завершить / остались вопросы); идемпотентен для COMPLETED |

### V2 (основной, используется фронтендом)
| Метод | Путь | Запрос | Ответ |
|-------|------|--------|-------|
| POST | `/api/testing/v2/start` | `StartTestDto` | `{sessionId, totalTasks: 15, timeLimitMinutes: 90, taskDistribution, firstTask, ...}` |
| GET | `/api/testing/v2/session/:id/current` | — | `{sessionId, status, taskNumber, totalTasks, currentScore, timeRemainingSeconds, isFinished, task}` |
| POST | `/api/testing/v2/session/:id/answer` | `SubmitAnswerDto + {behavioral?}` | `{taskId, correct, score, explanation, currentScore, theta, tasksRemaining, isFinished, behavioralFlags, nextTask, duplicate}` (повторная отправка → `duplicate: true`) |
| POST | `/api/testing/v2/session/:id/complete` | — | `TestResultV2` (см. ниже) |

**`StartTestDto`**: `specialization` (обязательный enum), `declaredGrade?` (enum).
**`SubmitAnswerDto`**: `questionId`; `answer?` (≤20, ID варианта V1), `selectedOptionId?` (≤40), `answerText?` (≤20000), `code?`/`fixedCode?` (≤20000), `bugsFound?` (string[]), `designDescription?`/`diagram?` (≤20000), `timeSpentSeconds?` (int ≥0), `behavioral?` (`Record<string, number>` — телеметрия анти-чита).

**`TestResultV2`**: `{sessionId, status, score, gradeBefore, gradeAfter, recommendation, theta, taskBreakdown[], antiCheat: {isSuspicious, flags, confidence, behavioralAnalysis}, cooldownUntil, gamification: {xp, level, levelName, streak} | null}`.

---

## 8. ФСП — `/api/fsp`
Контроллер: `src/modules/fsp/fsp.controller.ts`.

| Метод | Путь | Доступ | Запрос | Ответ |
|-------|------|--------|--------|-------|
| POST | `/api/fsp/connect` | 🧑 | `{fspId}` (regex `^[A-Za-z0-9_-]{4,40}$`) | `{fspId, connected, achievementsFound, achievements, note}` |
| POST | `/api/fsp/disconnect` | 🧑 | — | `200`, достижения удаляются |
| GET | `/api/fsp/achievements` | 🧑 | — | достижения + суммарные баллы |
| GET | `/api/fsp/catalog` | 🔑 | — | каталог достижений (сортировка по category, points desc) |

Ошибки: `400` (невалидный fspId), `404` (нет профиля), `409` (fspId привязан к другому профилю).
Интеграция реализована как детерминированный mock — см. [07](07-fsp-gamification-notifications.md).

---

## 9. Геймификация — `/api/gamification`
Контроллер: `src/modules/gamification/gamification.controller.ts`.

| Метод | Путь | Доступ | Запрос | Ответ |
|-------|------|--------|--------|-------|
| GET | `/api/gamification/profile` | 🧑 | — | XP, уровень, стрик, бейджи, прогресс |
| GET | `/api/gamification/badges` | 🧑 | — | все бейджи с `earnedAt` |
| GET | `/api/gamification/leaderboard` | 🔑 | query `type=global\|specialization`, `specialization?`, `limit` (default 20) | рейтинг кандидатов |
| GET | `/api/gamification/xp-history` | 🧑 | query `limit` (default 50) | журнал XP (новые сверху) |

⚠️ Модель `DailyTask` существует в БД, но эндпоинтов ежедневных заданий нет.
Детали формул — [07-fsp-gamification-notifications.md](07-fsp-gamification-notifications.md).

---

## 10. Уведомления — `/api/notifications`

| Метод | Путь | Доступ | Запрос | Ответ |
|-------|------|--------|--------|-------|
| GET | `/api/notifications` | 🔑 | query `limit` (1–100, default 20) | `{items[], total}` |

Типы элементов: `INVITATION_RECEIVED`, `INVITATION_STATUS`, `TEST_RESULT`. Лента собирается на лету из `Invitation`/`TestSession` (таблицы уведомлений нет, статус «прочитано» — производный). Подробнее — [07](07-fsp-gamification-notifications.md).

---

## 11. Диагностика — `/api/health`

| Метод | Путь | Доступ | Ответ |
|-------|------|--------|-------|
| GET | `/api/health` | 🔓 | `{status: 'ok'\|'degraded', timestamp, uptimeSeconds, database: 'up'\|'down', redis: 'up'\|'down (cache disabled)', embedding: {provider, model, dim}}` |

Используется как Docker HEALTHCHECK (`curl -sf http://localhost:3000/api/health`).
Отдельного readiness-эндпоинта (`/ready`) нет.
