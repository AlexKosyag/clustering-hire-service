# Модуль: FSP Integration (Интеграция с Федерацией спортивного программирования)

## 1. Назначение

Модуль отвечает за:
- Связь профиля кандидата с ID участника ФСП
- Получение и отображение достижений
- Обогащение профиля результатами соревнований
- Совместимость с Keycloak (OIDC) — заготовка, не подключена (см. §4.2)

**Зависимости**: database, auth
**Параллельно с**: frontend, matching-engine

## 2. Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                  FSP INTEGRATION MODULE                     │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  1. FSP ID Connection                                 │  │
│  │  - OAuth2/OIDC flow (future)                         │  │
│  │  - Manual ID entry (MVP)                             │  │
│  │  - Verification token                                │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                  │
│                          ▼                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  2. Achievement Fetcher                               │  │
│  │  - Abstract API client                                │  │
│  │  - Mock data for MVP                                  │  │
│  │  - Real API integration (future)                      │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                  │
│                          ▼                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  3. Profile Enrichment                                │  │
│  │  - Add achievements to profile                       │  │
│  │  - Generate embeddings                               │  │
│  │  - Update matching index                             │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## 3. API Endpoints

### 3.1. Подключить ФСП ID

```http
POST /api/fsp/connect
Authorization: Bearer {token}
Content-Type: application/json

{
  "fspId": "FSP-12345"
}

Response 200:
{
  "connected": true,
  "fspId": "FSP-12345",
  "achievements": [
    {
      "id": "uuid",
      "name": "Финалист ICPC 2024",
      "category": "competition",
      "year": 2024,
      "place": "Финалист",
      "points": 10
    }
  ]
}
```

### 3.2. Получить достижения

```http
GET /api/fsp/achievements
Authorization: Bearer {token}

Response 200:
{
  "fspId": "FSP-12345",
  "achievements": [
    {
      "id": "uuid",
      "name": "Финалист ICPC 2024",
      "description": "Международная олимпиада по программированию",
      "category": "competition",
      "year": 2024,
      "place": "Финалист",
      "points": 10,
      "verifiedAt": "2026-01-10T12:00:00Z"
    },
    {
      "id": "uuid",
      "name": "Участник Codeforces Round #900",
      "category": "competition",
      "year": 2024,
      "place": "Top 100",
      "points": 5,
      "verifiedAt": "2026-01-10T12:00:00Z"
    }
  ],
  "totalPoints": 15
}
```

### 3.3. Отключить ФСП ID

```http
POST /api/fsp/disconnect
Authorization: Bearer {token}

Response 200:
{
  "connected": false
}
```

### 3.4. Каталог достижений

```http
GET /api/fsp/catalog
Authorization: Bearer {token}

Response 200:
{
  "total": 39,
  "achievements": [
    {
      "id": "uuid",
      "name": "Финалист ICPC 2024",
      "description": "Международная олимпиада по программированию",
      "category": "competition",
      "year": 2024,
      "place": "Финалист",
      "points": 10
    }
  ]
}
```

Каталог — справочник `fsp_achievements` из seed (сортировка: category ASC, points DESC),
доступен любому авторизованному пользователю.

## 4. Реализация

### 4.1. FSP Service

```go
// backend/internal/modules/fsp/service.go

// Service — FSP Integration (MVP — mock).
type Service struct {
	db       *store.DB
	matching *matching.Service
}

// Connect — подключение аккаунта ФСП (транзакция как в prisma.$transaction).
func (s *Service) Connect(ctx context.Context, userID, fspID string) (*ConnectResult, error) {
	// 1. Профиль кандидата по user_id (findUnique({where: {userId}})) → 404, если нет
	profileID, err := s.profileIDByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 2. Валидация формата: @Matches(/^[A-Za-z0-9_-]{4,40}$/)
	normalized := strings.TrimSpace(fspID)
	if !fspIDRe.MatchString(normalized) {
		return nil, common.BadRequest("Некорректный формат FSP ID (4-40 символов: буквы, цифры, -, _)")
	}

	// 3. FSP ID не занят другим пользователем → 409 при конфликте
	var takenID string
	err = s.db.Pool.QueryRow(ctx, `SELECT id FROM candidate_profiles WHERE fsp_id = $1`, normalized).Scan(&takenID)
	if err == nil && takenID != profileID {
		return nil, common.Conflict("Этот FSP ID уже привязан к другому аккаунту")
	}

	// 4. Достижения (детерминированный mock)
	achievements, err := s.MockFetchAchievements(ctx, normalized)
	if err != nil {
		return nil, err
	}

	// 5. Транзакция: fspId + fspConnectedAt + upsert CandidateAchievement
	tx, err := s.db.Pool.Begin(ctx)
	// UPDATE candidate_profiles SET fsp_id = $1, fsp_connected_at = $2 ...
	// INSERT INTO candidate_achievements (...) ON CONFLICT (candidate_id, achievement_id) DO NOTHING
	// (upsert с update: {} → идемпотентно; сопоставление — по названию из каталога)

	// 6. Фоновые сайд-эффекты (ошибки игнорируются, как .catch(() => undefined)):
	// matching.InvalidateCache(ctx) + matching.ReindexCandidate(ctx, profileID)
	s.swallow(func() error { return s.matching.InvalidateCache(ctx) })
	s.swallow(func() error { return s.matching.ReindexCandidate(ctx, profileID) })

	return &ConnectResult{
		FspID:             normalized,
		Connected:         true,
		AchievementsFound: len(achievements),
		Achievements:      achievements,
		Note:              "Mock-интеграция: достижения сгенерированы детерминированно по FSP ID",
	}, nil
}

// MockFetchAchievements — детерминированный «запрос» к внешнему сервису ФСП.
//
// Для MVP — детерминированный mock: FNV-1a от fspId → LCG
// (seed×1664525 + 1013904223) → выбор 2–4 достижений из каталога
// fsp_achievements (сортировка по points desc); year ?? 2024.
// Один и тот же fspId всегда даёт одинаковый набор — удобно для демо.
// В продакшене — интеграция с реальным API ФСП.
func (s *Service) MockFetchAchievements(ctx context.Context, fspID string) ([]FspAchievementDto, error) {
	// FNV-1a 32-bit по UTF-16 code units (charCodeAt в JS), затем LCG-выборка
	// из каталога, прочитанного raw SQL-запросом.
	/* ... */
}

// Disconnect — обнуляет fspId/fspConnectedAt, удаляет все CandidateAchievement,
// инвалидирует кэш matching.
func (s *Service) Disconnect(ctx context.Context, userID string) (*DisconnectResult, error) {
	// UPDATE candidate_profiles SET fsp_id = NULL, fsp_connected_at = NULL ...
	// DELETE FROM candidate_achievements WHERE candidate_id = $1
	// matching.InvalidateCache(ctx)
	/* ... */
}

// Achievements — достижения кандидата: JOIN fsp_achievements по
// candidate_achievements, points суммируются.
func (s *Service) Achievements(ctx context.Context, userID string) (*AchievementsResult, error) {
	/* ... */
}

// Catalog — справочник достижений (category ASC, points DESC).
func (s *Service) Catalog(ctx context.Context) (*CatalogResult, error) {
	/* ... */
}
```

### 4.2. Keycloak Integration (подготовка)

Keycloak/OIDC в текущей реализации бэкенда **не подключён** — раздел оставлен как
подготовка к будущей интеграции. Аутентификация реализована в модуле
`backend/internal/modules/auth/`: JWT access/refresh (golang-jwt/v5) + bcrypt;
Guard-эквивалент на уровне маршрутов — `common.RouteOpts{Roles: ...}` в
`common.Router` (маршруты ФСП доступны только роли CANDIDATE).
При появлении Keycloak потребуется OIDC-стратегия доверия issuer и маппинг
ролей Keycloak-групп на `model.UserRole`.

## 5. Структура данных ФСП (проработка)

### 5.1. Что мы хотим получить от API ФСП

```go
// backend/internal/model/model.go (+ internal/modules/fsp/dto.go)

// FSPParticipant — цельный образ участника ФСП (проработка API ФСП).
type FSPParticipant struct {
	ID   string `json:"id"`   // FSP ID
	Name string `json:"name"` // ФИО
	Email *string `json:"email,omitempty"` // Email (если разрешено)

	Achievements []FSPAchievement `json:"achievements"`
	Competitions []FSPCompetition `json:"competitions"`
	Ratings      []FSPRating      `json:"ratings"`
}

// FSPAchievement — достижение (реализовано в model.FSPAchievement,
// DTO каталога — fsp.CatalogResult).
type FSPAchievement struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`        // "Финалист ICPC 2024"
	Description *string `json:"description,omitempty"`
	Category    string  `json:"category"` // competition | event | certification
	Year        int     `json:"year"`
	Place       *string `json:"place,omitempty"` // "1 место", "Финалист", "Top 100"
	Points      int     `json:"points"`          // Вес достижения
	Verified    bool    `json:"verified"`        // Подтверждено ФСП
}

// FSPCompetition — соревнование (проработка, в схеме БД пока нет).
type FSPCompetition struct {
	ID     string `json:"id"`
	Name   string `json:"name"`   // "ICPC World Finals 2024"
	Date   string `json:"date"`
	Format string `json:"format"` // individual | team
	Result string `json:"result"` // "Финалист", "1 место"
	Rating *int   `json:"rating,omitempty"` // Рейтинг после соревнования
}

// FSPRating — рейтинг участника (проработка, в схеме БД пока нет).
type FSPRating struct {
	System string `json:"system"` // "codeforces", "icpc", "fsp"
	Current int   `json:"current"` // Текущий рейтинг
	Max     int   `json:"max"`     // Максимальный рейтинг
	Rank    *string `json:"rank,omitempty"` // "Master", "Grandmaster"
}
```

### 5.2. Как отображать в профиле

```tsx
// components/fsp/AchievementBadge.tsx
export function AchievementBadge({ achievement }: { achievement: FSPAchievement }) {
  const icon = {
    competition: <TrophyIcon />,
    event: <CalendarIcon />,
    certification: <CertificateIcon />,
  }[achievement.category];

  return (
    <div className="flex items-center gap-2 p-2 bg-yellow-50 rounded">
      {icon}
      <div>
        <p className="font-medium">{achievement.name}</p>
        <p className="text-sm text-gray-500">
          {achievement.place} · {achievement.year}
        </p>
      </div>
      <span className="ml-auto text-sm font-bold text-yellow-600">
        +{achievement.points} очков
      </span>
    </div>
  );
}
```

## 6. Чеклист валидации

### Подключение
- [ ] Подключение ФСП ID работает
- [ ] Достижения загружаются (mock)
- [ ] Достижения сохраняются в БД
- [ ] Дубликат FSP ID отклоняется
- [ ] Отключение работает

### Отображение
- [ ] Достижения отображаются в профиле
- [ ] Бейджи корректно отображают данные
- [ ] Общий балл достижений рассчитывается
- [ ] Достижения влияют на matching score

### Безопасность
- [ ] FSP ID привязан к пользователю
- [ ] Нельзя подключить чужой FSP ID
- [ ] Данные достижений верифицируются

### Future (Keycloak)
- [ ] OIDC-стратегия для Keycloak (в текущей реализации не подключена)
- [ ] Конфигурация Keycloak описана
- [ ] Маппинг ролей работает

## 7. Промпт для агента

```
Ты — Backend-разработчик с опытом интеграции внешних API.

ТВОЯ ЗАДАЧА: Реализовать модуль fsp-integration для платформы ФСП.

ВХОДНЫЕ ДАННЫЕ:
- docs/ARCHITECTURE.md — общая архитектура проекта
- docs/modules/fsp-integration.md — ТЗ на модуль (этот файл)
- docs/modules/database.md — схема данных (fsp_achievements, candidate_achievements)

ЧТО НУЖНО СДЕЛАТЬ:
1. Создать backend/internal/modules/fsp/ со всеми файлами (service.go, handler.go, dto.go)
2. Реализовать Service — подключение, получение достижений
3. Создать детерминированный mock для демонстрации (FNV-1a + LCG по каталогу)
4. Реализовать API endpoints (gin-хендлеры + Register)
5. Создать DTO с валидацией (русские сообщения об ошибках)
6. Написать unit-тесты
7. Оставить Keycloak OIDC на будущее (в текущей реализации не подключается)

СТЕК:
- Go 1.25
- gin
- pgx/v5 (сырой SQL, без ORM)

ВАЛИДАЦИЯ:
- Подключение ФСП ID работает
- Mock достижения загружаются
- Достижения сохраняются в БД
- Тесты проходят (go test ./internal/modules/fsp/...)

ФОРМАТ ОТВЕТА:
- Список созданных файлов
- Примеры mock данных
- Результат тестов
```