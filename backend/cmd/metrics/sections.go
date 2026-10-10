package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"fsp-backend/internal/config"
	"fsp-backend/internal/store"
)

// ============ БД (для раздела Testing: пул вопросов) ============

var db *store.DB

func openStore() error {
	cfg := config.Load()
	var err error
	db, err = store.Connect(context.Background(), cfg.DatabaseURL)
	return err
}

func closeStore() {
	if db != nil {
		db.Close()
	}
}

// ============ 1. SMOKE ============

func smokeSection() error {
	checks := []check{}

	health := call(api+"/health", callOpts{})
	healthBody := asMap(health.Body)
	services := asMap(field(healthBody, "services"))
	embedding := asMap(field(services, "embedding"))
	checks = append(checks, check{
		Name: "GET /api/health",
		Ok:   health.Status == 200 && strField(healthBody, "status") == "ok",
		Detail: fmt.Sprintf("status=%d, db=%v, dim=%v",
			health.Status, field(services, "database"), field(embedding, "dim")),
	})

	docs := call(api+"/docs-json", callOpts{})
	docsBody := asMap(docs.Body)
	paths := asMap(field(docsBody, "paths"))
	checks = append(checks, check{
		Name:   "Swagger JSON публикуется",
		Ok:     docs.Status == 200 && len(paths) > 0,
		Detail: fmt.Sprintf("status=%d, путей: %d", docs.Status, len(paths)),
	})

	home := call(frontend+"/", callOpts{})
	homeLen := 0
	if s, ok := home.Body.(string); ok {
		homeLen = len(s)
	}
	checks = append(checks, check{
		Name:   "Frontend: главная страница",
		Ok:     home.Status == 200 && strings.Contains(asString(home.Body), "ФСП"),
		Detail: fmt.Sprintf("status=%d, %d мс, %d байт", home.Status, home.MS, homeLen),
	})

	login := call(frontend+"/login", callOpts{})
	checks = append(checks, check{
		Name:   "Frontend: /login",
		Ok:     login.Status == 200,
		Detail: fmt.Sprintf("status=%d, %d мс", login.Status, login.MS),
	})

	proxy := call(frontend+"/api/health", callOpts{})
	proxyBody := asMap(proxy.Body)
	checks = append(checks, check{
		Name:   "Прокси /api через frontend (rewrites)",
		Ok:     proxy.Status == 200 && strField(proxyBody, "status") == "ok",
		Detail: fmt.Sprintf("status=%d, %d мс", proxy.Status, proxy.MS),
	})

	add("1. Smoke", checks, "")
	return nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ============ 2. SECURITY ============

func securitySection() error {
	checks := []check{}

	candidate, err := register("CANDIDATE")
	if err != nil {
		return err
	}
	employer, err := register("EMPLOYER")
	if err != nil {
		return err
	}

	noToken := call(api+"/candidates/profile", callOpts{})
	checks = append(checks, check{
		Name: "Доступ без токена → 401", Ok: noToken.Status == 401,
		Detail: fmt.Sprintf("status=%d", noToken.Status),
	})

	badToken := call(api+"/candidates/profile", callOpts{Token: "invalid.token.value"})
	checks = append(checks, check{
		Name: "Невалидный JWT → 401", Ok: badToken.Status == 401,
		Detail: fmt.Sprintf("status=%d", badToken.Status),
	})

	roleMismatch := call(api+"/candidates/profile", callOpts{Token: employer.Token})
	checks = append(checks, check{
		Name: "RBAC: работодатель не читает профиль кандидата → 403", Ok: roleMismatch.Status == 403,
		Detail: fmt.Sprintf("status=%d", roleMismatch.Status),
	})

	roleMismatch2 := call(api+"/employers/dashboard", callOpts{Token: candidate.Token})
	checks = append(checks, check{
		Name: "RBAC: кандидат не читает дашборд работодателя → 403", Ok: roleMismatch2.Status == 403,
		Detail: fmt.Sprintf("status=%d", roleMismatch2.Status),
	})

	sqli := call(api+"/auth/login", callOpts{
		Method: "POST",
		Body:   map[string]any{"email": `admin@example.com" OR 1=1--`, "password": "test"},
	})
	checks = append(checks, check{
		Name: "SQL-injection в login → 4xx (не 500)", Ok: sqli.Status >= 400 && sqli.Status < 500,
		Detail: fmt.Sprintf("status=%d", sqli.Status),
	})

	weak := call(api+"/auth/register", callOpts{
		Method: "POST",
		Body:   map[string]any{"email": "weak-" + stamp() + "@example.com", "password": "1", "role": "CANDIDATE"},
	})
	checks = append(checks, check{
		Name: "Слабый пароль при регистрации → 400", Ok: weak.Status == 400,
		Detail: fmt.Sprintf("status=%d", weak.Status),
	})

	missingVacancy := call(api+"/vacancies/00000000-0000-0000-0000-000000000000", callOpts{})
	checks = append(checks, check{
		Name: "Несуществующая вакансия → 404", Ok: missingVacancy.Status == 404,
		Detail: fmt.Sprintf("status=%d", missingVacancy.Status),
	})

	foreign := call(api+"/invitations", callOpts{
		Method: "POST",
		Token:  candidate.Token,
		Body: map[string]any{
			"candidateId": "00000000-0000-0000-0000-000000000000",
			"message":     strings.Repeat("x", 20),
			"salaryFrom":  1, "salaryTo": 2,
		},
	})
	checks = append(checks, check{
		Name: "Кандидат не может создать приглашение → 403", Ok: foreign.Status == 403,
		Detail: fmt.Sprintf("status=%d", foreign.Status),
	})

	searchNoAuth := call(api+"/matching/search", callOpts{
		Method: "POST",
		Body:   map[string]any{"description": "тест тест тест тест тест"},
	})
	checks = append(checks, check{
		Name: "Поиск кандидатов без токена → 401", Ok: searchNoAuth.Status == 401,
		Detail: fmt.Sprintf("status=%d", searchNoAuth.Status),
	})

	add("2. Security", checks, "")
	return nil
}

// ============ 3. MATCHING QUALITY ============

type need struct {
	Title          string   `json:"title"`
	Specialization string   `json:"specialization"`
	Grade          string   `json:"grade"`
	Stack          []string `json:"stack"`
	Description    string   `json:"description"`
}

var needs = []need{
	{Title: "Backend: Node.js + PostgreSQL", Specialization: "BACKEND", Grade: "MIDDLE",
		Stack:       []string{"Node.js", "PostgreSQL"},
		Description: "Ищем backend-разработчика на Node.js: REST API, PostgreSQL, транзакции, высокая нагрузка"},
	{Title: "Backend: распределённые системы", Specialization: "BACKEND", Grade: "SENIOR",
		Stack:       []string{"Redis", "Docker", "Kafka"},
		Description: "Senior backend: Redis, очереди Kafka, Docker, отказоустойчивость"},
	{Title: "Frontend: React + TypeScript", Specialization: "FRONTEND", Grade: "MIDDLE",
		Stack:       []string{"React", "TypeScript"},
		Description: "Frontend-разработчик на React и TypeScript, дизайн-системы, производительность UI"},
	{Title: "DevOps: Kubernetes", Specialization: "DEVOPS", Grade: "MIDDLE",
		Stack:       []string{"Docker", "Kubernetes"},
		Description: "Инженер DevOps: Kubernetes, Docker, CI/CD, инфраструктура как код"},
	{Title: "Mobile: Flutter", Specialization: "MOBILE", Grade: "MIDDLE",
		Stack:       []string{"Flutter", "Android"},
		Description: "Mobile-разработчик: Flutter и Android, публикация в сторах"},
	{Title: "Data Science: ML-модели", Specialization: "DATA_SCIENCE", Grade: "MIDDLE",
		Stack:       []string{"Python", "PyTorch"},
		Description: "Data Scientist: Python, обучение моделей, продакшн-инференс"},
}

// isRelevant — специализация совпадает и пересечение стека ≥ 50%.
func isRelevant(candidate map[string]any, n need) bool {
	if n.Specialization != "" && strField(candidate, "specialization") != n.Specialization {
		return false
	}
	have := map[string]bool{}
	for _, s := range toStringSlice(field(candidate, "stack")) {
		have[norm(s)] = true
	}
	wanted := 0
	overlap := 0
	for _, s := range n.Stack {
		wanted++
		if have[norm(s)] {
			overlap++
		}
	}
	if wanted == 0 {
		return false
	}
	return float64(overlap)/float64(wanted) >= 0.5
}

func toStringSlice(v any) []string {
	if arr, ok := v.([]any); ok {
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	if arr, ok := v.([]string); ok {
		return arr
	}
	return nil
}

func precisionAt(ranked []bool, k int) float64 {
	top := ranked
	if len(top) > k {
		top = top[:k]
	}
	if len(top) == 0 {
		return 0
	}
	rel := 0
	for _, r := range top {
		if r {
			rel++
		}
	}
	return float64(rel) / float64(len(top))
}

func recallAt(ranked []bool, totalRel, k int) float64 {
	if totalRel == 0 {
		return 0
	}
	top := ranked
	if len(top) > k {
		top = top[:k]
	}
	rel := 0
	for _, r := range top {
		if r {
			rel++
		}
	}
	return float64(rel) / float64(totalRel)
}

func mrrAt(ranked []bool, k int) float64 {
	limit := k
	if len(ranked) < limit {
		limit = len(ranked)
	}
	for i := 0; i < limit; i++ {
		if ranked[i] {
			return 1 / float64(i+1)
		}
	}
	return 0
}

func ndcgAt(ranked []bool, k int) float64 {
	top := ranked
	if len(top) > k {
		top = top[:k]
	}
	dcg := 0.0
	ideal := 0
	for i, rel := range top {
		if rel {
			dcg += 1 / math.Log2(float64(i+2))
			ideal++
		}
	}
	idcg := 0.0
	for i := 0; i < ideal; i++ {
		idcg += 1 / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

type matchingResults struct {
	P5, P10, MrrAvg, NDCG, SearchMS float64
}

func matchingSection() (matchingResults, error) {
	employer, err := register("EMPLOYER")
	if err != nil {
		return matchingResults{}, err
	}
	checks := []check{}
	rows := []string{}

	var p5Sum, p10Sum, mrrSum, ndcgSum, searchMsSum float64

	for _, n := range needs {
		body := map[string]any{
			"title": n.Title, "specialization": n.Specialization, "grade": n.Grade,
			"stack": n.Stack, "description": n.Description, "limit": 100,
		}
		full := call(api+"/matching/search", callOpts{Method: "POST", Token: employer.Token, Body: body})
		fastBody := map[string]any{
			"title": n.Title, "specialization": n.Specialization, "grade": n.Grade,
			"stack": n.Stack, "description": n.Description, "limit": 10,
		}
		fast := call(api+"/matching/search", callOpts{Method: "POST", Token: employer.Token, Body: fastBody})

		fullBody := asMap(full.Body)
		list := []map[string]any{}
		if arr, ok := field(fullBody, "candidates").([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					list = append(list, m)
				}
			}
		}
		ranked := make([]bool, 0, len(list))
		relevant := 0
		for _, c := range list {
			r := isRelevant(c, n)
			if r {
				relevant++
			}
			ranked = append(ranked, r)
		}

		p5 := precisionAt(ranked, 5)
		p10 := precisionAt(ranked, 10)
		r10 := recallAt(ranked, relevant, 10)
		m := mrrAt(ranked, 10)
		nd := ndcgAt(ranked, 10)

		p5Sum += p5
		p10Sum += p10
		mrrSum += m
		ndcgSum += nd
		searchMsSum += float64(fast.MS)

		hardFiltered := true
		for _, c := range list {
			if strField(c, "specialization") != n.Specialization {
				hardFiltered = false
				break
			}
		}
		rows = append(rows, fmt.Sprintf("| %s | %d | %.0f%% | %.0f%% | %.0f%% | %.2f | %.3f | %d мс | %s |",
			n.Title, len(list), p5*100, p10*100, r10*100, m, nd, fast.MS, yesNo(hardFiltered)))

		checks = append(checks, check{
			Name:   fmt.Sprintf("precision@5 · %s", n.Title),
			Ok:     p5 >= 0.7,
			Detail: fmt.Sprintf("%.0f%% (цель ≥ 70%%), релевантных в базе: %d", p5*100, relevant),
		})
		checks = append(checks, check{
			Name:   fmt.Sprintf("hard-фильтр (только %s) · %s", n.Specialization, n.Title),
			Ok:     hardFiltered,
			Detail: fmt.Sprintf("кандидатов в выдаче: %d", len(list)),
		})
	}

	count := float64(len(needs))
	p5 := p5Sum / count
	p10 := p10Sum / count
	mrrAvg := mrrSum / count
	ndcg := ndcgSum / count
	searchMs := searchMsSum / count

	tableLines := []string{
		"| Need | Канд. | P@5 | P@10 | R@10 | MRR | nDCG@10 | Поиск | Hard-фильтр |",
		"|------|------|-----|------|------|-----|---------|-------|-------------|",
	}
	tableLines = append(tableLines, rows...)
	tableLines = append(tableLines,
		fmt.Sprintf("| **Среднее** | — | **%.0f%%** | **%.0f%%** | — | **%.2f** | **%.3f** | **%d мс** | — |",
			p5*100, p10*100, mrrAvg, ndcg, int(math.Round(searchMs))))

	add("3. Matching quality", checks, strings.Join(tableLines, "\n"))
	return matchingResults{P5: p5, P10: p10, MrrAvg: mrrAvg, NDCG: ndcg, SearchMS: searchMs}, nil
}

func yesNo(v bool) string {
	if v {
		return "да"
	}
	return "НЕТ"
}

// ============ 4. TESTING QUALITY ============

type testOutcome struct {
	Score          float64
	Theta          float64
	GradeAfter     string
	Recommendation string
	Suspicious     bool
	Flags          []string
}

type questionSource struct {
	Options       []map[string]any `json:"options"`
	CorrectAnswer *string          `json:"correctAnswer"`
}

// runTest — прогон V1-теста: правильные ответы (correct) или «не знаю» (wrong).
func runTest(mode string) (testOutcome, error) {
	account, err := register("CANDIDATE")
	if err != nil {
		return testOutcome{}, err
	}
	start := call(api+"/testing/start", callOpts{
		Method: "POST",
		Token:  account.Token,
		Body:   map[string]any{"specialization": "BACKEND", "declaredGrade": "MIDDLE"},
	})
	if start.Status != 201 {
		return testOutcome{}, fmt.Errorf("testing/start: %d", start.Status)
	}
	startBody := asMap(start.Body)
	sessionID := strField(startBody, "sessionId")

	ctx := context.Background()
	var categoryID string
	if err := db.Pool.QueryRow(ctx,
		`SELECT category_id FROM test_sessions WHERE id = $1`, sessionID).Scan(&categoryID); err != nil {
		return testOutcome{}, fmt.Errorf("session not found: %w", err)
	}

	rows, err := db.Pool.Query(ctx,
		`SELECT id, options, correct_answer FROM test_questions WHERE category_id = $1`, categoryID)
	if err != nil {
		return testOutcome{}, err
	}
	pool := map[string]questionSource{}
	for rows.Next() {
		var id string
		var opts []byte
		var correct *string
		if err := rows.Scan(&id, &opts, &correct); err != nil {
			rows.Close()
			return testOutcome{}, err
		}
		src := questionSource{CorrectAnswer: correct}
		if len(opts) > 0 && string(opts) != "null" {
			_ = json.Unmarshal(opts, &src.Options)
		}
		pool[id] = src
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return testOutcome{}, err
	}

	for i := 0; i < 25; i++ {
		current := call(fmt.Sprintf("%s/testing/session/%s/current", api, sessionID),
			callOpts{Token: account.Token})
		currentBody := asMap(current.Body)
		question := asMap(field(currentBody, "question"))
		if question == nil {
			break
		}
		questionID := strField(question, "id")
		questionType := strField(question, "questionType")

		source, ok := pool[questionID]
		if !ok {
			return testOutcome{}, fmt.Errorf("question %s not in pool", questionID)
		}

		body := map[string]any{
			"questionId":       questionID,
			"timeSpentSeconds": rnd(6, 55),
		}
		if questionType == "multiple_choice" {
			correctIdx, wrongIdx := -1, -1
			for i := range source.Options {
				isCorrect, _ := source.Options[i]["isCorrect"].(bool)
				if isCorrect && correctIdx == -1 {
					correctIdx = i
				}
				if !isCorrect && wrongIdx == -1 {
					wrongIdx = i
				}
			}
			chosenIdx := -1
			if mode == "correct" {
				chosenIdx = correctIdx
				if chosenIdx == -1 && len(source.Options) > 0 {
					chosenIdx = 0
				}
			} else {
				chosenIdx = wrongIdx
				if chosenIdx == -1 && len(source.Options) > 0 {
					chosenIdx = len(source.Options) - 1
				}
			}
			if chosenIdx == -1 {
				return testOutcome{}, fmt.Errorf("no options")
			}
			body["answer"] = source.Options[chosenIdx]["id"]
		} else {
			if mode == "correct" {
				if source.CorrectAnswer != nil && *source.CorrectAnswer != "" {
					body["answerText"] = *source.CorrectAnswer
				} else {
					body["answerText"] = "ключевые слова эталона покрыты полностью: "
				}
			} else {
				body["answerText"] = "не знаю"
			}
		}

		call(fmt.Sprintf("%s/testing/session/%s/answer", api, sessionID),
			callOpts{Method: "POST", Token: account.Token, Body: body})
		wait(int64(rnd(120, 420)))
	}

	complete := call(fmt.Sprintf("%s/testing/session/%s/complete", api, sessionID),
		callOpts{Method: "POST", Token: account.Token, Body: map[string]any{}})
	if complete.Status != 201 {
		raw, _ := json.Marshal(complete.Body)
		return testOutcome{}, fmt.Errorf("complete: %d %s", complete.Status, string(raw))
	}
	completeBody := asMap(complete.Body)
	antiCheat := asMap(field(completeBody, "antiCheat"))
	flags := []string{}
	if arr, ok := field(antiCheat, "flags").([]any); ok {
		for _, f := range arr {
			if s, ok := f.(string); ok {
				flags = append(flags, s)
			}
		}
	}
	suspicious, _ := field(antiCheat, "isSuspicious").(bool)

	return testOutcome{
		Score:          numField(completeBody, "score"),
		Theta:          numField(completeBody, "theta"),
		GradeAfter:     strField(completeBody, "gradeAfter"),
		Recommendation: strField(completeBody, "recommendation"),
		Suspicious:     suspicious,
		Flags:          flags,
	}, nil
}

type testingResults struct {
	Strong, Strong2, Weak testOutcome
}

func testingSection() (testingResults, error) {
	checks := []check{}

	strong, err := runTest("correct")
	if err != nil {
		return testingResults{}, err
	}
	strong2, err := runTest("correct")
	if err != nil {
		return testingResults{}, err
	}
	weak, err := runTest("wrong")
	if err != nil {
		return testingResults{}, err
	}

	checks = append(checks, check{
		Name:   "Дискриминативность: сильный кандидат получает высокий score",
		Ok:     strong.Score >= 80 && strong2.Score >= 80,
		Detail: fmt.Sprintf("score1=%v, score2=%v, θ1=%v, θ2=%v", fmtNum(strong.Score), fmtNum(strong2.Score), fmtNum(strong.Theta), fmtNum(strong2.Theta)),
	})
	checks = append(checks, check{
		Name:   "Дискриминативность: слабый кандидат получает низкий score",
		Ok:     weak.Score <= 30,
		Detail: fmt.Sprintf("score=%v, θ=%v", fmtNum(weak.Score), fmtNum(weak.Theta)),
	})
	checks = append(checks, check{
		Name: "Порядок оценок: сильный ≫ слабый (θ различие ≥ 2)",
		Ok:   strong.Theta-weak.Theta >= 2 && gradeIdx(strong.GradeAfter) > gradeIdx(weak.GradeAfter),
		Detail: fmt.Sprintf("θ: %v vs %v; грейд: %s vs %s",
			fmtNum(strong.Theta), fmtNum(weak.Theta), strong.GradeAfter, weak.GradeAfter),
	})
	checks = append(checks, check{
		Name: "Test-retest: повторный прогон даёт близкий результат",
		Ok:   math.Abs(strong.Score-strong2.Score) <= 10 && strong.GradeAfter == strong2.GradeAfter,
		Detail: fmt.Sprintf("|Δscore| = %v, грейды: %s / %s",
			fmtNum(math.Abs(strong.Score-strong2.Score)), strong.GradeAfter, strong2.GradeAfter),
	})
	checks = append(checks, check{
		Name: "Рекомендации соответствуют результату",
		Ok:   strong.Recommendation != "FAIL" && weak.Recommendation == "FAIL",
		Detail: fmt.Sprintf("сильный: %s, слабый: %s",
			strong.Recommendation, weak.Recommendation),
	})
	checks = append(checks, check{
		Name: "Анти-чит не даёт ложных срабатываний на честном прогоне",
		Ok:   !strong.Suspicious && !strong2.Suspicious,
		Detail: fmt.Sprintf("flags1: [%s] · flags2: [%s]",
			strings.Join(strong.Flags, ", "), strings.Join(strong2.Flags, ", ")),
	})

	table := strings.Join([]string{
		"| Прогон | Score | θ | Grade after | Recommendation | Suspicious |",
		"|--------|-------|---|-------------|----------------|------------|",
		fmt.Sprintf("| Сильный (1) | %v | %v | %s | %s | %t |",
			fmtNum(strong.Score), fmtNum(strong.Theta), strong.GradeAfter, strong.Recommendation, strong.Suspicious),
		fmt.Sprintf("| Сильный (2) — test-retest | %v | %v | %s | %s | %t |",
			fmtNum(strong2.Score), fmtNum(strong2.Theta), strong2.GradeAfter, strong2.Recommendation, strong2.Suspicious),
		fmt.Sprintf("| Слабый | %v | %v | %s | %s | %t |",
			fmtNum(weak.Score), fmtNum(weak.Theta), weak.GradeAfter, weak.Recommendation, weak.Suspicious),
	}, "\n")

	add("4. Testing quality", checks, table)
	return testingResults{Strong: strong, Strong2: strong2, Weak: weak}, nil
}

// fmtNum — JS number → строка (без .0 для целых).
func fmtNum(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// ============ 5. PERFORMANCE ============

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	// сортировка по возрастанию
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx > len(sorted)-1 {
		idx = len(sorted) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

type perfTarget struct {
	Title  string
	Run    func() httpResult
	MaxAvg float64
	MaxP95 float64 // 0 — не проверяется
}

type performanceResults struct {
	AvgAll, SearchAvg, FrontendMS float64
}

func performanceSection() (performanceResults, error) {
	candidate, err := register("CANDIDATE")
	if err != nil {
		return performanceResults{}, err
	}
	employer, err := register("EMPLOYER")
	if err != nil {
		return performanceResults{}, err
	}
	checks := []check{}
	rows := []string{}

	targets := []perfTarget{
		{Title: "GET /api/health", MaxAvg: 200,
			Run: func() httpResult { return call(api+"/health", callOpts{}) }},
		{Title: "GET /api/candidates/profile", MaxAvg: 200,
			Run: func() httpResult { return call(api+"/candidates/profile", callOpts{Token: candidate.Token}) }},
		{Title: "GET /api/vacancies?limit=20", MaxAvg: 200,
			Run: func() httpResult { return call(api+"/vacancies?limit=20", callOpts{}) }},
		{Title: "GET /api/notifications", MaxAvg: 200,
			Run: func() httpResult { return call(api+"/notifications", callOpts{Token: candidate.Token}) }},
		{Title: "GET /api/employers/dashboard", MaxAvg: 200,
			Run: func() httpResult { return call(api+"/employers/dashboard", callOpts{Token: employer.Token}) }},
		{Title: "POST /api/matching/search (limit 10)", MaxAvg: 200, MaxP95: 500,
			Run: func() httpResult {
				return call(api+"/matching/search", callOpts{
					Method: "POST", Token: employer.Token,
					Body: map[string]any{
						"description":    "backend разработчик Node.js PostgreSQL Redis",
						"specialization": "BACKEND", "grade": "MIDDLE",
						"stack": []string{"Node.js", "PostgreSQL"}, "limit": 10,
					},
				})
			}},
		{Title: "POST /api/matching/semantic (limit 10)", MaxAvg: 200, MaxP95: 500,
			Run: func() httpResult {
				return call(api+"/matching/semantic", callOpts{
					Method: "POST", Token: employer.Token,
					Body: map[string]any{
						"query": "разработчик с опытом Redis и очередей", "limit": 10,
					},
				})
			}},
	}

	const iterations = 30
	allAverages := []float64{}
	searchAvg := 0.0

	for _, target := range targets {
		times := []float64{}
		errors := 0
		target.Run() // прогрев
		for i := 0; i < iterations; i++ {
			res := target.Run()
			times = append(times, float64(res.MS))
			if res.Status >= 500 {
				errors++
			}
		}
		sum := 0.0
		for _, t := range times {
			sum += t
		}
		avg := sum / float64(len(times))
		p50 := percentile(times, 50)
		p95 := percentile(times, 95)
		max := 0.0
		for _, t := range times {
			if t > max {
				max = t
			}
		}
		allAverages = append(allAverages, avg)
		if strings.Contains(target.Title, "search") {
			searchAvg = avg
		}

		ok := avg < target.MaxAvg && errors == 0 && (target.MaxP95 == 0 || p95 < target.MaxP95)
		goal := fmt.Sprintf("avg < %.0f мс", target.MaxAvg)
		if target.MaxP95 > 0 {
			goal += fmt.Sprintf(", p95 < %.0f мс", target.MaxP95)
		}
		checks = append(checks, check{
			Name: target.Title,
			Ok:   ok,
			Detail: fmt.Sprintf("avg=%.0f мс, p50=%s мс, p95=%s мс, max=%s мс, 5xx=%d (цель: %s)",
				avg, fmtNum(p50), fmtNum(p95), fmtNum(max), errors, goal),
		})
		rows = append(rows, fmt.Sprintf("| %s | %.0f | %s | %s | %s | %d |",
			target.Title, avg, fmtNum(p50), fmtNum(p95), fmtNum(max), errors))
	}

	frontendStarted := time.Now()
	home := call(frontend+"/", callOpts{})
	frontendMS := float64(time.Since(frontendStarted).Milliseconds())
	checks = append(checks, check{
		Name:   "Frontend: полная загрузка документа < 3000 мс",
		Ok:     home.Status == 200 && frontendMS < 3000,
		Detail: fmt.Sprintf("status=%d, %d мс (SSR/TTFB; гидратация измеряется в браузере)", home.Status, int64(frontendMS)),
	})
	rows = append(rows, fmt.Sprintf("| GET %s/ (SSR документ) | %d | — | — | %d | 0 |",
		frontend, int64(frontendMS), int64(frontendMS)))

	avgAll := 0.0
	for _, a := range allAverages {
		avgAll += a
	}
	avgAll /= float64(len(allAverages))
	checks = append(checks, check{
		Name:   "Среднее время ответа API < 200 мс",
		Ok:     avgAll < 200,
		Detail: fmt.Sprintf("avg по всем endpoint'ам = %.0f мс", avgAll),
	})

	table := strings.Join(append([]string{
		"| Endpoint | avg, мс | p50 | p95 | max | 5xx |",
		"|----------|---------|-----|-----|-----|-----|",
	}, rows...), "\n")

	add("5. Performance", checks, table)
	return performanceResults{AvgAll: avgAll, SearchAvg: searchAvg, FrontendMS: frontendMS}, nil
}
