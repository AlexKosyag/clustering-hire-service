package seed

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"fsp-backend/internal/store"
)

// Демо-пароль всех аккаунтов сида (docs: VALIDATION.md).
const DemoPassword = "Test123!"

const bcryptCost = 10
const day = 24 * time.Hour

// thetaFor — thetaEstimate по грейду (seed.ts thetaFor).
func thetaFor(grade string) float64 {
	switch grade {
	case "JUNIOR":
		return -1.6
	case "MIDDLE":
		return -0.4
	case "MIDDLE_PLUS":
		return 0.3
	case "SENIOR":
		return 0.8
	case "LEAD":
		return 1.3
	case "PRINCIPAL":
		return 1.8
	}
	return 0
}

// Run — полный цикл prisma/seed.ts: очистка, справочник, вопросы, кандидаты,
// работодатели, вакансии, приглашения, потребности.
func Run(ctx context.Context, db *store.DB, out io.Writer) error {
	now := time.Now()

	fmt.Fprintln(out, "▶ Очистка данных...")
	if err := cleanup(ctx, db); err != nil {
		return err
	}

	fmt.Fprintln(out, "▶ Справочник достижений ФСП...")
	achievementIDByName, err := seedAchievements(ctx, db)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "▶ Категории и вопросы...")
	questionSeeds, err := LoadQuestions()
	if err != nil {
		return err
	}
	categoryIDBySpec, err := seedCategories(ctx, db, questionSeeds.Categories)
	if err != nil {
		return err
	}
	total, countsJSON, err := seedQuestions(ctx, db, questionSeeds, categoryIDBySpec)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "   категорий: %d, вопросов: %d\n", len(questionSeeds.Categories), total)
	fmt.Fprintf(out, "   по категориям: %s\n", countsJSON)

	fmt.Fprintln(out, "▶ Демо-пароль и кандидаты...")
	hash, err := bcrypt.GenerateFromPassword([]byte(DemoPassword), bcryptCost)
	if err != nil {
		return err
	}
	candidates, err := LoadCandidates()
	if err != nil {
		return err
	}
	cStats, err := seedCandidates(ctx, db, now, string(hash), candidates, achievementIDByName, categoryIDBySpec)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "   кандидатов: %d, связок достижений: %d, сессий тестов: %d\n",
		cStats.total, cStats.achievementLinks, cStats.testedSessions)

	fmt.Fprintln(out, "▶ Работодатели...")
	companies, err := LoadCompanies()
	if err != nil {
		return err
	}
	employerIDs, err := seedEmployers(ctx, db, now, string(hash), companies.Employers)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "   работодателей: %d\n", len(employerIDs))

	fmt.Fprintln(out, "▶ Вакансии...")
	vacancySeeds, err := LoadVacancies()
	if err != nil {
		return err
	}
	vacancyIDs, err := seedVacancies(ctx, db, now, vacancySeeds, employerIDs)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "   вакансий: %d\n", len(vacancyIDs))

	fmt.Fprintln(out, "▶ Приглашения...")
	invitationSeeds, err := LoadInvitations()
	if err != nil {
		return err
	}
	invitations, err := seedInvitations(ctx, db, now, invitationSeeds, employerIDs, vacancyIDs, cStats.profileIDByEmail)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "   приглашений: %d\n", invitations)

	fmt.Fprintln(out, "▶ Потребности работодателей (вход matching)...")
	needSeeds, err := LoadNeeds()
	if err != nil {
		return err
	}
	needs, err := seedNeeds(ctx, db, now, needSeeds, employerIDs)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "   потребностей: %d\n", needs)

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "✅ Seed завершён.")
	fmt.Fprintln(out, "   Демо-аккаунты (пароль Test123!):")
	fmt.Fprintln(out, "   - test@candidate.com  (кандидат, Backend/Middle)")
	fmt.Fprintln(out, "   - demo2@candidate.com (кандидат, Frontend/Senior)")
	fmt.Fprintln(out, "   - demo3@candidate.com (кандидат, DevOps/Middle+)")
	fmt.Fprintln(out, "   - test@employer.com   (работодатель, ООО «ТехноСтар»)")
	fmt.Fprintln(out, "   - hr@company.com      (работодатель, АО «Северный Кластер»)")
	return nil
}

// cleanup — prisma.*.deleteMany() в порядке seed.ts.
func cleanup(ctx context.Context, db *store.DB) error {
	tables := []string{
		"test_answers",
		"test_sessions",
		"test_questions",
		"test_categories",
		"invitations",
		"employer_needs",
		"vacancies",
		"candidate_achievements",
		"daily_tasks",
		"candidate_profiles",
		"employer_profiles",
		"fsp_achievements",
		"users",
	}
	for _, table := range tables {
		if _, err := db.Pool.Exec(ctx, `DELETE FROM `+table); err != nil {
			return fmt.Errorf("cleanup %s: %w", table, err)
		}
	}
	return nil
}

// seedAchievements — createMany(FSP_ACHIEVEMENTS, skipDuplicates) + findMany.
func seedAchievements(ctx context.Context, db *store.DB) (map[string]string, error) {
	achievements, err := LoadAchievements()
	if err != nil {
		return nil, err
	}
	for _, a := range achievements {
		if _, err := db.Pool.Exec(ctx, `
			INSERT INTO fsp_achievements (id, name, description, category, year, place, points)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT DO NOTHING`,
			uuid.NewString(), a.Name, a.Description, a.Category, a.Year, a.Place, a.Points); err != nil {
			return nil, fmt.Errorf("achievement %q: %w", a.Name, err)
		}
	}
	rows, err := db.Pool.Query(ctx, `SELECT id, name FROM fsp_achievements`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byName := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		byName[name] = id
	}
	return byName, rows.Err()
}

// seedCategories — createMany(testCategory, skipDuplicates) + findMany по specialization.
func seedCategories(ctx context.Context, db *store.DB, categories []Category) (map[string]string, error) {
	for _, c := range categories {
		if _, err := db.Pool.Exec(ctx, `
			INSERT INTO test_categories (id, name, description, specialization)
			VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
			uuid.NewString(), c.Name, c.Description, c.Specialization); err != nil {
			return nil, fmt.Errorf("category %q: %w", c.Key, err)
		}
	}
	rows, err := db.Pool.Query(ctx, `SELECT id, specialization FROM test_categories`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bySpec := map[string]string{}
	for rows.Next() {
		var id, spec string
		if err := rows.Scan(&id, &spec); err != nil {
			return nil, err
		}
		bySpec[spec] = id
	}
	return bySpec, rows.Err()
}

// seedQuestions — createMany(testQuestion, batches 200, skipDuplicates).
func seedQuestions(ctx context.Context, db *store.DB, seeds *QuestionSeeds, categoryIDBySpec map[string]string) (int, string, error) {
	// countsByCategory: порядок появления ключей как в объекте JS.
	counts := map[string]int{}
	order := []string{}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback(ctx)

	for i, q := range seeds.Questions {
		categoryID, ok := categoryIDBySpec[q.Key]
		if !ok {
			return 0, "", fmt.Errorf("нет категории для вопроса с ключом %q", q.Key)
		}
		if _, ok := counts[q.Key]; !ok {
			order = append(order, q.Key)
		}
		counts[q.Key]++

		// q.codeSnippet/correctAnswer/explanation/poolTag: null → SQL NULL;
		// options — JSONB, null → SQL NULL (seed.ts `?? null`).
		if _, err := tx.Exec(ctx, `
			INSERT INTO test_questions (id, category_id, question_type, text, code_snippet,
			                            options, correct_answer, explanation, difficulty,
			                            discrimination, guessing, is_unique, pool_tag)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT DO NOTHING`,
			uuid.NewString(), categoryID, q.QuestionType, q.Text, q.CodeSnippet,
			q.OptionsValue(), q.CorrectAnswer, q.Explanation, q.Difficulty,
			q.Discrimination, q.Guessing, q.IsUnique, q.PoolTag); err != nil {
			return 0, "", fmt.Errorf("question %d: %w", i, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, "", err
	}

	// JSON.stringify(counts) — ключи в порядке появления.
	var sb strings.Builder
	sb.WriteByte('{')
	for i, key := range order {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%q:%d", key, counts[key])
	}
	sb.WriteByte('}')
	return len(seeds.Questions), sb.String(), nil
}

type candidateStats struct {
	total            int
	achievementLinks int
	testedSessions   int
	profileIDByEmail map[string]string
}

// seedCandidates — user + candidateProfile на кандидата, связи достижений,
// история тестирований (без ответов).
func seedCandidates(ctx context.Context, db *store.DB, now time.Time, passwordHash string,
	candidates []Candidate, achievementIDByName map[string]string, categoryIDBySpec map[string]string,
) (*candidateStats, error) {
	stats := &candidateStats{profileIDByEmail: map[string]string{}}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	type created struct {
		seed      *Candidate
		profileID string
	}
	createdList := make([]created, 0, len(candidates))

	for i := range candidates {
		c := &candidates[i]
		userID := uuid.NewString()
		profileID := uuid.NewString()

		if _, err := tx.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, role, is_email_verified, created_at, updated_at)
			VALUES ($1, $2, $3, 'CANDIDATE', true, $4, $4)`,
			userID, c.Email, passwordHash, now); err != nil {
			return nil, fmt.Errorf("candidate user %s: %w", c.Email, err)
		}

		// fspConnectedAt: fspId ? now - 60d : null (seed.ts)
		var fspConnectedAt *time.Time
		if c.FspID != nil {
			t := now.Add(-60 * day)
			fspConnectedAt = &t
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO candidate_profiles (id, user_id, first_name, last_name, middle_name, phone, city, bio,
			                                specialization, declared_grade, verified_grade, stack, roles,
			                                experience_years, soft_skills, fsp_id, fsp_connected_at, test_status,
			                                last_test_date, test_cooldown_until, last_test_score,
			                                show_email, show_phone, show_full_name, consent_to_processing,
			                                created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8,
			        $9, $10, $11, $12, $13,
			        $14, $15, $16, $17, $18,
			        $19, $20, $21,
			        $22, $23, $24, $25,
			        $26, $26)`,
			profileID, userID, c.FirstName, c.LastName, c.MiddleName, c.Phone, c.City, c.Bio,
			c.Specialization, c.DeclaredGrade, c.VerifiedGrade, c.Stack, c.Roles,
			c.ExperienceYears, c.SoftSkills, c.FspID, fspConnectedAt, c.TestStatus,
			c.LastTestDate.At(now), c.TestCooldownUntil.At(now), c.LastTestScore,
			c.ShowEmail, c.ShowPhone, c.ShowFullName, c.ConsentToProcess,
			now); err != nil {
			return nil, fmt.Errorf("candidate profile %s: %w", c.Email, err)
		}

		createdList = append(createdList, created{seed: c, profileID: profileID})
		stats.profileIDByEmail[c.Email] = profileID
	}

	// Связка достижений ФСП (batches 500, skipDuplicates).
	for _, item := range createdList {
		for _, name := range item.seed.AchievementNames {
			achievementID, ok := achievementIDByName[name]
			if !ok {
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO candidate_achievements (id, candidate_id, achievement_id)
				VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
				uuid.NewString(), item.profileID, achievementID); err != nil {
				return nil, fmt.Errorf("candidate achievement: %w", err)
			}
			stats.achievementLinks++
		}
	}

	// История тестирований (без ответов — реальные сессии создаёт движок).
	for _, item := range createdList {
		c := item.seed
		if c.TestStatus == "NOT_STARTED" || !c.LastTestDate.ok {
			continue
		}
		completed := *c.LastTestDate.At(now)
		started := completed.Add(-45 * time.Minute)

		score := 0.0
		if c.LastTestScore != nil {
			score = *c.LastTestScore
		}
		gradeAfter := c.DeclaredGrade
		if c.VerifiedGrade != nil {
			gradeAfter = *c.VerifiedGrade
		}
		recommendation := "PASS"
		switch c.TestStatus {
		case "FAILED":
			recommendation = "FAIL"
		case "UPGRADED":
			recommendation = "UPGRADE"
		}
		categoryID := categoryIDBySpec[c.Specialization]

		if _, err := tx.Exec(ctx, `
			INSERT INTO test_sessions (id, candidate_id, category_id, status, started_at, completed_at,
			                           time_limit_minutes, score, raw_score, max_score, grade_before,
			                           grade_after, recommendation, cheat_flags, theta_estimate)
			VALUES ($1, $2, $3, 'COMPLETED', $4, $5, 60, $6, $6, 100, $7, $8, $9, '{}', $10)`,
			uuid.NewString(), item.profileID, categoryID, started, completed,
			score, c.DeclaredGrade, gradeAfter, recommendation, thetaFor(gradeAfter)); err != nil {
			return nil, fmt.Errorf("test session %s: %w", c.Email, err)
		}
		stats.testedSessions++
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	stats.total = len(createdList)
	return stats, nil
}

// seedEmployers — user + employerProfile на работодателя.
func seedEmployers(ctx context.Context, db *store.DB, now time.Time, passwordHash string, employers []Employer) ([]string, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	ids := make([]string, 0, len(employers))
	for _, e := range employers {
		userID := uuid.NewString()
		profileID := uuid.NewString()
		if _, err := tx.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, role, is_email_verified, created_at, updated_at)
			VALUES ($1, $2, $3, 'EMPLOYER', true, $4, $4)`,
			userID, e.Email, passwordHash, now); err != nil {
			return nil, fmt.Errorf("employer user %s: %w", e.Email, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO employer_profiles (id, user_id, company_name, company_description, industry, website,
			                               contact_person, contact_email, contact_phone, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)`,
			profileID, userID, e.CompanyName, e.CompanyDescription, e.Industry, e.Website,
			e.ContactPerson, e.ContactEmail, e.ContactPhone, now); err != nil {
			return nil, fmt.Errorf("employer profile %s: %w", e.Email, err)
		}
		ids = append(ids, profileID)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

// seedVacancies — prisma.vacancy.create на каждую вакансию.
func seedVacancies(ctx context.Context, db *store.DB, now time.Time, vacancies []Vacancy, employerIDs []string) ([]string, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	ids := make([]string, 0, len(vacancies))
	for _, v := range vacancies {
		if v.EmployerIndex < 0 || v.EmployerIndex >= len(employerIDs) {
			return nil, fmt.Errorf("vacancy %q: employerIndex %d вне диапазона", v.Title, v.EmployerIndex)
		}
		id := uuid.NewString()
		if _, err := tx.Exec(ctx, `
			INSERT INTO vacancies (id, employer_id, title, description, specialization, grade, stack,
			                       salary_from, salary_to, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)`,
			id, employerIDs[v.EmployerIndex], v.Title, v.Description, v.Specialization, v.Grade,
			v.Stack, v.SalaryFrom, v.SalaryTo, v.IsActive, now); err != nil {
			return nil, fmt.Errorf("vacancy %q: %w", v.Title, err)
		}
		ids = append(ids, id)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

// seedInvitations — приглашения со случайными (недетерминированными) датами.
// BUG-REF: F3 — createdAt/statusUpdatedAt зависят от Math.random при каждом запуске.
func seedInvitations(ctx context.Context, db *store.DB, now time.Time, invitations []Invitation,
	employerIDs []string, vacancyIDs []string, profileIDByEmail map[string]string,
) (int, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	created := 0
	for _, inv := range invitations {
		profileID, ok := profileIDByEmail[inv.CandidateEmail]
		if !ok {
			continue // TS: findFirst(...) → if (!candidate) continue
		}
		var vacancyID *string
		if inv.VacancyIndex != nil {
			if *inv.VacancyIndex < 0 || *inv.VacancyIndex >= len(vacancyIDs) {
				return 0, fmt.Errorf("invitation vacancyIndex %d вне диапазона", *inv.VacancyIndex)
			}
			v := vacancyIDs[*inv.VacancyIndex]
			vacancyID = &v
		}
		if inv.EmployerIndex < 0 || inv.EmployerIndex >= len(employerIDs) {
			return 0, fmt.Errorf("invitation employerIndex %d вне диапазона", inv.EmployerIndex)
		}

		statusUpdated := now.Add(-time.Duration(rand.IntN(7)) * day) // Math.floor(Math.random() * 7)
		createdAt := now.Add(-time.Duration(rand.IntN(20)) * day)    // Math.floor(Math.random() * 20)

		if _, err := tx.Exec(ctx, `
			INSERT INTO invitations (id, employer_id, candidate_id, vacancy_id, message,
			                         salary_from, salary_to, status, status_updated_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			uuid.NewString(), employerIDs[inv.EmployerIndex], profileID, vacancyID, inv.Message,
			inv.SalaryFrom, inv.SalaryTo, inv.Status, statusUpdated, createdAt, now); err != nil {
			return 0, fmt.Errorf("invitation %s: %w", inv.CandidateEmail, err)
		}
		created++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return created, nil
}

// seedNeeds — потребности работодателей со случайной createdAt (баг F3).
func seedNeeds(ctx context.Context, db *store.DB, now time.Time, needs []Need, employerIDs []string) (int, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	created := 0
	for _, n := range needs {
		if n.EmployerIndex < 0 || n.EmployerIndex >= len(employerIDs) {
			return 0, fmt.Errorf("need employerIndex %d вне диапазона", n.EmployerIndex)
		}
		createdAt := now.Add(-time.Duration(rand.IntN(10)) * day) // Math.floor(Math.random() * 10)
		if _, err := tx.Exec(ctx, `
			INSERT INTO employer_needs (id, employer_id, description, specialization, grade, stack, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			uuid.NewString(), employerIDs[n.EmployerIndex], n.Description, n.Specialization, n.Grade,
			n.Stack, createdAt); err != nil {
			return 0, fmt.Errorf("need: %w", err)
		}
		created++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return created, nil
}
