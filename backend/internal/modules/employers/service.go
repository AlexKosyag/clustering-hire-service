package employers

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
	"fsp-backend/internal/modules/candidateview"
	"fsp-backend/internal/modules/matching"
	"fsp-backend/internal/store"
)

// MatchingAPI — matching-движок (реализует *matching.Service).
type MatchingAPI interface {
	InvalidateCache(ctx context.Context) error
	Search(ctx context.Context, q matching.SearchCandidatesDTO) (map[string]any, error)
	CreateNeed(ctx context.Context, employerProfileID string, dto matching.EmployerNeedDTO) (map[string]any, error)
}

// StorageAPI — файловое хранилище (реализует *storage.Service).
type StorageAPI interface {
	Upload(ctx context.Context, kind, owner, filename string, buf []byte, contentType string) (string, error)
}

// Service — employers.service.ts.
type Service struct {
	db       *store.DB
	matching MatchingAPI
	storage  StorageAPI
}

func NewService(db *store.DB, m MatchingAPI, s StorageAPI) *Service {
	return &Service{db: db, matching: m, storage: s}
}

// ===== Профиль =====

func (s *Service) requireProfile(ctx context.Context, userID string) (*model.EmployerProfile, error) {
	var p model.EmployerProfile
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, user_id, company_name, company_description, industry, website, logo_url,
		       contact_person, contact_email, contact_phone, is_verified, verified_at,
		       created_at, updated_at
		FROM employer_profiles WHERE user_id = $1`, userID).Scan(
		&p.ID, &p.UserID, &p.CompanyName, &p.CompanyDescription, &p.Industry, &p.Website, &p.LogoURL,
		&p.ContactPerson, &p.ContactEmail, &p.ContactPhone, &p.IsVerified, &p.VerifiedAt,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, common.NotFound("Профиль работодателя не найден")
		}
		return nil, err
	}
	return &p, nil
}

// profileView — {id, ...safe, user: undefined}: userId в ответ не попадает.
func profileView(p *model.EmployerProfile) map[string]any {
	return map[string]any{
		"id":                 p.ID,
		"companyName":        p.CompanyName,
		"companyDescription": p.CompanyDescription,
		"industry":           p.Industry,
		"website":            p.Website,
		"logoUrl":            p.LogoURL,
		"contactPerson":      p.ContactPerson,
		"contactEmail":       p.ContactEmail,
		"contactPhone":       p.ContactPhone,
		"isVerified":         p.IsVerified,
		"verifiedAt":         p.VerifiedAt,
		"createdAt":          p.CreatedAt,
		"updatedAt":          p.UpdatedAt,
	}
}

// Profile — GET /employers/profile.
func (s *Service) Profile(ctx context.Context, userID string) (map[string]any, error) {
	p, err := s.requireProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return profileView(p), nil
}

// UpdateProfile — PUT /employers/profile: обновляются только присланные ключи;
// null обнуляет колонку (value !== undefined в TS).
func (s *Service) UpdateProfile(ctx context.Context, userID string, dto *UpdateEmployerProfileDto) (map[string]any, error) {
	if _, err := s.requireProfile(ctx, userID); err != nil {
		return nil, err
	}

	sets := []string{"updated_at = $1"}
	args := []any{model.Now()}
	idx := 2
	for _, f := range employerProfileFields {
		fv, ok := dto.fields[f.key]
		if !ok {
			continue
		}
		sets = append(sets, fmt.Sprintf("%s = $%d", f.column, idx))
		args = append(args, fv.value) // *string(nil) → NULL
		idx++
	}
	args = append(args, userID)
	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE employer_profiles SET `+strings.Join(sets, ", ")+` WHERE user_id = $`+fmt.Sprint(idx),
		args...); err != nil {
		return nil, err
	}

	// await this.matching.invalidateCache().catch(() => undefined)
	if err := s.matching.InvalidateCache(ctx); err != nil {
		// проглатываем — как .catch(() => undefined)
		_ = err
	}
	return s.Profile(ctx, userID)
}

// UploadLogo — POST /employers/profile/logo.
func (s *Service) UploadLogo(ctx context.Context, userID string, file *multipart.FileHeader) (map[string]any, error) {
	if file == nil {
		return nil, common.BadRequest("Файл не передан")
	}
	mimetype := file.Header.Get("Content-Type")
	if !strings.HasPrefix(mimetype, "image/") {
		return nil, common.BadRequest("Нужно изображение")
	}
	if file.Size > 2*1024*1024 {
		return nil, common.BadRequest("Максимальный размер — 2 МБ")
	}

	profile, err := s.requireProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	// (String(file.mimetype).split('/')[1] || 'png').replace('jpeg', 'jpg')
	ext := "png"
	if parts := strings.Split(mimetype, "/"); len(parts) > 1 && parts[1] != "" {
		ext = parts[1]
	}
	ext = strings.ReplaceAll(ext, "jpeg", "jpg")

	url, err := s.storage.Upload(ctx, "logos", profile.ID, "logo."+ext, buf, mimetype)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE employer_profiles SET logo_url = $1, updated_at = $2 WHERE id = $3`,
		url, model.Now(), profile.ID); err != nil {
		return nil, err
	}
	return map[string]any{"logoUrl": url}, nil
}

// ===== Потребность и поиск =====

// CreateNeed — POST /employers/need → matching.createNeed(profile.id, dto).
func (s *Service) CreateNeed(ctx context.Context, userID string, dto *CreateNeedDto) (map[string]any, error) {
	profile, err := s.requireProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.matching.CreateNeed(ctx, profile.ID, matching.EmployerNeedDTO{
		Description:    dto.Description,
		Specialization: dto.Specialization,
		Grade:          dto.Grade,
		Stack:          dto.Stack,
	})
}

// SearchCandidates — GET /employers/candidates/search → matching.search(query).
func (s *Service) SearchCandidates(ctx context.Context, userID string, q *EmployerSearchQueryDto) (map[string]any, error) {
	if _, err := s.requireProfile(ctx, userID); err != nil {
		return nil, err
	}
	var spec, grade *string
	if q.Specialization != nil {
		s := string(*q.Specialization)
		spec = &s
	}
	if q.Grade != nil {
		g := string(*q.Grade)
		grade = &g
	}
	return s.matching.Search(ctx, matching.SearchCandidatesDTO{
		Description:     q.Description,
		Specialization:  spec,
		Grade:           grade,
		Stack:           q.Stack,
		HasAchievements: q.HasAchievements,
		Limit:           q.Limit,
		Offset:          q.Offset,
	})
}

// ===== Карточка кандидата (гейтинг контактов) =====

// CandidateDetail — GET /employers/candidates/:id.
// Контакты раскрываются ТОЛЬКО после ACCEPTED; BUG-REF: S1 — showEmail/showPhone
// и showFullName здесь не проверяются (как в TS).
func (s *Service) CandidateDetail(ctx context.Context, userID, candidateID string) (map[string]any, error) {
	employer, err := s.requireProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	candidate, err := candidateview.Load(ctx, s.db, candidateID)
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, common.NotFound("Кандидат не найден")
	}

	var status *model.InvitationStatus
	err = s.db.Pool.QueryRow(ctx, `
		SELECT status FROM invitations
		WHERE employer_id = $1 AND candidate_id = $2
		ORDER BY created_at DESC LIMIT 1`, employer.ID, candidate.ID).Scan(&status)
	if err != nil && err.Error() != "no rows in result set" {
		return nil, err
	}

	accepted := status != nil && *status == model.InvAccepted

	view := candidateview.ToPublic(candidate, true)
	view["email"] = candidate.Email // candidate.user?.email ?? null
	if status != nil {
		view["invitationStatus"] = *status
	} else {
		view["invitationStatus"] = nil
	}
	view["contactsLocked"] = !accepted
	if accepted {
		fullName := strings.TrimSpace(strings.Join(filterNil(candidate.FirstName, candidate.MiddleName, candidate.LastName), " "))
		var email any
		if candidate.Email != nil {
			email = *candidate.Email
		}
		view["contacts"] = map[string]any{
			"fullName": fullName,
			"email":    email,
			"phone":    candidate.Phone,
			"city":     candidate.City,
		}
		view["contactsHint"] = "Кандидат принял приглашение — контакты открыты"
	} else {
		view["contacts"] = nil
		view["contactsHint"] = "Контактные данные будут доступны после принятия приглашения"
	}
	return view, nil
}

func filterNil(vals ...*string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if v != nil && *v != "" {
			out = append(out, *v)
		}
	}
	return out
}

// ===== Дашборд =====

type dashboardInvitation struct {
	ID          string
	Status      model.InvitationStatus
	CreatedAt   model.Time
	CandidateID string
}

// Dashboard — GET /employers/dashboard.
func (s *Service) Dashboard(ctx context.Context, userID string) (map[string]any, error) {
	employer, err := s.requireProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	invitations := []dashboardInvitation{}
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, status, created_at, candidate_id
		FROM invitations WHERE employer_id = $1
		ORDER BY created_at DESC LIMIT 10`, employer.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var i dashboardInvitation
		if err := rows.Scan(&i.ID, &i.Status, &i.CreatedAt, &i.CandidateID); err != nil {
			rows.Close()
			return nil, err
		}
		invitations = append(invitations, i)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	vacancies := []map[string]any{}
	vrows, err := s.db.Pool.Query(ctx, `
		SELECT id, title, specialization, grade, salary_from, salary_to, is_active, created_at
		FROM vacancies WHERE employer_id = $1
		ORDER BY created_at DESC LIMIT 10`, employer.ID)
	if err != nil {
		return nil, err
	}
	for vrows.Next() {
		var id, title string
		var spec model.Specialization
		var grade model.Grade
		var salaryFrom, salaryTo int
		var isActive bool
		var createdAt model.Time
		if err := vrows.Scan(&id, &title, &spec, &grade, &salaryFrom, &salaryTo, &isActive, &createdAt); err != nil {
			vrows.Close()
			return nil, err
		}
		vacancies = append(vacancies, map[string]any{
			"id": id, "title": title, "specialization": spec, "grade": grade,
			"salaryFrom": salaryFrom, "salaryTo": salaryTo, "isActive": isActive,
			"createdAt": createdAt,
		})
	}
	vrows.Close()
	if err := vrows.Err(); err != nil {
		return nil, err
	}

	// Последняя потребность работодателя.
	var need *struct {
		Description    string
		Specialization *model.Specialization
		Grade          *model.Grade
		Stack          []string
	}
	var needDesc string
	var needSpec *model.Specialization
	var needGrade *model.Grade
	var needStack []string
	err = s.db.Pool.QueryRow(ctx, `
		SELECT description, specialization, grade, stack
		FROM employer_needs WHERE employer_id = $1
		ORDER BY created_at DESC LIMIT 1`, employer.ID).Scan(&needDesc, &needSpec, &needGrade, &needStack)
	if err == nil {
		need = &struct {
			Description    string
			Specialization *model.Specialization
			Grade          *model.Grade
			Stack          []string
		}{needDesc, needSpec, needGrade, needStack}
	} else if err.Error() != "no rows in result set" {
		return nil, err
	}

	recommendations := map[string]any{"total": 0, "candidates": []any{}}
	if need != nil {
		stack := need.Stack
		if stack == nil {
			stack = []string{}
		}
		var spec, grade *string
		if need.Specialization != nil {
			v := string(*need.Specialization)
			spec = &v
		}
		if need.Grade != nil {
			v := string(*need.Grade)
			grade = &v
		}
		rec, err := s.matching.Search(ctx, matching.SearchCandidatesDTO{
			Description:    &need.Description,
			Specialization: spec,
			Grade:          grade,
			Stack:          stack,
			Limit:          5,
			Offset:         0,
		})
		if err == nil && rec != nil {
			recommendations = rec
		}
	}

	byStatus := map[string]int{}
	for _, i := range invitations {
		byStatus[string(i.Status)]++
	}

	recent := make([]map[string]any, 0, len(invitations))
	for _, i := range invitations {
		cand, err := candidateview.Load(ctx, s.db, i.CandidateID)
		if err != nil {
			return nil, err
		}
		var public any
		if cand != nil {
			public = candidateview.ToPublic(cand, true)
		} else {
			public = nil
		}
		recent = append(recent, map[string]any{
			"id":        i.ID,
			"status":    i.Status,
			"createdAt": i.CreatedAt,
			"candidate": public,
		})
	}

	total := 0
	if t, ok := recommendations["total"]; ok {
		switch n := t.(type) {
		case int:
			total = n
		case int64:
			total = int(n)
		case float64:
			total = int(n)
		}
	}
	var topCandidates any = []any{}
	if c, ok := recommendations["candidates"]; ok && c != nil {
		topCandidates = c
	}

	return map[string]any{
		"stats": map[string]any{
			"invitations":         len(invitations),
			"invitationsByStatus": byStatus,
			"vacancies":           len(vacancies),
			"candidatesMatched":   total,
		},
		"recentInvitations": recent,
		"vacancies":         vacancies,
		"topCandidates":     topCandidates,
	}, nil
}
