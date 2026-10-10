package vacancies

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
	"fsp-backend/internal/store"
)

// Service — vacancies.service.ts.
type Service struct {
	db *store.DB
}

// NewService — конструктор для интегратора.
func NewService(db *store.DB) *Service { return &Service{db: db} }

// vacancyColumns — выборка строки вакансии (без join).
const vacancyColumns = `v.id, v.employer_id, v.title, v.description, v.specialization, v.grade,
		       v.stack, v.salary_from, v.salary_to, v.is_active, v.created_at, v.updated_at`

// scanVacancy — скан строки вакансии в model.Vacancy.
func scanVacancy(row pgx.Row, v *model.Vacancy) error {
	return row.Scan(
		&v.ID, &v.EmployerID, &v.Title, &v.Description, &v.Specialization, &v.Grade,
		&v.Stack, &v.SalaryFrom, &v.SalaryTo, &v.IsActive, &v.CreatedAt, &v.UpdatedAt,
	)
}

// requireEmployer — employerProfile.findUnique({where: {userId}}).
func (s *Service) requireEmployer(ctx context.Context, userID string) (string, error) {
	var id string
	err := s.db.Pool.QueryRow(ctx,
		`SELECT id FROM employer_profiles WHERE user_id = $1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", common.NotFound("Профиль работодателя не найден")
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

// requireOwnVacancy — вакансия существует и принадлежит работодателю;
// и то, и другое нарушается одним и тем же 403 (requireOwnVacancy в TS).
func (s *Service) requireOwnVacancy(ctx context.Context, employerID, vacancyID string) error {
	var owner string
	err := s.db.Pool.QueryRow(ctx,
		`SELECT employer_id FROM vacancies WHERE id = $1`, vacancyID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return common.Forbidden("Это не ваша вакансия")
	}
	if err != nil {
		return err
	}
	if owner != employerID {
		return common.Forbidden("Это не ваша вакансия")
	}
	return nil
}

// List — GET /vacancies (публичный список с фильтрами).
func (s *Service) List(ctx context.Context, q *ListQuery) (*ListResult, error) {
	where := []string{"TRUE"}
	args := []any{}
	idx := 1
	if q.Specialization != nil {
		where = append(where, fmt.Sprintf("v.specialization = $%d", idx))
		args = append(args, *q.Specialization)
		idx++
	}
	if q.Grade != nil {
		where = append(where, fmt.Sprintf("v.grade = $%d", idx))
		args = append(args, *q.Grade)
		idx++
	}
	if q.EmployerID != nil {
		where = append(where, fmt.Sprintf("v.employer_id = $%d", idx))
		args = append(args, *q.EmployerID)
		idx++
	}
	// По умолчанию только активные; onlyActive=false отключает фильтр.
	if q.OnlyActive {
		where = append(where, "v.is_active = true")
	}
	if q.Search != nil {
		// Prisma contains + mode: 'insensitive' → ILIKE с экранированием %/_.
		where = append(where, fmt.Sprintf("(v.title ILIKE $%d OR v.description ILIKE $%d)", idx, idx))
		args = append(args, ilikePattern(*q.Search))
		idx++
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM vacancies v WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, err
	}

	listArgs := append(append([]any{}, args...), q.Limit, q.Offset)
	rows, err := s.db.Pool.Query(ctx, `
		SELECT `+vacancyColumns+`,
		       e.id, e.company_name, e.logo_url, e.industry
		FROM vacancies v
		JOIN employer_profiles e ON e.id = v.employer_id
		WHERE `+cond+`
		ORDER BY v.created_at DESC
		OFFSET $`+fmt.Sprint(idx+1)+` LIMIT $`+fmt.Sprint(idx),
		listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []VacancyWithEmployer{}
	for rows.Next() {
		var item VacancyWithEmployer
		if err := rows.Scan(
			&item.ID, &item.EmployerID, &item.Title, &item.Description, &item.Specialization, &item.Grade,
			&item.Stack, &item.SalaryFrom, &item.SalaryTo, &item.IsActive, &item.CreatedAt, &item.UpdatedAt,
			&item.Employer.ID, &item.Employer.CompanyName, &item.Employer.LogoURL, &item.Employer.Industry,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ListResult{Total: total, Vacancies: items}, nil
}

// ilikePattern — экранирование LIKE-спецсимволов (Prisma экранирует их в contains).
func ilikePattern(search string) string {
	r := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_")
	return "%" + r.Replace(search) + "%"
}

// Get — GET /vacancies/:id (вакансия + работодатель, включая website).
func (s *Service) Get(ctx context.Context, id string) (*VacancyWithEmployerFull, error) {
	var v VacancyWithEmployerFull
	err := scanVacancy(s.db.Pool.QueryRow(ctx,
		`SELECT `+vacancyColumns+` FROM vacancies v WHERE v.id = $1`, id), &v.Vacancy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, common.NotFound("Вакансия не найдена")
	}
	if err != nil {
		return nil, err
	}
	if err := s.db.Pool.QueryRow(ctx,
		`SELECT id, company_name, logo_url, industry, website FROM employer_profiles WHERE id = $1`,
		v.EmployerID).Scan(&v.Employer.ID, &v.Employer.CompanyName, &v.Employer.LogoURL,
		&v.Employer.Industry, &v.Employer.Website); err != nil {
		return nil, err
	}
	return &v, nil
}

// Create — POST /vacancies: проверка salaryFrom ≤ salaryTo только здесь (docs 04 §4).
func (s *Service) Create(ctx context.Context, userID string, dto *CreateVacancyDto) (*VacancyWithEmployerShort, error) {
	employerID, err := s.requireEmployer(ctx, userID)
	if err != nil {
		return nil, err
	}
	data := dto.Value()
	if data.SalaryFrom > data.SalaryTo {
		return nil, common.BadRequest("salaryFrom не может быть больше salaryTo")
	}

	id := uuid.NewString()
	now := model.Now()
	if _, err := s.db.Pool.Exec(ctx, `
		INSERT INTO vacancies (id, employer_id, title, description, specialization, grade, stack,
		                       salary_from, salary_to, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)`,
		id, employerID, data.Title, data.Description, data.Specialization, data.Grade, data.Stack,
		data.SalaryFrom, data.SalaryTo, data.IsActive, now); err != nil {
		return nil, err
	}

	var out VacancyWithEmployerShort
	if err := scanVacancy(s.db.Pool.QueryRow(ctx,
		`SELECT `+vacancyColumns+` FROM vacancies v WHERE v.id = $1`, id), &out.Vacancy); err != nil {
		return nil, err
	}
	if err := s.db.Pool.QueryRow(ctx,
		`SELECT id, company_name, logo_url FROM employer_profiles WHERE id = $1`,
		employerID).Scan(&out.Employer.ID, &out.Employer.CompanyName, &out.Employer.LogoURL); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update — PUT /vacancies/:id: владелец (403), затем частичное обновление.
// BUG-REF: S5 — проверка salaryFrom ≤ salaryTo при обновлении НЕ выполняется
// (как в vacancies.service.ts → update): прислать salaryFrom > salaryTo можно.
func (s *Service) Update(ctx context.Context, userID, id string, dto *UpdateVacancyDto) (*model.Vacancy, error) {
	employerID, err := s.requireEmployer(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := s.requireOwnVacancy(ctx, employerID, id); err != nil {
		return nil, err
	}

	sets := []string{"updated_at = $1"}
	args := []any{model.Now()}
	idx := 2
	for _, u := range dto.Updates() {
		sets = append(sets, fmt.Sprintf("%s = $%d", u.Column, idx))
		args = append(args, u.Value)
		idx++
	}
	args = append(args, id)
	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE vacancies SET `+strings.Join(sets, ", ")+` WHERE id = $`+fmt.Sprint(idx),
		args...); err != nil {
		return nil, err
	}

	out := &model.Vacancy{}
	if err := scanVacancy(s.db.Pool.QueryRow(ctx,
		`SELECT `+vacancyColumns+` FROM vacancies v WHERE v.id = $1`, id), out); err != nil {
		return nil, err
	}
	return out, nil
}

// Remove — DELETE /vacancies/:id: жёсткое удаление; приглашения остаются
// с vacancyId = null (onDelete: SetNull в schema.prisma).
func (s *Service) Remove(ctx context.Context, userID, id string) (*DeleteVacancyResult, error) {
	employerID, err := s.requireEmployer(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := s.requireOwnVacancy(ctx, employerID, id); err != nil {
		return nil, err
	}
	if _, err := s.db.Pool.Exec(ctx, `DELETE FROM vacancies WHERE id = $1`, id); err != nil {
		return nil, err
	}
	return &DeleteVacancyResult{Deleted: true}, nil
}
