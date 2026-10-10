package invitations

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
	"fsp-backend/internal/modules/candidateview"
	"fsp-backend/internal/store"
)

func newUUID() string { return uuid.NewString() }

// MatchingAPI — matching-движок (реализует *matching.Service).
type MatchingAPI interface {
	InvalidateCache(ctx context.Context) error
}

// Service — invitations.service.ts.
type Service struct {
	db       *store.DB
	matching MatchingAPI
}

func NewService(db *store.DB, m MatchingAPI) *Service {
	return &Service{db: db, matching: m}
}

// ===== Вспомогательные =====

const invitationColumns = `id, employer_id, candidate_id, vacancy_id, message,
	salary_from, salary_to, status, status_updated_at, created_at, updated_at`

func scanInvitation(row interface{ Scan(...any) error }) (*model.Invitation, error) {
	var inv model.Invitation
	err := row.Scan(&inv.ID, &inv.EmployerID, &inv.CandidateID, &inv.VacancyID, &inv.Message,
		&inv.SalaryFrom, &inv.SalaryTo, &inv.Status, &inv.StatusUpdatedAt,
		&inv.CreatedAt, &inv.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// entityView — приглашение как у Prisma (все скалярные поля).
func entityView(inv *model.Invitation) map[string]any {
	return map[string]any{
		"id":              inv.ID,
		"employerId":      inv.EmployerID,
		"candidateId":     inv.CandidateID,
		"vacancyId":       inv.VacancyID,
		"message":         inv.Message,
		"salaryFrom":      inv.SalaryFrom,
		"salaryTo":        inv.SalaryTo,
		"status":          inv.Status,
		"statusUpdatedAt": inv.StatusUpdatedAt,
		"createdAt":       inv.CreatedAt,
		"updatedAt":       inv.UpdatedAt,
	}
}

func (s *Service) employerProfile(ctx context.Context, userID string) (*model.EmployerProfile, error) {
	var id string
	err := s.db.Pool.QueryRow(ctx, `SELECT id FROM employer_profiles WHERE user_id = $1`, userID).Scan(&id)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, common.NotFound("Профиль работодателя не найден")
		}
		return nil, err
	}
	return &model.EmployerProfile{ID: id}, nil
}

func (s *Service) candidateProfile(ctx context.Context, userID string) (*model.CandidateProfile, error) {
	var id string
	err := s.db.Pool.QueryRow(ctx, `SELECT id FROM candidate_profiles WHERE user_id = $1`, userID).Scan(&id)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, common.NotFound("Профиль кандидата не найден")
		}
		return nil, err
	}
	return &model.CandidateProfile{ID: id}, nil
}

// scopeWhere — WHERE по роли: кандидат видит входящие, работодатель — исходящие.
func (s *Service) scopeWhere(ctx context.Context, userID string, role model.UserRole) (column string, profileID string, err error) {
	if role == model.RoleCandidate {
		p, err := s.candidateProfile(ctx, userID)
		if err != nil {
			return "", "", err
		}
		return "candidate_id", p.ID, nil
	}
	p, err := s.employerProfile(ctx, userID)
	if err != nil {
		return "", "", err
	}
	return "employer_id", p.ID, nil
}

// ===== POST /invitations =====

// Create — работодатель приглашает кандидата (EMPLOYER-first).
func (s *Service) Create(ctx context.Context, userID string, dto *CreateInvitationDto) (map[string]any, error) {
	employer, err := s.employerProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	var candidateID string
	err = s.db.Pool.QueryRow(ctx, `SELECT id FROM candidate_profiles WHERE id = $1`, dto.CandidateID()).Scan(&candidateID)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, common.NotFound("Кандидат не найден")
		}
		return nil, err
	}

	if vacancyID := dto.VacancyID(); vacancyID != nil {
		var found string
		err = s.db.Pool.QueryRow(ctx,
			`SELECT id FROM vacancies WHERE id = $1 AND employer_id = $2`,
			*vacancyID, employer.ID).Scan(&found)
		if err != nil {
			if err.Error() == "no rows in result set" {
				return nil, common.BadRequest("Вакансия не найдена у этого работодателя")
			}
			return nil, err
		}
	}

	var duplicate string
	err = s.db.Pool.QueryRow(ctx, `
		SELECT id FROM invitations
		WHERE employer_id = $1 AND candidate_id = $2
		  AND status IN ('SENT', 'VIEWED', 'ACCEPTED')
		LIMIT 1`, employer.ID, candidateID).Scan(&duplicate)
	if err == nil {
		return nil, common.Conflict("Кандидат уже приглашён по этому приглашению")
	}
	if err.Error() != "no rows in result set" {
		return nil, err
	}

	salaryFrom, salaryTo := dto.Salary()
	if salaryFrom > salaryTo {
		return nil, common.BadRequest("salaryFrom не может быть больше salaryTo")
	}

	now := model.Now()
	inv, err := scanInvitation(s.db.Pool.QueryRow(ctx, `
		INSERT INTO invitations (id, employer_id, candidate_id, vacancy_id, message,
		                         salary_from, salary_to, status, status_updated_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'SENT', $8, $8, $8)
		RETURNING `+invitationColumns,
		newUUID(), employer.ID, candidateID, dto.VacancyID(), dto.Message(),
		salaryFrom, salaryTo, now))
	if err != nil {
		return nil, err
	}

	// await this.matching.invalidateCache().catch(() => undefined)
	_ = s.matching.InvalidateCache(ctx)

	view := entityView(inv)
	cand, err := candidateview.Load(ctx, s.db, candidateID)
	if err != nil {
		return nil, err
	}
	if cand != nil {
		view["candidate"] = candidateview.ToPublic(cand, true)
	}
	return view, nil
}

// ===== GET /invitations =====

// List — список приглашений (≤100, скоуп по роли).
func (s *Service) List(ctx context.Context, userID string, role model.UserRole, status string) (map[string]any, error) {
	column, profileID, err := s.scopeWhere(ctx, userID, role)
	if err != nil {
		return nil, err
	}

	query := `SELECT ` + invitationColumns + ` FROM invitations WHERE ` + column + ` = $1`
	args := []any{profileID}
	if status != "" {
		query += ` AND status = $2`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC LIMIT 100`

	rows, err := s.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		inv, err := scanInvitation(rows)
		if err != nil {
			return nil, err
		}
		item := map[string]any{
			"id":              inv.ID,
			"status":          inv.Status,
			"statusUpdatedAt": inv.StatusUpdatedAt,
			"createdAt":       inv.CreatedAt,
			"message":         inv.Message,
			"salaryFrom":      inv.SalaryFrom,
			"salaryTo":        inv.SalaryTo,
			"vacancy":         nil,
		}
		if inv.VacancyID != nil {
			var vID, vTitle string
			var vGrade model.Grade
			if err := s.db.Pool.QueryRow(ctx,
				`SELECT id, title, grade FROM vacancies WHERE id = $1`, *inv.VacancyID).
				Scan(&vID, &vTitle, &vGrade); err == nil {
				item["vacancy"] = map[string]any{"id": vID, "title": vTitle, "grade": vGrade}
			}
		}
		if role == model.RoleCandidate {
			var eID, eName string
			var eIndustry, eLogo *string
			if err := s.db.Pool.QueryRow(ctx,
				`SELECT id, company_name, industry, logo_url FROM employer_profiles WHERE id = $1`, inv.EmployerID).
				Scan(&eID, &eName, &eIndustry, &eLogo); err == nil {
				item["employer"] = map[string]any{
					"id": eID, "companyName": eName, "industry": eIndustry, "logoUrl": eLogo,
				}
			}
		}
		if role == model.RoleEmployer {
			cand, err := candidateview.Load(ctx, s.db, inv.CandidateID)
			if err != nil {
				return nil, err
			}
			if cand != nil {
				item["candidate"] = candidateview.ToPublic(cand, true)
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"invitations": items}, nil
}

// ===== GET /invitations/:id =====

// Get — детали приглашения; кандидат, открывший SENT-приглашение, помечает его VIEWED.
func (s *Service) Get(ctx context.Context, userID string, role model.UserRole, id string) (map[string]any, error) {
	column, profileID, err := s.scopeWhere(ctx, userID, role)
	if err != nil {
		return nil, err
	}

	inv, err := scanInvitation(s.db.Pool.QueryRow(ctx,
		`SELECT `+invitationColumns+` FROM invitations WHERE id = $1 AND `+column+` = $2`,
		id, profileID))
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, common.NotFound("Приглашение не найдено")
		}
		return nil, err
	}

	// Отметка «просмотрено» — входящее приглашение кандидатом.
	if role == model.RoleCandidate && inv.Status == model.InvSent {
		now := model.Now()
		if _, err := s.db.Pool.Exec(ctx,
			`UPDATE invitations SET status = 'VIEWED', status_updated_at = $1, updated_at = $1 WHERE id = $2`,
			now, inv.ID); err != nil {
			return nil, err
		}
		inv.Status = model.InvViewed
		inv.StatusUpdatedAt = now
		inv.UpdatedAt = now
	}

	accepted := inv.Status == model.InvAccepted

	view := map[string]any{
		"id":              inv.ID,
		"status":          inv.Status,
		"message":         inv.Message,
		"salaryFrom":      inv.SalaryFrom,
		"salaryTo":        inv.SalaryTo,
		"createdAt":       inv.CreatedAt,
		"statusUpdatedAt": inv.StatusUpdatedAt,
		"vacancy":         nil,
		"employer":        nil,
	}

	if inv.VacancyID != nil {
		var v model.Vacancy
		err := s.db.Pool.QueryRow(ctx, `
			SELECT id, employer_id, title, description, specialization, grade, stack,
			       salary_from, salary_to, is_active, created_at, updated_at
			FROM vacancies WHERE id = $1`, *inv.VacancyID).Scan(
			&v.ID, &v.EmployerID, &v.Title, &v.Description, &v.Specialization, &v.Grade, &v.Stack,
			&v.SalaryFrom, &v.SalaryTo, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
		if err == nil {
			view["vacancy"] = v
		} else if err.Error() != "no rows in result set" {
			return nil, err
		}
	}

	var eID, eName string
	var eIndustry, eLogo, eWebsite *string
	if err := s.db.Pool.QueryRow(ctx,
		`SELECT id, company_name, industry, logo_url, website FROM employer_profiles WHERE id = $1`,
		inv.EmployerID).Scan(&eID, &eName, &eIndustry, &eLogo, &eWebsite); err == nil {
		view["employer"] = map[string]any{
			"id": eID, "companyName": eName, "industry": eIndustry,
			"logoUrl": eLogo, "website": eWebsite,
		}
	}

	if role == model.RoleEmployer {
		cand, err := candidateview.Load(ctx, s.db, inv.CandidateID)
		if err != nil {
			return nil, err
		}
		if cand != nil {
			// BUG-REF: S1 — showEmail/showPhone при раскрытии контактов не проверяются.
			candidateView := candidateview.ToPublic(cand, true)
			if accepted {
				var email any
				if cand.Email != nil {
					email = *cand.Email
				}
				candidateView["contacts"] = map[string]any{"email": email, "phone": cand.Phone}
			} else {
				candidateView["contacts"] = nil
			}
			candidateView["contactsLocked"] = !accepted
			view["candidate"] = candidateView
		}
	}
	return view, nil
}

// ===== POST /invitations/:id/accept|reject =====

// Respond — принять/отклонить (только кандидат).
func (s *Service) Respond(ctx context.Context, userID, id string, accept bool) (map[string]any, error) {
	profile, err := s.candidateProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	inv, err := scanInvitation(s.db.Pool.QueryRow(ctx,
		`SELECT `+invitationColumns+` FROM invitations WHERE id = $1`, id))
	if err != nil {
		if err.Error() == "no rows in result set" || inv == nil {
			return nil, common.NotFound("Приглашение не найдено")
		}
		return nil, err
	}
	if inv.CandidateID != profile.ID {
		return nil, common.NotFound("Приглашение не найдено")
	}
	if inv.Status != model.InvSent && inv.Status != model.InvViewed {
		return nil, common.Conflict(fmt.Sprintf("Приглашение уже обработано: %s", inv.Status))
	}

	status := model.InvRejected
	if accept {
		status = model.InvAccepted
	}
	now := model.Now()
	inv.Status = status
	inv.StatusUpdatedAt = now
	inv.UpdatedAt = now
	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE invitations SET status = $1, status_updated_at = $2, updated_at = $2 WHERE id = $3`,
		status, now, id); err != nil {
		return nil, err
	}

	// await this.matching.invalidateCache().catch(() => undefined)
	_ = s.matching.InvalidateCache(ctx)

	view := entityView(inv)
	cand, err := candidateview.Load(ctx, s.db, inv.CandidateID)
	if err != nil {
		return nil, err
	}
	if cand != nil {
		view["candidate"] = candidateview.ToPublic(cand, true)
	}
	if accept {
		view["message"] = "Приглашение принято — работодатель получил ваши контакты"
	} else {
		view["message"] = "Приглашение отклонено"
	}
	return view, nil
}

// ===== POST /invitations/:id/withdraw =====

// Withdraw — отзыв приглашения (только работодатель, нельзя отозвать ACCEPTED).
func (s *Service) Withdraw(ctx context.Context, userID, id string) (map[string]any, error) {
	employer, err := s.employerProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	inv, err := scanInvitation(s.db.Pool.QueryRow(ctx,
		`SELECT `+invitationColumns+` FROM invitations WHERE id = $1`, id))
	if err != nil {
		if err.Error() == "no rows in result set" || inv == nil {
			return nil, common.NotFound("Приглашение не найдено")
		}
		return nil, err
	}
	if inv.EmployerID != employer.ID {
		return nil, common.Forbidden("Это не ваше приглашение")
	}
	if inv.Status == model.InvAccepted {
		return nil, common.Conflict("Принятое приглашение нельзя отозвать")
	}

	now := model.Now()
	inv.Status = model.InvWithdrawn
	inv.StatusUpdatedAt = now
	inv.UpdatedAt = now
	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE invitations SET status = 'WITHDRAWN', status_updated_at = $1, updated_at = $1 WHERE id = $2`,
		now, id); err != nil {
		return nil, err
	}
	return entityView(inv), nil
}
