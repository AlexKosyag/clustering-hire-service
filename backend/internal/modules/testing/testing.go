package testing

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"strings"
	"time"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
	"fsp-backend/internal/modules/testing/services"
)

// MatchingAPI — публичный API matching-подсистемы (реализует *matching.Service).
type MatchingAPI interface {
	InvalidateCache(ctx context.Context) error
	IndexCandidate(ctx context.Context, profileID string) error
}

// ============ Ответы V1 (docs 04 §7, «V1 (legacy)») ============

// QuestionView — вопрос для клиента: без правильного ответа и объяснения,
// варианты в сессионном порядке (viewQuestion в TS).
type QuestionView struct {
	ID           string       `json:"id"`
	Text         string       `json:"text"`
	CodeSnippet  *string      `json:"codeSnippet"`
	QuestionType string       `json:"questionType"`
	Options      []OptionView `json:"options"`
	Difficulty   float64      `json:"difficulty"`
}

// OptionView — вариант ответа для клиента (без isCorrect).
type OptionView struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// StartResult — POST /testing/start, /testing/retake.
type StartResult struct {
	SessionID        string               `json:"sessionId"`
	TotalQuestions   int                  `json:"totalQuestions"`
	TimeLimitMinutes int                  `json:"timeLimitMinutes"`
	Specialization   model.Specialization `json:"specialization"`
	GradeBefore      *model.Grade         `json:"gradeBefore"`
	FirstQuestion    *QuestionView        `json:"firstQuestion"`
}

// ActiveSessionInfo — активная сессия в GET /testing/active.
type ActiveSessionInfo struct {
	ID        string     `json:"id"`
	StartedAt model.Time `json:"startedAt"`
}

// ActiveResult — GET /testing/active.
type ActiveResult struct {
	Session     *ActiveSessionInfo `json:"session"`
	CanRetakeAt *model.Time        `json:"canRetakeAt"`
}

// CurrentResult — GET /testing/session/:id/current (сессия идёт).
type CurrentResult struct {
	SessionID            string              `json:"sessionId"`
	Status               model.SessionStatus `json:"status"`
	QuestionNumber       int                 `json:"questionNumber"`
	TotalQuestions       int                 `json:"totalQuestions"`
	CurrentScore         float64             `json:"currentScore"`
	TimeRemainingSeconds int64               `json:"timeRemainingSeconds"`
	IsFinished           bool                `json:"isFinished"`
	Question             *QuestionView       `json:"question"`
}

// CurrentFinishedResult — GET /testing/session/:id/current (сессия не идёт).
type CurrentFinishedResult struct {
	SessionID  string              `json:"sessionId"`
	Status     model.SessionStatus `json:"status"`
	Question   *QuestionView       `json:"question"`
	IsFinished bool                `json:"isFinished"`
}

// AnswerResult — POST /testing/session/:id/answer.
type AnswerResult struct {
	Correct            bool          `json:"correct"`
	Score              float64       `json:"score"`
	Explanation        *string       `json:"explanation"`
	CurrentScore       float64       `json:"currentScore"`
	Theta              float64       `json:"theta"`
	QuestionsRemaining int           `json:"questionsRemaining"`
	IsFinished         bool          `json:"isFinished"`
	NextQuestion       *QuestionView `json:"nextQuestion"`
}

// AntiCheatView — анти-чит сводка (V1).
type AntiCheatView struct {
	IsSuspicious bool     `json:"isSuspicious"`
	Flags        []string `json:"flags"`
	Confidence   float64  `json:"confidence"`
}

// ResultView — итог сессии (resultView в TS; используется и для идемпотентного complete()).
type ResultView struct {
	SessionID      string              `json:"sessionId"`
	Status         model.SessionStatus `json:"status"`
	Score          *float64            `json:"score"`
	GradeBefore    *model.Grade        `json:"gradeBefore"`
	GradeAfter     *model.Grade        `json:"gradeAfter"`
	Recommendation *string             `json:"recommendation"`
	Theta          *float64            `json:"theta"`
	IsSuspicious   bool                `json:"isSuspicious"`
	CheatFlags     []string            `json:"cheatFlags"`
	CompletedAt    *model.Time         `json:"completedAt"`
}

// CompleteResult — POST /testing/session/:id/complete.
type CompleteResult struct {
	ResultView
	Details       services.DifficultyDetails `json:"details"`
	CooldownUntil model.Time                 `json:"cooldownUntil"`
	AntiCheat     AntiCheatView              `json:"antiCheat"`
}

// ============ Service (V1, legacy) ============

// Service — движок V1 (legacy): 20 вопросов, 60 минут.
type Service struct {
	store          Store
	pool           *services.QuestionPool
	cat            *services.CAT
	scoring        *services.Scoring
	antiCheat      *services.AntiCheat
	matching       MatchingAPI
	totalQuestions int
	cooldownDays   int
}

// NewService — конструктор V1.
//
// BUG-REF: T5 — V1 берёт размеры теста/кулдауна из конфигурации
// (cfg.TestingQuestionsPerTest=20, cfg.TestingCooldownDays=90), V2 — из env
// (TESTING_TASKS_PER_TEST / TESTING_COOLDOWN_DAYS); единой конфигурации нет.
func NewService(store Store, cfg TestingConfig, matching MatchingAPI) *Service {
	return &Service{
		store:          store,
		pool:           services.NewQuestionPool(store),
		cat:            services.NewCAT(),
		scoring:        services.NewScoring(),
		antiCheat:      services.NewAntiCheat(),
		matching:       matching,
		totalQuestions: cfg.QuestionsPerTest,
		cooldownDays:   cfg.CooldownDays,
	}
}

// TestingConfig — параметры движка (из config.Config; см. BUG-REF: T5).
type TestingConfig struct {
	QuestionsPerTest int
	CooldownDays     int
	TasksPerTest     int
	V2CooldownDays   int
}

// Start — начать тестирование.
func (s *Service) Start(ctx context.Context, userID string, dto StartTestDto) (any, error) {
	profile, err := s.profileOf(ctx, userID)
	if err != nil {
		return nil, err
	}

	if profile.TestCooldownUntil != nil && profile.TestCooldownUntil.Time.After(time.Now()) {
		return nil, common.BadRequest("Пересдача возможна не раньше " + isoString(profile.TestCooldownUntil))
	}

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

	category, err := s.pool.CategoryForSpecialization(ctx, *dto.Specialization)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, common.BadRequest("Для этой специализации нет теста")
	}

	poolQuestions, err := s.pool.PoolFor(ctx, category.ID)
	if err != nil {
		return nil, err
	}
	if len(poolQuestions) < s.totalQuestions {
		return nil, common.BadRequest("Банк вопросов неполный — тест временно недоступен")
	}

	declared := dto.DeclaredGrade
	if declared == nil {
		declared = profile.DeclaredGrade
	}

	session := &model.TestSession{
		ID:                newID(),
		CandidateID:       profile.ID,
		CategoryID:        category.ID,
		Status:            model.SessionInProgress,
		StartedAt:         model.Now(),
		TimeLimitMinutes:  60,
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

	first := services.SelectNext(poolQuestions, nil, session.ID)
	var firstQuestion *QuestionView
	if first != nil {
		firstQuestion = s.viewQuestion(*first, session.ID)
	}

	return &StartResult{
		SessionID:        session.ID,
		TotalQuestions:   s.totalQuestions,
		TimeLimitMinutes: session.TimeLimitMinutes,
		Specialization:   category.Specialization,
		GradeBefore:      declared,
		FirstQuestion:    firstQuestion,
	}, nil
}

// Retake — пересдача (делегирует в Start).
// BUG-REF: T8 — RetakeDto.targetGrade принимается, но игнорируется;
// SessionStatus.ABANDONED не используется нигде.
func (s *Service) Retake(ctx context.Context, userID string, dto RetakeDto) (any, error) {
	return s.Start(ctx, userID, StartTestDto{Specialization: dto.Specialization})
}

// Active — активная сессия (для восстановления теста после перезагрузки страницы).
func (s *Service) Active(ctx context.Context, userID string) (any, error) {
	profile, err := s.profileOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	session, err := s.store.FindActiveSession(ctx, profile.ID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return &ActiveResult{Session: nil, CanRetakeAt: profile.TestCooldownUntil}, nil
	}
	if s.isExpired(session) {
		session.Status = model.SessionTimedOut
		now := model.Now()
		session.CompletedAt = &now
		if err := s.store.UpdateSession(ctx, &session.TestSession); err != nil {
			return nil, err
		}
		return &ActiveResult{Session: nil, CanRetakeAt: profile.TestCooldownUntil}, nil
	}
	return &ActiveResult{
		Session:     &ActiveSessionInfo{ID: session.ID, StartedAt: session.StartedAt},
		CanRetakeAt: profile.TestCooldownUntil,
	}, nil
}

// Current — текущий вопрос.
func (s *Service) Current(ctx context.Context, userID, sessionID string) (any, error) {
	session, err := s.loadOwned(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	if session.Status != model.SessionInProgress {
		return &CurrentFinishedResult{SessionID: session.ID, Status: session.Status, Question: nil, IsFinished: true}, nil
	}
	if s.isExpired(session) {
		session.Status = model.SessionTimedOut
		now := model.Now()
		session.CompletedAt = &now
		if err := s.store.UpdateSession(ctx, &session.TestSession); err != nil {
			return nil, err
		}
		return &CurrentFinishedResult{SessionID: session.ID, Status: model.SessionTimedOut, Question: nil, IsFinished: true}, nil
	}

	answered := session.Answers
	irt := toIrt(answered)
	poolQuestions, err := s.pool.PoolFor(ctx, session.CategoryID)
	if err != nil {
		return nil, err
	}
	next := services.SelectNext(poolQuestions, irt, session.ID)

	var question *QuestionView
	if next != nil {
		question = s.viewQuestion(*next, session.ID)
	}
	return &CurrentResult{
		SessionID:            session.ID,
		Status:               session.Status,
		QuestionNumber:       len(answered) + 1,
		TotalQuestions:       s.totalQuestions,
		CurrentScore:         s.scoring.ScoreFromAnswers(toStats(answered)),
		TimeRemainingSeconds: s.timeRemaining(session),
		IsFinished:           len(answered) >= s.totalQuestions,
		Question:             question,
	}, nil
}

// Answer — отправить ответ.
func (s *Service) Answer(ctx context.Context, userID, sessionID string, dto SubmitAnswerDto) (any, error) {
	session, err := s.loadOwned(ctx, userID, sessionID)
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
	for _, a := range session.Answers {
		if a.QuestionID == questionID {
			return nil, common.Conflict("Ответ на этот вопрос уже записан")
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

	graded, err := s.gradeAnswer(*question, dto)
	if err != nil {
		return nil, err
	}

	answer := &model.TestAnswer{
		ID:               newID(),
		SessionID:        session.ID,
		QuestionID:       question.ID,
		AnswerText:       strPtr(graded.answerText),
		IsCorrect:        boolPtr(graded.isCorrect),
		Score:            floatPtr(graded.score),
		TimeSpentSeconds: dto.TimeSpentSeconds,
		AnsweredAt:       model.Now(),
	}
	if err := s.store.CreateAnswer(ctx, answer); err != nil {
		if errors.Is(err, ErrDuplicateAnswer) {
			return nil, common.Conflict("Ответ на этот вопрос уже записан")
		}
		return nil, err
	}

	updated, err := s.store.FindSessionWithAnswers(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	answered := updated.Answers
	irt := toIrt(answered)
	theta := s.cat.EstimateTheta(irt)
	nextQuestion := services.SelectNext(poolQuestions, irt, session.ID)

	updated.ThetaEstimate = &theta
	updated.CurrentDifficulty = 1
	if nextQuestion != nil {
		updated.CurrentDifficulty = nextQuestion.Difficulty
	}
	if err := s.store.UpdateSession(ctx, &updated.TestSession); err != nil {
		return nil, err
	}

	remaining := s.totalQuestions - len(answered)
	if remaining < 0 {
		remaining = 0
	}

	var nextView *QuestionView
	if remaining > 0 && nextQuestion != nil {
		nextView = s.viewQuestion(*nextQuestion, session.ID)
	}
	return &AnswerResult{
		Correct:            graded.isCorrect,
		Score:              round2(graded.score),
		Explanation:        orNull(question.Explanation),
		CurrentScore:       s.scoring.ScoreFromAnswers(toStats(answered)),
		Theta:              round3(theta),
		QuestionsRemaining: remaining,
		IsFinished:         remaining == 0,
		NextQuestion:       nextView,
	}, nil
}

// Complete — завершить тест (идемпотентен для COMPLETED).
func (s *Service) Complete(ctx context.Context, userID, sessionID string) (any, error) {
	session, err := s.loadOwned(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	if session.Status == model.SessionCompleted {
		result := s.resultView(session)
		return &result, nil
	}
	if session.Status != model.SessionInProgress {
		return nil, common.BadRequest("Сессию нельзя завершить")
	}

	profile, err := s.profileOf(ctx, userID)
	if err != nil {
		return nil, err
	}

	expired := s.isExpired(session)
	if len(session.Answers) < s.totalQuestions && !expired {
		return nil, common.BadRequest(fmt.Sprintf("Осталось неотвеченных вопросов: %d", s.totalQuestions-len(session.Answers)))
	}

	answered := session.Answers
	irt := toIrt(answered)
	stats := toStats(answered)

	theta := s.cat.EstimateTheta(irt)
	score := s.scoring.ScoreFromAnswers(stats)
	declaredGrade := session.GradeBefore // session.gradeBefore ?? profile.declaredGrade
	if declaredGrade == nil {
		declaredGrade = profile.DeclaredGrade
	}
	gradeAfter := s.scoring.CalculateGrade(theta, profile.Specialization)
	recommendation := s.scoring.GetRecommendation(declaredGrade, gradeAfter, score)
	details := s.scoring.DetailsByDifficulty(stats)

	history := s.antiCheat.ThetaHistory(irt, s.cat.EstimateTheta)
	timed := make([]services.TimedAnswer, 0, len(answered))
	for _, a := range answered {
		poolTag := a.Question.PoolTag
		timed = append(timed, services.TimedAnswer{
			TimeSpentSeconds: a.TimeSpentSeconds,
			Difficulty:       a.Question.Difficulty,
			PoolTag:          poolTag,
			IsCorrect:        a.IsCorrect != nil && *a.IsCorrect,
		})
	}
	analysis := s.antiCheat.Analyze(timed, history)
	// BUG-REF: T4 — cheatFlags/isSuspicious не влияют на score/грейд/рекомендацию.

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
	session.CheatFlags = analysis.Flags
	session.IsSuspicious = analysis.IsSuspicious
	if err := s.store.UpdateSession(ctx, &session.TestSession); err != nil {
		return nil, err
	}

	if err := s.store.SetProfileTestResult(ctx, profile.ID, gradeAfter, testStatus, now.Time, cooldownUntil.Time, score); err != nil {
		return nil, err
	}

	_ = safeCall(func() error { return s.matching.InvalidateCache(ctx) }) // .catch(() => undefined)
	if err := safeCall(func() error { return s.matching.IndexCandidate(ctx, profile.ID) }); err != nil {
		log.Printf("[Testing] Переиндексация: %s", err.Error())
	}

	view := s.resultView(session)
	rounded := round3(theta)
	view.Theta = &rounded
	return &CompleteResult{
		ResultView:    view,
		Details:       details,
		CooldownUntil: cooldownUntil,
		AntiCheat: AntiCheatView{
			IsSuspicious: analysis.IsSuspicious,
			Flags:        analysis.Flags,
			Confidence:   analysis.Confidence,
		},
	}, nil
}

// ===== Вспомогательные методы =====

func (s *Service) profileOf(ctx context.Context, userID string) (*model.CandidateProfile, error) {
	profile, err := s.store.FindProfileByUserID(ctx, userID)
	if err == nil && profile == nil {
		err = errNotFound
	}
	if err != nil {
		return nil, common.NotFound("Профиль кандидата не найден")
	}
	return profile, nil
}

func (s *Service) loadOwned(ctx context.Context, userID, sessionID string) (*Session, error) {
	profile, err := s.profileOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	session, err := s.store.FindSessionWithAnswers(ctx, sessionID)
	if err == nil && session == nil {
		return nil, common.NotFound("Сессия не найдена")
	}
	if err != nil {
		return nil, err
	}
	if session.CandidateID != profile.ID {
		return nil, common.Forbidden("Это чужая сессия")
	}
	return session, nil
}

func (s *Service) isExpired(session *Session) bool {
	deadline := session.StartedAt.Time.Add(time.Duration(session.TimeLimitMinutes) * time.Minute)
	return time.Now().After(deadline)
}

func (s *Service) timeRemaining(session *Session) int64 {
	deadline := session.StartedAt.Time.Add(time.Duration(session.TimeLimitMinutes) * time.Minute)
	remaining := int64(math.Round(deadline.Sub(time.Now()).Seconds()))
	if remaining < 0 {
		return 0
	}
	return remaining
}

// toIrt — все ответы (V1 не фильтрует null-оценки).
func toIrt(answers []SessionAnswer) []services.IrtAnswer {
	out := make([]services.IrtAnswer, 0, len(answers))
	for _, a := range answers {
		out = append(out, services.IrtAnswer{
			Question:  a.Question.IrtParams(),
			IsCorrect: a.IsCorrect != nil && *a.IsCorrect,
		})
	}
	return out
}

func toStats(answers []SessionAnswer) []services.AnswerStat {
	out := make([]services.AnswerStat, 0, len(answers))
	for _, a := range answers {
		out = append(out, services.AnswerStat{
			IsCorrect:  a.IsCorrect != nil && *a.IsCorrect,
			Difficulty: a.Question.Difficulty,
			Score:      a.Score,
		})
	}
	return out
}

// viewQuestion — вопрос для клиента: без правильного ответа и объяснения,
// варианты в сессионном порядке.
func (s *Service) viewQuestion(question services.PoolQuestion, sessionID string) *QuestionView {
	parsed := question.ParsedOptions()
	options := make([]OptionView, 0, len(parsed))
	for _, o := range parsed {
		options = append(options, OptionView{ID: o.ID, Text: o.Text})
	}
	return &QuestionView{
		ID:           question.ID,
		Text:         question.Text,
		CodeSnippet:  question.CodeSnippet,
		QuestionType: question.QuestionType,
		Options:      services.ShuffleOptions(sessionID, question.ID, options),
		Difficulty:   question.Difficulty,
	}
}

type gradedAnswer struct {
	isCorrect  bool
	score      float64
	answerText string
}

// gradeAnswer — оценка ответа V1 (docs 06 §4.4).
func (s *Service) gradeAnswer(question services.PoolQuestion, dto SubmitAnswerDto) (gradedAnswer, error) {
	if question.QuestionType == "multiple_choice" {
		options := question.ParsedOptions()
		answer := ""
		if dto.Answer != nil {
			answer = *dto.Answer
		}
		var selected *model.QuestionOption
		for i := range options {
			if options[i].ID == answer {
				selected = &options[i]
				break
			}
		}
		if selected == nil {
			return gradedAnswer{}, common.BadRequest("Не выбран вариант ответа")
		}
		isCorrect := selected.IsCorrect
		score := 0.0
		if isCorrect {
			score = 1
		}
		return gradedAnswer{isCorrect: isCorrect, score: score, answerText: selected.ID}, nil
	}

	text := ""
	if dto.AnswerText != nil {
		text = strings.TrimSpace(*dto.AnswerText)
	}
	if text == "" {
		return gradedAnswer{}, common.BadRequest("Ответ пустой")
	}

	reference := question.CorrectAnswer
	if reference == nil || *reference == "" {
		// Без эталона — частичные баллы за объём, но не засчитываем как верный ответ
		score := math.Min(0.6, round2(float64(services.JSStringLen(text))/500))
		return gradedAnswer{isCorrect: false, score: score, answerText: text}, nil
	}

	score := referenceCoverage(*reference, text)
	return gradedAnswer{isCorrect: score >= 0.6, score: round2(score), answerText: text}, nil
}

// referenceCoverage — покрытие эталона: либо набор ключевых фраз через '|',
// либо пересечение токенов. Детерминированно и не требует внешних моделей.
func referenceCoverage(reference, answer string) float64 {
	var parts []string
	if strings.Contains(reference, "|") {
		for _, p := range strings.Split(reference, "|") {
			if n := normalize(p); n != "" {
				parts = append(parts, n)
			}
		}
	} else {
		parts = []string{normalize(reference)}
	}

	if len(parts) == 0 {
		return 0
	}

	answerTokens := map[string]bool{}
	for _, t := range tokenize(answer) {
		answerTokens[t] = true
	}
	total := 0.0
	for _, part := range parts {
		referenceTokens := tokenize(part)
		if len(referenceTokens) == 0 {
			continue
		}
		hit := 0
		for _, t := range referenceTokens {
			if answerTokens[t] {
				hit++
			}
		}
		coverage := float64(hit) / float64(len(referenceTokens))
		phraseHit := 0.0
		if strings.Contains(normalize(answer), part) {
			phraseHit = 1
		}
		total += math.Max(coverage, phraseHit)
	}
	return math.Max(0, math.Min(1, total/float64(len(parts))))
}

func (s *Service) resultView(session *Session) ResultView {
	return ResultView{
		SessionID:      session.ID,
		Status:         session.Status,
		Score:          session.Score,
		GradeBefore:    session.GradeBefore,
		GradeAfter:     session.GradeAfter,
		Recommendation: session.Recommendation,
		Theta:          session.ThetaEstimate,
		IsSuspicious:   session.IsSuspicious,
		CheatFlags:     session.CheatFlags,
		CompletedAt:    session.CompletedAt,
	}
}

// ============ Общие помощники ============

var normalizeRe = regexp.MustCompile(`[^a-zа-я0-9]+`)

func normalize(value string) string {
	s := strings.ToLower(value)
	s = strings.ReplaceAll(s, "ё", "е")
	s = normalizeRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func tokenize(value string) []string {
	out := []string{}
	for _, t := range strings.Split(normalize(value), " ") {
		if services.JSStringLen(t) > 1 {
			out = append(out, t)
		}
	}
	return out
}

// newID — uuid.NewString()-эквивалент (UUID v4, crypto/rand; без новых зависимостей).
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// safeCall — внешний вызов с перехватом паники (аналог .catch в TS).
func safeCall(fn func() error) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("%v", rec)
		}
	}()
	return fn()
}

// isoString — Date.prototype.toISOString (без таймзоны, UTC).
func isoString(t *model.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func round2(v float64) float64 { return services.JSRound(v*100) / 100 }
func round3(v float64) float64 { return services.JSRound(v*1000) / 1000 }

func strPtr(s string) *string     { return &s }
func boolPtr(b bool) *bool        { return &b }
func floatPtr(f float64) *float64 { return &f }

// orNull — `value || null` из TS (пустая строка → null).
func orNull(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

var errNotFound = errors.New("not found")
