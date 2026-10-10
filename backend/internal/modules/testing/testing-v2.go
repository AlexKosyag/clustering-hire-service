// V2 — Testing Engine V2 (backend/src/modules/testing/testing-v2.service.ts):
// мультиформатные задания (multiple_choice, code_execution, llm_open,
// code_review, system_design), поведенческий анти-чит, песочница кода,
// LLM-оценка и геймификация. docs 06 §4–§6, docs 04 §7 «V2».
package testing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
	"fsp-backend/internal/modules/gamification"
	"fsp-backend/internal/modules/testing/services"
)

// ============ V2 Types (TS: TaskAnswer/TaskResult/TestResultV2) ============

// TaskResult — детальный результат одного задания (TS TaskResult).
//
// В HTTP-ответе complete() TS возвращает АГРЕГАТ по типам задания
// (calculateTaskBreakdown → Record<тип, {total, passed, avgScore}>), а не
// список TaskResult; тип сохранён для паритета с TS-интерфейсом.
type TaskResult struct {
	TaskID    string  `json:"taskId"`
	Type      string  `json:"type"`
	Score     float64 `json:"score"`
	MaxScore  float64 `json:"maxScore"`
	IsCorrect *bool   `json:"isCorrect"`

	CodeExecution *struct {
		PassedTests int     `json:"passedTests"`
		TotalTests  int     `json:"totalTests"`
		Score       float64 `json:"score"`
	} `json:"codeExecution,omitempty"`

	LLMEvaluation *struct {
		Score      float64            `json:"score"`
		Breakdown  map[string]float64 `json:"breakdown"`
		Strengths  []string           `json:"strengths"`
		Weaknesses []string           `json:"weaknesses"`
		Feedback   string             `json:"feedback"`
	} `json:"llmEvaluation,omitempty"`

	AntiCheat *struct {
		SuspicionScore float64  `json:"suspicionScore"`
		Flags          []string `json:"flags"`
	} `json:"antiCheat,omitempty"`
}

// TaskBreakdownEntry — агрегат по типу задания {total, passed, avgScore}.
type TaskBreakdownEntry struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	AvgScore int `json:"avgScore"`
}

// TaskBreakdown — Record<string, {total, passed, avgScore}> с сохранением
// порядка первого появления типа (Object.entries в JS идёт в порядке
// вставки; обычная map в Go сериализовалась бы в алфавитном порядке).
type TaskBreakdown struct {
	keys  []string
	items map[string]TaskBreakdownEntry
}

func newTaskBreakdown() TaskBreakdown {
	return TaskBreakdown{keys: []string{}, items: map[string]TaskBreakdownEntry{}}
}

// MarshalJSON — объект с ключами в порядке первого появления; пустой → {}.
func (b TaskBreakdown) MarshalJSON() ([]byte, error) {
	var sb strings.Builder
	sb.WriteByte('{')
	for i, key := range b.keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		k, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(b.items[key])
		if err != nil {
			return nil, err
		}
		sb.Write(k)
		sb.WriteByte(':')
		sb.Write(v)
	}
	sb.WriteByte('}')
	return []byte(sb.String()), nil
}

// AntiCheatV2 — сводка анти-чита в TestResultV2 (docs 06 §6.2).
type AntiCheatV2 struct {
	IsSuspicious       bool                                 `json:"isSuspicious"`
	Flags              []string                             `json:"flags"`
	Confidence         float64                              `json:"confidence"`
	BehavioralAnalysis map[string]services.BehavioralDetail `json:"behavioralAnalysis"`
}

// GamificationSummary — блок gamification в ответе complete (TS: 4 поля из getProfile).
type GamificationSummary struct {
	Xp        int    `json:"xp"`
	Level     int    `json:"level"`
	LevelName string `json:"levelName"`
	Streak    int    `json:"streak"`
}

// TestResultV2 — ответ POST /testing/v2/session/:id/complete (TS TestResultV2).
type TestResultV2 struct {
	SessionID      string               `json:"sessionId"`
	Status         model.SessionStatus  `json:"status"`
	Score          float64              `json:"score"`
	GradeBefore    *model.Grade         `json:"gradeBefore"`
	GradeAfter     model.Grade          `json:"gradeAfter"`
	Recommendation string               `json:"recommendation"`
	Theta          float64              `json:"theta"`
	TaskBreakdown  TaskBreakdown        `json:"taskBreakdown"`
	AntiCheat      AntiCheatV2          `json:"antiCheat"`
	CooldownUntil  model.Time           `json:"cooldownUntil"`
	Gamification   *GamificationSummary `json:"gamification"` // null при ошибке геймификации
}

// StartResultV2 — ответ POST /testing/v2/start.
type StartResultV2 struct {
	SessionID        string                    `json:"sessionId"`
	TotalTasks       int                       `json:"totalTasks"`
	TimeLimitMinutes int                       `json:"timeLimitMinutes"`
	Specialization   model.Specialization      `json:"specialization"`
	GradeBefore      *model.Grade              `json:"gradeBefore"`
	TaskDistribution services.TaskDistribution `json:"taskDistribution"`
	FirstTask        *services.TestTask        `json:"firstTask"`
}

// CurrentResultV2 — GET /testing/v2/session/:id/current (сессия идёт).
type CurrentResultV2 struct {
	SessionID            string              `json:"sessionId"`
	Status               model.SessionStatus `json:"status"`
	TaskNumber           int                 `json:"taskNumber"`
	TotalTasks           int                 `json:"totalTasks"`
	CurrentScore         float64             `json:"currentScore"`
	TimeRemainingSeconds int64               `json:"timeRemainingSeconds"`
	IsFinished           bool                `json:"isFinished"`
	Task                 *services.TestTask  `json:"task"`
}

// CurrentFinishedResultV2 — GET /testing/v2/session/:id/current (сессия не идёт).
type CurrentFinishedResultV2 struct {
	SessionID  string              `json:"sessionId"`
	Status     model.SessionStatus `json:"status"`
	Task       *services.TestTask  `json:"task"`
	IsFinished bool                `json:"isFinished"`
}

// AnswerResultV2 — ответ POST /testing/v2/session/:id/answer (в т.ч. duplicate).
type AnswerResultV2 struct {
	TaskID          string             `json:"taskId"`
	Correct         bool               `json:"correct"`
	Score           float64            `json:"score"`
	Explanation     *string            `json:"explanation"`
	CurrentScore    float64            `json:"currentScore"`
	Theta           float64            `json:"theta"`
	TasksRemaining  int                `json:"tasksRemaining"`
	IsFinished      bool               `json:"isFinished"`
	BehavioralFlags []string           `json:"behavioralFlags"`
	NextTask        *services.TestTask `json:"nextTask"`
	Duplicate       bool               `json:"duplicate"`
}

// ============ Service V2 ============

// ServiceV2 — движок V2 (основной): 15 мультиформатных заданий, 90 минут.
//
// Отличия V1 → V2 (docs 06 §1):
//   - минимум банка 10 вопросов (V1 — 20), лимит времени 90 минут;
//   - повторная отправка ответа идемпотентна (duplicate: true вместо 409);
//   - выбор грейдера по присланным полям ответа + поведенческий анти-чит;
//   - complete() дополнительно пишет геймификацию (XP/бейджи/стрик).
type ServiceV2 struct {
	store         Store
	pool          *services.QuestionPool
	cat           *services.CAT
	scoring       *services.Scoring
	antiCheat     *services.AntiCheat
	behavioral    *services.BehavioralAntiCheat
	codeSandbox   *services.CodeSandbox
	llm           *services.LLMEvaluation
	taskGenerator *services.TaskGenerator
	matching      MatchingAPI
	gamification  *gamification.Service
	totalTasks    int
	cooldownDays  int
}

// NewServiceV2 — конструктор V2 для интегратора.
//
// BUG-REF: T5 — V2 читает размер теста/кулдауна из env-дублей: config.Config
// (TestingTasksPerTest/TestingV2CooldownDays) уже дублирует переменные
// TESTING_TASKS_PER_TEST/TESTING_COOLDOWN_DAYS, здесь они читаются ещё раз
// (как process.env в testing-v2.service.ts), единой конфигурации нет.
// Порог: cfg > env > default (15 / 90), т.е. при env=... поведение совпадает с TS.
func NewServiceV2(store Store, cfg TestingConfig, matching MatchingAPI, gam *gamification.Service) *ServiceV2 {
	return &ServiceV2{
		store:   store,
		pool:    services.NewQuestionPool(store),
		cat:     services.NewCAT(),
		scoring: services.NewScoring(),

		antiCheat:  services.NewAntiCheat(),
		behavioral: services.NewBehavioralAntiCheat(),

		// Песочница и LLM — конфигурация только из process.env (как в TS).
		// BUG-REF: T3 — SANDBOX_DOCKER/SANDBOX_MEMORY_MB читаются, но Docker-режим
		// и лимит памяти не реализованы (см. services/code-sandbox.go).
		codeSandbox: services.NewCodeSandbox(
			time.Duration(envIntOr("SANDBOX_TIMEOUT_MS", 10000))*time.Millisecond,
			envIntOr("SANDBOX_MEMORY_MB", 256),
			envBoolOr("SANDBOX_DOCKER", false),
		),
		llm: services.NewLLMEvaluation(
			os.Getenv("OPENAI_API_KEY"),
			envStrOr("OPENAI_API_URL", "https://api.openai.com/v1/chat/completions"),
			envStrOr("OPENAI_MODEL", "gpt-4o-mini"),
		),

		taskGenerator: services.NewTaskGenerator(),
		matching:      matching,
		gamification:  gam,

		totalTasks:   cfgOrEnv(cfg.TasksPerTest, "TESTING_TASKS_PER_TEST", 15),
		cooldownDays: cfgOrEnv(cfg.V2CooldownDays, "TESTING_COOLDOWN_DAYS", 90),
	}
}

// Start — начать тест V2 с мультиформатными заданиями.
func (s *ServiceV2) Start(ctx context.Context, userID string, dto StartTestDto) (any, error) {
	profile, err := s.profileOf(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Check cooldown
	if profile.TestCooldownUntil != nil && profile.TestCooldownUntil.Time.After(time.Now()) {
		return nil, common.BadRequest("Пересдача возможна не раньше " + isoString(profile.TestCooldownUntil))
	}

	// Check for active session
	active, err := s.store.FindActiveSession(ctx, profile.ID)
	if err != nil {
		return nil, err
	}
	if active != nil {
		if !s.isExpired(active) {
			return nil, common.Conflict("Тестирование уже начато")
		}
		// Просроченная сессия помечается TIMED_OUT (BUG-REF: T7 — кулдаун не ставится)
		active.Status = model.SessionTimedOut
		now := model.Now()
		active.CompletedAt = &now
		if err := s.store.UpdateSession(ctx, &active.TestSession); err != nil {
			return nil, err
		}
	}

	if dto.Specialization == nil {
		return nil, common.BadRequest(common.MsgIsIn("specialization", specializationValues))
	}

	// Get category
	category, err := s.pool.CategoryForSpecialization(ctx, *dto.Specialization)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, common.BadRequest("Для этой специализации нет теста")
	}

	// Get pool (порог V2 — 10 вопросов, docs 06 §1)
	poolQuestions, err := s.pool.PoolFor(ctx, category.ID)
	if err != nil {
		return nil, err
	}
	if len(poolQuestions) < 10 {
		return nil, common.BadRequest("Банк вопросов неполный — тест временно недоступен")
	}

	declared := dto.DeclaredGrade
	if declared == nil {
		declared = profile.DeclaredGrade
	}

	// Create session (V2: 90 минут — под мультиформатные задания)
	session := &model.TestSession{
		ID:                newID(),
		CandidateID:       profile.ID,
		CategoryID:        category.ID,
		Status:            model.SessionInProgress,
		StartedAt:         model.Now(),
		TimeLimitMinutes:  90,
		GradeBefore:       declared,
		CheatFlags:        []string{},
		CurrentDifficulty: 0.5,
	}
	if err := s.store.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	if err := s.store.SetProfileTestStatus(ctx, profile.ID, model.TestInProgress); err != nil {
		return nil, err
	}
	_ = safeCall(func() error { return s.matching.InvalidateCache(ctx) }) // .catch(() => undefined)

	// V2: Generate first task
	firstTask := s.generateTask(poolQuestions, profile, session.ID, 0, nil)

	gradeForDistribution := model.GradeMiddle
	if declared != nil {
		gradeForDistribution = *declared
	}

	return &StartResultV2{
		SessionID:        session.ID,
		TotalTasks:       s.totalTasks,
		TimeLimitMinutes: session.TimeLimitMinutes,
		Specialization:   category.Specialization,
		GradeBefore:      declared,
		TaskDistribution: s.taskGenerator.GetDistribution(gradeForDistribution),
		FirstTask:        firstTask,
	}, nil
}

// CurrentTask — текущее задание V2 (восстановление после перезагрузки страницы).
func (s *ServiceV2) CurrentTask(ctx context.Context, userID, sessionID string) (any, error) {
	session, profile, err := s.loadOwned(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	if session.Status != model.SessionInProgress {
		return &CurrentFinishedResultV2{SessionID: session.ID, Status: session.Status, Task: nil, IsFinished: true}, nil
	}
	if s.isExpired(session) {
		session.Status = model.SessionTimedOut
		now := model.Now()
		session.CompletedAt = &now
		if err := s.store.UpdateSession(ctx, &session.TestSession); err != nil {
			return nil, err
		}
		return &CurrentFinishedResultV2{SessionID: session.ID, Status: model.SessionTimedOut, Task: nil, IsFinished: true}, nil
	}

	answered := session.Answers
	poolQuestions, err := s.pool.PoolFor(ctx, session.CategoryID)
	if err != nil {
		return nil, err
	}
	taskIndex := len(answered)
	task := s.generateTask(poolQuestions, profile, session.ID, taskIndex, answered)

	return &CurrentResultV2{
		SessionID:            session.ID,
		Status:               session.Status,
		TaskNumber:           taskIndex + 1,
		TotalTasks:           s.totalTasks,
		CurrentScore:         s.scoring.ScoreFromAnswers(toStats(answered)),
		TimeRemainingSeconds: s.timeRemaining(session),
		IsFinished:           len(answered) >= s.totalTasks,
		Task:                 task,
	}, nil
}

// SubmitAnswer — отправить ответ V2 (с поведенческой телеметрией).
//
// Повторная отправка уже отвеченного вопроса идемпотентна: возвращается уже
// записанный результат с duplicate: true вместо блокирующего 409 (docs 06 §1).
func (s *ServiceV2) SubmitAnswer(ctx context.Context, userID, sessionID string, dto SubmitAnswerDto) (any, error) {
	session, profile, err := s.loadOwned(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	if session.Status != model.SessionInProgress {
		return nil, common.BadRequest("Сессия тестирования уже завершена")
	}
	if s.isExpired(session) {
		return nil, common.BadRequest("Время тестирования истекло")
	}

	questionID := ""
	if dto.QuestionID != nil {
		questionID = *dto.QuestionID
	}

	// Идемпотентность повторной отправки (двойной клик, ретрай после потерянного
	// ответа, вторая вкладка): возвращаем уже записанный результат и следующую
	// задачу вместо блокирующего 409.
	for i := range session.Answers {
		if session.Answers[i].QuestionID == questionID {
			poolQuestions, err := s.pool.PoolFor(ctx, session.CategoryID)
			if err != nil {
				return nil, err
			}
			return s.duplicateAnswerView(session, profile, session.Answers[i], poolQuestions), nil
		}
	}

	poolQuestions, err := s.pool.PoolFor(ctx, session.CategoryID)
	if err != nil {
		return nil, err
	}
	var question *services.PoolQuestion
	for i := range poolQuestions {
		if poolQuestions[i].ID == questionID {
			question = &poolQuestions[i]
			break
		}
	}
	if question == nil {
		return nil, common.NotFound("Вопрос не найден в этом тесте")
	}

	// Grade the answer based on the fields the client actually sent
	graded, err := s.gradeAnswerV2(ctx, *question, dto)
	if err != nil {
		return nil, err
	}

	// V2: Behavioral anti-cheat analysis (флаги возвращаются, но НЕ персистятся)
	behavioralFlags := []string{}
	if dto.Behavioral != nil {
		answerLen := 0
		if dto.AnswerText != nil {
			answerLen = services.JSStringLen(*dto.AnswerText)
		}
		analysis := s.behavioral.AnalyzeQuestion(parseBehavioral(dto.Behavioral), question.Difficulty, answerLen)
		behavioralFlags = analysis.Flags
	}

	// Save answer
	answer := &model.TestAnswer{
		ID:               newID(),
		SessionID:        session.ID,
		QuestionID:       question.ID,
		AnswerText:       graded.answerText,
		IsCorrect:        boolPtr(graded.isCorrect),
		Score:            floatPtr(graded.score),
		TimeSpentSeconds: dto.TimeSpentSeconds,
		AnsweredAt:       model.Now(),
	}
	if err := s.store.CreateAnswer(ctx, answer); err != nil {
		if errorsIsDuplicate(err) {
			// Гонка параллельных запросов (уникальный индекс сработал раньше
			// проверки выше) — отвечаем идемпотентно, как и при duplicate.
			updatedSession, updErr := s.store.FindSessionWithAnswers(ctx, session.ID)
			if updErr != nil {
				return nil, updErr
			}
			var recorded SessionAnswer
			for i := range updatedSession.Answers {
				if updatedSession.Answers[i].QuestionID == questionID {
					recorded = updatedSession.Answers[i]
					break
				}
			}
			return s.duplicateAnswerView(updatedSession, profile, recorded, poolQuestions), nil
		}
		return nil, err
	}

	updated, err := s.store.FindSessionWithAnswers(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	answered := updated.Answers
	irt := toIrtV2(answered)
	theta := s.cat.EstimateTheta(irt)
	remaining := s.totalTasks - len(answered)
	if remaining < 0 {
		remaining = 0
	}

	var nextTask *services.TestTask
	if remaining > 0 {
		nextTask = s.generateTask(poolQuestions, profile, session.ID, len(answered), answered)
	}

	updated.ThetaEstimate = &theta
	updated.CurrentDifficulty = 1
	if nextTask != nil {
		updated.CurrentDifficulty = nextTask.Difficulty
	}
	if err := s.store.UpdateSession(ctx, &updated.TestSession); err != nil {
		return nil, err
	}

	return &AnswerResultV2{
		TaskID:          question.ID,
		Correct:         graded.isCorrect,
		Score:           round2(graded.score),
		Explanation:     orNull(question.Explanation),
		CurrentScore:    s.scoring.ScoreFromAnswers(toStats(answered)),
		Theta:           round3(theta),
		TasksRemaining:  remaining,
		IsFinished:      remaining == 0,
		BehavioralFlags: behavioralFlags,
		NextTask:        nextTask,
		Duplicate:       false,
	}, nil
}

// duplicateAnswerView — повторная отправка уже отвеченного вопроса:
// возвращаем записанный результат и актуальное состояние сессии, чтобы не
// блокировать прохождение теста (duplicate: true).
func (s *ServiceV2) duplicateAnswerView(session *Session, profile *model.CandidateProfile, recorded SessionAnswer, poolQuestions []services.PoolQuestion) *AnswerResultV2 {
	var question *services.PoolQuestion
	for i := range poolQuestions {
		if poolQuestions[i].ID == recorded.QuestionID {
			question = &poolQuestions[i]
			break
		}
	}

	answered := session.Answers
	irt := toIrtV2(answered)
	theta := s.cat.EstimateTheta(irt)
	remaining := s.totalTasks - len(answered)
	if remaining < 0 {
		remaining = 0
	}

	var nextTask *services.TestTask
	if remaining > 0 {
		nextTask = s.generateTask(poolQuestions, profile, session.ID, len(answered), answered)
	}

	var explanation *string
	if question != nil {
		explanation = orNull(question.Explanation)
	}
	score := 0.0
	if recorded.Score != nil {
		score = *recorded.Score
	}

	return &AnswerResultV2{
		TaskID:          recorded.QuestionID,
		Correct:         recorded.IsCorrect != nil && *recorded.IsCorrect,
		Score:           round2(score),
		Explanation:     explanation,
		CurrentScore:    s.scoring.ScoreFromAnswers(toStats(answered)),
		Theta:           round3(theta),
		TasksRemaining:  remaining,
		IsFinished:      remaining == 0,
		BehavioralFlags: []string{},
		NextTask:        nextTask,
		Duplicate:       true,
	}
}

// Complete — завершить тест V2: итог, поведенческий анти-чит, геймификация.
func (s *ServiceV2) Complete(ctx context.Context, userID, sessionID string) (any, error) {
	session, profile, err := s.loadOwned(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	if session.Status == model.SessionCompleted {
		return s.resultViewV2(session), nil
	}
	if session.Status != model.SessionInProgress {
		return nil, common.BadRequest("Сессию нельзя завершить")
	}

	expired := s.isExpired(session)
	if len(session.Answers) < s.totalTasks && !expired {
		return nil, common.BadRequest(fmt.Sprintf("Осталось неотвеченных задач: %d", s.totalTasks-len(session.Answers)))
	}

	answered := session.Answers
	irt := toIrtV2(answered)
	stats := toStats(answered)

	theta := s.cat.EstimateTheta(irt)
	score := s.scoring.ScoreFromAnswers(stats)
	declaredGrade := session.GradeBefore // session.gradeBefore ?? profile.declaredGrade
	if declaredGrade == nil {
		declaredGrade = profile.DeclaredGrade
	}
	gradeAfter := s.scoring.CalculateGrade(theta, profile.Specialization)
	recommendation := s.scoring.GetRecommendation(declaredGrade, gradeAfter, score)

	// V1 anti-cheat (статистический)
	history := s.antiCheat.ThetaHistory(irt, s.cat.EstimateTheta)
	timed := make([]services.TimedAnswer, 0, len(answered))
	for _, a := range answered {
		timed = append(timed, services.TimedAnswer{
			TimeSpentSeconds: a.TimeSpentSeconds,
			Difficulty:       a.Question.Difficulty,
			PoolTag:          a.Question.PoolTag,
			IsCorrect:        a.IsCorrect != nil && *a.IsCorrect,
		})
	}
	analysis := s.antiCheat.Analyze(timed, history)
	// BUG-REF: T4 — cheatFlags/isSuspicious не влияют на score/грейд/рекомендацию.

	// V2: Behavioral anti-cheat (aggregate)
	//
	// BUG-REF: T1 — поведенческая телеметрия НЕ персистится в test_answers:
	// на complete() она пересчитывается из сохранённых ответов с нулевыми
	// значениями («Would come from client»), т.е. сессионный анализ фактически
	// видит только тайминги.
	behavioralAnalyses := make([]services.BehavioralAnalysis, 0, len(answered))
	for _, a := range answered {
		ts := 0.0
		if a.TimeSpentSeconds != nil {
			ts = float64(*a.TimeSpentSeconds)
		}
		zero := 0.0
		answerLen := 0
		if a.AnswerText != nil {
			answerLen = services.JSStringLen(*a.AnswerText)
		}
		behavioralAnalyses = append(behavioralAnalyses, s.behavioral.AnalyzeQuestion(
			services.BehavioralData{
				TimeSpentSeconds: &ts,
				CopyPasteEvents:  &zero,
				TabSwitches:      &zero,
				KeystrokeCount:   &zero,
				IdleTimeSeconds:  &zero,
			},
			a.Question.Difficulty,
			answerLen,
		))
	}
	behavioralSession := s.behavioral.AnalyzeSession(behavioralAnalyses, history)

	// Merge flags ([...new Set([...a, ...b])])
	allFlags := uniqueFlags(analysis.Flags, behavioralSession.Flags)
	isSuspicious := analysis.IsSuspicious || behavioralSession.SuspicionScore > 50

	cooldownUntil := model.FromTime(time.Now().Add(time.Duration(s.cooldownDays) * 24 * time.Hour))
	status := model.SessionCompleted
	if expired {
		status = model.SessionTimedOut
	}
	testStatus := model.TestPassed
	if recommendation == services.RecFail {
		testStatus = model.TestFailed
	} else if recommendation == services.RecUpgrade {
		testStatus = model.TestUpgraded
	}

	// Update session
	session.Status = status
	now := model.Now()
	session.CompletedAt = &now
	session.Score = &score
	session.RawScore = &score
	maxScore := 100.0
	session.MaxScore = &maxScore
	session.GradeAfter = &gradeAfter
	rec := string(recommendation)
	session.Recommendation = &rec
	session.ThetaEstimate = &theta
	session.CheatFlags = allFlags
	session.IsSuspicious = isSuspicious
	if err := s.store.UpdateSession(ctx, &session.TestSession); err != nil {
		return nil, err
	}

	// Update profile: verifiedGrade, testStatus, lastTestDate, cooldown, score
	if err := s.store.SetProfileTestResult(ctx, profile.ID, gradeAfter, testStatus, now.Time, cooldownUntil.Time, score); err != nil {
		return nil, err
	}

	// Сайд-эффекты: matching-кэш и переиндексация (ошибки не валят тест)
	_ = safeCall(func() error { return s.matching.InvalidateCache(ctx) }) // .catch(() => undefined)
	if err := safeCall(func() error { return s.matching.IndexCandidate(ctx, profile.ID) }); err != nil {
		log.Printf("[TestingV2] Indexing: %s", err.Error())
	}

	// V2: Gamification — award XP and badges (ошибки не валят тест)
	var gamificationResult *GamificationSummary
	if s.gamification != nil {
		summary, err := s.awardResults(ctx, profile.ID, score, recommendation, gradeAfter)
		if err != nil {
			log.Printf("[TestingV2] Gamification failed: %s", err.Error())
		} else {
			gamificationResult = summary
		}
	}

	return &TestResultV2{
		SessionID:      session.ID,
		Status:         status,
		Score:          score,
		GradeBefore:    declaredGrade,
		GradeAfter:     gradeAfter,
		Recommendation: rec,
		Theta:          round3(theta),
		TaskBreakdown:  s.calculateTaskBreakdown(answered),
		AntiCheat: AntiCheatV2{
			IsSuspicious:       isSuspicious,
			Flags:              allFlags,
			Confidence:         math.Max(analysis.Confidence, behavioralSession.Confidence),
			BehavioralAnalysis: behavioralSession.Details,
		},
		CooldownUntil: cooldownUntil,
		Gamification:  gamificationResult,
	}, nil
}

// awardResults — блок геймификации в complete() (TS: один try/catch вокруг
// всех вызовов — первая ошибка прерывает остальные и обнуляет результат).
func (s *ServiceV2) awardResults(ctx context.Context, profileID string, score float64, recommendation services.Recommendation, gradeAfter model.Grade) (*GamificationSummary, error) {
	if err := s.gamification.UpdateStreak(ctx, profileID); err != nil {
		return nil, err
	}

	// Award XP based on result
	if err := s.gamification.AwardXp(ctx, profileID, "TEST_COMPLETED", map[string]any{
		"score":          score,
		"recommendation": string(recommendation),
	}); err != nil {
		return nil, err
	}

	if recommendation == services.RecPass {
		if err := s.gamification.AwardXp(ctx, profileID, "TEST_PASSED", map[string]any{"score": score}); err != nil {
			return nil, err
		}
		if err := s.gamification.AwardBadge(ctx, profileID, "TEST_PASSED"); err != nil {
			return nil, err
		}
	} else if recommendation == services.RecUpgrade {
		if err := s.gamification.AwardXp(ctx, profileID, "TEST_UPGRADED", map[string]any{
			"score":    score,
			"newGrade": string(gradeAfter),
		}); err != nil {
			return nil, err
		}
		if err := s.gamification.AwardBadge(ctx, profileID, "TEST_PASSED"); err != nil {
			return nil, err
		}
	}

	// Award first test badge
	if err := s.gamification.AwardBadge(ctx, profileID, "FIRST_TEST"); err != nil {
		return nil, err
	}

	// Award grade badges
	if gradeBadge, ok := gradeBadgeMap[string(gradeAfter)]; ok {
		if err := s.gamification.AwardBadge(ctx, profileID, gradeBadge); err != nil {
			return nil, err
		}
	}

	profile, err := s.gamification.GetProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}
	return gamificationSummary(profile), nil
}

// gradeBadgeMap — GRADE_<gradeAfter> (TS: record gradeBadgeMap).
var gradeBadgeMap = map[string]string{
	"JUNIOR":      "GRADE_JUNIOR",
	"MIDDLE":      "GRADE_MIDDLE",
	"MIDDLE_PLUS": "GRADE_MIDDLE_PLUS",
	"SENIOR":      "GRADE_SENIOR",
	"LEAD":        "GRADE_LEAD",
	"PRINCIPAL":   "GRADE_PRINCIPAL",
}

// gamificationSummary — 4 поля из gamification-профиля (xp/level/levelName/streak).
func gamificationSummary(profile map[string]any) *GamificationSummary {
	out := &GamificationSummary{}
	out.Xp = intNumber(profile["xp"])
	out.Level = intNumber(profile["level"])
	out.Streak = intNumber(profile["streak"])
	if name, ok := profile["levelName"].(string); ok {
		out.LevelName = name
	}
	return out
}

// intNumber — int/float64 из map[string]any → int.
func intNumber(v any) int {
	switch typed := v.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	}
	return 0
}

// ============ V2 Helpers ============

// generateTask — задание из вопроса банка: CAT выбирает следующий вопрос
// (исключая уже отвеченные), генератор превращает его в мультиформатное задание.
//
// ВАЖНО: в CAT передаются ответы сессии — с пустым списком CAT детерминированно
// возвращал один и тот же вопрос, повторная отправка которого в V1 падало
// 409 «Ответ на этот вопрос уже записан».
func (s *ServiceV2) generateTask(poolQuestions []services.PoolQuestion, profile *model.CandidateProfile, sessionID string, index int, answered []SessionAnswer) *services.TestTask {
	irtAnswers := make([]services.IrtAnswer, 0, len(answered))
	for _, a := range answered {
		irtAnswers = append(irtAnswers, services.IrtAnswer{
			Question:  a.Question.IrtParams(),
			IsCorrect: a.IsCorrect != nil && *a.IsCorrect,
		})
	}
	selected := services.SelectNext(poolQuestions, irtAnswers, sessionID)
	if selected == nil {
		return nil
	}

	grade := model.GradeMiddle // profile.verifiedGrade || profile.declaredGrade || 'MIDDLE'
	if profile.VerifiedGrade != nil {
		grade = *profile.VerifiedGrade
	} else if profile.DeclaredGrade != nil {
		grade = *profile.DeclaredGrade
	}

	questionType := selected.QuestionType
	if questionType == "" {
		questionType = "multiple_choice"
	}
	return s.taskGenerator.CreateTaskFromQuestion(services.TaskQuestion{
		ID:            selected.ID,
		QuestionType:  questionType,
		Text:          selected.Text,
		CodeSnippet:   selected.CodeSnippet,
		Options:       selected.Options,
		CorrectAnswer: selected.CorrectAnswer,
		Difficulty:    selected.Difficulty,
		PoolTag:       selected.PoolTag,
	}, grade, index)
}

// gradedAnswerV2 — результат оценки одного ответа (answerText может быть null).
type gradedAnswerV2 struct {
	answerText *string
	isCorrect  bool
	score      float64
}

// gradeAnswerV2 — оценка ответа V2 (docs 06 §4.4): грейдер выбирается по
// ПРИСЛАННЫМ полям ответа (показанный тип задачи определяет поля — TaskAnswer),
// иначе ответ текстом/кодом на задачу из multiple_choice-вопроса всегда
// обнулялся бы оценщиком вариантов. Fallback — questionType исходного вопроса.
func (s *ServiceV2) gradeAnswerV2(ctx context.Context, question services.PoolQuestion, dto SubmitAnswerDto) (gradedAnswerV2, error) {
	if dto.Code != nil && *dto.Code != "" {
		return s.gradeCodeExecution(ctx, question, *dto.Code)
	}
	if dto.DesignDescription != nil && *dto.DesignDescription != "" {
		return s.gradeSystemDesign(ctx, question, *dto.DesignDescription)
	}
	if (dto.FixedCode != nil && *dto.FixedCode != "") || len(dto.BugsFound) > 0 {
		return s.gradeCodeReview(question, dto)
	}
	if dto.SelectedOptionID != nil && *dto.SelectedOptionID != "" {
		return s.gradeMultipleChoice(question, *dto.SelectedOptionID)
	}
	if dto.AnswerText != nil && *dto.AnswerText != "" {
		return s.gradeLLMOpen(ctx, question, *dto.AnswerText)
	}

	// Фолбэк: формат ответа не определён — ориентируемся на тип исходного вопроса
	questionType := question.QuestionType
	if questionType == "" {
		questionType = "multiple_choice"
	}
	switch questionType {
	case "code_execution":
		return s.gradeCodeExecution(ctx, question, deref(dto.Code))
	case "llm_open":
		return s.gradeLLMOpen(ctx, question, deref(dto.AnswerText))
	case "code_review":
		return s.gradeCodeReview(question, dto)
	case "system_design":
		return s.gradeSystemDesign(ctx, question, deref(dto.DesignDescription))
	case "multiple_choice":
	default:
	}
	// Fall back to V1 grading (MC)
	return s.gradeMultipleChoice(question, deref(dto.SelectedOptionID))
}

// gradeCodeExecution — исполнение кода в песочнице + тесты.
func (s *ServiceV2) gradeCodeExecution(ctx context.Context, question services.PoolQuestion, code string) (gradedAnswerV2, error) {
	if code == "" {
		return gradedAnswerV2{answerText: nil, isCorrect: false, score: 0}, nil
	}

	// Parse sample tests from question options
	var options struct {
		SampleTests []services.SampleTest `json:"sampleTests"`
	}
	parseQuestionOptions(question.Options, &options)

	report, err := s.codeSandbox.ExecuteWithTests(ctx, code, "javascript", options.SampleTests, 5, &services.SandboxOptions{
		TimeLimitMs: 10000,
	})
	if err != nil {
		log.Printf("[TestingV2] Code execution failed: %s", err.Error())
		return gradedAnswerV2{answerText: strPtr(code), isCorrect: false, score: 0}, nil
	}

	return gradedAnswerV2{
		answerText: strPtr(code),
		isCorrect:  report.Score >= 60, // Pass if 60%+ tests pass
		score:      report.Score / 100, // Normalize to 0-1
	}, nil
}

// gradeLLMOpen — LLM-оценка развёрнутого текстового ответа.
func (s *ServiceV2) gradeLLMOpen(ctx context.Context, question services.PoolQuestion, answerText string) (gradedAnswerV2, error) {
	if answerText == "" {
		return gradedAnswerV2{answerText: nil, isCorrect: false, score: 0}, nil
	}

	var options struct {
		EvaluationCriteria []services.LLMEvaluationCriteria `json:"evaluationCriteria"`
		KeyConcepts        []string                         `json:"keyConcepts"`
	}
	parseQuestionOptions(question.Options, &options)

	criteria := options.EvaluationCriteria
	if criteria == nil {
		// Критерии по умолчанию (docs 06 §4.4 + шаблон llm_open, который видит
		// клиент): accuracy 0.4 / completeness 0.3 / structure 0.2 / examples 0.1.
		// РАСХОЖДЕНИЕ с TS: gradeLLMOpen по умолчанию отдаёт 3 критерия
		// (accuracy 0.4 / completeness 0.3 / structure 0.3) — взята версия из
		// документации/шаблона задания; при наличии options.evaluationCriteria
		// приоритет у них (как в TS).
		criteria = []services.LLMEvaluationCriteria{
			{Name: "accuracy", Weight: 0.4},
			{Name: "completeness", Weight: 0.3},
			{Name: "structure", Weight: 0.2},
			{Name: "examples", Weight: 0.1},
		}
	}

	result, err := s.llm.Evaluate(ctx, services.LLMEvaluationRequest{
		Question:        question.Text,
		CandidateAnswer: answerText,
		Criteria:        criteria,
		KeyConcepts:     options.KeyConcepts,
	})
	if err != nil {
		log.Printf("[TestingV2] LLM evaluation failed: %s", err.Error())
		return gradedAnswerV2{answerText: strPtr(answerText), isCorrect: false, score: 0}, nil
	}

	return gradedAnswerV2{
		answerText: strPtr(answerText),
		isCorrect:  result.Score >= 50,
		score:      result.Score / 100,
	}, nil
}

// gradeCodeReview — эвристика по числу найденных багов + наличие исправления.
func (s *ServiceV2) gradeCodeReview(question services.PoolQuestion, dto SubmitAnswerDto) (gradedAnswerV2, error) {
	fixedCode := ""
	if dto.FixedCode != nil {
		fixedCode = *dto.FixedCode
	}
	if fixedCode == "" && len(dto.BugsFound) == 0 {
		return gradedAnswerV2{answerText: nil, isCorrect: false, score: 0}, nil
	}

	var options struct {
		ExpectedBugCount *int `json:"expectedBugCount"`
	}
	parseQuestionOptions(question.Options, &options)

	expectedBugs := 2 // options?.expectedBugCount || 2
	if options.ExpectedBugCount != nil && *options.ExpectedBugCount != 0 {
		expectedBugs = *options.ExpectedBugCount
	}
	foundBugs := len(dto.BugsFound)

	// Score based on bugs found
	bugScore := math.Min(1, float64(foundBugs)/float64(expectedBugs)) * 0.6
	// Bonus for fix quality (simple check)
	fixScore := 0.0
	if fixedCode != "" {
		fixScore = 0.4
	}
	totalScore := (bugScore + fixScore) * 100

	// JSON.stringify({ bugsFound, fixedCode }) — отсутствующие поля опускаются;
	// порядок ключей объекта (bugsFound, fixedCode) совпадает с алфавитным.
	payload := map[string]any{}
	if dto.BugsFound != nil {
		payload["bugsFound"] = dto.BugsFound
	}
	if dto.FixedCode != nil {
		payload["fixedCode"] = *dto.FixedCode
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return gradedAnswerV2{answerText: nil, isCorrect: false, score: 0}, nil
	}
	text := string(raw)
	if len(payload) == 0 {
		text = "{}" // JSON.stringify({bugsFound: undefined, fixedCode: undefined})
	}

	return gradedAnswerV2{
		answerText: strPtr(text),
		isCorrect:  totalScore >= 50,
		score:      totalScore / 100,
	}, nil
}

// gradeSystemDesign — LLM-оценка описания системы.
func (s *ServiceV2) gradeSystemDesign(ctx context.Context, question services.PoolQuestion, designDescription string) (gradedAnswerV2, error) {
	if designDescription == "" {
		return gradedAnswerV2{answerText: nil, isCorrect: false, score: 0}, nil
	}

	result, err := s.llm.Evaluate(ctx, services.LLMEvaluationRequest{
		Question:        question.Text,
		CandidateAnswer: designDescription,
		Criteria: []services.LLMEvaluationCriteria{
			{Name: "architecture", Weight: 0.3},
			{Name: "scalability", Weight: 0.25},
			{Name: "tradeoffs", Weight: 0.25},
			{Name: "clarity", Weight: 0.2},
		},
		KeyConcepts: nil,
	})
	if err != nil {
		// В TS нет try/catch — ошибка пробрасывается (500).
		return gradedAnswerV2{}, err
	}

	return gradedAnswerV2{
		answerText: strPtr(designDescription),
		isCorrect:  result.Score >= 50,
		score:      result.Score / 100,
	}, nil
}

// gradeMultipleChoice — оценка варианта (как в V1, но по selectedOptionId).
func (s *ServiceV2) gradeMultipleChoice(question services.PoolQuestion, selectedOptionID string) (gradedAnswerV2, error) {
	options := question.ParsedOptions()
	if len(options) == 0 || selectedOptionID == "" {
		var answerText *string
		if selectedOptionID != "" {
			answerText = strPtr(selectedOptionID)
		}
		return gradedAnswerV2{answerText: answerText, isCorrect: false, score: 0}, nil
	}

	var selected *model.QuestionOption
	for i := range options {
		if options[i].ID == selectedOptionID {
			selected = &options[i]
			break
		}
	}
	isCorrect := selected != nil && selected.IsCorrect
	score := 0.0
	if isCorrect {
		score = 1
	}
	return gradedAnswerV2{answerText: strPtr(selectedOptionID), isCorrect: isCorrect, score: score}, nil
}

// calculateTaskBreakdown — агрегат результатов по типам заданий
// {total, passed, avgScore}; порядок ключей — первое появление типа
// (Object.entries в JS), avgScore = round(sum/len * 100).
func (s *ServiceV2) calculateTaskBreakdown(answered []SessionAnswer) TaskBreakdown {
	type accumulator struct {
		total  int
		passed int
		scores []float64
	}

	order := []string{}
	acc := map[string]*accumulator{}

	for _, a := range answered {
		taskType := a.Question.QuestionType
		if taskType == "" {
			taskType = "multiple_choice"
		}
		bucket, ok := acc[taskType]
		if !ok {
			bucket = &accumulator{}
			acc[taskType] = bucket
			order = append(order, taskType)
		}
		bucket.total++
		if a.IsCorrect != nil && *a.IsCorrect {
			bucket.passed++
		}
		if a.Score != nil {
			bucket.scores = append(bucket.scores, *a.Score)
		}
	}

	breakdown := newTaskBreakdown()
	for _, taskType := range order {
		bucket := acc[taskType]
		avg := 0
		if len(bucket.scores) > 0 {
			sum := 0.0
			for _, score := range bucket.scores {
				sum += score
			}
			avg = int(math.Round((sum / float64(len(bucket.scores))) * 100))
		}
		breakdown.keys = append(breakdown.keys, taskType)
		breakdown.items[taskType] = TaskBreakdownEntry{
			Total:    bucket.total,
			Passed:   bucket.passed,
			AvgScore: avg,
		}
	}
	return breakdown
}

// resultViewV2 — итог для идемпотентного повторного complete() (session уже
// COMPLETED): пустые taskBreakdown/antiCheat, cooldownUntil = now, без gamification.
func (s *ServiceV2) resultViewV2(session *Session) *TestResultV2 {
	score := 0.0
	if session.Score != nil {
		score = *session.Score
	}
	gradeAfter := model.GradeJunior
	if session.GradeAfter != nil {
		gradeAfter = *session.GradeAfter
	}
	recommendation := "FAIL"
	if session.Recommendation != nil && *session.Recommendation != "" {
		recommendation = *session.Recommendation
	}
	theta := 0.0
	if session.ThetaEstimate != nil {
		theta = *session.ThetaEstimate
	}

	return &TestResultV2{
		SessionID:      session.ID,
		Status:         session.Status,
		Score:          score,
		GradeBefore:    session.GradeBefore,
		GradeAfter:     gradeAfter,
		Recommendation: recommendation,
		Theta:          theta,
		TaskBreakdown:  newTaskBreakdown(),
		AntiCheat: AntiCheatV2{
			IsSuspicious:       false,
			Flags:              []string{},
			Confidence:         0,
			BehavioralAnalysis: map[string]services.BehavioralDetail{},
		},
		CooldownUntil: model.Now(),
		Gamification:  nil,
	}
}

// ============ Shared helpers V2 ============

func (s *ServiceV2) profileOf(ctx context.Context, userID string) (*model.CandidateProfile, error) {
	profile, err := s.store.FindProfileByUserID(ctx, userID)
	if err == nil && profile == nil {
		err = errNotFound
	}
	if err != nil {
		return nil, common.NotFound("Профиль кандидата не найден")
	}
	return profile, nil
}

// loadOwned — профиль + сессия с ответами с проверкой владельца.
func (s *ServiceV2) loadOwned(ctx context.Context, userID, sessionID string) (*Session, *model.CandidateProfile, error) {
	profile, err := s.profileOf(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	session, err := s.store.FindSessionWithAnswers(ctx, sessionID)
	if err == nil && session == nil {
		return nil, nil, common.NotFound("Сессия не найдена")
	}
	if err != nil {
		return nil, nil, err
	}
	if session.CandidateID != profile.ID {
		return nil, nil, common.Forbidden("Это чужая сессия")
	}
	return session, profile, nil
}

func (s *ServiceV2) isExpired(session *Session) bool {
	deadline := session.StartedAt.Time.Add(time.Duration(session.TimeLimitMinutes) * time.Minute)
	return time.Now().After(deadline)
}

func (s *ServiceV2) timeRemaining(session *Session) int64 {
	deadline := session.StartedAt.Time.Add(time.Duration(session.TimeLimitMinutes) * time.Minute)
	remaining := int64(math.Floor(deadline.Sub(time.Now()).Seconds())) // Math.floor в TS
	if remaining < 0 {
		return 0
	}
	return remaining
}

// toIrtV2 — только ответы с известной оценкой (V2 TS фильтрует isCorrect !== null).
func toIrtV2(answers []SessionAnswer) []services.IrtAnswer {
	out := make([]services.IrtAnswer, 0, len(answers))
	for _, a := range answers {
		if a.IsCorrect == nil {
			continue
		}
		out = append(out, services.IrtAnswer{
			Question:  a.Question.IrtParams(),
			IsCorrect: *a.IsCorrect,
		})
	}
	return out
}

// uniqueFlags — [...new Set([...first, ...second])] с сохранением порядка.
func uniqueFlags(first, second []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(first)+len(second))
	for _, flags := range [][]string{first, second} {
		for _, flag := range flags {
			if !seen[flag] {
				seen[flag] = true
				out = append(out, flag)
			}
		}
	}
	return out
}

// parseBehavioral — map[string]any из DTO → BehavioralData (Record<string, number>).
// Нечисловые значения трактуются как отсутствие поля (undefined в TS).
func parseBehavioral(raw map[string]any) services.BehavioralData {
	data := services.BehavioralData{
		TimeSpentSeconds:      behavioralNumber(raw, "timeSpentSeconds"),
		CopyPasteEvents:       behavioralNumber(raw, "copyPasteEvents"),
		TabSwitches:           behavioralNumber(raw, "tabSwitches"),
		KeystrokeCount:        behavioralNumber(raw, "keystrokeCount"),
		IdleTimeSeconds:       behavioralNumber(raw, "idleTimeSeconds"),
		MouseMovementDistance: behavioralNumber(raw, "mouseMovementDistance"),
		FocusLossEvents:       behavioralNumber(raw, "focusLossEvents"),
	}
	if sources, ok := raw["pasteSources"].([]any); ok {
		out := make([]string, 0, len(sources))
		for _, item := range sources {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		data.PasteSources = out
	}
	return data
}

// behavioralNumber — значение поля телеметрии (null/не-число → nil = undefined).
func behavioralNumber(raw map[string]any, key string) *float64 {
	value, ok := raw[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		return &typed
	case int:
		f := float64(typed)
		return &f
	case json.Number:
		if f, err := typed.Float64(); err == nil {
			return &f
		}
	}
	return nil
}

// parseQuestionOptions — разбор options вопроса в нужный структурный вид
// (в TS — «as»-каст, ошибка разбора игнорируется).
func parseQuestionOptions(raw json.RawMessage, out any) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	_ = json.Unmarshal(raw, out)
}

// deref — *string → string (nil = "").
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// errorsIsDuplicate — уникальный индекс test_answers (sessionId, questionId).
func errorsIsDuplicate(err error) bool { return errors.Is(err, ErrDuplicateAnswer) }

// cfgOrEnv — приоритет: переданная конфигурация > env > default (BUG-REF: T5).
func cfgOrEnv(cfgValue int, envKey string, def int) int {
	if cfgValue > 0 {
		return cfgValue
	}
	return envIntOr(envKey, def)
}

// envIntOr — process.env.<key> parseInt(...) || default.
func envIntOr(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// envStrOr — process.env.<key> || default.
func envStrOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// envBoolOr — process.env.<key> === 'true' || default.
func envBoolOr(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
