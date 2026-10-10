package matching

import (
	"math"
	"strings"
	"testing"
	"time"

	"fsp-backend/internal/model"
)

// ExplanationService + MatchingScoringService (V1) — порт explanation.service.spec.ts.

func testCandidate(overrides func(*ScoreableCandidate)) ScoreableCandidate {
	c := ScoreableCandidate{
		Specialization:    model.SpecBackend,
		VerifiedGrade:     model.GradeMiddle,
		DeclaredGrade:     model.GradeMiddle,
		Stack:             []string{"Node.js", "PostgreSQL", "Redis", "Docker"},
		AchievementPoints: 0,
	}
	if overrides != nil {
		overrides(&c)
	}
	return c
}

func testNeed(overrides func(*Need)) Need {
	n := Need{
		Specialization: model.SpecBackend,
		Grade:          model.GradeMiddle,
		Stack:          []string{"Node.js", "PostgreSQL"},
	}
	if overrides != nil {
		overrides(&n)
	}
	return n
}

func testScore(total float64, overrides func(*MatchComponents)) MatchScore {
	components := MatchComponents{
		CategoryMatch:      100,
		StackOverlap:       100,
		SemanticSimilarity: 80,
		GradeMatch:         100,
		AchievementBoost:   0,
	}
	if overrides != nil {
		overrides(&components)
	}
	return MatchScore{Total: total, Components: components}
}

func approxClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.05 { // jest toBeCloseTo(x, 1)
		t.Errorf("%s = %v, ожидалось ~%v", name, got, want)
	}
}

func TestScoringV1WeightsSum(t *testing.T) {
	sum := WEIGHTS.Category + WEIGHTS.Stack + WEIGHTS.Semantic + WEIGHTS.Grade + WEIGHTS.Achievement
	approxClose(t, "сумма весов", sum, 1)
}

func TestScoringV1PerfectMatch(t *testing.T) {
	scoring := scoringService{}
	result := scoring.calculateMatch(
		testCandidate(func(c *ScoreableCandidate) { c.AchievementPoints = 10 }),
		testNeed(nil),
		1,
	)
	approxClose(t, "total", result.Total, 100)
	if result.Components.CategoryMatch != 100 {
		t.Errorf("categoryMatch = %v, ожидался 100", result.Components.CategoryMatch)
	}
	if result.Components.StackOverlap != 100 {
		t.Errorf("stackOverlap = %v, ожидался 100", result.Components.StackOverlap)
	}
	if result.Components.AchievementBoost != 100 {
		t.Errorf("achievementBoost = %v, ожидался 100", result.Components.AchievementBoost)
	}
}

func TestScoringV1NoAchievements(t *testing.T) {
	scoring := scoringService{}
	result := scoring.calculateMatch(testCandidate(nil), testNeed(nil), 1)
	approxClose(t, "total", result.Total, 90)
	if result.Components.AchievementBoost != 0 {
		t.Errorf("achievementBoost = %v, ожидался 0", result.Components.AchievementBoost)
	}
}

func TestScoringV1CategoryScore(t *testing.T) {
	scoring := scoringService{}

	if got := scoring.categoryScore(
		testCandidate(func(c *ScoreableCandidate) { c.Specialization = model.SpecBackend }),
		testNeed(func(n *Need) { n.Specialization = model.SpecBackend })); got != 100 {
		t.Errorf("categoryScore(BACKEND/BACKEND) = %v, ожидался 100", got)
	}
	if got := scoring.categoryScore(
		testCandidate(func(c *ScoreableCandidate) { c.Specialization = model.SpecDevops }),
		testNeed(func(n *Need) { n.Specialization = model.SpecBackend })); got != 60 {
		t.Errorf("categoryScore(DEVOPS/BACKEND) = %v, ожидался 60", got)
	}
	if got := scoring.categoryScore(
		testCandidate(func(c *ScoreableCandidate) { c.Specialization = model.SpecGameDev }),
		testNeed(func(n *Need) { n.Specialization = model.SpecBackend })); got != 0 {
		t.Errorf("categoryScore(GAME_DEV/BACKEND) = %v, ожидался 0", got)
	}
	if got := scoring.categoryScore(testCandidate(nil), Need{Stack: []string{}}); got != 50 {
		t.Errorf("categoryScore(c, need без специализации) = %v, ожидался 50", got)
	}
	if got := scoring.categoryScore(
		testCandidate(func(c *ScoreableCandidate) { c.Specialization = "" }),
		testNeed(nil)); got != 30 {
		t.Errorf("categoryScore(без специализации) = %v, ожидался 30", got)
	}
}

func TestScoringV1StackScore(t *testing.T) {
	scoring := scoringService{}

	if got := scoring.stackScore([]string{"node js", "POSTGRESQL"}, []string{"Node.js", "PostgreSQL"}); got != 100 {
		t.Errorf("stackScore(полное совпадение) = %v, ожидался 100", got)
	}
	if got := scoring.stackScore([]string{"Node.js"}, []string{"Node.js", "PostgreSQL", "Redis"}); got != 33 {
		t.Errorf("stackScore(1/3) = %v, ожидался 33", got)
	}
	if got := scoring.stackScore([]string{}, []string{"Node.js"}); got != 0 {
		t.Errorf("stackScore(пустой кандидат) = %v, ожидался 0", got)
	}
	if got := scoring.stackScore([]string{"Node.js"}, []string{}); got != 50 {
		t.Errorf("stackScore(пустая потребность) = %v, ожидался 50", got)
	}
}

func TestScoringV1GradeScore(t *testing.T) {
	scoring := scoringService{}

	cases := []struct {
		name      string
		verified  model.Grade
		needGrade model.Grade
		want      float64
	}{
		{"точное совпадение", model.GradeMiddle, model.GradeMiddle, 100},
		{"на грейд выше", model.GradeMiddlePlus, model.GradeMiddle, 80},
		{"на грейд ниже", model.GradeJunior, model.GradeMiddle, 60},
		{"значительно выше", model.GradeSenior, model.GradeMiddle, 40},
		{"значительно ниже", model.GradeMiddle, model.GradePrincipal, 20},
	}
	for _, tc := range cases {
		got := scoring.gradeScore(
			testCandidate(func(c *ScoreableCandidate) { c.VerifiedGrade = tc.verified }),
			testNeed(func(n *Need) { n.Grade = tc.needGrade }))
		if got != tc.want {
			t.Errorf("%s: gradeScore = %v, ожидался %v", tc.name, got, tc.want)
		}
	}
	if got := scoring.gradeScore(testCandidate(nil), Need{Stack: []string{}}); got != 50 {
		t.Errorf("gradeScore(need без грейда) = %v, ожидался 50", got)
	}
	if got := scoring.gradeScore(
		testCandidate(func(c *ScoreableCandidate) { c.VerifiedGrade = ""; c.DeclaredGrade = "" }),
		testNeed(nil)); got != 30 {
		t.Errorf("gradeScore(кандидат без грейда) = %v, ожидался 30", got)
	}
}

func TestScoringV1VerifiedGradeWins(t *testing.T) {
	scoring := scoringService{}
	result := scoring.calculateMatch(
		testCandidate(func(c *ScoreableCandidate) {
			c.VerifiedGrade = model.GradeSenior
			c.DeclaredGrade = model.GradeJunior
		}),
		testNeed(func(n *Need) { n.Grade = model.GradeSenior }),
		1,
	)
	if result.Components.GradeMatch != 100 {
		t.Errorf("gradeMatch = %v, ожидался 100 (verified важнее declared)", result.Components.GradeMatch)
	}
}

func TestScoringV1AchievementScore(t *testing.T) {
	scoring := scoringService{}
	if got := scoring.achievementScore(0); got != 0 {
		t.Errorf("achievementScore(0) = %v, ожидался 0", got)
	}
	if got := scoring.achievementScore(5); got != 50 {
		t.Errorf("achievementScore(5) = %v, ожидался 50", got)
	}
	if got := scoring.achievementScore(1000); got != 100 {
		t.Errorf("achievementScore(1000) = %v, ожидался 100", got)
	}
}

func TestScoringV1BoundaryNoNaN(t *testing.T) {
	scoring := scoringService{}
	result := scoring.calculateMatch(
		testCandidate(func(c *ScoreableCandidate) {
			c.Specialization = ""
			c.VerifiedGrade = ""
			c.DeclaredGrade = ""
			c.Stack = []string{}
		}),
		Need{Stack: []string{"Node.js"}, Grade: model.GradePrincipal},
		math.NaN(),
	)
	if !isFinite(result.Total) {
		t.Fatalf("total = %v, ожидалось конечное число", result.Total)
	}
	if result.Total < 0 || result.Total > 100 {
		t.Errorf("total = %v, ожидалось в пределах 0..100", result.Total)
	}
}

func TestNormalizeTechV1(t *testing.T) {
	if got := normalizeTech(" Node JS "); got != "nodejs" {
		t.Errorf("normalizeTech(\" Node JS \") = %q, ожидался 'nodejs'", got)
	}
	if got := normalizeTech("NODEJS"); got != "nodejs" {
		t.Errorf("normalizeTech(\"NODEJS\") = %q, ожидался 'nodejs'", got)
	}
	if got := normalizeTech(""); got != "" {
		t.Errorf("normalizeTech(\"\") = %q, ожидалась пустая строка", got)
	}
}

func TestExplanationDetailsContainWeights(t *testing.T) {
	service := explanationService{}
	lastTest := time.Now().Add(-10 * 86400000 * time.Millisecond)
	explanation := service.generate(
		ExplanationCandidate{
			ScoreableCandidate: testCandidate(func(c *ScoreableCandidate) { c.AchievementPoints = 50 }),
			LastTestDate:       &lastTest,
			AchievementNames:   []string{"Хакатон «Цифровая экономика»"},
		},
		testNeed(nil),
		testScore(85, nil),
	)

	weights := map[float64]bool{}
	for _, d := range explanation.Details {
		weights[d.Weight] = true
	}
	for _, w := range []float64{WEIGHTS.Category, WEIGHTS.Stack, WEIGHTS.Grade, WEIGHTS.Semantic, WEIGHTS.Achievement} {
		if !weights[w] {
			t.Errorf("нет детали с весом %v (детали: %+v)", w, explanation.Details)
		}
	}
	if len(explanation.Overall) == 0 {
		t.Error("overall пуст")
	}
}

func TestExplanationNoAchievementNoTestBlocks(t *testing.T) {
	service := explanationService{}
	explanation := service.generate(
		ExplanationCandidate{ScoreableCandidate: testCandidate(nil)},
		testNeed(nil),
		testScore(50, nil),
	)
	for _, d := range explanation.Details {
		if d.Weight == WEIGHTS.Achievement {
			t.Error("не должно быть блока достижений")
		}
		if strings.Contains(strings.ToLower(d.Factor), "тестирован") {
			t.Error("не должно быть блока тестирования")
		}
	}
}

func TestExplanationStackOverlap(t *testing.T) {
	service := explanationService{}
	explanation := service.generate(
		ExplanationCandidate{ScoreableCandidate: testCandidate(nil)},
		testNeed(func(n *Need) { n.Stack = []string{"Node.js", "GraphQL"} }),
		testScore(60, nil),
	)

	var stackDetail *ExplanationDetail
	for i, d := range explanation.Details {
		if d.Weight == WEIGHTS.Stack {
			stackDetail = &explanation.Details[i]
			break
		}
	}
	if stackDetail == nil {
		t.Fatal("нет детали по стеку")
	}
	if stackDetail.Overlap == nil {
		t.Fatal("overlap = nil")
	}
	if len(*stackDetail.Overlap) != 1 || (*stackDetail.Overlap)[0] != "Node.js" {
		t.Errorf("overlap = %v, ожидался [Node.js]", *stackDetail.Overlap)
	}
	if stackDetail.Match != true {
		t.Errorf("match = %v, ожидался true", stackDetail.Match)
	}
}

func TestExplanationSemanticThreshold(t *testing.T) {
	service := explanationService{}
	candidate := ExplanationCandidate{ScoreableCandidate: testCandidate(nil)}
	need := testNeed(nil)

	high := service.generate(candidate, need, testScore(70, func(c *MatchComponents) { c.SemanticSimilarity = 80 }))
	low := service.generate(candidate, need, testScore(70, func(c *MatchComponents) { c.SemanticSimilarity = 20 }))

	semanticMatch := func(e MatchExplanation) any {
		for _, d := range e.Details {
			if d.Weight == WEIGHTS.Semantic {
				return d.Match
			}
		}
		return nil
	}
	if semanticMatch(high) != true {
		t.Errorf("semantic(80) match = %v, ожидался true", semanticMatch(high))
	}
	if semanticMatch(low) != false {
		t.Errorf("semantic(20) match = %v, ожидался false", semanticMatch(low))
	}
}

func TestExplanationTestRecency(t *testing.T) {
	service := explanationService{}
	candidate := ExplanationCandidate{ScoreableCandidate: testCandidate(nil)}
	need := testNeed(nil)
	score := testScore(60, nil)

	withFresh := candidate
	fresh := time.Now().Add(-5 * 86400000 * time.Millisecond)
	withFresh.LastTestDate = &fresh

	withOld := candidate
	old := time.Now().Add(-400 * 86400000 * time.Millisecond)
	withOld.LastTestDate = &old

	block := func(e MatchExplanation) any {
		for _, d := range e.Details {
			if d.Weight == 0 && d.Match != nil {
				return d.Match
			}
		}
		return nil
	}

	freshExplanation := service.generate(candidate, need, score)
	for _, d := range freshExplanation.Details {
		if d.Weight == 0 {
			t.Error("без lastTestDate не должно быть блока с весом 0")
		}
	}
	if block(service.generate(withFresh, need, score)) != true {
		t.Error("свежий тест: match должен быть true")
	}
	if block(service.generate(withOld, need, score)) != false {
		t.Error("старый тест: match должен быть false")
	}
}

func TestExplanationSummaryLevels(t *testing.T) {
	service := explanationService{}
	if service.summary(90) == service.summary(70) {
		t.Error("уровни 90 и 70 должны различаться")
	}
	if service.summary(70) == service.summary(50) {
		t.Error("уровни 70 и 50 должны различаться")
	}
	if service.summary(50) == service.summary(20) {
		t.Error("уровни 50 и 20 должны различаться")
	}
	if len(service.summary(100)) == 0 {
		t.Error("summary(100) пуст")
	}
}

func TestExplanationDetailsHaveText(t *testing.T) {
	service := explanationService{}
	explanation := service.generate(
		ExplanationCandidate{ScoreableCandidate: testCandidate(nil)},
		testNeed(nil),
		testScore(45, nil),
	)
	for _, d := range explanation.Details {
		if len(d.Explanation) == 0 {
			t.Errorf("деталь %q без текста объяснения", d.Factor)
		}
	}
}
