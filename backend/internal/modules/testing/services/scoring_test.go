package services

import (
	"math"
	"testing"

	"fsp-backend/internal/model"
)

// Порт backend/src/modules/testing/services/scoring.spec.ts (ScoringService).

func f(v float64) *float64 { return &v }

func TestScoringCalculateGrade(t *testing.T) {
	scoring := NewScoring()

	cases := []struct {
		theta float64
		want  model.Grade
	}{
		{-4, model.GradeJunior},
		{-1.5, model.GradeJunior},
		{-1, model.GradeMiddle},
		{-0.5, model.GradeMiddle},
		{0, model.GradeMiddlePlus},
		{0.49, model.GradeMiddlePlus},
		{0.5, model.GradeSenior},
		{0.99, model.GradeSenior},
		{1, model.GradeLead},
		{1.49, model.GradeLead},
		{1.5, model.GradePrincipal},
		{3, model.GradePrincipal},
	}
	for _, c := range cases {
		if got := scoring.CalculateGrade(c.theta, nil); got != c.want {
			t.Errorf("theta=%v → %s, ожидалось %s", c.theta, got, c.want)
		}
	}

	t.Run("нечисловое theta трактуется как 0", func(t *testing.T) {
		if got := scoring.CalculateGrade(math.NaN(), nil); got != model.GradeMiddlePlus {
			t.Errorf("NaN → %s, ожидалось MIDDLE_PLUS", got)
		}
	})

	t.Run("пороги соответствуют docs/modules/testing-engine.md", func(t *testing.T) {
		byGrade := map[model.Grade]float64{}
		for _, th := range GradeThresholds {
			byGrade[th.Grade] = th.Min
		}
		if byGrade[model.GradeMiddle] != -1.0 {
			t.Errorf("MIDDLE.min = %v, ожидалось -1.0", byGrade[model.GradeMiddle])
		}
		if byGrade[model.GradeMiddlePlus] != 0.0 {
			t.Errorf("MIDDLE_PLUS.min = %v, ожидалось 0.0", byGrade[model.GradeMiddlePlus])
		}
		if byGrade[model.GradeSenior] != 0.5 {
			t.Errorf("SENIOR.min = %v, ожидалось 0.5", byGrade[model.GradeSenior])
		}
		if byGrade[model.GradeLead] != 1.0 {
			t.Errorf("LEAD.min = %v, ожидалось 1.0", byGrade[model.GradeLead])
		}
		if byGrade[model.GradePrincipal] != 1.5 {
			t.Errorf("PRINCIPAL.min = %v, ожидалось 1.5", byGrade[model.GradePrincipal])
		}
		if !math.IsInf(byGrade[model.GradeJunior], -1) {
			t.Errorf("JUNIOR.min = %v, ожидалось -Infinity", byGrade[model.GradeJunior])
		}
	})
}

func TestScoringGetRecommendation(t *testing.T) {
	scoring := NewScoring()
	middle := model.GradeMiddle
	middlePlus := model.GradeMiddlePlus
	junior := model.GradeJunior
	senior := model.GradeSenior

	t.Run("рассчитанный грейд выше объявленного и score >= 85 → UPGRADE", func(t *testing.T) {
		if got := scoring.GetRecommendation(&middle, middlePlus, 85); got != RecUpgrade {
			t.Errorf("MIDDLE→MIDDLE_PLUS@85 = %s, ожидалось UPGRADE", got)
		}
		if got := scoring.GetRecommendation(&junior, senior, 95); got != RecUpgrade {
			t.Errorf("JUNIOR→SENIOR@95 = %s, ожидалось UPGRADE", got)
		}
	})

	t.Run("рассчитанный грейд выше объявленного, но score < 85 → PASS", func(t *testing.T) {
		if got := scoring.GetRecommendation(&middle, senior, 70); got != RecPass {
			t.Errorf("MIDDLE→SENIOR@70 = %s, ожидалось PASS", got)
		}
	})

	t.Run("рейтинг совпал или выше → PASS", func(t *testing.T) {
		if got := scoring.GetRecommendation(&middle, model.GradeMiddle, 60); got != RecPass {
			t.Errorf("MIDDLE→MIDDLE@60 = %s, ожидалось PASS", got)
		}
		if got := scoring.GetRecommendation(&middle, model.GradeLead, 50); got != RecPass {
			t.Errorf("MIDDLE→LEAD@50 = %s, ожидалось PASS", got)
		}
	})

	t.Run("рассчитанный грейд ниже объявленного → FAIL", func(t *testing.T) {
		if got := scoring.GetRecommendation(&senior, model.GradeMiddle, 90); got != RecFail {
			t.Errorf("SENIOR→MIDDLE@90 = %s, ожидалось FAIL", got)
		}
	})

	t.Run("без объявленного грейда решает score (порог 50)", func(t *testing.T) {
		if got := scoring.GetRecommendation(nil, model.GradeJunior, 50); got != RecPass {
			t.Errorf("nil→JUNIOR@50 = %s, ожидалось PASS", got)
		}
		if got := scoring.GetRecommendation(nil, model.GradeJunior, 49.9); got != RecFail {
			t.Errorf("nil→JUNIOR@49.9 = %s, ожидалось FAIL", got)
		}
	})
}

func TestScoringScoreFromAnswers(t *testing.T) {
	scoring := NewScoring()

	t.Run("пустая история → 0", func(t *testing.T) {
		if got := scoring.ScoreFromAnswers(nil); got != 0 {
			t.Errorf("ScoreFromAnswers(nil) = %v, ожидалось 0", got)
		}
	})

	t.Run("все верные → 100", func(t *testing.T) {
		answers := []AnswerStat{
			{IsCorrect: true, Difficulty: 0.2},
			{IsCorrect: true, Difficulty: 0.8},
		}
		if got := scoring.ScoreFromAnswers(answers); got != 100 {
			t.Errorf("ScoreFromAnswers = %v, ожидалось 100", got)
		}
	})

	t.Run("половина верных при равной сложности → около 50", func(t *testing.T) {
		answers := []AnswerStat{
			{IsCorrect: true, Difficulty: 0.5},
			{IsCorrect: false, Difficulty: 0.5},
		}
		if got := scoring.ScoreFromAnswers(answers); math.Abs(got-50) > 1e-5 {
			t.Errorf("ScoreFromAnswers = %v, ожидалось ≈50", got)
		}
	})

	t.Run("сложные вопросы весят больше простых", func(t *testing.T) {
		hardCorrect := []AnswerStat{
			{IsCorrect: true, Difficulty: 0.9},
			{IsCorrect: false, Difficulty: 0.1},
		}
		hardWrong := []AnswerStat{
			{IsCorrect: false, Difficulty: 0.9},
			{IsCorrect: true, Difficulty: 0.1},
		}
		if !(scoring.ScoreFromAnswers(hardCorrect) > scoring.ScoreFromAnswers(hardWrong)) {
			t.Errorf("hardCorrect=%v не больше hardWrong=%v",
				scoring.ScoreFromAnswers(hardCorrect), scoring.ScoreFromAnswers(hardWrong))
		}
	})

	t.Run("частичный балл текстового ответа используется вместо isCorrect", func(t *testing.T) {
		answers := []AnswerStat{{IsCorrect: false, Difficulty: 0.5, Score: f(0.6)}}
		if got := scoring.ScoreFromAnswers(answers); math.Abs(got-60) > 1e-5 {
			t.Errorf("ScoreFromAnswers = %v, ожидалось ≈60", got)
		}
	})

	t.Run("значение score ограничивается диапазоном [0, 1]", func(t *testing.T) {
		if got := scoring.ScoreFromAnswers([]AnswerStat{{IsCorrect: false, Difficulty: 0.5, Score: f(42)}}); got != 100 {
			t.Errorf("score=42 → %v, ожидалось 100", got)
		}
		if got := scoring.ScoreFromAnswers([]AnswerStat{{IsCorrect: true, Difficulty: 0.5, Score: f(-3)}}); got != 0 {
			t.Errorf("score=-3 → %v, ожидалось 0", got)
		}
	})
}

func TestScoringDetailsByDifficulty(t *testing.T) {
	scoring := NewScoring()

	t.Run("распределяет ответы по корзинам сложности", func(t *testing.T) {
		answers := []AnswerStat{
			{IsCorrect: true, Difficulty: 0.2},
			{IsCorrect: false, Difficulty: 0.35},
			{IsCorrect: true, Difficulty: 0.5},
			{IsCorrect: true, Difficulty: 0.65},
			{IsCorrect: false, Difficulty: 0.7},
			{IsCorrect: true, Difficulty: 0.95},
		}
		details := scoring.DetailsByDifficulty(answers)
		if details.Easy.Correct != 1 || details.Easy.Total != 2 {
			t.Errorf("easy = %+v, ожидалось {1 2}", details.Easy)
		}
		if details.Medium.Correct != 2 || details.Medium.Total != 2 {
			t.Errorf("medium = %+v, ожидалось {2 2}", details.Medium)
		}
		if details.Hard.Correct != 1 || details.Hard.Total != 2 {
			t.Errorf("hard = %+v, ожидалось {1 2}", details.Hard)
		}
	})
}

func TestScoringGradeIndex(t *testing.T) {
	scoring := NewScoring()

	t.Run("задаёт строгий порядок грейдов", func(t *testing.T) {
		if scoring.GradeIndex(model.GradeJunior) != 0 {
			t.Errorf("JUNIOR = %d, ожидалось 0", scoring.GradeIndex(model.GradeJunior))
		}
		if scoring.GradeIndex(model.GradeMiddle) != 1 {
			t.Errorf("MIDDLE = %d, ожидалось 1", scoring.GradeIndex(model.GradeMiddle))
		}
		if scoring.GradeIndex(model.GradePrincipal) != 5 {
			t.Errorf("PRINCIPAL = %d, ожидалось 5", scoring.GradeIndex(model.GradePrincipal))
		}
		if scoring.GradeIndex(model.GradeSenior) <= scoring.GradeIndex(model.GradeMiddle) {
			t.Error("gradeIndex(SENIOR) должен быть больше gradeIndex(MIDDLE)")
		}
	})
}
