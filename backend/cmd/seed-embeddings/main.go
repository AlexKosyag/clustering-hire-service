// cmd/seed-embeddings — предзаполнение TF-IDF-эмбеддингов (порт prisma/embeddings.ts).
// Запуск: go run ./cmd/seed-embeddings (нужен DATABASE_URL).
//
// Нужен, чтобы semantic-поиск работал сразу после сида, не дожидаясь фоновой
// переиндексации при старте приложения.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"fsp-backend/internal/config"
	"fsp-backend/internal/modules/matching"
	"fsp-backend/internal/store"
)

// textRow — профиль/потребность с уже собранным текстом.
type textRow struct {
	id   string
	text string
}

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fail(err)
	}
	defer db.Close()

	candidates, err := loadCandidates(ctx, db)
	if err != nil {
		fail(fmt.Errorf("кандидаты: %w", err))
	}
	needs, err := loadNeeds(ctx, db)
	if err != nil {
		fail(fmt.Errorf("потребности: %w", err))
	}

	// общий корпус из профилей и потребностей, один fit
	corpus := make([]string, 0, len(candidates)+len(needs))
	for _, r := range candidates {
		corpus = append(corpus, r.text)
	}
	for _, r := range needs {
		corpus = append(corpus, r.text)
	}
	vectorizer := matching.NewSeedVectorizer()
	vectorizer.Fit(corpus)

	started := time.Now()
	for _, row := range candidates {
		vector := vectorizer.Embed(row.text)
		if _, err := db.Pool.Exec(ctx,
			`UPDATE candidate_profiles SET profile_embedding = $1::vector WHERE id = $2`,
			formatVector(vector), row.id); err != nil {
			fail(fmt.Errorf("запись вектора профиля %s: %w", row.id, err))
		}
	}
	for _, row := range needs {
		vector := vectorizer.Embed(row.text)
		if _, err := db.Pool.Exec(ctx,
			`UPDATE employer_needs SET need_embedding = $1::vector WHERE id = $2`,
			formatVector(vector), row.id); err != nil {
			fail(fmt.Errorf("запись вектора потребности %s: %w", row.id, err))
		}
	}

	elapsed := time.Since(started).Milliseconds()
	fmt.Printf("✓ Эмбеддинги записаны: %d кандидатов, %d потребностей за %dms (TF-IDF, 384-dim)\n",
		len(candidates), len(needs), elapsed)
}

// loadCandidates — профили с достижениями (candidateText из embeddings.ts).
func loadCandidates(ctx context.Context, db *store.DB) ([]textRow, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, first_name, last_name, specialization, declared_grade, verified_grade,
		       stack, roles, soft_skills, city, bio
		FROM candidate_profiles
		ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type profile struct {
		id                                 string
		firstName, lastName                *string
		specialization, declared, verified *string
		stack, roles, softSkills           []string
		city, bio                          *string
	}
	profiles := map[string]*profile{}
	order := make([]string, 0)
	for rows.Next() {
		var p profile
		var stack, roles, softSkills []string
		if err := rows.Scan(&p.id, &p.firstName, &p.lastName, &p.specialization,
			&p.declared, &p.verified, &stack, &roles, &softSkills, &p.city, &p.bio); err != nil {
			return nil, err
		}
		p.stack, p.roles, p.softSkills = stack, roles, softSkills
		profiles[p.id] = &p
		order = append(order, p.id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// достижения: candidate_achievements → fsp_achievements.name
	achRows, err := db.Pool.Query(ctx, `
		SELECT ca.candidate_id, f.name
		FROM candidate_achievements ca
		JOIN fsp_achievements f ON f.id = ca.achievement_id
		ORDER BY ca.candidate_id`)
	if err != nil {
		return nil, err
	}
	defer achRows.Close()
	achievements := map[string][]string{}
	for achRows.Next() {
		var candidateID, name string
		if err := achRows.Scan(&candidateID, &name); err != nil {
			return nil, err
		}
		achievements[candidateID] = append(achievements[candidateID], name)
	}
	if err := achRows.Err(); err != nil {
		return nil, err
	}

	out := make([]textRow, 0, len(order))
	for _, id := range order {
		p := profiles[id]
		text := joinTruthy(
			strDeref(p.firstName), strDeref(p.lastName), strDeref(p.specialization),
			strDeref(p.declared), strDeref(p.verified),
			strings.Join(p.stack, " "), strings.Join(p.roles, " "), strings.Join(p.softSkills, " "),
			strDeref(p.city), strDeref(p.bio), strings.Join(achievements[id], " "),
		)
		out = append(out, textRow{id: id, text: text})
	}
	return out, nil
}

// loadNeeds — потребности работодателей (needText из embeddings.ts).
func loadNeeds(ctx context.Context, db *store.DB) ([]textRow, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, description, specialization, grade, stack
		FROM employer_needs
		ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []textRow{}
	for rows.Next() {
		var id, description string
		var specialization, grade *string
		var stack []string
		if err := rows.Scan(&id, &description, &specialization, &grade, &stack); err != nil {
			return nil, err
		}
		text := joinTruthy(description, strDeref(specialization), strDeref(grade), strings.Join(stack, " "))
		out = append(out, textRow{id: id, text: text})
	}
	return out, rows.Err()
}

// joinTruthy — .filter(Boolean).join(' ') из embeddings.ts.
func joinTruthy(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " ")
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// formatVector — вектор как "[0.123456,...]" (JS toFixed(6)).
func formatVector(v []float64) string {
	parts := make([]string, 0, len(v))
	for _, x := range v {
		parts = append(parts, fmt.Sprintf("%.6f", x))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "Seed embeddings failed:", err)
	os.Exit(1)
}
