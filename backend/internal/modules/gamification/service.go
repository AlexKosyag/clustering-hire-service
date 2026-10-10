package gamification

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"fsp-backend/internal/model"
	"fsp-backend/internal/store"
)

// Service — фасад геймификации (GamificationService в gamification.service.ts).
type Service struct {
	db *store.DB
}

// NewService — конструктор для интегратора.
func NewService(db *store.DB) *Service { return &Service{db: db} }

// ─────────────────────────── Публичный API ───────────────────────────
// Сигнатуры совпадают с api_stub.go (их используют testing- и fsp-модули).

// UpdateStreak — пересчёт стрика (call on daily activity), updateStreak в TS.
func (s *Service) UpdateStreak(ctx context.Context, candidateID string) error {
	p, err := s.getOrCreateProfile(ctx, candidateID)
	if err != nil {
		return err
	}

	now := time.Now()
	// «Сегодня» — локальная полночь (new Date(y, m, d) в TS).
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	if p.LastActiveDate == nil {
		// Первая активность.
		return s.updateStreakFields(ctx, candidateID, 1, 1.0, false)
	}

	last := p.LastActiveDate.Time.In(time.Local)
	lastActiveDay := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, time.Local)
	// floor((today - lastActiveDay) / 86_400_000) — как в TS (включая дни с переводом часов).
	diffDays := int(math.Floor(float64(today.UnixMilli()-lastActiveDay.UnixMilli()) / 86_400_000))

	switch {
	case diffDays == 0:
		// Уже активен сегодня — без изменений.
		return nil
	case diffDays == 1:
		// Следующий день подряд.
		newStreak := p.Streak + 1
		newMultiplier := 1.0
		if newStreak >= 7 {
			newMultiplier = 2.0
		} else if newStreak >= 3 {
			newMultiplier = 1.5
		}
		if err := s.updateStreakFields(ctx, candidateID, newStreak, newMultiplier, true); err != nil {
			return err
		}
		return s.checkStreakBadges(ctx, candidateID, newStreak)
	default:
		// Стрик сброшен (>= 2 дней пропуска).
		return s.updateStreakFields(ctx, candidateID, 1, 1.0, false)
	}
}

// AwardXp — начисление XP за действие (awardXp в TS).
//
// BUG-REF: G2 — awardXp не идемпотентен: нет ключа дедупликации событий,
// повторный вызов начисляет XP повторно и пишет ещё одно событие в xp_events.
func (s *Service) AwardXp(ctx context.Context, candidateID, action string, metadata map[string]any) error {
	baseXp := XP_REWARDS[action] // BUG-REF: G6 — реально вызываются только действия из testing-v2.
	if baseXp == 0 {
		// TS: return { xpAwarded: 0, totalXp: 0, levelUp: false, newLevel: 1 } — без записи в БД.
		return nil
	}

	p, err := s.getOrCreateProfile(ctx, candidateID)
	if err != nil {
		return err
	}

	// Множитель стрика: >= 7 → x2.0; >= 3 → x1.5; иначе x1.0.
	streakMultiplier := 1.0
	if p.Streak >= 7 {
		streakMultiplier = 2.0
	} else if p.Streak >= 3 {
		streakMultiplier = 1.5
	}
	xpAwarded := int(math.Round(float64(baseXp) * streakMultiplier))

	newXp := p.Xp + xpAwarded
	newLevel := calculateLevel(newXp)

	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE gamification_profiles SET xp = $1, level = $2, updated_at = $3 WHERE candidate_id = $4`,
		newXp, newLevel, model.Now(), candidateID); err != nil {
		return err
	}

	var metaArg any
	if metadata != nil {
		raw, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		metaArg = string(raw)
	}
	if _, err := s.db.Pool.Exec(ctx,
		`INSERT INTO xp_events (id, candidate_id, action, xp, metadata)
		 VALUES ($1, $2, $3, $4, $5::jsonb)`,
		newID(), candidateID, action, xpAwarded, metaArg); err != nil {
		return err
	}

	log.Printf("[Gamification] Awarded %d XP to %s for %s (total: %d, level: %d)",
		xpAwarded, candidateID, action, newXp, newLevel)
	return nil
}

// AwardBadge — выдача бейджа (awardBadge в TS); идемпотентен (unique candidate+badge).
//
// BUG-REF: G1 — XP за бейджи фактически не начисляется: awardBadge вызывает
// awardXp('BADGE_<code>'), а такого ключа нет в XP_REWARDS → начисление 0 XP
// (поле Badge.xpReward декоративное). Воспроизводится дословно.
func (s *Service) AwardBadge(ctx context.Context, candidateID, badgeCode string) error {
	var badge struct {
		id       string
		name     string
		xpReward int
	}
	err := s.db.Pool.QueryRow(ctx,
		`SELECT id, name, xp_reward FROM badges WHERE code = $1`, badgeCode).
		Scan(&badge.id, &badge.name, &badge.xpReward)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // TS: return false
	}
	if err != nil {
		return err
	}

	// candidate_badges ссылаются на gamification_profiles (FK); в TS профиль уже создан
	// предшествующим updateStreak — здесь создаём лениво, чтобы выдача была идемпотентной.
	if _, err := s.getOrCreateProfile(ctx, candidateID); err != nil {
		return err
	}

	tag, err := s.db.Pool.Exec(ctx,
		`INSERT INTO candidate_badges (id, candidate_id, badge_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (candidate_id, badge_id) DO NOTHING`,
		newID(), candidateID, badge.id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil // уже получен (TS: return false)
	}

	if badge.xpReward > 0 {
		// BUG-REF: G1 — ключа BADGE_<code> нет в XP_REWARDS → 0 XP, событие не пишется.
		if err := s.AwardXp(ctx, candidateID, "BADGE_"+badgeCode, map[string]any{"badgeName": badge.name}); err != nil {
			return err
		}
	}

	log.Printf("[Gamification] Awarded badge %s to %s", badgeCode, candidateID)
	return nil
}

// GetProfile — профиль геймификации (ключи как в GET /api/gamification/profile).
//
// BUG-REF: G4 — toGamificationProfile читает несуществующее поле profile.xpRequired:
// xpProgress вырождается в 100, xpToNextLevel возвращает абсолютный порог, а не остаток.
func (s *Service) GetProfile(ctx context.Context, candidateID string) (map[string]any, error) {
	p, err := s.getOrCreateProfile(ctx, candidateID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Pool.Query(ctx, `
		SELECT b.id, b.code, b.name, b.description, b.icon, b.category, b.rarity, cb.earned_at
		FROM candidate_badges cb
		JOIN badges b ON b.id = cb.badge_id
		WHERE cb.candidate_id = $1`, candidateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	badges := []ProfileBadge{}
	for rows.Next() {
		var b ProfileBadge
		if err := rows.Scan(&b.ID, &b.Code, &b.Name, &b.Description, &b.Icon,
			&b.Category, &b.Rarity, &b.EarnedAt); err != nil {
			return nil, err
		}
		badges = append(badges, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return s.toGamificationProfile(p, badges), nil
}

// ─────────────────────────── HTTP-слой (по user_id) ───────────────────────────

// GetProfileForUser — GET /api/gamification/profile (в TS getProfile(userId)).
func (s *Service) GetProfileForUser(ctx context.Context, userID string) (map[string]any, error) {
	candidateID, err := s.resolveCandidateID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, candidateID)
}

// GetBadges — все бейджи с earnedAt (getBadges в TS).
func (s *Service) GetBadges(ctx context.Context, userID string) ([]BadgeView, error) {
	candidateID, err := s.resolveCandidateID(ctx, userID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, code, name, description, icon, category, rarity, xp_reward
		FROM badges
		ORDER BY category ASC`)
	if err != nil {
		return nil, err
	}

	all := []BadgeView{}
	for rows.Next() {
		var b BadgeView
		if err := rows.Scan(&b.ID, &b.Code, &b.Name, &b.Description, &b.Icon,
			&b.Category, &b.Rarity, &b.XpReward); err != nil {
			rows.Close()
			return nil, err
		}
		all = append(all, b)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	earnedRows, err := s.db.Pool.Query(ctx,
		`SELECT badge_id, earned_at FROM candidate_badges WHERE candidate_id = $1`, candidateID)
	if err != nil {
		return nil, err
	}
	defer earnedRows.Close()

	earned := map[string]model.Time{}
	for earnedRows.Next() {
		var badgeID string
		var earnedAt model.Time
		if err := earnedRows.Scan(&badgeID, &earnedAt); err != nil {
			return nil, err
		}
		earned[badgeID] = earnedAt
	}
	if err := earnedRows.Err(); err != nil {
		return nil, err
	}

	for i := range all {
		if at, ok := earned[all[i].ID]; ok {
			t := at
			all[i].EarnedAt = &t
		}
	}
	return all, nil
}

// GetLeaderboard — рейтинг кандидатов (getLeaderboard в TS).
func (s *Service) GetLeaderboard(ctx context.Context, typ, specialization string, limit int) ([]LeaderboardEntry, error) {
	filter := typ == "specialization" && specialization != ""
	if filter && !isKnownSpecialization(specialization) {
		// TS: PrismaClientValidationError (невалидное значение enum Specialization) → 500.
		return nil, fmt.Errorf(
			"Invalid `prisma.gamificationProfile.findMany()` invocation: argument specialization of type Specialization needs a valid enum value, got %q",
			specialization)
	}

	// BUG-REF: G2 — XP можно начислить повторно; порядок xp DESC как в TS.
	order, take := "DESC", limit
	if limit < 0 {
		// Prisma: отрицательный take — «хвост» выборки (последние N в порядке сортировки).
		order, take = "ASC", -limit
	}

	query := `
		SELECT gp.candidate_id, gp.xp, gp.level, cp.first_name, cp.last_name,
		       (SELECT count(*) FROM candidate_badges cb WHERE cb.candidate_id = gp.candidate_id)
		FROM gamification_profiles gp
		JOIN candidate_profiles cp ON cp.id = gp.candidate_id`
	args := []any{}
	if filter {
		args = append(args, specialization)
		query += fmt.Sprintf(` WHERE cp.specialization::text = $%d`, len(args))
	}
	args = append(args, take)
	query += fmt.Sprintf(` ORDER BY gp.xp %s LIMIT $%d`, order, len(args))

	rows, err := s.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type row struct {
		candidateID string
		xp          int
		level       int
		firstName   *string
		lastName    *string
		badges      int
	}
	list := []row{}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.candidateID, &r.xp, &r.level, &r.firstName, &r.lastName, &r.badges); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if limit < 0 {
		// Вернуть в порядке сортировки (xp DESC), как это делает Prisma для отрицательного take.
		for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
			list[i], list[j] = list[j], list[i]
		}
	}

	out := make([]LeaderboardEntry, 0, len(list))
	for idx, r := range list {
		out = append(out, LeaderboardEntry{
			Rank:        idx + 1,
			CandidateID: r.candidateID,
			DisplayName: displayName(r.firstName, r.lastName, r.candidateID),
			Xp:          r.xp,
			Level:       r.level,
			Badges:      r.badges,
		})
	}
	return out, nil
}

// GetXpHistory — журнал начисления XP (getXpHistory в TS; новые сверху).
func (s *Service) GetXpHistory(ctx context.Context, userID string, limit int) ([]XpEventView, error) {
	candidateID, err := s.resolveCandidateID(ctx, userID)
	if err != nil {
		return nil, err
	}

	order, take := "DESC", limit
	if limit < 0 {
		// Prisma: отрицательный take — «хвост» журнала (самые старые события).
		order, take = "ASC", -limit
	}

	rows, err := s.db.Pool.Query(ctx, fmt.Sprintf(`
		SELECT id, action, xp, metadata::text, created_at
		FROM xp_events
		WHERE candidate_id = $1
		ORDER BY created_at %s
		LIMIT $2`, order), candidateID, take)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []XpEventView{}
	for rows.Next() {
		var e XpEventView
		var metaText *string
		if err := rows.Scan(&e.ID, &e.Action, &e.Xp, &metaText, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Metadata = jsTruthyJSON(metaText)
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if limit < 0 {
		for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
			list[i], list[j] = list[j], list[i]
		}
	}
	return list, nil
}

// SeedBadges — заполнение таблицы badges определениями (seedBadges в TS).
//
// BUG-REF: G3 — в TS метод не имеет вызывающего кода; здесь он тоже реализован,
// но НИГДЕ не вызывается (как и в оригинале) — бейджи попадают в БД сидом/вручную.
func (s *Service) SeedBadges(ctx context.Context) (int, error) {
	count := 0
	for _, def := range BADGE_DEFINITIONS {
		var existing string
		err := s.db.Pool.QueryRow(ctx, `SELECT id FROM badges WHERE code = $1`, def.Code).Scan(&existing)
		if errors.Is(err, pgx.ErrNoRows) {
			if _, err := s.db.Pool.Exec(ctx, `
				INSERT INTO badges (id, code, name, description, icon, category, rarity, xp_reward)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				newID(), def.Code, def.Name, def.Description, def.Icon, def.Category, def.Rarity, def.XpReward); err != nil {
				return count, err
			}
			count++
			continue
		}
		if err != nil {
			return count, err
		}
	}
	log.Printf("[Gamification] Seeded %d badges", count)
	return count, nil
}

// ─────────────────────────── Внутренние помощники ───────────────────────────

type profileRow struct {
	ID             string
	CandidateID    string
	Xp             int
	Level          int
	Streak         int
	LongestStreak  int
	LastActiveDate *model.Time
	Multiplier     float64
}

// resolveCandidateId — userId → CandidateProfile.id (resolveCandidateId в TS).
func (s *Service) resolveCandidateID(ctx context.Context, userID string) (string, error) {
	var id string
	err := s.db.Pool.QueryRow(ctx, `SELECT id FROM candidate_profiles WHERE user_id = $1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		// TS: throw new Error(`Candidate profile not found for user ${userId}`) → 500 с этим текстом.
		return "", fmt.Errorf("Candidate profile not found for user %s", userID)
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Service) loadProfile(ctx context.Context, candidateID string) (*profileRow, error) {
	var p profileRow
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, candidate_id, xp, level, streak, longest_streak, last_active_date, multiplier
		FROM gamification_profiles
		WHERE candidate_id = $1`, candidateID).
		Scan(&p.ID, &p.CandidateID, &p.Xp, &p.Level, &p.Streak, &p.LongestStreak, &p.LastActiveDate, &p.Multiplier)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// getOrCreateProfile — getOrCreateProfile в TS (upsert по unique candidate_id).
func (s *Service) getOrCreateProfile(ctx context.Context, candidateID string) (*profileRow, error) {
	p, err := s.loadProfile(ctx, candidateID)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if _, err := s.db.Pool.Exec(ctx,
		`INSERT INTO gamification_profiles (id, candidate_id, updated_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (candidate_id) DO NOTHING`,
		newID(), candidateID, model.Now()); err != nil {
		return nil, err
	}
	return s.loadProfile(ctx, candidateID)
}

// updateStreakFields — update({data: {...}}) в updateStreak.
// touchLongest — пересчёт longestStreak = max(...) только для «соседнего» дня.
func (s *Service) updateStreakFields(ctx context.Context, candidateID string, streak int, multiplier float64, touchLongest bool) error {
	now := model.Now()
	if touchLongest {
		_, err := s.db.Pool.Exec(ctx, `
			UPDATE gamification_profiles
			SET streak = $1,
			    longest_streak = GREATEST($1, longest_streak),
			    last_active_date = $2,
			    multiplier = $3,
			    updated_at = $2
			WHERE candidate_id = $4`,
			streak, now, multiplier, candidateID)
		return err
	}
	_, err := s.db.Pool.Exec(ctx, `
		UPDATE gamification_profiles
		SET streak = $1,
		    last_active_date = $2,
		    multiplier = $3,
		    updated_at = $2
		WHERE candidate_id = $4`,
		streak, now, multiplier, candidateID)
	return err
}

// checkStreakBadges — проверка стрик-бейджей (3/7/30).
func (s *Service) checkStreakBadges(ctx context.Context, candidateID string, streak int) error {
	if streak >= 3 {
		if err := s.AwardBadge(ctx, candidateID, "STREAK_3"); err != nil {
			return err
		}
	}
	if streak >= 7 {
		if err := s.AwardBadge(ctx, candidateID, "STREAK_7"); err != nil {
			return err
		}
	}
	if streak >= 30 {
		if err := s.AwardBadge(ctx, candidateID, "STREAK_30"); err != nil {
			return err
		}
	}
	return nil
}

// toGamificationProfile — DTO профиля (toGamificationProfile в TS).
//
// BUG-REF: G4 — воспроизводится дословно: переменная xpToNextLevel читает
// несуществующее поле profile.xpRequired (undefined → NaN), поэтому условие
// `xpToNextLevel > 0` всегда ложно → xpProgress = 100, а в ответ уходит
// `nextLevel?.xpRequired || profile.xp` — абсолютный порог, а не остаток.
func (s *Service) toGamificationProfile(p *profileRow, badges []ProfileBadge) map[string]any {
	levelInfo := getLevelInfo(p.Level)
	next := nextLevel(p.Level)

	xpRequiredMissing := math.NaN() // аналог profile.xpRequired — поля нет в модели
	xpToNextLevel := 0.0
	if next != nil {
		xpToNextLevel = float64(next.XpRequired) - xpRequiredMissing // NaN
	}
	xpProgress := 100.0
	if xpToNextLevel > 0 { // NaN > 0 → false (ветка недостижима, как и в TS)
		xpProgress = math.Min(100, ((float64(p.Xp)-float64(levelInfo.XpRequired))/xpToNextLevel)*100)
	}

	xpToNextLevelOut := p.Xp
	if next != nil {
		xpToNextLevelOut = next.XpRequired
	}

	var lastActiveDate any
	if p.LastActiveDate != nil {
		lastActiveDate = *p.LastActiveDate
	}

	return map[string]any{
		"candidateId":    p.CandidateID,
		"xp":             p.Xp,
		"level":          p.Level,
		"streak":         p.Streak,
		"longestStreak":  p.LongestStreak,
		"lastActiveDate": lastActiveDate,
		"multiplier":     p.Multiplier,
		"levelName":      levelInfo.Name,
		"xpToNextLevel":  xpToNextLevelOut,
		"xpProgress":     int(math.Round(xpProgress)),
		"badges":         badges,
	}
}

// displayName — формат имени для leaderboard.
// `Кандидат #<первые 4 символа id>` или `Имя Ф.` (с сохранением пробела перед точкой,
// как в шаблоне `${firstName} ${lastName?.charAt(0) || ”}.`).
func displayName(firstName, lastName *string, candidateID string) string {
	if firstName != nil && *firstName != "" {
		initial := ""
		if lastName != nil && *lastName != "" {
			for _, r := range *lastName {
				initial = string(r)
				break
			}
		}
		return fmt.Sprintf("%s %s.", *firstName, initial)
	}
	return fmt.Sprintf("Кандидат #%s", firstChars(candidateID, 4))
}

func firstChars(s string, n int) string {
	i := 0
	for idx := range s {
		if i == n {
			return s[:idx]
		}
		i++
	}
	return s
}

// isKnownSpecialization — проверка значения enum Specialization (валидация Prisma в TS).
func isKnownSpecialization(s string) bool {
	for _, v := range model.AllSpecializations {
		if string(v) == s {
			return true
		}
	}
	return false
}

// jsTruthyJSON — `(e.metadata as Record<string, unknown>) || undefined`:
// null/ложные JSON-значения превращаются в отсутствие ключа.
func jsTruthyJSON(text *string) any {
	if text == nil {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(*text), &v); err != nil {
		return nil
	}
	switch typed := v.(type) {
	case nil:
		return nil
	case bool:
		if !typed {
			return nil
		}
	case float64:
		if typed == 0 {
			return nil
		}
	case string:
		if typed == "" {
			return nil
		}
	}
	return v
}

// newID — UUID v4 (uuid.NewString() в TS-версии), формат google/uuid.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
