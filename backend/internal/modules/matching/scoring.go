package matching

import (
	"math"
	"strings"

	"fsp-backend/internal/model"
)

// Порт backend/src/modules/matching/scoring.service.ts (V1, as-is).

// GRADE_ORDER — порядок грейдов значим (аналог GRADE_ORDER из scoring.service.ts).
var GRADE_ORDER = model.GradeOrder

// RELATED — близкие специализации — частичное совпадение категории.
var RELATED = map[string][]model.Specialization{
	"BACKEND":      {model.SpecFullstack, model.SpecDevops},
	"FRONTEND":     {model.SpecFullstack, model.SpecDesignUX},
	"FULLSTACK":    {model.SpecBackend, model.SpecFrontend},
	"MOBILE":       {model.SpecFullstack},
	"DEVOPS":       {model.SpecBackend, model.SpecSecurity},
	"DATA_SCIENCE": {model.SpecMLAI},
	"ML_AI":        {model.SpecDataScience},
	"QA":           {model.SpecBackend, model.SpecFrontend},
	"SECURITY":     {model.SpecDevops},
	"EMBEDDED":     {model.SpecBackend},
	"GAME_DEV":     {model.SpecBackend, model.SpecDesignUX},
	"DESIGN_UX":    {model.SpecFrontend},
	"MANAGEMENT":   {model.SpecFullstack},
}

// WEIGHTS — веса компонент V1 (сумма = 1.0).
var WEIGHTS = struct {
	Category    float64
	Stack       float64
	Semantic    float64
	Grade       float64
	Achievement float64
}{
	Category:    0.3,
	Stack:       0.25,
	Semantic:    0.2,
	Grade:       0.15,
	Achievement: 0.1,
}

// MatchComponents — компоненты счёта V1.
type MatchComponents struct {
	CategoryMatch      float64 `json:"categoryMatch"`
	StackOverlap       float64 `json:"stackOverlap"`
	SemanticSimilarity float64 `json:"semanticSimilarity"`
	GradeMatch         float64 `json:"gradeMatch"`
	AchievementBoost   float64 `json:"achievementBoost"`
}

// MatchScore — итоговый счёт V1.
type MatchScore struct {
	Total      float64         `json:"total"`
	Components MatchComponents `json:"components"`
}

// ScoreableCandidate — кандидат для скоринга V1 (null = "" — пустая строка).
type ScoreableCandidate struct {
	Specialization    model.Specialization
	VerifiedGrade     model.Grade
	DeclaredGrade     model.Grade
	Stack             []string
	AchievementPoints int
}

// Need — потребность для скоринга V1.
type Need struct {
	Specialization model.Specialization // "" = null
	Grade          model.Grade          // "" = null
	Stack          []string
}

// scoringService — MatchingScoringService (V1).
type scoringService struct{}

// calculateMatch — итоговый счёт V1: Σ component × weight, округление до 0.1.
func (s scoringService) calculateMatch(candidate ScoreableCandidate, need Need, semanticSimilarity float64) MatchScore {
	components := MatchComponents{
		CategoryMatch:      s.categoryScore(candidate, need),
		StackOverlap:       s.stackScore(candidate.Stack, need.Stack),
		SemanticSimilarity: s.clamp100(semanticSimilarity * 100),
		GradeMatch:         s.gradeScore(candidate, need),
		AchievementBoost:   s.achievementScore(candidate.AchievementPoints),
	}

	total := components.CategoryMatch*WEIGHTS.Category +
		components.StackOverlap*WEIGHTS.Stack +
		components.SemanticSimilarity*WEIGHTS.Semantic +
		components.GradeMatch*WEIGHTS.Grade +
		components.AchievementBoost*WEIGHTS.Achievement

	return MatchScore{Total: jsRound(total*10) / 10, Components: components}
}

// categoryScore — совпадение специализации: 100 / 60 (смежная) / 30 / 50 / 0.
func (s scoringService) categoryScore(candidate ScoreableCandidate, need Need) float64 {
	if need.Specialization == "" {
		return 50
	}
	if candidate.Specialization == "" {
		return 30
	}
	if candidate.Specialization == need.Specialization {
		return 100
	}
	if containsSpec(RELATED[string(need.Specialization)], candidate.Specialization) {
		return 60
	}
	return 0
}

// stackScore — доля пересечения стеков с нормализацией названий.
func (s scoringService) stackScore(candidateStack, needStack []string) float64 {
	if len(needStack) == 0 {
		return 50
	}
	if len(candidateStack) == 0 {
		return 0
	}
	need := make([]string, 0, len(needStack))
	for _, s := range needStack {
		need = append(need, normalizeTech(s))
	}
	have := map[string]bool{}
	for _, s := range candidateStack {
		have[normalizeTech(s)] = true
	}
	overlap := 0
	for _, s := range need {
		if have[s] {
			overlap++
		}
	}
	return s.clamp100(jsRound(float64(overlap) / float64(len(need)) * 100))
}

// gradeScore — ±1 грейд (100/80/60), дальше 40/20.
func (s scoringService) gradeScore(candidate ScoreableCandidate, need Need) float64 {
	if need.Grade == "" {
		return 50
	}
	candidateGrade := candidate.VerifiedGrade
	if candidateGrade == "" {
		candidateGrade = candidate.DeclaredGrade
	}
	if candidateGrade == "" {
		return 30
	}

	candidateIdx := gradeIndexOf(candidateGrade)
	needIdx := gradeIndexOf(need.Grade)

	switch {
	case candidateIdx == needIdx:
		return 100
	case candidateIdx == needIdx+1:
		return 80 // One above — good
	case candidateIdx == needIdx-1:
		return 60 // One below — acceptable
	case candidateIdx > needIdx+1:
		return 40 // Overqualified
	default:
		return 20 // Underqualified
	}
}

// achievementScore — очки достижений ×10, cap 100.
func (scoringService) achievementScore(totalPoints int) float64 {
	if totalPoints <= 0 {
		return 0
	}
	return math.Min(100, float64(totalPoints)*10)
}

func (scoringService) clamp100(value float64) float64 {
	if !isFinite(value) {
		return 0
	}
	return math.Max(0, math.Min(100, jsRound(value)))
}

// gradeIndexOf — GRADE_ORDER.indexOf(grade) (-1 если не найден, как в JS).
func gradeIndexOf(g model.Grade) int {
	for i, v := range GRADE_ORDER {
		if v == g {
			return i
		}
	}
	return -1
}

func containsSpec(list []model.Specialization, spec model.Specialization) bool {
	for _, s := range list {
		if s == spec {
			return true
		}
	}
	return false
}

/**
 * Нормализация названий технологий для сравнения стеков:
 * регистр не важен, «Node.js» = «Node JS» = «node-js» = «nodejs».
 * Знаки +/# сохраняются («C++» ≠ «C»).
 */
func normalizeTech(value string) string {
	return removeDotSpaceDash(strings.ToLower(strings.TrimSpace(value)))
}

// removeDotSpaceDash — replace(/[.\s-]+/g, ”).
func removeDotSpaceDash(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '.' || r == '-' || isSpaceRune(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isSpaceRune(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\f', '\v', 0x00A0, 0x1680,
		0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006,
		0x2007, 0x2008, 0x2009, 0x200A, 0x2028, 0x2029, 0x202F,
		0x205F, 0x3000, 0xFEFF:
		return true
	}
	return false
}

// jsRound — Math.round в стиле JS (floor(x+0.5), «вверх от .5»).
func jsRound(x float64) float64 {
	if math.IsNaN(x) {
		return math.NaN()
	}
	return math.Floor(x + 0.5)
}

// jsRound1 — Math.round(x * 10) / 10 (округление до 0.1).
func jsRound1(x float64) float64 {
	return jsRound(x*10) / 10
}
