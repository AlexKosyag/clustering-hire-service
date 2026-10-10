package fsp

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
	"fsp-backend/internal/modules/matching"
	"fsp-backend/internal/store"
)

// Service — FSP Integration (MVP — mock), см. fsp.service.ts.
type Service struct {
	db       *store.DB
	matching *matching.Service
}

// NewService — конструктор для интегратора.
func NewService(db *store.DB, matching *matching.Service) *Service {
	return &Service{db: db, matching: matching}
}

// profileIDByUser — профиль кандидата по user_id (findUnique({where: {userId}})).
func (s *Service) profileIDByUser(ctx context.Context, userID string) (string, error) {
	var id string
	err := s.db.Pool.QueryRow(ctx, `SELECT id FROM candidate_profiles WHERE user_id = $1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", common.NotFound("Профиль кандидата не найден")
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

// Connect — подключение аккаунта ФСП (FspService.connect).
func (s *Service) Connect(ctx context.Context, userID, fspID string) (*ConnectResult, error) {
	profileID, err := s.profileIDByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	normalized := strings.TrimSpace(fspID)
	if !fspIDRe.MatchString(normalized) {
		// Дубль проверки из DTO (по HTTP недостижима: ValidationPipe отсекает раньше).
		return nil, common.BadRequest("Некорректный формат FSP ID (4-40 символов: буквы, цифры, -, _)")
	}

	var takenID string
	err = s.db.Pool.QueryRow(ctx, `SELECT id FROM candidate_profiles WHERE fsp_id = $1`, normalized).Scan(&takenID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil && takenID != profileID {
		// В TS — BadRequestException (400); docs 04 §8 и задание требуют 409.
		return nil, common.Conflict("Этот FSP ID уже привязан к другому аккаунту")
	}

	achievements, err := s.MockFetchAchievements(ctx, normalized)
	if err != nil {
		return nil, err
	}

	// prisma.$transaction: fspId + fspConnectedAt + upsert CandidateAchievement.
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	now := model.Now()
	if _, err := tx.Exec(ctx,
		`UPDATE candidate_profiles SET fsp_id = $1, fsp_connected_at = $2, updated_at = $2 WHERE id = $3`,
		normalized, now, profileID); err != nil {
		return nil, err
	}

	// Каталог для сопоставления по названию (как new Map(catalog.map(a => [a.name, a.id])) —
	// при дублях названий выигрывает последняя запись).
	rows, err := tx.Query(ctx, `SELECT id, name FROM fsp_achievements`)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]string)
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		byKey[name] = id
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	for _, a := range achievements {
		achievementID, ok := byKey[a.Name]
		if !ok {
			continue
		}
		// upsert с update: {} → идемпотентно (ON CONFLICT DO NOTHING).
		if _, err := tx.Exec(ctx,
			`INSERT INTO candidate_achievements (id, candidate_id, achievement_id)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (candidate_id, achievement_id) DO NOTHING`,
			newID(), profileID, achievementID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Фоновые сайд-эффекты; ошибки проглатываются (.catch(() => undefined) в TS).
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

// Disconnect — отключение аккаунта ФСП (FspService.disconnect).
func (s *Service) Disconnect(ctx context.Context, userID string) (*DisconnectResult, error) {
	profileID, err := s.profileIDByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE candidate_profiles SET fsp_id = NULL, fsp_connected_at = NULL, updated_at = $2 WHERE id = $1`,
		profileID, model.Now()); err != nil {
		return nil, err
	}
	if _, err := s.db.Pool.Exec(ctx,
		`DELETE FROM candidate_achievements WHERE candidate_id = $1`, profileID); err != nil {
		return nil, err
	}

	s.swallow(func() error { return s.matching.InvalidateCache(ctx) })

	return &DisconnectResult{FspID: nil, Disconnected: true}, nil
}

// Achievements — достижения ФСП кандидата (FspService.achievements).
func (s *Service) Achievements(ctx context.Context, userID string) (*AchievementsResult, error) {
	profileID, err := s.profileIDByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var fspID *string
	var fspConnectedAt *model.Time
	if err := s.db.Pool.QueryRow(ctx,
		`SELECT fsp_id, fsp_connected_at FROM candidate_profiles WHERE id = $1`, profileID).
		Scan(&fspID, &fspConnectedAt); err != nil {
		return nil, err
	}

	rows, err := s.db.Pool.Query(ctx, `
		SELECT a.id, a.name, a.description, a.category, a.year, a.place, a.points, ca.verified_at
		FROM candidate_achievements ca
		JOIN fsp_achievements a ON a.id = ca.achievement_id
		WHERE ca.candidate_id = $1`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AchievementItem{}
	points := 0
	for rows.Next() {
		var item AchievementItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Category,
			&item.Year, &item.Place, &item.Points, &item.VerifiedAt); err != nil {
			return nil, err
		}
		points += item.Points
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &AchievementsResult{
		FspID:          fspID,
		FspConnectedAt: fspConnectedAt,
		Achievements:   items,
		Points:         points,
	}, nil
}

// Catalog — справочник достижений (FspService.catalog).
func (s *Service) Catalog(ctx context.Context) (*CatalogResult, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, name, description, category, year, place, points
		FROM fsp_achievements
		ORDER BY category ASC, points DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []model.FSPAchievement{}
	for rows.Next() {
		var item model.FSPAchievement
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Category,
			&item.Year, &item.Place, &item.Points); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &CatalogResult{Total: len(items), Achievements: items}, nil
}

// MockFetchAchievements — детерминированный «запрос» к внешнему сервису ФСП (mock).
// FNV-1a от fspId → LCG → выбор 2–4 достижений из каталога (сортировка по points desc).
func (s *Service) MockFetchAchievements(ctx context.Context, fspID string) ([]FspAchievementDto, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT name, description, category, year, place, points
		FROM fsp_achievements
		ORDER BY points DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type catalogRow struct {
		name        string
		description *string
		category    string
		year        *int
		place       *string
		points      int
	}
	catalog := []catalogRow{}
	for rows.Next() {
		var r catalogRow
		if err := rows.Scan(&r.name, &r.description, &r.category, &r.year, &r.place, &r.points); err != nil {
			return nil, err
		}
		catalog = append(catalog, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(catalog) == 0 {
		return []FspAchievementDto{}, nil
	}

	// FNV-1a 32-bit по UTF-16 code units (charCodeAt в TS).
	hash := uint32(2166136261)
	for _, cu := range utf16Units(fspID) {
		hash ^= uint32(cu)
		hash = hash * 16777619 // Math.imul(hash, 16777619) — младшие 32 бита
	}
	seed := hash // hash >>> 0
	random := func() float64 {
		seed = seed*1664525 + 1013904223 // Math.imul + >>> 0 (переполнение uint32)
		return float64(seed) / 4294967296
	}

	count := 2 + int(math.Floor(random()*3)) // 2-4 достижения
	// Set в JS сохраняет порядок вставки — повторяем и порядок, и потребление random().
	picked := make([]int, 0, count)
	seen := make(map[int]bool, count)
	for len(picked) < int(math.Min(float64(count), float64(len(catalog)))) {
		i := int(math.Floor(random() * float64(len(catalog))))
		if !seen[i] {
			seen[i] = true
			picked = append(picked, i)
		}
	}

	out := make([]FspAchievementDto, 0, len(picked))
	for _, i := range picked {
		r := catalog[i]
		year := 2024
		if r.year != nil {
			year = *r.year
		}
		out = append(out, FspAchievementDto{
			Name:        r.name,
			Description: r.description,
			Category:    r.category,
			Year:        year, // year ?? 2024
			Place:       r.place,
			Points:      r.points,
		})
	}
	return out, nil
}

// swallow — эквивалент .catch(() => undefined): ошибка/паника сайд-эффекта игнорируется.
func (s *Service) swallow(fn func() error) {
	defer func() { _ = recover() }()
	_ = fn()
}

// utf16Units — UTF-16 code units строки (charCodeAt в JS).
func utf16Units(s string) []uint16 {
	out := make([]uint16, 0, len(s))
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		} else {
			out = append(out, uint16(r))
		}
	}
	return out
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
