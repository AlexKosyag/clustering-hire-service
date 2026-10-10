package testing_test

// Порт backend/src/modules/testing/testing-v2.service.spec.ts:
// регресс бага «409 Ответ на этот вопрос уже записан на втором вопросе» (Testing V2).
// Вместо in-memory Prisma — реализация testing.Store поверх срезов (interface Store).

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"testing"
	"time"

	"fsp-backend/internal/model"
	testingmod "fsp-backend/internal/modules/testing"
	"fsp-backend/internal/modules/testing/services"
)

// ============ In-memory Store (аналог createV2Context из TS-спека) ============

type fakeStore struct {
	mu            sync.Mutex
	categories    []services.Category
	questions     map[string][]services.PoolQuestion // categoryID → пул (id asc)
	questionsByID map[string]services.PoolQuestion
	sessions      []*model.TestSession
	answers       []testingmod.SessionAnswer
	profiles      []*model.CandidateProfile
}

func newFakeStore(categories []services.Category, questions map[string][]services.PoolQuestion) *fakeStore {
	f := &fakeStore{
		categories:    categories,
		questions:     questions,
		questionsByID: map[string]services.PoolQuestion{},
	}
	for _, qs := range questions {
		for _, q := range qs {
			f.questionsByID[q.ID] = q
		}
	}
	return f
}

func (f *fakeStore) ListQuestions(_ context.Context, categoryID string) ([]services.PoolQuestion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]services.PoolQuestion, len(f.questions[categoryID]))
	copy(out, f.questions[categoryID])
	return out, nil
}

func (f *fakeStore) FindCategoryBySpecialization(_ context.Context, spec model.Specialization) (*services.Category, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.categories {
		if c.Specialization == spec {
			cp := c
			return &cp, nil
		}
	}
	return nil, nil // pgx.ErrNoRows → nil (в pool маппится в «нет категории»)
}

func (f *fakeStore) FindAnyCategory(_ context.Context) (*services.Category, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.categories) == 0 {
		return nil, nil
	}
	sorted := append([]services.Category(nil), f.categories...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	return &sorted[0], nil
}

func (f *fakeStore) FindProfileByUserID(_ context.Context, userID string) (*model.CandidateProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.profiles {
		if p.UserID == userID {
			cp := *p
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeStore) SetProfileTestStatus(_ context.Context, profileID string, status model.TestStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.profiles {
		if p.ID == profileID {
			p.TestStatus = status
		}
	}
	return nil
}

func (f *fakeStore) SetProfileTestResult(_ context.Context, profileID string, gradeAfter model.Grade,
	testStatus model.TestStatus, lastTestDate, cooldownUntil time.Time, lastTestScore float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	last := model.FromTime(lastTestDate)
	cooldown := model.FromTime(cooldownUntil)
	for _, p := range f.profiles {
		if p.ID == profileID {
			p.TestStatus = testStatus
			p.LastTestDate = &last
			p.TestCooldownUntil = &cooldown
			score := lastTestScore
			p.LastTestScore = &score
			p.VerifiedGrade = &gradeAfter
		}
	}
	return nil
}

func (f *fakeStore) FindActiveSession(_ context.Context, candidateID string) (*testingmod.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest *model.TestSession
	for _, s := range f.sessions {
		if s.CandidateID == candidateID && s.Status == model.SessionInProgress {
			if latest == nil || s.StartedAt.Time.After(latest.StartedAt.Time) {
				latest = s
			}
		}
	}
	if latest == nil {
		return nil, nil
	}
	cp := *latest
	return &testingmod.Session{TestSession: cp}, nil
}

func (f *fakeStore) FindSessionWithAnswers(_ context.Context, sessionID string) (*testingmod.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sessions {
		if s.ID != sessionID {
			continue
		}
		out := &testingmod.Session{TestSession: *s}
		for _, a := range f.answers {
			if a.SessionID != sessionID {
				continue
			}
			qa := a
			if q, ok := f.questionsByID[a.QuestionID]; ok {
				qa.Question = q
			}
			out.Answers = append(out.Answers, qa)
		}
		sort.SliceStable(out.Answers, func(i, j int) bool {
			return out.Answers[i].AnsweredAt.Time.Before(out.Answers[j].AnsweredAt.Time)
		})
		return out, nil
	}
	return nil, nil
}

func (f *fakeStore) CreateSession(_ context.Context, session *model.TestSession) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *session
	f.sessions = append(f.sessions, &cp)
	return nil
}

func (f *fakeStore) UpdateSession(_ context.Context, session *model.TestSession) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sessions {
		if s.ID == session.ID {
			*s = *session
			return nil
		}
	}
	return nil
}

func (f *fakeStore) CreateAnswer(_ context.Context, answer *model.TestAnswer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.answers {
		if a.SessionID == answer.SessionID && a.QuestionID == answer.QuestionID {
			return testingmod.ErrDuplicateAnswer
		}
	}
	f.answers = append(f.answers, testingmod.SessionAnswer{TestAnswer: *answer})
	return nil
}

// ============ Заглушки зависимостей ============

type stubMatching struct{}

func (stubMatching) InvalidateCache(context.Context) error        { return nil }
func (stubMatching) IndexCandidate(context.Context, string) error { return nil }

// ============ Фикстуры вопросов (QUESTIONS из TS-спека) ============

const categoryID = "cat-1"

func mcqQuestion(id string, difficulty float64) services.PoolQuestion {
	correct := "a"
	explanation := "Пояснение"
	tag := "backend_concept"
	return services.PoolQuestion{
		ID:           id,
		QuestionType: "multiple_choice",
		Text:         "Вопрос " + id,
		Options: json.RawMessage(`[` +
			`{"id":"a","text":"Верный","isCorrect":true},` +
			`{"id":"b","text":"Неверный 1","isCorrect":false},` +
			`{"id":"c","text":"Неверный 2","isCorrect":false},` +
			`{"id":"d","text":"Неверный 3","isCorrect":false}]`),
		CorrectAnswer:  &correct,
		Explanation:    &explanation,
		Difficulty:     difficulty,
		Discrimination: 1.2,
		Guessing:       0.25,
		PoolTag:        &tag,
	}
}

func textQuestion(id string, difficulty float64) services.PoolQuestion {
	correct := "SELECT|JOIN|ON"
	explanation := "Пояснение"
	tag := "backend_practice"
	return services.PoolQuestion{
		ID:             id,
		QuestionType:   "coding",
		Text:           "Практический вопрос " + id,
		Options:        nil, // null — вопрос банка без вариантов
		CorrectAnswer:  &correct,
		Explanation:    &explanation,
		Difficulty:     difficulty,
		Discrimination: 1.3,
		Guessing:       0.1,
		IsUnique:       true,
		PoolTag:        &tag,
	}
}

// testQuestions — 12 MC + 2 текстовых (start() требует пул >= 10).
func testQuestions() []services.PoolQuestion {
	qs := make([]services.PoolQuestion, 0, 14)
	for i := 0; i < 12; i++ {
		id := "q-" + string(rune('0'+(i+1)/10)) + string(rune('0'+(i+1)%10))
		difficulty := float64(int((0.15+float64(i)*0.06)*100+0.5)) / 100
		qs = append(qs, mcqQuestion(id, difficulty))
	}
	return append(qs, textQuestion("q-13", 0.12), textQuestion("q-14", 0.95))
}

// ============ Контекст V2 (аналог createV2Context) ============

type v2Context struct {
	svc   *testingmod.ServiceV2
	store *fakeStore
	spec  model.Specialization
}

func newV2Context(totalTasks int) *v2Context {
	spec := model.Specialization("BACKEND")
	grade := model.GradeMiddle
	store := newFakeStore(
		[]services.Category{{ID: categoryID, Name: "Backend Basic", Specialization: spec}},
		map[string][]services.PoolQuestion{categoryID: testQuestions()},
	)
	store.profiles = []*model.CandidateProfile{{
		ID:             "profile-1",
		UserID:         "user-1",
		DeclaredGrade:  &grade,
		Specialization: &spec,
		TestStatus:     model.TestNotStarted,
	}}
	svc := testingmod.NewServiceV2(store, testingmod.TestingConfig{
		QuestionsPerTest: 20,
		CooldownDays:     7,
		TasksPerTest:     totalTasks,
		V2CooldownDays:   7,
	}, stubMatching{}, nil) // gamification: nil → блок геймификации пропускается
	return &v2Context{svc: svc, store: store, spec: spec}
}

// answerFor — ответ клиента на задачу (MC-фикстуры: 'a' — верный вариант).
func answerFor(task *services.TestTask, correct bool) testingmod.SubmitAnswerDto {
	option := "a"
	if !correct {
		option = "b"
	}
	secs := 10
	return testingmod.SubmitAnswerDto{
		QuestionID:       &task.ID,
		SelectedOptionID: &option,
		TimeSpentSeconds: &secs,
	}
}

func mustStart(t *testing.T, c *v2Context) *testingmod.StartResultV2 {
	t.Helper()
	spec := c.spec
	raw, err := c.svc.Start(context.Background(), "user-1", testingmod.StartTestDto{Specialization: &spec})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	start, ok := raw.(*testingmod.StartResultV2)
	if !ok {
		t.Fatalf("Start вернул %T, ожидался *StartResultV2", raw)
	}
	return start
}

func mustSubmit(t *testing.T, c *v2Context, sessionID string, dto testingmod.SubmitAnswerDto) *testingmod.AnswerResultV2 {
	t.Helper()
	raw, err := c.svc.SubmitAnswer(context.Background(), "user-1", sessionID, dto)
	if err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	res, ok := raw.(*testingmod.AnswerResultV2)
	if !ok {
		t.Fatalf("SubmitAnswer вернул %T, ожидался *AnswerResultV2", raw)
	}
	return res
}

// ============ Тесты ============

func TestV2AnswerFlowNoDuplicateConflict(t *testing.T) {
	t.Run("следующая задача не совпадает с только что отвеченной (корневая причина 409)", func(t *testing.T) {
		c := newV2Context(5)
		start := mustStart(t, c)

		res := mustSubmit(t, c, start.SessionID, answerFor(start.FirstTask, true))

		if res.NextTask == nil {
			t.Fatal("nextTask отсутствует")
		}
		if res.NextTask.ID == start.FirstTask.ID {
			t.Errorf("nextTask = %s, совпадает с отвеченной", res.NextTask.ID)
		}
		if res.Duplicate {
			t.Error("duplicate = true, ожидалось false")
		}
		if len(c.store.answers) != 1 {
			t.Errorf("ответов = %d, ожидался 1", len(c.store.answers))
		}
	})

	t.Run("ответ на nextTask из предыдущего ответа принимается без 409 (сценарий из баг-репорта)", func(t *testing.T) {
		c := newV2Context(5)
		start := mustStart(t, c)

		first := mustSubmit(t, c, start.SessionID, answerFor(start.FirstTask, true))
		// раньше этот шаг падал ConflictException на «втором вопросе»
		second := mustSubmit(t, c, start.SessionID, answerFor(first.NextTask, true))

		if second.Duplicate {
			t.Error("duplicate = true, ожидалось false")
		}
		if second.NextTask == nil {
			t.Fatal("nextTask отсутствует")
		}
		if second.NextTask.ID == first.NextTask.ID {
			t.Errorf("nextTask = %s совпадает с предыдущим", second.NextTask.ID)
		}
		if len(c.store.answers) != 2 {
			t.Errorf("ответов = %d, ожидалось 2", len(c.store.answers))
		}
		seen := map[string]bool{}
		for _, a := range c.store.answers {
			seen[a.QuestionID] = true
		}
		if len(seen) != 2 {
			t.Errorf("уникальных questionId = %d, ожидалось 2", len(seen))
		}
	})

	t.Run("повторная отправка ответа идемпотентна и не блокирует тест", func(t *testing.T) {
		c := newV2Context(5)
		start := mustStart(t, c)

		first := mustSubmit(t, c, start.SessionID, answerFor(start.FirstTask, true))
		repeat := mustSubmit(t, c, start.SessionID, answerFor(start.FirstTask, true))

		if !repeat.Duplicate {
			t.Error("duplicate = false, ожидалось true")
		}
		if repeat.Correct != first.Correct {
			t.Errorf("correct = %v, ожидалось %v", repeat.Correct, first.Correct)
		}
		if repeat.Score != first.Score {
			t.Errorf("score = %v, ожидалось %v", repeat.Score, first.Score)
		}
		if repeat.TasksRemaining != first.TasksRemaining {
			t.Errorf("tasksRemaining = %d, ожидалось %d", repeat.TasksRemaining, first.TasksRemaining)
		}
		if first.NextTask == nil || repeat.NextTask == nil || repeat.NextTask.ID != first.NextTask.ID {
			t.Errorf("nextTask повтора не совпадает: %v vs %v", repeat.NextTask, first.NextTask)
		}
		if len(c.store.answers) != 1 {
			t.Errorf("ответов = %d, ожидался 1", len(c.store.answers))
		}

		// тест продолжается, а не застревает на 409
		next := mustSubmit(t, c, start.SessionID, answerFor(repeat.NextTask, true))
		if next.Duplicate {
			t.Error("duplicate = true, ожидалось false")
		}
		if len(c.store.answers) != 2 {
			t.Errorf("ответов = %d, ожидалось 2", len(c.store.answers))
		}
	})

	t.Run("повторная отправка не перезаписывает записанный ответ (первый выигрывает)", func(t *testing.T) {
		c := newV2Context(5)
		start := mustStart(t, c)

		mustSubmit(t, c, start.SessionID, answerFor(start.FirstTask, true))
		repeat := mustSubmit(t, c, start.SessionID, answerFor(start.FirstTask, false))

		if !repeat.Correct {
			t.Error("correct = false, ожидался результат первой записи (true)")
		}
		if len(c.store.answers) != 1 {
			t.Errorf("ответов = %d, ожидался 1", len(c.store.answers))
		}
		if stored := c.store.answers[0]; stored.IsCorrect == nil || !*stored.IsCorrect {
			t.Errorf("записанный isCorrect = %v, ожидалось true", stored.IsCorrect)
		}
	})

	t.Run("currentTask не возвращает отвеченную задачу", func(t *testing.T) {
		c := newV2Context(5)
		start := mustStart(t, c)
		mustSubmit(t, c, start.SessionID, answerFor(start.FirstTask, true))

		raw, err := c.svc.CurrentTask(context.Background(), "user-1", start.SessionID)
		if err != nil {
			t.Fatalf("CurrentTask: %v", err)
		}
		current, ok := raw.(*testingmod.CurrentResultV2)
		if !ok {
			t.Fatalf("CurrentTask вернул %T, ожидался *CurrentResultV2", raw)
		}
		if current.Task == nil {
			t.Fatal("task отсутствует")
		}
		if current.Task.ID == start.FirstTask.ID {
			t.Errorf("task = %s, отвеченная задача возвращена повторно", current.Task.ID)
		}
		if current.TaskNumber != 2 {
			t.Errorf("taskNumber = %d, ожидалось 2", current.TaskNumber)
		}
	})

	t.Run("selectedOptionId учитывается при оценке multiple_choice", func(t *testing.T) {
		c := newV2Context(5)
		start := mustStart(t, c)

		correct := mustSubmit(t, c, start.SessionID, answerFor(start.FirstTask, true))
		if !correct.Correct {
			t.Error("correct = false, ожидалось true")
		}
		if correct.Score != 1 {
			t.Errorf("score = %v, ожидалось 1", correct.Score)
		}

		wrong := mustSubmit(t, c, start.SessionID, answerFor(correct.NextTask, false))
		if wrong.Correct {
			t.Error("correct = true, ожидалось false")
		}
		if wrong.Score != 0 {
			t.Errorf("score = %v, ожидалось 0", wrong.Score)
		}
	})

	t.Run("ответ оценивается по формату показанной задачи, а не по типу исходного вопроса", func(t *testing.T) {
		c := newV2Context(5)
		start := mustStart(t, c)
		task := start.FirstTask
		if task == nil {
			t.Fatal("firstTask отсутствует")
		}

		// Викторина показана не как multiple_choice: клиент заполняет поля своего формата,
		// оценщик не должен обнулять их «оценщиком вариантов».
		if string(task.Type) == "multiple_choice" {
			if task.MultipleChoice == nil || len(task.MultipleChoice.Options) == 0 {
				t.Error("multipleChoice.options пуст у MC-задачи")
			}
			return
		}

		dto := testingmod.SubmitAnswerDto{QuestionID: &task.ID}
		secs := 10
		dto.TimeSpentSeconds = &secs
		answerText := "Развёрнутый ответ по существу задачи"
		switch string(task.Type) {
		case "code_execution":
			code := "function solve() { return 42; }"
			dto.Code = &code
		case "system_design":
			dto.DesignDescription = &answerText
		case "code_review":
			fixed := "исправленный код"
			dto.FixedCode = &fixed
		default:
			dto.AnswerText = &answerText
		}

		res := mustSubmit(t, c, start.SessionID, dto)
		if res.Duplicate {
			t.Error("duplicate = true, ожидалось false")
		}
		// TS-спек фиксировал score>0 (там песочница/LLM были замоканы и всегда
		// отдавали >0); в Go оценка реальная — фиксируем, что грейдер формата
		// задачи выбран (ответ не отклонён и результат записан).
		if len(c.store.answers) != 1 {
			t.Errorf("ответов = %d, ожидался 1", len(c.store.answers))
		}
	})
}

func TestV2TaskFromQuestionWithoutOptionsIsNotNull(t *testing.T) {
	generator := services.NewTaskGenerator()
	question := services.TaskQuestion{
		ID:           "text-q",
		QuestionType: "coding",
		Text:         "Практический вопрос без вариантов",
		Difficulty:   0.5,
	}
	for index := 0; index < 20; index++ {
		task := generator.CreateTaskFromQuestion(question, model.GradeMiddle, index)
		if task == nil {
			t.Fatalf("задача = nil на позиции %d (раньше на multiple_choice-слотах возвращался null)", index)
		}
	}
}

func TestV2NextTaskNeverMissingMidTest(t *testing.T) {
	c := newV2Context(5)
	start := mustStart(t, c)

	task := start.FirstTask
	for step := 0; step < 20; step++ {
		if task == nil {
			t.Fatalf("задача отсутствует на шаге %d", step)
		}
		res := mustSubmit(t, c, start.SessionID, answerFor(task, true))
		if res.IsFinished || res.TasksRemaining == 0 {
			break
		}
		if res.NextTask == nil {
			t.Fatalf("nextTask = nil на шаге %d, пока тест не завершён", step)
		}
		task = res.NextTask
	}
}

func TestV2FullFrontendFlow(t *testing.T) {
	c := newV2Context(5)
	start := mustStart(t, c)

	seen := map[string]bool{}
	task := start.FirstTask
	for step := 0; step < 20; step++ {
		if task == nil {
			t.Fatalf("задача отсутствует на шаге %d", step)
		}
		if seen[task.ID] {
			t.Fatalf("задача %s повторилась", task.ID)
		}
		seen[task.ID] = true
		res := mustSubmit(t, c, start.SessionID, answerFor(task, true))
		if res.Duplicate {
			t.Fatalf("duplicate = true на шаге %d", step)
		}
		if res.NextTask == nil {
			break
		}
		task = res.NextTask
	}

	if len(seen) != 5 {
		t.Errorf("уникальных задач = %d, ожидалось 5 (TESTING_TASKS_PER_TEST = 5)", len(seen))
	}
	if len(c.store.answers) != 5 {
		t.Errorf("ответов = %d, ожидалось 5", len(c.store.answers))
	}

	raw, err := c.svc.Complete(context.Background(), "user-1", start.SessionID)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	result, ok := raw.(*testingmod.TestResultV2)
	if !ok {
		t.Fatalf("Complete вернул %T, ожидался *TestResultV2", raw)
	}
	switch result.Recommendation {
	case "PASS", "FAIL", "UPGRADE":
	default:
		t.Errorf("recommendation = %q, ожидался PASS|FAIL|UPGRADE", result.Recommendation)
	}
	if result.Status != model.SessionCompleted {
		t.Errorf("status = %s, ожидался COMPLETED", result.Status)
	}
}
