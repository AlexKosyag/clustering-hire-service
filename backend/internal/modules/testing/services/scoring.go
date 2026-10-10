package services

import (
	"math"

	"fsp-backend/internal/model"
)

// AnswerStat — данные ответа для скоринга (score может быть частичным).
type AnswerStat struct {
	IsCorrect  bool
	Difficulty float64
	Score      *float64
}

// DifficultyBucket — счётчики корзины сложности.
type DifficultyBucket struct {
	Correct int `json:"correct"`
	Total   int `json:"total"`
}

// DifficultyDetails — распределение ответов по сложности.
type DifficultyDetails struct {
	Easy   DifficultyBucket `json:"easy"`
	Medium DifficultyBucket `json:"medium"`
	Hard   DifficultyBucket `json:"hard"`
}

// GradeThreshold — порог theta для грейда.
type GradeThreshold struct {
	Grade model.Grade
	Min   float64
}

// GradeThresholds — пороговые значения theta для каждого грейда (docs 06 §4.2).
var GradeThresholds = []GradeThreshold{
	{Grade: model.GradeJunior, Min: math.Inf(-1)},
	{Grade: model.GradeMiddle, Min: -1.0},
	{Grade: model.GradeMiddlePlus, Min: 0.0},
	{Grade: model.GradeSenior, Min: 0.5},
	{Grade: model.GradeLead, Min: 1.0},
	{Grade: model.GradePrincipal, Min: 1.5},
}

// specOffset — небольшая коррекция сложности по специализации (в доках указана
// как normalizeTheta). Таблица существует, но пустая — normalizeTheta ничего не делает.
var specOffset = map[model.Specialization]float64{}

// Recommendation — итог теста.
type Recommendation string

const (
	RecPass    Recommendation = "PASS"
	RecFail    Recommendation = "FAIL"
	RecUpgrade Recommendation = "UPGRADE"
)

// Scoring — баллы, грейд из θ, рекомендация.
type Scoring struct{}

// NewScoring — конструктор (в TS — ScoringService без зависимостей).
func NewScoring() *Scoring { return &Scoring{} }

// CalculateGrade — грейд из theta (верхний достигнутый порог).
func (s *Scoring) CalculateGrade(theta float64, specialization *model.Specialization) model.Grade {
	value := theta
	if !isFinite(value) {
		value = 0
	}
	normalized := value
	if specialization != nil {
		normalized += specOffset[*specialization]
	}

	result := model.GradeJunior
	for _, threshold := range GradeThresholds {
		if normalized >= threshold.Min {
			result = threshold.Grade
		}
	}
	return result
}

// GetRecommendation — PASS/FAIL/UPGRADE (docs 06 §4.3).
func (s *Scoring) GetRecommendation(declared *model.Grade, calculated model.Grade, score float64) Recommendation {
	if declared == nil {
		if score >= 50 {
			return RecPass
		}
		return RecFail
	}
	declaredIndex := s.GradeIndex(*declared)
	calculatedIndex := s.GradeIndex(calculated)

	if calculatedIndex >= declaredIndex+1 && score >= 85 {
		return RecUpgrade
	}
	if calculatedIndex >= declaredIndex {
		return RecPass
	}
	return RecFail
}

// ScoreFromAnswers — взвешенная оценка 0-100: сложные вопросы весят больше.
func (s *Scoring) ScoreFromAnswers(answers []AnswerStat) float64 {
	if len(answers) == 0 {
		return 0
	}
	earned := 0.0
	weight := 0.0
	for _, answer := range answers {
		w := 0.5 + answer.Difficulty
		weight += w
		value := 0.0
		if answer.Score != nil {
			value = *answer.Score
		} else if answer.IsCorrect {
			value = 1
		}
		earned += w * math.Max(0, math.Min(1, value))
	}
	if weight == 0 {
		return 0
	}
	return JSRound((earned/weight)*1000) / 10
}

// DetailsByDifficulty — распределение верных ответов по корзинам сложности.
func (s *Scoring) DetailsByDifficulty(answers []AnswerStat) DifficultyDetails {
	details := DifficultyDetails{
		Easy:   DifficultyBucket{},
		Medium: DifficultyBucket{},
		Hard:   DifficultyBucket{},
	}
	for _, answer := range answers {
		bucket := &details.Easy
		if answer.Difficulty < 0.4 {
			bucket = &details.Easy
		} else if answer.Difficulty <= 0.65 {
			bucket = &details.Medium
		} else {
			bucket = &details.Hard
		}
		bucket.Total += 1
		if answer.IsCorrect {
			bucket.Correct += 1
		}
	}
	return details
}

// GradeIndex — строгий порядок грейдов (0..5); -1 для неизвестного (как indexOf).
func (s *Scoring) GradeIndex(grade model.Grade) int {
	for i, v := range model.GradeOrder {
		if v == grade {
			return i
		}
	}
	return -1
}
