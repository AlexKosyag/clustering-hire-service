package matching

import (
	"context"
	"errors"
	"math"
	"strings"

	"fsp-backend/internal/model"
)

// Порт backend/src/modules/matching/scoring-v2.service.ts (as-is, включая баги).

// MatchComponentsV2 — 7 компонент V2.
type MatchComponentsV2 struct {
	SpecializationMatch float64 `json:"specializationMatch"`
	StackOverlap        float64 `json:"stackOverlap"`
	SemanticSimilarity  float64 `json:"semanticSimilarity"`
	GradeMatch          float64 `json:"gradeMatch"`
	AchievementBoost    float64 `json:"achievementBoost"`
	ExperienceRelevance float64 `json:"experienceRelevance"`
	TestScore           float64 `json:"testScore"`
}

// MatchScoreV2 — итоговый счёт V2 с вкладами компонент.
type MatchScoreV2 struct {
	Total         float64            `json:"total"`
	Components    MatchComponentsV2  `json:"components"`
	Contributions map[string]float64 `json:"contributions"`
	WeightsSource string             `json:"weightsSource"`
}

// ScoreableCandidateV2 — кандидат для скоринга V2.
type ScoreableCandidateV2 struct {
	Specialization    model.Specialization
	VerifiedGrade     model.Grade
	DeclaredGrade     model.Grade
	Stack             []string
	AchievementPoints int
	TestScore         *float64
	ExperienceYears   *float64
}

// NeedV2 — потребность для скоринга V2.
type NeedV2 struct {
	Specialization  model.Specialization // "" = null
	Grade           model.Grade          // "" = null
	Stack           []string
	RequiredSkills  []string
	PreferredSkills []string
}

// weightEntry — элемент DEFAULT_WEIGHTS (объявленный порядок ключей важен:
// Object.entries идёт в порядке объявления свойств).
type weightEntry struct {
	key    string
	weight float64
}

// DEFAULT_WEIGHTS — веса компонент V2 (порядок объявления как в TS).
var DEFAULT_WEIGHTS = []weightEntry{
	{"specialization", 0.18},
	{"stack", 0.15},
	{"semantic", 0.20},
	{"grade", 0.12},
	{"achievement", 0.08},
	{"experience", 0.12},
	{"testScore", 0.15},
}

// SKILL_IMPORTANCE — «must-have» vs «nice-to-have» (порядок неважен — только поиск).
var SKILL_IMPORTANCE = map[string]float64{
	// Languages — high importance
	"node.js": 1.0, "nodejs": 1.0, "javascript": 1.0, "typescript": 1.0,
	"python": 1.0, "java": 1.0, "go": 1.0, "golang": 1.0,
	"c++": 0.9, "c#": 0.9, "rust": 0.9, "ruby": 0.8, "php": 0.8,
	"swift": 0.9, "kotlin": 0.9,
	// Frameworks — medium-high
	"react": 0.9, "vue": 0.85, "angular": 0.85, "django": 0.9, "flask": 0.8,
	"fastapi": 0.85, "spring": 0.9, "nextjs": 0.85,
	// Databases — high
	"postgresql": 1.0, "postgres": 1.0, "mysql": 0.9, "mongodb": 0.85, "redis": 0.9,
	"elasticsearch": 0.8,
	// Infrastructure — medium
	"docker": 0.85, "kubernetes": 0.9, "k8s": 0.9, "aws": 0.85, "gcp": 0.8, "azure": 0.8,
	// Messaging — medium
	"kafka": 0.85, "rabbitmq": 0.8, "grpc": 0.8, "graphql": 0.8, "rest": 0.75,
	// Concepts — medium
	"microservices": 0.85, "cicd": 0.75, "ci/cd": 0.75,
}

// scoringV2Service — MatchingScoringServiceV2 (ml-клиент инжектируется как в TS).
type scoringV2Service struct {
	ml mlAPI
}

func newScoringV2Service(ml mlAPI) scoringV2Service {
	return scoringV2Service{ml: ml}
}

// errExperienceNotIterable — TS: `const [min, max] = expected[needGrade]` при
// неизвестном needGrade бросает TypeError (см. BUG-REF в отчёте: достижимо только
// через ML-разобранный не-enum seniority).
var errExperienceNotIterable = errors.New("undefined is not iterable (cannot read property Symbol(Symbol.iterator))")

// calculateMatch — V2: расширенный скоринг + опциональный re-ranking boost.
func (s scoringV2Service) calculateMatch(
	ctx context.Context,
	candidate ScoreableCandidateV2,
	need NeedV2,
	semanticSimilarity float64,
	candidateText string,
	queryText string,
) (MatchScoreV2, error) {
	// Calculate all components (порядок объявления объекта как в TS)
	var components MatchComponentsV2
	components.SpecializationMatch = s.specializationScore(candidate.Specialization, need.Specialization)
	components.StackOverlap = s.stackScore(candidate.Stack, need.Stack, need.RequiredSkills)
	components.SemanticSimilarity = s.clamp100(semanticSimilarity * 100)
	components.GradeMatch = s.gradeScore(candidate.VerifiedGrade, candidate.DeclaredGrade, need.Grade)
	components.AchievementBoost = s.achievementScore(candidate.AchievementPoints)

	exp, err := s.experienceScore(candidate.ExperienceYears, need.Grade)
	if err != nil {
		return MatchScoreV2{}, err
	}
	components.ExperienceRelevance = exp
	components.TestScore = s.testScoreComponent(candidate.TestScore)

	// V2: Optional re-ranking boost
	// BUG-REF: M3 — HTTP-вызов rerank на КАЖДОГО кандидата (fan-out), при доступном ML.
	rerankBoost := 0.0
	if candidateText != "" && queryText != "" && s.ml != nil && s.ml.available() {
		rerankBoost = s.getRerankBoost(ctx, queryText, candidateText)
	}

	// Use default weights (V2: could be learned per-employer)
	weightsSource := "default"

	// Calculate weighted score
	contributions := map[string]float64{}
	total := 0.0
	componentsByKey := map[string]float64{
		"specializationMatch": components.SpecializationMatch,
		"stackOverlap":        components.StackOverlap,
		"semanticSimilarity":  components.SemanticSimilarity,
		"gradeMatch":          components.GradeMatch,
		"achievementBoost":    components.AchievementBoost,
		"experienceRelevance": components.ExperienceRelevance,
		"testScore":           components.TestScore,
	}
	for _, entry := range DEFAULT_WEIGHTS {
		componentKey := v2ComponentKey(entry.key)
		componentValue := 0.0
		// Вклад читается по ключу компоненты (тернарная цепочка TS); неизвестный ключ → 0.
		if v, ok := componentsByKey[componentKey]; ok {
			componentValue = v
		}
		contribution := componentValue * entry.weight
		contributions[entry.key] = jsRound1(contribution)
		total += contribution
	}
	// Add re-ranking boost (up to +10 points)
	if rerankBoost > 0 {
		boostContribution := rerankBoost * 10
		contributions["rerank"] = jsRound1(boostContribution)
		total += boostContribution
	}

	return MatchScoreV2{
		Total:         jsRound1(total),
		Components:    components,
		Contributions: contributions,
		WeightsSource: weightsSource,
	}, nil
}

// v2ComponentKey — маппинг ключа веса на ключ компоненты (тернарная цепочка из TS).
func v2ComponentKey(key string) string {
	switch key {
	case "specialization":
		return "specializationMatch"
	case "stack":
		return "stackOverlap"
	case "semantic":
		return "semanticSimilarity"
	case "grade":
		return "gradeMatch"
	case "achievement":
		return "achievementBoost"
	case "experience":
		return "experienceRelevance"
	case "testScore":
		return "testScore"
	default:
		return key
	}
}

// getRerankBoost — 0-1 по оценке cross-encoder (одиночный rerank-запрос).
func (s scoringV2Service) getRerankBoost(ctx context.Context, query, candidateText string) float64 {
	results, available := s.ml.Rerank(ctx, query, []string{candidateText}, intPtr(1))
	if available && len(results) > 0 {
		// Normalize cross-encoder score (typically -10 to 10) to 0-1
		rawScore := results[0].Score
		return math.Max(0, math.Min(1, (rawScore+10)/20))
	}
	return 0
}

// specializationScore — совпадение специализации (собственный RELATED из V2).
func (s scoringV2Service) specializationScore(candidateSpec, needSpec model.Specialization) float64 {
	if needSpec == "" {
		return 50 // Neutral if not specified
	}
	if candidateSpec == "" {
		return 30
	}
	if candidateSpec == needSpec {
		return 100
	}
	if related, ok := relatedV2()[string(needSpec)]; ok && containsSpec(related, candidateSpec) {
		return 60
	}
	return 0
}

// relatedV2 — RELATED, объявленный внутри метода в TS (идентичен V1).
func relatedV2() map[string][]model.Specialization {
	return map[string][]model.Specialization{
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
}

// stackScore — V2: взвешенное пересечение (requiredSkills ×1.5).
func (s scoringV2Service) stackScore(candidateStack, needStack, requiredSkills []string) float64 {
	if len(needStack) == 0 && len(requiredSkills) == 0 {
		return 50
	}
	if len(candidateStack) == 0 {
		return 0
	}

	candidateNormalized := map[string]bool{}
	for _, t := range candidateStack {
		candidateNormalized[normalizeTechV2(t)] = true
	}

	// Calculate weighted overlap
	totalWeight := 0.0
	matchedWeight := 0.0

	// Required skills (higher weight)
	for _, skill := range requiredSkills {
		importance := 0.7
		if v, ok := SKILL_IMPORTANCE[strings.ToLower(skill)]; ok {
			importance = v
		}
		totalWeight += importance * 1.5 // Required skills weighted 1.5x

		if candidateNormalized[normalizeTechV2(skill)] {
			matchedWeight += importance * 1.5
		}
	}

	// Need stack (base weight)
	for _, skill := range needStack {
		importance := 0.7
		if v, ok := SKILL_IMPORTANCE[strings.ToLower(skill)]; ok {
			importance = v
		}
		totalWeight += importance

		if candidateNormalized[normalizeTechV2(skill)] {
			matchedWeight += importance
		}
	}

	if totalWeight == 0 {
		return 50
	}
	return s.clamp100(jsRound(matchedWeight / totalWeight * 100))
}

// gradeScore — V2: ±1 грейд с обработкой суб-грейдов (логика идентична V1).
func (s scoringV2Service) gradeScore(verifiedGrade, declaredGrade, needGrade model.Grade) float64 {
	if needGrade == "" {
		return 50
	}
	candidateGrade := verifiedGrade
	if candidateGrade == "" {
		candidateGrade = declaredGrade
	}
	if candidateGrade == "" {
		return 30
	}

	candidateIdx := gradeIndexOf(candidateGrade)
	needIdx := gradeIndexOf(needGrade)

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

// achievementScore — V2: логарифмическая шкала (убывающая отдача).
func (s scoringV2Service) achievementScore(totalPoints int) float64 {
	if totalPoints <= 0 {
		return 0
	}
	return math.Min(100, jsRound(math.Log2(1+float64(totalPoints))*15))
}

// experienceScore — V2: релевантность опыта грейду.
// BUG: неизвестный needGrade (не из enum) → в TS TypeError (итерирование undefined).
func (s scoringV2Service) experienceScore(experienceYears *float64, needGrade model.Grade) (float64, error) {
	if experienceYears == nil {
		return 40, nil // Neutral
	}
	if needGrade == "" {
		return 50, nil
	}

	// Expected experience by grade
	expected := map[model.Grade][2]float64{
		model.GradeJunior:     {0, 2},
		model.GradeMiddle:     {2, 5},
		model.GradeMiddlePlus: {3, 7},
		model.GradeSenior:     {5, 10},
		model.GradeLead:       {8, 15},
		model.GradePrincipal:  {10, 20},
	}
	bounds, ok := expected[needGrade]
	if !ok {
		// const [min, max] = undefined → TypeError в TS → 500.
		return 0, errExperienceNotIterable
	}
	min, max := bounds[0], bounds[1]
	y := *experienceYears

	if y >= min && y <= max {
		return 100, nil
	}
	if y < min {
		// Below minimum — proportional
		if y >= min*0.5 {
			return 60, nil
		}
		return 20, nil
	}
	// Above maximum — still good, slight penalty for overqualification
	if y <= max*1.5 {
		return 80, nil
	}
	return 50, nil
}

// testScoreComponent — V2: балл теста (нейтрально 40, если не тестировался).
func (s scoringV2Service) testScoreComponent(testScore *float64) float64 {
	if testScore == nil {
		return 40 // Neutral — not tested
	}
	return s.clamp100(jsRound(*testScore))
}

func (s scoringV2Service) clamp100(value float64) float64 {
	if !isFinite(value) {
		return 0
	}
	return math.Max(0, math.Min(100, jsRound(value)))
}

/**
 * V2: Normalize tech names for comparison (алиасы + удаление [. \s -]).
 */
var techAliases = map[string]string{
	"node.js":    "nodejs",
	"node js":    "nodejs",
	"node":       "nodejs",
	"postgres":   "postgresql",
	"react.js":   "react",
	"react js":   "react",
	"vue.js":     "vue",
	"vue js":     "vue",
	"angular.js": "angular",
	"k8s":        "kubernetes",
	"js":         "javascript",
	"ts":         "typescript",
	"py":         "python",
	"golang":     "go",
	"c sharp":    "csharp",
	"c#":         "csharp",
	".net":       "dotnet",
	"dot net":    "dotnet",
}

func normalizeTechV2(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))

	// Handle common aliases
	if v, ok := techAliases[normalized]; ok {
		return v
	}

	// Remove dots, spaces, dashes for comparison
	return removeDotSpaceDash(normalized)
}

func intPtr(v int) *int { return &v }
