# 07. ФСП, геймификация, уведомления и файлы

Содержание: [README](README.md) · [01](01-architecture.md) · [02](02-data-model.md) · [03](03-auth-and-security.md) · [04](04-api-reference.md) · [05](05-matching-engine.md) · [06](06-testing-engine.md) · **07** · [08](08-infrastructure.md) · [09](09-known-issues.md)

---

## 1. Интеграция с ФСП (`src/modules/fsp/`)

**Что такое ФСП** — внешний сервис достижений кандидатов (олимпиады, хакатоны, сертификаты, мероприятия). Интеграция реализована как **детерминированный mock**: вместо вызова внешнего API `FspService.mockFetchAchievements()` генерирует набор достижений из локального каталога.

### Эндпоинты (см. [04-api-reference.md](04-api-reference.md) §8)
`POST /api/fsp/connect`, `POST /api/fsp/disconnect`, `GET /api/fsp/achievements`, `GET /api/fsp/catalog`.

### Логика `connect(userId, fspId)`
1. Профиль кандидата по `userId` → `404` при отсутствии;
2. нормализация/валидация `fspId` (regex `^[A-Za-z0-9_-]{4,40}$`); проверка уникальности (`CandidateProfile.fspId` — unique) → `409`, если привязан к другому профилю;
3. транзакция: `fspId`, `fspConnectedAt = now()` + upsert `CandidateAchievement` (сопоставление по названию из каталога, `update: {}` → **идемпотентно**);
4. фоновые сайд-эффекты: `matching.invalidateCache()` + `matching.reindexCandidate(profileId)` (достижения входят в текст эмбеддинга и в `achievementBoost`);
5. ответ: `{fspId, connected, achievementsFound, achievements, note}`.

`disconnect` — обнуляет `fspId/fspConnectedAt`, удаляет все `CandidateAchievement`, инвалидирует кэш.

### Mock-генерация
FNV-1a хеш от `fspId` → LCG (`seed = seed×1664525 + 1013904223`) → детерминированный выбор **2–4 достижений** из каталога (сортировка по `points desc`); `year` по умолчанию 2024. Один и тот же `fspId` всегда даёт одинаковый набор — удобно для демо.

### Каталог (`FSPAchievement`)
39 записей (`prisma/seed/achievements.ts`): категории `competition` | `event` | `certification`, баллы 10–90. Баллы достижений участвуют в matching (`achievementBoost`) и в профиле кандидата (`achievementPoints = Σ points`).

---

## 2. Геймификация (`src/modules/gamification/`)

Модели: `GamificationProfile`, `Badge`, `CandidateBadge`, `XpEvent` (см. [02-data-model.md](02-data-model.md) §3.7).

### Эндпоинты
| Путь | Доступ | Содержимое |
|------|--------|-----------|
| `GET /api/gamification/profile` | 🧑 | XP, уровень, стрик, множитель, бейджи, прогресс до следующего уровня |
| `GET /api/gamification/badges` | 🧑 | все бейджи с `earnedAt` (и признак получения) |
| `GET /api/gamification/leaderboard` | 🔑 | `?type=global\|specialization&specialization=&limit=` (default 20) |
| `GET /api/gamification/xp-history` | 🧑 | `?limit=` (default 50), журнал `XpEvent` (новые сверху) |

### Награды XP (`XP_REWARDS`)
| Действие | XP | Действие | XP |
|----------|----|----------|----|
| `TEST_COMPLETED` | 100 | `FIRST_LOGIN` | 10 |
| `TEST_PASSED` | 200 | `STREAK_3_DAYS` | 50 |
| `TEST_UPGRADED` | 300 | `STREAK_7_DAYS` | 150 |
| `PROFILE_COMPLETE` | 50 | `STREAK_30_DAYS` | 500 |
| `DAILY_TASK_DONE` | 25 | `INVITATION_RECEIVED` | 10 |
| `FSP_CONNECTED` | 100 | `INVITATION_ACCEPTED` | 50 |

**Множитель стрика** (применяется в `awardXp`): стрик ≥ 7 → `×2.0`; ≥ 3 → `×1.5`; иначе `×1.0`. Итог: `xpAwarded = round(baseXp × multiplier)`.

### Уровни (`LEVEL_THRESHOLDS`, кумулятивный XP)
| Уровень | Название | XP | Уровень | Название | XP |
|---------|----------|----|---------|----------|----|
| 1 | Новичок | 0 | 6 | Мастер | 1500 |
| 2 | Ученик | 100 | 7 | Эксперт | 2200 |
| 3 | Практик | 300 | 8 | Профи | 3000 |
| 4 | Специалист | 600 | 9 | Ветеран | 4000 |
| 5 | Опытный | 1000 | 10 | Легенда | 5500 |

`calculateLevel(xp)` — верхний достигнутый порог (максимум 10).

### Бейджи (`BADGE_DEFINITIONS`, 18 шт.)
Категории: `testing` (`FIRST_TEST`, `TEST_PASSED`, `GRADE_JUNIOR` … `GRADE_PRINCIPAL`), `profile` (`PROFILE_COMPLETE`, `STACK_MASTER`, `BIO_WRITER`), `fsp` (`FSP_CONNECTED`, `FSP_ACHIEVEMENT`), `activity` (`STREAK_3/7/30`, `FIRST_INVITATION`, `INVITATION_ACCEPTED`). Редкости: common/rare/epic/legendary, `xpReward` 20–1000.
`awardBadge(candidateId, code)` — **идемпотентен**: повторная выдача невозможна (unique `[candidateId, badgeId]`).
⚠️ XP за бейджи фактически не начисляется: `awardBadge` вызывает `awardXp('BADGE_<code>')`, а такого ключа нет в `XP_REWARDS` → начисление 0 (см. [09](09-known-issues.md)). Также `seedBadges()` не имеет вызывающего кода — бейджи должны попасть в БД через миграцию/сид вручную.

### Стрики (`updateStreak`)
Сравнение `lastActiveDate` с «сегодня» (локальная полночь):
- разница 0 дней — без изменений; 1 день — `streak + 1`, `longestStreak = max(...)`, пересчёт множителя, проверка стрик-бейджей (3/7/30);
- ≥ 2 дней — стрик сбрасывается в 1; первая активность — стрик 1.

### Где реально начисляется XP
Единственная точка вызова — `testing-v2.service.ts → complete()`: `updateStreak` → `TEST_COMPLETED` → при PASS/UPGRADE `TEST_PASSED`/`TEST_UPGRADED` + бейджи `TEST_PASSED`, `FIRST_TEST`, `GRADE_<gradeAfter>`. Остальные действия из `XP_REWARDS` (приглашения, ФСП, профиль, первый вход, стрик-дни) пока не вызываются. Ошибки геймификации перехватываются и логируются (`Gamification failed`) — тест не падает.

⚠️ Ежедневные задания (`DailyTask`) — **концепция**: модель и XP-константа есть, сервиса/эндпоинтов нет.
⚠️ Поля прогресса `xpProgress`/`xpToNextLevel` считаются от несуществующего поля `xpRequired` — значения некорректны (`xpProgress` ≈ 100).

---

## 3. Уведомления (`src/modules/notifications/`)

**Read-model без хранения.** Отдельной таблицы уведомлений нет (комментарий в коде: «источник истины — события предметной области»), поэтому нет и статуса прочитано/непрочитано — он **выводится** по правилам.

Единственный эндпоинт: `GET /api/notifications?limit=` (1–100, default 20) → `{items, total}`.

### Источники и правила
**Лента кандидата:**
| Источник | Тип | `read` | Содержимое |
|----------|-----|--------|-----------|
| `Invitation` (до 30 последних по `statusUpdatedAt`) со статусом `SENT` | `INVITATION_RECEIVED` | `false` | вилка зарплаты `salaryFrom–salaryTo ₽`, компания |
| `Invitation` в терминальных статусах `ACCEPTED/REJECTED/WITHDRAWN` | `INVITATION_STATUS` | `true` | статус приглашения |
| Последняя завершённая `TestSession` | `TEST_RESULT` | `true` | score, `gradeBefore → gradeAfter`, recommendation |

**Лента работодателя:**
| Источник | Тип | `read` |
|----------|-----|--------|
| `Invitation` (до 30) со статусом ≠ `SENT` | `INVITATION_STATUS` (с именем кандидата) | `status !== ACCEPTED` (принятые — «непрочитанные») |

Итог: элементы сортируются по `createdAt` desc и обрезаются до `limit`. Канал доставки — **только in-app**; SMTP-переменные окружения зарезервированы для будущего e-mail-канала, но в коде не используются.

---

## 4. Файловое хранилище (`src/modules/storage/storage.service.ts`)

| Параметр | Значение |
|----------|---------|
| Провайдер | MinIO (S3-совместимый), клиент `minio` |
| Бакет | `MINIO_BUCKET` (default `fsp-platform`), создание ленивое (`ensureBucket`) |
| Ключи | `${kind}/${owner}/${filename}`, где `kind ∈ resumes \| avatars \| logos`, `owner` — id профиля |
| URL | `${MINIO_PUBLIC_ENDPOINT || http://localhost:9000}/${bucket}/${key}` (публичная ссылка, без presigned URL) |
| Сбой | `503 ServiceUnavailableException('Файловое хранилище недоступно')` |

Единственный метод — `upload(kind, owner, filename, buffer, contentType)` → `putObject`. Presigned-URL, скачивание и удаление файлов не реализованы.

### Использование
| Сценарий | Endpoint | Ограничения | Результат |
|----------|----------|-------------|-----------|
| Резюме кандидата | `POST /api/candidates/profile/resume` | PDF, ≤ 5 МБ | `resumes/{profileId}/resume.pdf` → `resumePdfUrl` + парсинг |
| Аватар кандидата | `POST /api/candidates/profile/avatar` | `image/*`, ≤ 2 МБ | `avatars/{profileId}/avatar.{ext}` → `avatarUrl` |
| Логотип компании | `POST /api/employers/profile/logo` | `image/*`, ≤ 2 МБ | `logos/{profileId}/logo.{ext}` → `logoUrl` |

### Парсинг PDF-резюме (`src/modules/candidates/pdf-parsing.service.ts`)
Извлечение из текста (`pdf-parse`, динамический import; сбой чтения → ошибка, но загрузка не падает):
| Поле | Метод |
|------|-------|
| Email | regex `/[\w.-]+@[\w.-]+\.\w{2,}/` |
| Телефон | 2 regex (RU/международный формат) |
| Город | substring-сверка со списком `CITY_KEYWORDS` (Москва, СПб, Новосибирск, Минск, Алматы, Ташкент, Бишкек, Душанбе, Астана, …) |
| Навыки | `SKILL_PATTERN` — ~90 технологий + карта нормализации (`node.?js → Node.js`, `k8s → Kubernetes`, `golang → Go`, `postgres → PostgreSQL`, …), дедупликация, **лимит 30** |
| Опыт | regex-секция «опыт/experience», до 5 записей, обрезка описания до 300 символов |
| Образование | regex-секция «образование/education», до 3 записей |
| Имя | первые 5 непустых строк по regex ФИ (кириллица/латиница) |
| О себе | regex «о себе/about/summary», 20–500 символов |
| `rawText` | обрезка до 5000 символов |

Автозаполнение: заполняются **только пустые** поля профиля (имя разбивается на first/last/middle), навыки **добавляются** к существующему `stack` без перезаписи. Ответ содержит `parsed` (превью) и `autoFilled` (флаги).
