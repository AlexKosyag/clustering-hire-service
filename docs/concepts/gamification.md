# Концепция: Gamification (Геймификация)

## 1. Обзор

Геймификация — опциональный модуль, повышающий вовлечённость кандидатов и работодателей. Не является обязательным для MVP, но даёт +7% к оценке (критерий 6).

**Статус**: Реализовано в Go-бэкенде: `backend/internal/modules/gamification/` (`service.go`, `handler.go`, `dto.go`); таблицы `gamification_profiles`, `badges`, `candidate_badges`, `xp_events` — `backend/migrations/000003_add_gamification.up.sql`
**Зависимости**: database, backend-api, testing-engine

## 2. Элементы геймификации

### 2.1. Бейджи и достижения

**Цель**: Визуализировать прогресс и достижения кандидата.

```go
// Система бейджей (актуальные определения: BADGE_DEFINITIONS в
// backend/internal/modules/gamification/dto.go; таблица badges —
// backend/migrations/000003_add_gamification.up.sql)
type Badge struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`        // "Первый тест"
	Description string         `json:"description"` // "Прошёл первое тестирование"
	Icon        string         `json:"icon"`        // Emoji или URL иконки
	Category    string         `json:"category"`    // "testing" | "profile" | "activity" | "fsp"
	Rarity      string         `json:"rarity"`      // "common" | "rare" | "epic" | "legendary"
	Condition   BadgeCondition `json:"condition"`
}

type BadgeCondition struct {
	Type  string `json:"type"`  // "test_completed" | "grade_achieved" | "profile_complete" |
	// "streak" | "invitations_received" | "fsp_connected"
	Value *int   `json:"value,omitempty"` // Пороговое значение
}
```

**Примеры бейджей**:

| Бейдж | Условие | Редкость |
|-------|---------|----------|
| Первый тест | Завершить первое тестирование | Common |
| Junior | Подтвердить грейд Junior | Common |
| Middle | Подтвердить грейд Middle | Rare |
| Senior | Подтвердить грейд Senior | Epic |
| Full Stack | Заполнить профиль на 100% | Rare |
| Олимпиец | Подключить ФСП ID | Rare |
| Финалист | Получить достижение "Финалист" | Epic |
| Стратег | 7 дней подряд выполнять задания | Epic |
| Легенда | Получить 10+ приглашений | Legendary |

### 2.2. Уровни и опыт (XP)

**Цель**: Мотивировать кандидатов к регулярной активности.

```go
// Уровни: LEVEL_THRESHOLDS (10 уровней, кумулятивный XP) —
// backend/internal/modules/gamification/dto.go
type Level struct {
	Level      int      `json:"level"`
	Name       string   `json:"name"`       // "Новичок", "Ученик", "Мастер", "Легенда"
	XpRequired int      `json:"xpRequired"` // XP для достижения уровня
	Perks      []string `json:"perks"`      // Бонусы уровня
}

// XP начисляется за (реализация: XP_REWARDS —
// backend/internal/modules/gamification/dto.go):
var XP_REWARDS = map[string]int{
	"TEST_COMPLETED":      100, // Завершение теста
	"TEST_PASSED":         200, // Успешное прохождение
	"TEST_UPGRADED":       300, // Повышение грейда
	"PROFILE_COMPLETE":    50,  // Заполненный профиль
	"DAILY_TASK_DONE":     25,  // Ежедневное задание
	"STREAK_3_DAYS":       50,  // 3 дня подряд
	"STREAK_7_DAYS":       150, // 7 дней подряд
	"INVITATION_RECEIVED": 10,  // Получено приглашение
	"FSP_CONNECTED":       100, // Подключён ФСП
}
```

### 2.3. Стрики (серии активности)

**Цель**: Мотивировать к регулярной активности.

```go
// Стрики (строки таблицы gamification_profiles: streak, longest_streak,
// last_active_date, multiplier — backend/migrations/000003_add_gamification.up.sql)
type Streak struct {
	Current        int         `json:"current"`        // Текущая серия дней
	Longest        int         `json:"longest"`        // Лучшая серия
	LastActiveDate *model.Time `json:"lastActiveDate"` // Дата последней активности
	Multiplier     float64     `json:"multiplier"`     // Множитель XP (1x → 1.5x → 2x)
}

// Логика стрика (Service.UpdateStreak — backend/internal/modules/gamification/service.go):
// - Кандидат выполняет задание или проходит тест → стрик +1
// - Пропуск дня → стрик сбрасывается
// - Стрик >= 3 → множитель x1.5, стрик >= 7 → множитель x2.0
```

### 2.4. Рейтинг кандидатов (Leaderboard)

**Цель**: Создать здоровую конкуренцию.

```go
// Рейтинги (реализация: Service.GetLeaderboard —
// backend/internal/modules/gamification/service.go)
type Leaderboard struct {
	Type    string             `json:"type"`    // "global" | "specialization"
	Entries []LeaderboardEntry `json:"entries"`
}

type LeaderboardEntry struct {
	Rank        int    `json:"rank"`
	CandidateID string `json:"candidateId"`
	DisplayName string `json:"displayName"` // "Иван И."
	Xp          int    `json:"xp"`
	Level       int    `json:"level"`
	Badges      int    `json:"badges"` // Количество полученных бейджей
}

// Рейтинги:
// 1. Общий рейтинг по XP
// 2. Рейтинг по специализации (Backend, Frontend, etc.)
// 3. Месячный рейтинг — не реализован / вне объёма порта
```

### 2.5. Прогресс-бары

**Цель**: Визуализировать прогресс.

```go
// Прогресс заполнения профиля (концепция; в Go-порте не реализован /
// вне объёма порта — GET /api/gamification/profile возвращает
// xpProgress/xpToNextLevel, см. toGamificationProfile в
// backend/internal/modules/gamification/service.go)
type ProfileProgress struct {
	Total    int  `json:"total"`    // 100%
	Filled   int  `json:"filled"`   // Заполнено %
	Sections struct {
		Basic        bool `json:"basic"`        // ФИО, контакты
		Professional bool `json:"professional"` // Стек, грейд, опыт
		Testing      bool `json:"testing"`      // Тест пройден
		Fsp          bool `json:"fsp"`          // ФСП подключён
		Resume       bool `json:"resume"`       // Резюме загружено
	} `json:"sections"`
}
```

## 3. UI компоненты

### 3.1. Badge Display

```tsx
// components/gamification/BadgeDisplay.tsx
export function BadgeDisplay({ badge }: { badge: Badge }) {
  const rarityColor = {
    common: 'bg-gray-100 text-gray-700',
    rare: 'bg-blue-100 text-blue-700',
    epic: 'bg-purple-100 text-purple-700',
    legendary: 'bg-yellow-100 text-yellow-700',
  }[badge.rarity];

  return (
    <div className={`flex items-center gap-2 p-2 rounded ${rarityColor}`}>
      <span className="text-2xl">{badge.icon}</span>
      <div>
        <p className="font-medium">{badge.name}</p>
        <p className="text-xs opacity-70">{badge.description}</p>
      </div>
    </div>
  );
}
```

### 3.2. Level Progress

```tsx
// components/gamification/LevelProgress.tsx
export function LevelProgress({ level, xp, xpToNext }: Props) {
  const progress = (xp / xpToNext) * 100;

  return (
    <div className="flex items-center gap-4">
      <div className="w-12 h-12 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold">
        {level}
      </div>
      <div className="flex-1">
        <div className="flex justify-between text-sm mb-1">
          <span>Уровень {level}</span>
          <span>{xp}/{xpToNext} XP</span>
        </div>
        <div className="w-full bg-gray-200 rounded-full h-2">
          <div
            className="bg-blue-500 h-2 rounded-full transition-all"
            style={{ width: `${progress}%` }}
          />
        </div>
      </div>
    </div>
  );
}
```

### 3.3. Streak Counter

```tsx
// components/gamification/StreakCounter.tsx
export function StreakCounter({ streak }: { streak: Streak }) {
  return (
    <div className="flex items-center gap-2">
      <span className="text-2xl">🔥</span>
      <div>
        <p className="font-bold text-lg">{streak.current} дней</p>
        <p className="text-xs text-gray-500">Серия активности</p>
      </div>
      {streak.multiplier > 1 && (
        <span className="ml-2 px-2 py-1 bg-yellow-100 text-yellow-700 rounded text-sm">
          x{streak.multiplier} XP
        </span>
      )}
    </div>
  );
}
```

## 4. API Endpoints

Реализация HTTP-слоя: `backend/internal/modules/gamification/handler.go` (маршруты `/api/gamification/*`). Маршруты: `GET /api/gamification/profile`, `GET /api/gamification/badges`, `GET /api/gamification/leaderboard`, `GET /api/gamification/xp-history`.

### 4.1. Получить бейджи

**Статус**: реализовано — `GET /api/gamification/badges`

```http
GET /api/gamification/badges
Authorization: Bearer {token}

Response 200:
{
  "badges": [
    {
      "id": "badge-1",
      "name": "Первый тест",
      "description": "Прошёл первое тестирование",
      "icon": "🎯",
      "category": "testing",
      "rarity": "common",
      "earnedAt": "2026-01-10T12:00:00Z"
    }
  ],
  "totalBadges": 5,
  "availableBadges": 15
}
```

### 4.2. Получить уровень и XP

**Статус**: отдельного маршрута нет — уровень, XP, `xpToNextLevel`, `xpProgress` и `levelName` возвращает `GET /api/gamification/profile` (см. `toGamificationProfile` в `backend/internal/modules/gamification/service.go`); история XP вынесена в `GET /api/gamification/xp-history` (раздел 4.5).

```http
GET /api/gamification/level
Authorization: Bearer {token}

Response 200:
{
  "level": 5,
  "name": "Ученик",
  "xp": 450,
  "xpToNextLevel": 500,
  "xpHistory": [
    { "action": "TEST_PASSED", "xp": 200, "date": "2026-01-10" },
    { "action": "DAILY_TASK_DONE", "xp": 25, "date": "2026-01-11" }
  ]
}
```

### 4.3. Получить стрик

**Статус**: отдельного маршрута нет — `streak`, `longestStreak`, `lastActiveDate`, `multiplier` возвращает `GET /api/gamification/profile`.

```http
GET /api/gamification/streak
Authorization: Bearer {token}

Response 200:
{
  "current": 5,
  "longest": 12,
  "lastActiveDate": "2026-01-15",
  "multiplier": 1.0,
  "nextMultiplierAt": 7
}
```

### 4.4. Рейтинг

**Статус**: реализовано — `GET /api/gamification/leaderboard` (query-параметры: `type` — `global` | `specialization`, `specialization`, `limit`, default 20; доступен любому авторизованному пользователю).

```http
GET /api/gamification/leaderboard?type=specialization&spec=BACKEND&limit=20

Response 200:
{
  "type": "specialization",
  "specialization": "BACKEND",
  "entries": [
    {
      "rank": 1,
      "displayName": "Дмитрий С.",
      "xp": 2500,
      "level": 8,
      "badges": ["🏆", "⭐", "🔥"]
    }
  ],
  "myRank": 15
}
```

### 4.5. История XP

**Статус**: реализовано — `GET /api/gamification/xp-history` (query-параметр `limit`, default 50; журнал `xp_events`, новые сверху; реализация: `GetXpHistory` в `backend/internal/modules/gamification/service.go`).

```http
GET /api/gamification/xp-history?limit=50
Authorization: Bearer {token}

Response 200:
[
  {
    "id": "event-1",
    "action": "TEST_PASSED",
    "xp": 200,
    "metadata": { "score": 85 },
    "createdAt": "2026-01-10T12:00:00Z"
  }
]
```

## 5. Интеграция с существующими модулями

### 5.1. Testing Engine → Gamification

**Статус**: реализовано — вызовы из `complete()` в `backend/internal/modules/testing/testing-v2.go`.

```go
// После завершения теста (как в testing-v2.go, complete()):
func onTestCompleted(ctx context.Context, g *gamification.Service, session *TestSession) {
	// Начислить XP
	g.UpdateStreak(ctx, session.CandidateID)
	g.AwardXp(ctx, session.CandidateID, "TEST_COMPLETED", map[string]any{"sessionId": session.ID})

	if session.Recommendation == "PASS" {
		g.AwardXp(ctx, session.CandidateID, "TEST_PASSED", map[string]any{"score": session.Score})
		g.AwardBadge(ctx, session.CandidateID, "TEST_PASSED")
	}

	if session.Recommendation == "UPGRADE" {
		g.AwardXp(ctx, session.CandidateID, "TEST_UPGRADED", map[string]any{"from": session.PreviousGrade, "to": session.Grade})
	}

	// Выдать бейджи
	g.AwardBadge(ctx, session.CandidateID, "FIRST_TEST")
	g.AwardBadge(ctx, session.CandidateID, gradeBadge) // GRADE_JUNIOR и т.д.
}
```

Известные особенности (намеренно воспроизведены из поведения исходной версии, см. `temp-docs/09-known-issues.md`):
- G6 — реально вызываются только действия `TEST_COMPLETED`/`TEST_PASSED`/`TEST_UPGRADED`; `INVITATION_*`, `FSP_CONNECTED`, `PROFILE_COMPLETE`, `STREAK_*` — константы без вызовов;
- G1 — XP за бейджи (`BADGE_<code>`) не начисляется, поле `xpReward` декоративное.

### 5.2. Daily Tasks → Gamification

**Статус**: не реализовано / вне объёма порта — модель `DailyTask` есть в БД (`daily_tasks`, `backend/migrations/000001_init.up.sql`), но сервисов и эндпоинтов нет (G5).

```go
// После выполнения задания (концепция):
func onDailyTaskCompleted(ctx context.Context, g *gamification.Service, task *DailyTask) {
	g.AwardXp(ctx, task.CandidateID, "DAILY_TASK_DONE", nil)
	g.UpdateStreak(ctx, task.CandidateID)
}
```

### 5.3. Invitations → Gamification

**Статус**: не реализовано / вне объёма порта — вызовов из invitations-модуля нет (G6).

```go
// После получения приглашения (концепция):
func onInvitationReceived(ctx context.Context, g *gamification.Service, invitation *Invitation) {
	g.AwardXp(ctx, invitation.CandidateID, "INVITATION_RECEIVED", nil)
	g.AwardBadge(ctx, invitation.CandidateID, "FIRST_INVITATION")
}
```

## 6. Чеклист валидации

- [ ] Бейджи начисляются при выполнении условий
- [ ] XP начисляется за все действия
- [ ] Уровни рассчитываются корректно
- [ ] Стрики отслеживаются
- [ ] Множитель работает
- [ ] Leaderboard отображается
- [ ] UI компоненты рендерятся

## 7. Промпт для агента

```
Ты — Full-stack разработчик с опытом в геймификации.

ТВОЯ ЗАДАЧА: Реализовать модуль gamification для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/concepts/gamification.md — ТЗ на модуль (этот файл)
- docs/modules/database.md — схема данных

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать backend/internal/modules/gamification/
2. Реализовать Service — XP, levels, badges, streaks
3. Создать API endpoints
4. Создать UI компоненты: BadgeDisplay, LevelProgress, StreakCounter, Leaderboard
5. Интегрировать с testing и daily tasks
6. Написать unit-тесты

СТЕК:
- Go 1.25, gin, pgx/v5 (backend)
- React + Ant Design (frontend)

ВАЛИДАЦИЯ:
- Бейджи начисляются
- XP рассчитывается
- UI отображается корректно

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Примеры бейджей
- Результат тестов
```