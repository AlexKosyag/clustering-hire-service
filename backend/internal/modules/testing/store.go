package testing

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"fsp-backend/internal/model"
	"fsp-backend/internal/modules/testing/services"
	"fsp-backend/internal/store"
)

// ErrDuplicateAnswer — нарушение unique (session_id, question_id) в test_answers
// (pg error code 23505); V1 маппится в 409 «Ответ на этот вопрос уже записан».
var ErrDuplicateAnswer = errors.New("duplicate test answer")

// SessionAnswer — ответ сессии с присоединённым вопросом
// (include: { answers: { include: { question: true }, orderBy: { answeredAt: 'asc' } } }).
type SessionAnswer struct {
	model.TestAnswer
	Question services.PoolQuestion
}

// Session — сессия с ответами.
type Session struct {
	model.TestSession
	Answers []SessionAnswer
}

// Store — доступ к БД testing engine (реализация: PGStore, в тестах — in-memory).
type Store interface {
	services.PoolStore

	// FindProfileByUserID — candidate_profiles.findUnique({ where: { userId } }).
	FindProfileByUserID(ctx context.Context, userID string) (*model.CandidateProfile, error)
	// SetProfileTestStatus — candidateProfile.update({ testStatus }).
	SetProfileTestStatus(ctx context.Context, profileID string, status model.TestStatus) error
	// SetProfileTestResult — candidateProfile.update(результат теста).
	SetProfileTestResult(ctx context.Context, profileID string, gradeAfter model.Grade, testStatus model.TestStatus, lastTestDate, cooldownUntil time.Time, lastTestScore float64) error

	// FindActiveSession — testSession.findFirst({ candidateId, status: IN_PROGRESS, orderBy: { startedAt: 'desc' } }).
	FindActiveSession(ctx context.Context, candidateID string) (*Session, error)
	// FindSessionWithAnswers — testSession.findUnique({ id, include: answers+question }).
	FindSessionWithAnswers(ctx context.Context, sessionID string) (*Session, error)
	// CreateSession — testSession.create.
	CreateSession(ctx context.Context, session *model.TestSession) error
	// UpdateSession — testSession.update({ where: { id }, data }).
	UpdateSession(ctx context.Context, session *model.TestSession) error
	// CreateAnswer — testAnswer.create (unique session_id+question_id).
	CreateAnswer(ctx context.Context, answer *model.TestAnswer) error
}

// PGStore — реализация Store поверх PostgreSQL (pgx, raw SQL).
type PGStore struct {
	db *store.DB
}

// NewPGStore — конструктор.
func NewPGStore(db *store.DB) *PGStore { return &PGStore{db: db} }

// ===== Банк вопросов (services.PoolStore) =====

func (s *PGStore) ListQuestions(ctx context.Context, categoryID string) ([]services.PoolQuestion, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, question_type, text, code_snippet, options, correct_answer, explanation,
		       difficulty, discrimination, guessing, is_unique, pool_tag
		FROM test_questions
		WHERE category_id = $1
		ORDER BY id ASC`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []services.PoolQuestion{}
	for rows.Next() {
		var q services.PoolQuestion
		if err := rows.Scan(&q.ID, &q.QuestionType, &q.Text, &q.CodeSnippet, &q.Options,
			&q.CorrectAnswer, &q.Explanation, &q.Difficulty, &q.Discrimination, &q.Guessing,
			&q.IsUnique, &q.PoolTag); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (s *PGStore) FindCategoryBySpecialization(ctx context.Context, spec model.Specialization) (*services.Category, error) {
	row := s.db.Pool.QueryRow(ctx, `
		SELECT id, name, description, specialization
		FROM test_categories
		WHERE specialization = $1::"Specialization"
		LIMIT 1`, string(spec))
	return scanCategory(row)
}

func (s *PGStore) FindAnyCategory(ctx context.Context) (*services.Category, error) {
	row := s.db.Pool.QueryRow(ctx, `
		SELECT id, name, description, specialization
		FROM test_categories
		ORDER BY name ASC
		LIMIT 1`)
	return scanCategory(row)
}

func scanCategory(row pgx.Row) (*services.Category, error) {
	var c services.Category
	var spec string
	if err := row.Scan(&c.ID, &c.Name, &c.Description, &spec); err != nil {
		return nil, err
	}
	c.Specialization = model.Specialization(spec)
	return &c, nil
}

// ===== Профиль =====

func (s *PGStore) FindProfileByUserID(ctx context.Context, userID string) (*model.CandidateProfile, error) {
	// Читаются только колонки, используемые testing engine (поля CandidateProfile).
	row := s.db.Pool.QueryRow(ctx, `
		SELECT id, user_id, specialization, declared_grade, verified_grade,
		       test_status, test_cooldown_until
		FROM candidate_profiles
		WHERE user_id = $1`, userID)
	var p model.CandidateProfile
	var spec, declared, verified, testStatus *string
	var cooldown *time.Time
	if err := row.Scan(&p.ID, &p.UserID, &spec, &declared, &verified, &testStatus, &cooldown); err != nil {
		return nil, err
	}
	if spec != nil {
		v := model.Specialization(*spec)
		p.Specialization = &v
	}
	if declared != nil {
		v := model.Grade(*declared)
		p.DeclaredGrade = &v
	}
	if verified != nil {
		v := model.Grade(*verified)
		p.VerifiedGrade = &v
	}
	if testStatus != nil {
		p.TestStatus = model.TestStatus(*testStatus)
	}
	if cooldown != nil {
		mt := model.FromTime(*cooldown)
		p.TestCooldownUntil = &mt
	}
	return &p, nil
}

func (s *PGStore) SetProfileTestStatus(ctx context.Context, profileID string, status model.TestStatus) error {
	// updated_at — ЯВНО (Prisma @updatedAt)
	_, err := s.db.Pool.Exec(ctx, `
		UPDATE candidate_profiles
		SET test_status = $1::"TestStatus", updated_at = $2
		WHERE id = $3`, string(status), model.Now(), profileID)
	return err
}

func (s *PGStore) SetProfileTestResult(ctx context.Context, profileID string, gradeAfter model.Grade, testStatus model.TestStatus, lastTestDate, cooldownUntil time.Time, lastTestScore float64) error {
	_, err := s.db.Pool.Exec(ctx, `
		UPDATE candidate_profiles
		SET verified_grade = $1::"Grade",
		    test_status = $2::"TestStatus",
		    last_test_date = $3,
		    test_cooldown_until = $4,
		    last_test_score = $5,
		    updated_at = $6
		WHERE id = $7`,
		string(gradeAfter), string(testStatus), model.FromTime(lastTestDate),
		model.FromTime(cooldownUntil), lastTestScore, model.Now(), profileID)
	return err
}

// ===== Сессии =====

const sessionColumns = `id, candidate_id, category_id, status, started_at, completed_at,
       time_limit_minutes, score, raw_score, max_score, grade_before, grade_after,
       recommendation, cheat_flags, is_suspicious, current_difficulty, theta_estimate`

func scanSession(row pgx.Row) (*Session, error) {
	var s Session
	var status string
	var startedAt, completedAt *time.Time
	var gradeBefore, gradeAfter *string
	if err := row.Scan(&s.ID, &s.CandidateID, &s.CategoryID, &status, &startedAt, &completedAt,
		&s.TimeLimitMinutes, &s.Score, &s.RawScore, &s.MaxScore, &gradeBefore, &gradeAfter,
		&s.Recommendation, &s.CheatFlags, &s.IsSuspicious, &s.CurrentDifficulty, &s.ThetaEstimate); err != nil {
		return nil, err
	}
	s.Status = model.SessionStatus(status)
	if startedAt != nil {
		s.StartedAt = model.FromTime(*startedAt)
	}
	if completedAt != nil {
		mt := model.FromTime(*completedAt)
		s.CompletedAt = &mt
	}
	if gradeBefore != nil {
		v := model.Grade(*gradeBefore)
		s.GradeBefore = &v
	}
	if gradeAfter != nil {
		v := model.Grade(*gradeAfter)
		s.GradeAfter = &v
	}
	if s.CheatFlags == nil {
		s.CheatFlags = []string{}
	}
	s.Answers = []SessionAnswer{}
	return &s, nil
}

func (s *PGStore) FindActiveSession(ctx context.Context, candidateID string) (*Session, error) {
	row := s.db.Pool.QueryRow(ctx, `
		SELECT `+sessionColumns+`
		FROM test_sessions
		WHERE candidate_id = $1 AND status = 'IN_PROGRESS'
		ORDER BY started_at DESC
		LIMIT 1`, candidateID)
	sess, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *PGStore) FindSessionWithAnswers(ctx context.Context, sessionID string) (*Session, error) {
	row := s.db.Pool.QueryRow(ctx, `
		SELECT `+sessionColumns+`
		FROM test_sessions
		WHERE id = $1`, sessionID)
	sess, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Pool.Query(ctx, `
		SELECT a.id, a.session_id, a.question_id, a.answer_text, a."isCorrect", a.score,
		       a.time_spent_seconds, a.answered_at,
		       q.id, q.question_type, q.text, q.code_snippet, q.options, q.correct_answer,
		       q.explanation, q.difficulty, q.discrimination, q.guessing, q.is_unique, q.pool_tag
		FROM test_answers a
		JOIN test_questions q ON q.id = a.question_id
		WHERE a.session_id = $1
		ORDER BY a.answered_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var a SessionAnswer
		var isCorrect *bool
		var answeredAt *time.Time
		if err := rows.Scan(&a.ID, &a.SessionID, &a.QuestionID, &a.AnswerText, &isCorrect, &a.Score,
			&a.TimeSpentSeconds, &answeredAt,
			&a.Question.ID, &a.Question.QuestionType, &a.Question.Text, &a.Question.CodeSnippet,
			&a.Question.Options, &a.Question.CorrectAnswer, &a.Question.Explanation,
			&a.Question.Difficulty, &a.Question.Discrimination, &a.Question.Guessing,
			&a.Question.IsUnique, &a.Question.PoolTag); err != nil {
			return nil, err
		}
		a.IsCorrect = isCorrect
		if answeredAt != nil {
			a.AnsweredAt = model.FromTime(*answeredAt)
		}
		sess.Answers = append(sess.Answers, a)
	}
	return sess, rows.Err()
}

func (s *PGStore) CreateSession(ctx context.Context, session *model.TestSession) error {
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO test_sessions (`+sessionColumns+`)
		VALUES ($1, $2, $3, $4::"SessionStatus", $5, $6, $7, $8, $9, $10,
		        $11::"Grade", $12::"Grade", $13, $14, $15, $16, $17)`,
		session.ID, session.CandidateID, session.CategoryID, string(session.Status),
		session.StartedAt, session.CompletedAt, session.TimeLimitMinutes,
		session.Score, session.RawScore, session.MaxScore,
		nullableGrade(session.GradeBefore), nullableGrade(session.GradeAfter),
		session.Recommendation, session.CheatFlags, session.IsSuspicious,
		session.CurrentDifficulty, session.ThetaEstimate)
	return err
}

func (s *PGStore) UpdateSession(ctx context.Context, session *model.TestSession) error {
	_, err := s.db.Pool.Exec(ctx, `
		UPDATE test_sessions
		SET status = $1::"SessionStatus",
		    completed_at = $2,
		    score = $3,
		    raw_score = $4,
		    max_score = $5,
		    grade_before = $6::"Grade",
		    grade_after = $7::"Grade",
		    recommendation = $8,
		    cheat_flags = $9,
		    is_suspicious = $10,
		    current_difficulty = $11,
		    theta_estimate = $12
		WHERE id = $13`,
		string(session.Status), session.CompletedAt, session.Score, session.RawScore,
		session.MaxScore, nullableGrade(session.GradeBefore), nullableGrade(session.GradeAfter),
		session.Recommendation, session.CheatFlags, session.IsSuspicious,
		session.CurrentDifficulty, session.ThetaEstimate, session.ID)
	return err
}

func (s *PGStore) CreateAnswer(ctx context.Context, answer *model.TestAnswer) error {
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO test_answers (id, session_id, question_id, answer_text, "isCorrect", score, time_spent_seconds, answered_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		answer.ID, answer.SessionID, answer.QuestionID, answer.AnswerText,
		answer.IsCorrect, answer.Score, answer.TimeSpentSeconds, answer.AnsweredAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		// unique (session_id, question_id)
		return ErrDuplicateAnswer
	}
	return err
}

func nullableGrade(g *model.Grade) *string {
	if g == nil {
		return nil
	}
	v := string(*g)
	return &v
}
