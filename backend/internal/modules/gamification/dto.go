// Package gamification — XP, уровни, бейджи, стрики (docs/07 §2).
package gamification

import (
	"fsp-backend/internal/model"
)

// XP_REWARDS — таблица вознаграждений (gamification.service.ts).
// BUG-REF: G5 — DAILY_TASK_DONE: модель DailyTask есть в БД, сервиса/эндпоинтов нет;
// BUG-REF: G6 — реально начисляются только TEST_COMPLETED/TEST_PASSED/TEST_UPGRADED
// (вызовы в testing-v2.complete()), остальные действия — константы без вызовов.
var XP_REWARDS = map[string]int{
	"TEST_COMPLETED":      100,
	"TEST_PASSED":         200,
	"TEST_UPGRADED":       300,
	"PROFILE_COMPLETE":    50,
	"DAILY_TASK_DONE":     25,
	"INVITATION_RECEIVED": 10,
	"INVITATION_ACCEPTED": 50,
	"FSP_CONNECTED":       100,
	"FIRST_LOGIN":         10,
	"STREAK_3_DAYS":       50,
	"STREAK_7_DAYS":       150,
	"STREAK_30_DAYS":      500,
}

// LevelThreshold — уровень и его порог (LEVEL_THRESHOLDS в TS).
type LevelThreshold struct {
	Level      int    `json:"level"`
	XpRequired int    `json:"xpRequired"`
	Name       string `json:"name"`
}

// LEVEL_THRESHOLDS — 10 уровней (кумулятивный XP).
var LEVEL_THRESHOLDS = []LevelThreshold{
	{Level: 1, XpRequired: 0, Name: "Новичок"},
	{Level: 2, XpRequired: 100, Name: "Ученик"},
	{Level: 3, XpRequired: 300, Name: "Практик"},
	{Level: 4, XpRequired: 600, Name: "Специалист"},
	{Level: 5, XpRequired: 1000, Name: "Опытный"},
	{Level: 6, XpRequired: 1500, Name: "Мастер"},
	{Level: 7, XpRequired: 2200, Name: "Эксперт"},
	{Level: 8, XpRequired: 3000, Name: "Профи"},
	{Level: 9, XpRequired: 4000, Name: "Ветеран"},
	{Level: 10, XpRequired: 5500, Name: "Легенда"},
}

// BadgeDef — определение бейджа (BADGE_DEFINITIONS в TS, без id/earnedAt).
type BadgeDef struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
	Rarity      string `json:"rarity"`
	XpReward    int    `json:"xpReward"`
}

// BADGE_DEFINITIONS — 18 бейджей (перенесены дословно из gamification.service.ts).
var BADGE_DEFINITIONS = []BadgeDef{
	// Testing badges
	{Code: "FIRST_TEST", Name: "Первый тест", Description: "Прошёл первое тестирование", Icon: "🎯", Category: "testing", Rarity: "common", XpReward: 50},
	{Code: "TEST_PASSED", Name: "Тест пройден", Description: "Успешно прошёл тестирование", Icon: "✅", Category: "testing", Rarity: "common", XpReward: 100},
	{Code: "GRADE_JUNIOR", Name: "Junior", Description: "Подтвердил грейд Junior", Icon: "🥉", Category: "testing", Rarity: "common", XpReward: 50},
	{Code: "GRADE_MIDDLE", Name: "Middle", Description: "Подтвердил грейд Middle", Icon: "🥈", Category: "testing", Rarity: "rare", XpReward: 150},
	{Code: "GRADE_MIDDLE_PLUS", Name: "Middle+", Description: "Подтвердил грейд Middle+", Icon: "🥈", Category: "testing", Rarity: "rare", XpReward: 200},
	{Code: "GRADE_SENIOR", Name: "Senior", Description: "Подтвердил грейд Senior", Icon: "🥇", Category: "testing", Rarity: "epic", XpReward: 300},
	{Code: "GRADE_LEAD", Name: "Lead", Description: "Подтвердил грейд Lead", Icon: "👑", Category: "testing", Rarity: "epic", XpReward: 500},
	{Code: "GRADE_PRINCIPAL", Name: "Principal", Description: "Подтвердил грейд Principal", Icon: "🏆", Category: "testing", Rarity: "legendary", XpReward: 1000},

	// Profile badges
	{Code: "PROFILE_COMPLETE", Name: "Полный профиль", Description: "Заполнил все поля профиля", Icon: "📋", Category: "profile", Rarity: "common", XpReward: 50},
	{Code: "STACK_MASTER", Name: "Мастер стека", Description: "Указал 5+ технологий", Icon: "🛠️", Category: "profile", Rarity: "common", XpReward: 30},
	{Code: "BIO_WRITER", Name: "Автор", Description: "Написал описание о себе", Icon: "✍️", Category: "profile", Rarity: "common", XpReward: 20},

	// FSP badges
	{Code: "FSP_CONNECTED", Name: "Олимпиец", Description: "Подключил ФСП ID", Icon: "🔗", Category: "fsp", Rarity: "rare", XpReward: 100},
	{Code: "FSP_ACHIEVEMENT", Name: "Чемпион", Description: "Получил достижение ФСП", Icon: "🏆", Category: "fsp", Rarity: "rare", XpReward: 150},

	// Activity badges
	{Code: "STREAK_3", Name: "Разогрев", Description: "3 дня активности подряд", Icon: "🔥", Category: "activity", Rarity: "common", XpReward: 50},
	{Code: "STREAK_7", Name: "Неделя силы", Description: "7 дней активности подряд", Icon: "🔥", Category: "activity", Rarity: "rare", XpReward: 150},
	{Code: "STREAK_30", Name: "Марафонец", Description: "30 дней активности подряд", Icon: "🔥", Category: "activity", Rarity: "epic", XpReward: 500},

	// Invitation badges
	{Code: "FIRST_INVITATION", Name: "Востребованный", Description: "Получил первое приглашение", Icon: "📬", Category: "activity", Rarity: "common", XpReward: 30},
	{Code: "INVITATION_ACCEPTED", Name: "Выбранный", Description: "Принял приглашение", Icon: "🤝", Category: "activity", Rarity: "rare", XpReward: 100},
}

// BadgeView — элемент GET /api/gamification/badges: {...badge, earnedAt}.
// earnedAt опускается для неполученных бейджей (earnedAt: undefined → ключа нет в TS).
type BadgeView struct {
	model.Badge
	EarnedAt *model.Time `json:"earnedAt,omitempty"`
}

// ProfileBadge — бейдж внутри GET /api/gamification/profile (без xpReward, как в toGamificationProfile).
type ProfileBadge struct {
	ID          string     `json:"id"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Icon        string     `json:"icon"`
	Category    string     `json:"category"`
	Rarity      string     `json:"rarity"`
	EarnedAt    model.Time `json:"earnedAt"`
}

// XpEventView — элемент GET /api/gamification/xp-history.
type XpEventView struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Xp     int    `json:"xp"`
	// Metadata: TS возвращает `metadata || undefined` — ключ опускается для null/ложных значений.
	Metadata  any        `json:"metadata,omitempty"`
	CreatedAt model.Time `json:"createdAt"`
}

// LeaderboardEntry — элемент GET /api/gamification/leaderboard.
type LeaderboardEntry struct {
	Rank        int    `json:"rank"`
	CandidateID string `json:"candidateId"`
	DisplayName string `json:"displayName"`
	Xp          int    `json:"xp"`
	Level       int    `json:"level"`
	Badges      int    `json:"badges"`
}

// calculateLevel — верхний достигнутый порог (максимум 10).
func calculateLevel(xp int) int {
	level := 1
	for _, t := range LEVEL_THRESHOLDS {
		if xp >= t.XpRequired {
			level = t.Level
		}
	}
	return level
}

// getLevelInfo — название/порог уровня (с фолбэком на первый уровень).
func getLevelInfo(level int) LevelThreshold {
	for _, t := range LEVEL_THRESHOLDS {
		if t.Level == level {
			return t
		}
	}
	return LEVEL_THRESHOLDS[0]
}

// nextLevel — порог следующего уровня (nil для 10-го).
func nextLevel(level int) *LevelThreshold {
	for i := range LEVEL_THRESHOLDS {
		if LEVEL_THRESHOLDS[i].Level == level+1 {
			return &LEVEL_THRESHOLDS[i]
		}
	}
	return nil
}
