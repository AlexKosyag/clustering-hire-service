package matching

import (
	"context"

	"fsp-backend/internal/modules/candidateview"
	"fsp-backend/internal/store"
)

// Загрузка кандидатов с вложениями (candidateInclude: achievements + user.email)
// для matching-пула. Порядок вложенных достижений — ORDER BY ca.id (как в
// candidateview.Load); пул-запросы без ORDER BY — Prisma findMany их не делает
// (см. отчёт об амбивалентностях).

const candidateColumns = `cp.id,
			cp.first_name, cp.last_name, cp.middle_name,
			cp.phone, cp.city, cp.bio,
			cp.specialization, cp.declared_grade, cp.verified_grade,
			cp.stack, cp.roles, cp.soft_skills, cp.experience_years,
			cp.test_status, cp.last_test_date, cp.last_test_score,
			cp.show_full_name, cp.updated_at,
			u.email`

const candidateFrom = ` FROM candidate_profiles cp
			JOIN users u ON u.id = cp.user_id`

// scanCandidates — выполнение запроса с колонками candidateInclude → []*Candidate.
func scanCandidates(ctx context.Context, db *store.DB, query string, args ...any) ([]*candidateview.Candidate, error) {
	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*candidateview.Candidate
	for rows.Next() {
		var c candidateview.Candidate
		if err := rows.Scan(
			&c.ID,
			&c.FirstName, &c.LastName, &c.MiddleName,
			&c.Phone, &c.City, &c.Bio,
			&c.Specialization, &c.DeclaredGrade, &c.VerifiedGrade,
			&c.Stack, &c.Roles, &c.SoftSkills, &c.ExperienceYears,
			&c.TestStatus, &c.LastTestDate, &c.LastTestScore,
			&c.ShowFullName, &c.UpdatedAt,
			&c.Email,
		); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// loadAchievements — достижения для пула кандидатов (по candidate_id).
func loadAchievements(ctx context.Context, db *store.DB, ids []string) (map[string][]candidateview.Achievement, error) {
	out := map[string][]candidateview.Achievement{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT ca.candidate_id, a.name, a.category, a.points, a.year, a.place
		FROM candidate_achievements ca
		JOIN fsp_achievements a ON a.id = ca.achievement_id
		WHERE ca.candidate_id = ANY($1::text[])
		ORDER BY ca.candidate_id, ca.id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var candidateID string
		var a candidateview.Achievement
		if err := rows.Scan(&candidateID, &a.Name, &a.Category, &a.Points, &a.Year, &a.Place); err != nil {
			return nil, err
		}
		out[candidateID] = append(out[candidateID], a)
	}
	return out, rows.Err()
}

func attachAchievements(candidates []*candidateview.Candidate, byID map[string][]candidateview.Achievement) {
	for _, c := range candidates {
		if a, ok := byID[c.ID]; ok {
			c.Achievements = a
		} else {
			c.Achievements = []candidateview.Achievement{}
		}
	}
}

// loadCandidatesByID — findMany({where: {id: {in: ids}}, include: candidateInclude}).
func loadCandidatesByID(ctx context.Context, db *store.DB, ids []string) ([]*candidateview.Candidate, error) {
	if len(ids) == 0 {
		return []*candidateview.Candidate{}, nil
	}
	candidates, err := scanCandidates(ctx, db,
		`SELECT `+candidateColumns+candidateFrom+` WHERE cp.id = ANY($1::text[])`, ids)
	if err != nil {
		return nil, err
	}
	byID, err := loadAchievements(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	attachAchievements(candidates, byID)
	return candidates, nil
}

// loadCandidatesOrdered — то же, что loadCandidatesByID, но результат
// переупорядочивается по порядку id-списка (Prisma in-запрос порядок не гарантирует).
func loadCandidatesOrdered(ctx context.Context, db *store.DB, ids []string) ([]*candidateview.Candidate, error) {
	candidates, err := loadCandidatesByID(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	byID := map[string]*candidateview.Candidate{}
	for _, c := range candidates {
		byID[c.ID] = c
	}
	ordered := make([]*candidateview.Candidate, 0, len(ids))
	for _, id := range ids {
		if c, ok := byID[id]; ok {
			ordered = append(ordered, c)
		}
	}
	return ordered, nil
}

// loadAllCandidates — findMany({include: candidateInclude, take: limit}).
func loadAllCandidates(ctx context.Context, db *store.DB, limit int) ([]*candidateview.Candidate, error) {
	candidates, err := scanCandidates(ctx, db,
		`SELECT `+candidateColumns+candidateFrom+` LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(candidates))
	for _, c := range candidates {
		ids = append(ids, c.ID)
	}
	byID, err := loadAchievements(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	attachAchievements(candidates, byID)
	return candidates, nil
}
