package matching

import (
	"context"
	"math"
	"testing"

	"fsp-backend/internal/model"
)

// MatchingScoringServiceV2 — порт scoring-v2.service.spec.ts.

// stubML — заглушка mlAPI (в TS-спеке HttpService замокан через jest.mock).
type stubML struct {
	avail     bool
	reranked  []RerankResult
	rerankOK  bool
	rerankErr bool
}

func (s stubML) available() bool { return s.avail }

func (s stubML) ParseQuery(context.Context, string) QueryUnderstanding { return QueryUnderstanding{} }

func (s stubML) Rerank(_ context.Context, _ string, _ []string, _ *int) ([]RerankResult, bool) {
	if s.rerankErr {
		return nil, false
	}
	return s.reranked, s.rerankOK
}

func floatVal(v float64) *float64 { return &v }

func TestScoringV2CalculateMatchPerfect(t *testing.T) {
	service := newScoringV2Service(stubML{})

	perfectCandidate := ScoreableCandidateV2{
		Specialization:    model.SpecBackend,
		VerifiedGrade:     model.GradeMiddle,
		DeclaredGrade:     model.GradeMiddle,
		Stack:             []string{"Node.js", "PostgreSQL", "Redis", "Docker"},
		AchievementPoints: 15,
		TestScore:         floatVal(85),
		ExperienceYears:   floatVal(4),
	}
	perfectNeed := NeedV2{
		Specialization: model.SpecBackend,
		Grade:          model.GradeMiddle,
		Stack:          []string{"Node.js", "PostgreSQL"},
		RequiredSkills: []string{"Node.js", "PostgreSQL"},
	}

	result, err := service.calculateMatch(context.Background(), perfectCandidate, perfectNeed, 0.85, "", "")
	if err != nil {
		t.Fatalf("calculateMatch вернул ошибку: %v", err)
	}
	if result.Total < 70 {
		t.Errorf("total = %v, ожидался >= 70", result.Total)
	}
	if result.Components.SpecializationMatch != 100 {
		t.Errorf("specializationMatch = %v, ожидался 100", result.Components.SpecializationMatch)
	}
	if result.Components.GradeMatch != 100 {
		t.Errorf("gradeMatch = %v, ожидался 100", result.Components.GradeMatch)
	}
	if result.WeightsSource != "default" {
		t.Errorf("weightsSource = %q, ожидался 'default'", result.WeightsSource)
	}
}

func TestScoringV2CalculateMatchMismatched(t *testing.T) {
	service := newScoringV2Service(stubML{})

	mismatchedCandidate := ScoreableCandidateV2{
		Specialization:    model.SpecFrontend,
		VerifiedGrade:     model.GradeJunior,
		DeclaredGrade:     model.GradeJunior,
		Stack:             []string{"React", "CSS"},
		AchievementPoints: 0,
		TestScore:         floatVal(40),
		ExperienceYears:   floatVal(1),
	}
	perfectNeed := NeedV2{
		Specialization: model.SpecBackend,
		Grade:          model.GradeMiddle,
		Stack:          []string{"Node.js", "PostgreSQL"},
		RequiredSkills: []string{"Node.js", "PostgreSQL"},
	}

	result, err := service.calculateMatch(context.Background(), mismatchedCandidate, perfectNeed, 0.2, "", "")
	if err != nil {
		t.Fatalf("calculateMatch вернул ошибку: %v", err)
	}
	if result.Total > 50 {
		t.Errorf("total = %v, ожидался <= 50", result.Total)
	}
	if result.Components.SpecializationMatch != 0 {
		t.Errorf("specializationMatch = %v, ожидался 0", result.Components.SpecializationMatch)
	}
}

func TestScoringV2Contributions(t *testing.T) {
	service := newScoringV2Service(stubML{})

	perfectCandidate := ScoreableCandidateV2{
		Specialization:    model.SpecBackend,
		VerifiedGrade:     model.GradeMiddle,
		DeclaredGrade:     model.GradeMiddle,
		Stack:             []string{"Node.js", "PostgreSQL", "Redis", "Docker"},
		AchievementPoints: 15,
		TestScore:         floatVal(85),
		ExperienceYears:   floatVal(4),
	}
	perfectNeed := NeedV2{
		Specialization: model.SpecBackend,
		Grade:          model.GradeMiddle,
		Stack:          []string{"Node.js", "PostgreSQL"},
		RequiredSkills: []string{"Node.js", "PostgreSQL"},
	}

	result, err := service.calculateMatch(context.Background(), perfectCandidate, perfectNeed, 0.85, "", "")
	if err != nil {
		t.Fatalf("calculateMatch вернул ошибку: %v", err)
	}
	if result.Contributions == nil {
		t.Fatal("contributions = nil, ожидалась карта")
	}
	if result.Contributions["specialization"] <= 0 {
		t.Errorf("contributions[specialization] = %v, ожидалось > 0", result.Contributions["specialization"])
	}
	if result.Contributions["semantic"] <= 0 {
		t.Errorf("contributions[semantic] = %v, ожидалось > 0", result.Contributions["semantic"])
	}
}

func TestScoringV2StackScore(t *testing.T) {
	service := newScoringV2Service(stubML{})

	t.Run("should return 100 for exact match", func(t *testing.T) {
		if score := service.stackScore([]string{"Node.js", "PostgreSQL"}, []string{"Node.js", "PostgreSQL"}, nil); score != 100 {
			t.Errorf("stackScore = %v, ожидался 100", score)
		}
	})

	t.Run("should return 0 for no overlap", func(t *testing.T) {
		if score := service.stackScore([]string{"React", "CSS"}, []string{"Node.js", "PostgreSQL"}, nil); score != 0 {
			t.Errorf("stackScore = %v, ожидался 0", score)
		}
	})

	t.Run("should handle aliases correctly", func(t *testing.T) {
		if score := service.stackScore([]string{"nodejs", "postgres"}, []string{"Node.js", "PostgreSQL"}, nil); score != 100 {
			t.Errorf("stackScore = %v, ожидался 100", score)
		}
	})
}

func TestScoringV2ExperienceScoreErrorPath(t *testing.T) {
	// BUG: неизвестный needGrade (не из enum) → в TS TypeError → 500.
	service := newScoringV2Service(stubML{})
	if _, err := service.experienceScore(floatVal(3), "SOME_UNKNOWN_GRADE"); err == nil {
		t.Error("ожидалась ошибка итерации по неизвестному грейду")
	}
	if _, err := service.experienceScore(floatVal(3), model.GradeLead); err != nil {
		t.Errorf("известный грейд не должен давать ошибку: %v", err)
	}
	if v, err := service.experienceScore(nil, model.GradeLead); err != nil || v != 40 {
		t.Errorf("experienceScore(nil) = %v/%v, ожидалась 40/nil", v, err)
	}
}

func TestScoringV2RerankBoost(t *testing.T) {
	ml := stubML{avail: true, rerankOK: true, reranked: []RerankResult{{Score: 10}}}
	service := newScoringV2Service(ml)

	perfectCandidate := ScoreableCandidateV2{
		Specialization:    model.SpecBackend,
		VerifiedGrade:     model.GradeMiddle,
		DeclaredGrade:     model.GradeMiddle,
		Stack:             []string{"Node.js"},
		AchievementPoints: 0,
		TestScore:         floatVal(85),
		ExperienceYears:   floatVal(4),
	}
	need := NeedV2{Specialization: model.SpecBackend, Grade: model.GradeMiddle, Stack: []string{"Node.js"}}

	result, err := service.calculateMatch(context.Background(), perfectCandidate, need, 0.5, "кандидат", "запрос")
	if err != nil {
		t.Fatalf("calculateMatch вернул ошибку: %v", err)
	}
	// boost = (10+10)/20 = 1 → вклад rerank = 10
	if rerank, ok := result.Contributions["rerank"]; !ok || math.Abs(rerank-10) > 1e-9 {
		t.Errorf("contributions[rerank] = %v (есть=%v), ожидался 10", rerank, ok)
	}

	// Без текстов (candidateText/queryText) boost не запрашивается — M3 не срабатывает.
	plain, err := service.calculateMatch(context.Background(), perfectCandidate, need, 0.5, "", "")
	if err != nil {
		t.Fatalf("calculateMatch вернул ошибку: %v", err)
	}
	if _, ok := plain.Contributions["rerank"]; ok {
		t.Error("без текстов rerank-вклад не должен добавляться")
	}
}

func TestNormalizeTechV2(t *testing.T) {
	t.Run("should normalize common aliases", func(t *testing.T) {
		cases := map[string]string{
			"Node.js":    "nodejs",
			"node js":    "nodejs",
			"PostgreSQL": "postgresql",
			"postgres":   "postgresql",
			"React.js":   "react",
			"K8s":        "kubernetes",
		}
		for in, want := range cases {
			if got := normalizeTechV2(in); got != want {
				t.Errorf("normalizeTechV2(%q) = %q, ожидался %q", in, got, want)
			}
		}
	})

	t.Run("should handle case and whitespace", func(t *testing.T) {
		if got := normalizeTechV2("  NODE.JS  "); got != "nodejs" {
			t.Errorf("normalizeTechV2(\"  NODE.JS  \") = %q, ожидался 'nodejs'", got)
		}
		if got := normalizeTechV2("Type Script"); got != "typescript" {
			t.Errorf("normalizeTechV2(\"Type Script\") = %q, ожидался 'typescript'", got)
		}
	})

	t.Run("should preserve special characters like C++", func(t *testing.T) {
		if got := normalizeTechV2("C++"); got != "c++" {
			t.Errorf("normalizeTechV2(\"C++\") = %q, ожидался 'c++'", got)
		}
	})
}
