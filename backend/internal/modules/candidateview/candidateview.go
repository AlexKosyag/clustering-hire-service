// Package candidateview — публичное представление кандидата (аналог
// backend/src/modules/matching/candidate-view.ts: candidateInclude,
// buildDisplayName, achievementPoints, toCandidatePublic) и его загрузка из БД.
//
// Общий хелпер модулей employers и invitations (docs 03 §10: email/телефон/ФИО
// в публичное представление не входят).
package candidateview

import (
	"context"
	"strings"

	"fsp-backend/internal/model"
	"fsp-backend/internal/store"
)

// Achievement — достижение ФСП кандидата (без служебных полей связи).
type Achievement struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Points   int     `json:"points"`
	Year     *int    `json:"year"`
	Place    *string `json:"place"`
}

// Candidate — профиль кандидата с вложениями (аналог CandidateWithRelations).
type Candidate struct {
	ID string

	FirstName  *string
	LastName   *string
	MiddleName *string
	Phone      *string
	City       *string
	Bio        *string

	Specialization  *string
	DeclaredGrade   *string
	VerifiedGrade   *string
	Stack           []string
	Roles           []string
	ExperienceYears *float64
	SoftSkills      []string

	TestStatus    string
	LastTestDate  *model.Time
	LastTestScore *float64

	ShowFullName bool

	UpdatedAt model.Time

	Email        *string // users.email (include: user { select: { email: true } })
	Achievements []Achievement
}

// Load — кандидат по id профиля вместе с достижениями и email пользователя
// (candidateInclude). При отсутствии возвращает (nil, nil).
func Load(ctx context.Context, db *store.DB, candidateID string) (*Candidate, error) {
	var c Candidate
	err := db.Pool.QueryRow(ctx, `
		SELECT cp.id,
		       cp.first_name, cp.last_name, cp.middle_name,
		       cp.phone, cp.city, cp.bio,
		       cp.specialization, cp.declared_grade, cp.verified_grade,
		       cp.stack, cp.roles, cp.soft_skills, cp.experience_years,
		       cp.test_status, cp.last_test_date, cp.last_test_score,
		       cp.show_full_name, cp.updated_at,
		       u.email
		FROM candidate_profiles cp
		JOIN users u ON u.id = cp.user_id
		WHERE cp.id = $1`,
		candidateID,
	).Scan(
		&c.ID,
		&c.FirstName, &c.LastName, &c.MiddleName,
		&c.Phone, &c.City, &c.Bio,
		&c.Specialization, &c.DeclaredGrade, &c.VerifiedGrade,
		&c.Stack, &c.Roles, &c.SoftSkills, &c.ExperienceYears,
		&c.TestStatus, &c.LastTestDate, &c.LastTestScore,
		&c.ShowFullName, &c.UpdatedAt,
		&c.Email,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, err
	}

	rows, err := db.Pool.Query(ctx, `
		SELECT a.name, a.category, a.points, a.year, a.place
		FROM candidate_achievements ca
		JOIN fsp_achievements a ON a.id = ca.achievement_id
		WHERE ca.candidate_id = $1
		ORDER BY ca.id`,
		candidateID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	c.Achievements = []Achievement{}
	for rows.Next() {
		var a Achievement
		if err := rows.Scan(&a.Name, &a.Category, &a.Points, &a.Year, &a.Place); err != nil {
			return nil, err
		}
		c.Achievements = append(c.Achievements, a)
	}
	return &c, rows.Err()
}

// BuildDisplayName — «Иван И.» — как в документации; при showFullName === false
// — псевдоним «Кандидат #<первые 4 символа id>».
func BuildDisplayName(c *Candidate) string {
	first := ""
	if c.FirstName != nil {
		first = *c.FirstName
	}
	last := ""
	if c.LastName != nil {
		last = *c.LastName
	}
	anonymized := !c.ShowFullName
	if anonymized || (first == "" && last == "") {
		id := c.ID
		if len(id) > 4 {
			id = id[:4]
		}
		return "Кандидат #" + id
	}
	lastShort := ""
	if last != "" {
		lastShort = last[:1] + "."
	}
	return strings.TrimSpace(first + " " + lastShort)
}

// AchievementPoints — сумма points всех достижений.
func AchievementPoints(c *Candidate) int {
	sum := 0
	for _, a := range c.Achievements {
		sum += a.Points
	}
	return sum
}

// ToPublic — публичное представление кандидата (toCandidatePublic).
// matchScore/matchExplanation/matchComponents сюда не входят: их добавляет
// matching-движок поверх этого представления.
func ToPublic(c *Candidate, indexed bool) map[string]any {
	stack := c.Stack
	if stack == nil {
		stack = []string{}
	}
	roles := c.Roles
	if roles == nil {
		roles = []string{}
	}
	softSkills := c.SoftSkills
	if softSkills == nil {
		softSkills = []string{}
	}
	achievements := make([]map[string]any, 0, len(c.Achievements))
	for _, a := range c.Achievements {
		achievements = append(achievements, map[string]any{
			"name":     a.Name,
			"category": a.Category,
			"points":   a.Points,
			"year":     a.Year,
			"place":    a.Place,
		})
	}
	return map[string]any{
		"id":                c.ID,
		"displayName":       BuildDisplayName(c),
		"specialization":    c.Specialization,
		"declaredGrade":     c.DeclaredGrade,
		"verifiedGrade":     c.VerifiedGrade,
		"stack":             stack,
		"roles":             roles,
		"experienceYears":   c.ExperienceYears,
		"city":              c.City,
		"softSkills":        softSkills,
		"bio":               c.Bio,
		"testStatus":        c.TestStatus,
		"testScore":         c.LastTestScore,
		"lastTestDate":      c.LastTestDate,
		"achievements":      achievements,
		"achievementPoints": AchievementPoints(c),
		"lastActive":        c.UpdatedAt,
		"indexed":           indexed,
	}
}
