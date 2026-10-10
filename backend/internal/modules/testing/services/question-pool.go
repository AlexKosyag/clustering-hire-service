package services

import (
	"context"
	"encoding/json"

	"fsp-backend/internal/model"
)

// Category — строка test_categories.
type Category struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Description    *string              `json:"description"`
	Specialization model.Specialization `json:"specialization"`
}

// PoolQuestion — вопрос банка (вся строка test_questions с IRT-параметрами).
type PoolQuestion struct {
	ID            string
	QuestionType  string
	Text          string
	CodeSnippet   *string
	Options       json.RawMessage // JSON из БД: null/[]/[...] (nil = null)
	CorrectAnswer *string
	Explanation   *string

	Difficulty     float64
	Discrimination float64
	Guessing       float64

	PoolTag  *string
	IsUnique bool
}

// IrtParams — IRT-параметры для CAT.
func (q PoolQuestion) IrtParams() IrtQuestion {
	return IrtQuestion{
		ID:             q.ID,
		Difficulty:     q.Difficulty,
		Discrimination: q.Discrimination,
		Guessing:       q.Guessing,
	}
}

// ParsedOptions разбирает Options в []model.QuestionOption
// (nil при JSON null / отсутствии / ошибке разбора).
func (q PoolQuestion) ParsedOptions() []model.QuestionOption {
	if isJSONNullOrAbsent(q.Options) {
		return nil
	}
	var out []model.QuestionOption
	if err := json.Unmarshal(q.Options, &out); err != nil {
		return nil
	}
	return out
}

// HasOptions — заданы ли options (в JS: !!question.options; null/undefined → false).
func (q PoolQuestion) HasOptions() bool {
	return !isJSONNullOrAbsent(q.Options)
}

func isJSONNullOrAbsent(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}

// PoolStore — доступ к банку вопросов (реализация: PGStore в пакете testing,
// в тестах — in-memory).
type PoolStore interface {
	// ListQuestions — вся категория (orderBy: id asc).
	ListQuestions(ctx context.Context, categoryID string) ([]PoolQuestion, error)
	// FindCategoryBySpecialization — категория по специализации (findFirst).
	FindCategoryBySpecialization(ctx context.Context, spec model.Specialization) (*Category, error)
	// FindAnyCategory — fallback: ЛЮБАЯ категория (orderBy: name asc).
	FindAnyCategory(ctx context.Context) (*Category, error)
}

// QuestionPool — банк вопросов: загрузка пула по категории специализации.
type QuestionPool struct {
	store PoolStore
}

// NewQuestionPool — конструктор (в TS — QuestionPoolService(prisma)).
func NewQuestionPool(store PoolStore) *QuestionPool { return &QuestionPool{store: store} }

// PoolFor — все вопросы категории.
func (p *QuestionPool) PoolFor(ctx context.Context, categoryID string) ([]PoolQuestion, error) {
	return p.store.ListQuestions(ctx, categoryID)
}

// CategoryForSpecialization — категория по специализации.
// BUG-REF: T8 — при отсутствии категории специализации берётся ЛЮБАЯ категория
// (orderBy: name asc) — кандидат может получить тест по чужой специализации.
func (p *QuestionPool) CategoryForSpecialization(ctx context.Context, spec model.Specialization) (*Category, error) {
	bySpec, err := p.store.FindCategoryBySpecialization(ctx, spec)
	if err != nil {
		return nil, err
	}
	if bySpec != nil {
		return bySpec, nil
	}
	return p.store.FindAnyCategory(ctx)
}
