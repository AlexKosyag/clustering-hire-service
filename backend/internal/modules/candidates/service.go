package candidates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"strings"

	"github.com/jackc/pgx/v5"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
	"fsp-backend/internal/modules/candidateview"
	"fsp-backend/internal/store"
)

const (
	maxResumeBytes = 5 * 1024 * 1024 // MAX_RESUME_BYTES
	maxImageBytes  = 2 * 1024 * 1024 // MAX_IMAGE_BYTES
)

// MatchingAPI — matching-движок (реализует *matching.Service).
type MatchingAPI interface {
	InvalidateCache(ctx context.Context) error
	IndexCandidate(ctx context.Context, profileID string) error
}

// StorageAPI — файловое хранилище (реализует *storage.Service).
type StorageAPI interface {
	Upload(ctx context.Context, kind, owner, filename string, buf []byte, contentType string) (string, error)
}

// Service — candidates.service.ts.
type Service struct {
	db       *store.DB
	matching MatchingAPI
	storage  StorageAPI
}

// NewService — конструктор для интегратора.
func NewService(db *store.DB, m MatchingAPI, st StorageAPI) *Service {
	return &Service{db: db, matching: m, storage: st}
}

// profileRecord — CandidateProfile + user.email + достижения (candidateInclude).
type profileRecord struct {
	P            model.CandidateProfile
	Email        *string
	Achievements []model.FSPAchievement
}

// requireProfile — профиль по user_id (findUnique({where: {userId}, include})).
func (s *Service) requireProfile(ctx context.Context, userID string) (*profileRecord, error) {
	rec := &profileRecord{}
	err := s.db.Pool.QueryRow(ctx, `
		SELECT cp.id, cp.user_id,
		       cp.first_name, cp.last_name, cp.middle_name, cp.phone, cp.city, cp.bio, cp.avatar_url,
		       cp.specialization, cp.declared_grade, cp.verified_grade,
		       cp.stack, cp.roles, cp.experience_years, cp.soft_skills,
		       cp.fsp_id, cp.fsp_connected_at,
		       cp.test_status, cp.last_test_date, cp.test_cooldown_until, cp.last_test_score,
		       cp.resume_pdf_url, cp.resume_parsed,
		       cp.show_email, cp.show_phone, cp.show_full_name, cp.consent_to_processing,
		       cp.last_match_score, cp.created_at, cp.updated_at,
		       u.email
		FROM candidate_profiles cp
		JOIN users u ON u.id = cp.user_id
		WHERE cp.user_id = $1`, userID).Scan(
		&rec.P.ID, &rec.P.UserID,
		&rec.P.FirstName, &rec.P.LastName, &rec.P.MiddleName, &rec.P.Phone, &rec.P.City, &rec.P.Bio, &rec.P.AvatarURL,
		&rec.P.Specialization, &rec.P.DeclaredGrade, &rec.P.VerifiedGrade,
		&rec.P.Stack, &rec.P.Roles, &rec.P.ExperienceYears, &rec.P.SoftSkills,
		&rec.P.FspID, &rec.P.FspConnectedAt,
		&rec.P.TestStatus, &rec.P.LastTestDate, &rec.P.TestCooldownUntil, &rec.P.LastTestScore,
		&rec.P.ResumePDFURL, &rec.P.ResumeParsed,
		&rec.P.ShowEmail, &rec.P.ShowPhone, &rec.P.ShowFullName, &rec.P.ConsentToProcessing,
		&rec.P.LastMatchScore, &rec.P.CreatedAt, &rec.P.UpdatedAt,
		&rec.Email,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, common.NotFound("Профиль не найден")
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Pool.Query(ctx, `
		SELECT a.id, a.name, a.description, a.category, a.year, a.place, a.points
		FROM candidate_achievements ca
		JOIN fsp_achievements a ON a.id = ca.achievement_id
		WHERE ca.candidate_id = $1
		ORDER BY ca.id`, rec.P.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rec.Achievements = []model.FSPAchievement{}
	for rows.Next() {
		var a model.FSPAchievement
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.Category, &a.Year, &a.Place, &a.Points); err != nil {
			return nil, err
		}
		rec.Achievements = append(rec.Achievements, a)
	}
	return rec, rows.Err()
}

// view — профиль в виде для toCandidatePublic.
func (rec *profileRecord) view() *candidateview.Candidate {
	c := &candidateview.Candidate{
		ID:              rec.P.ID,
		FirstName:       rec.P.FirstName,
		LastName:        rec.P.LastName,
		MiddleName:      rec.P.MiddleName,
		Phone:           rec.P.Phone,
		City:            rec.P.City,
		Bio:             rec.P.Bio,
		Stack:           rec.P.Stack,
		Roles:           rec.P.Roles,
		ExperienceYears: rec.P.ExperienceYears,
		SoftSkills:      rec.P.SoftSkills,
		TestStatus:      string(rec.P.TestStatus),
		LastTestDate:    rec.P.LastTestDate,
		LastTestScore:   rec.P.LastTestScore,
		ShowFullName:    rec.P.ShowFullName,
		UpdatedAt:       rec.P.UpdatedAt,
		Email:           rec.Email,
	}
	if rec.P.Specialization != nil {
		v := string(*rec.P.Specialization)
		c.Specialization = &v
	}
	if rec.P.DeclaredGrade != nil {
		v := string(*rec.P.DeclaredGrade)
		c.DeclaredGrade = &v
	}
	if rec.P.VerifiedGrade != nil {
		v := string(*rec.P.VerifiedGrade)
		c.VerifiedGrade = &v
	}
	c.Achievements = make([]candidateview.Achievement, 0, len(rec.Achievements))
	for _, a := range rec.Achievements {
		c.Achievements = append(c.Achievements, candidateview.Achievement{
			Name:     a.Name,
			Category: a.Category,
			Points:   a.Points,
			Year:     a.Year,
			Place:    a.Place,
		})
	}
	return c
}

// response — расширенный профиль: {...toCandidatePublic(profile), приватные поля владельца}.
func (rec *profileRecord) response() map[string]any {
	res := candidateview.ToPublic(rec.view(), true)
	res["email"] = rec.Email
	res["phone"] = rec.P.Phone
	res["middleName"] = rec.P.MiddleName
	res["avatarUrl"] = rec.P.AvatarURL
	res["fspId"] = rec.P.FspID
	res["fspConnectedAt"] = rec.P.FspConnectedAt
	res["resumePdfUrl"] = rec.P.ResumePDFURL
	res["resumeParsed"] = rec.P.ResumeParsed
	res["showEmail"] = rec.P.ShowEmail
	res["showPhone"] = rec.P.ShowPhone
	res["showFullName"] = rec.P.ShowFullName
	res["consentToProcessing"] = rec.P.ConsentToProcessing
	res["testCooldownUntil"] = rec.P.TestCooldownUntil
	res["createdAt"] = rec.P.CreatedAt
	return res
}

// Profile — GET /candidates/profile.
func (s *Service) Profile(ctx context.Context, userID string) (map[string]any, error) {
	rec, err := s.requireProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return rec.response(), nil
}

// UpdateProfile — PUT /candidates/profile: обновляются только присланные поля
// (null обнуляет колонку), затем переиндексация для matching и повторное чтение профиля.
func (s *Service) UpdateProfile(ctx context.Context, userID string, dto *UpdateCandidateProfileDto) (map[string]any, error) {
	rec, err := s.requireProfile(ctx, userID)
	if err != nil {
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
	args = append(args, rec.P.ID)
	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE candidate_profiles SET `+strings.Join(sets, ", ")+` WHERE id = $`+fmt.Sprint(idx),
		args...); err != nil {
		return nil, err
	}

	// await this.matching.invalidateCache().catch(() => undefined)
	// await this.indexing.indexCandidate(updated.id).catch(() => undefined)
	s.swallow(func() error { return s.matching.InvalidateCache(ctx) })
	s.swallow(func() error { return s.matching.IndexCandidate(ctx, rec.P.ID) })

	return s.Profile(ctx, userID)
}

// Category — GET /candidates/category: текущая категория/грейд и статус тестирования.
func (s *Service) Category(ctx context.Context, userID string) (map[string]any, error) {
	rec, err := s.requireProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	var completedTests int
	if err := s.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM test_sessions WHERE candidate_id = $1 AND status = 'COMPLETED'`,
		rec.P.ID).Scan(&completedTests); err != nil {
		return nil, err
	}

	return map[string]any{
		"specialization":    rec.P.Specialization,
		"declaredGrade":     rec.P.DeclaredGrade,
		"verifiedGrade":     rec.P.VerifiedGrade,
		"testStatus":        rec.P.TestStatus,
		"lastTestDate":      rec.P.LastTestDate,
		"lastTestScore":     rec.P.LastTestScore,
		"canRetakeAt":       rec.P.TestCooldownUntil,
		"completedTests":    completedTests,
		"achievements":      rec.Achievements,
		"achievementPoints": achievementPoints(rec.Achievements),
		"fspId":             rec.P.FspID,
	}, nil
}

// achievementPoints — сумма points достижений (achievementPoints в TS).
func achievementPoints(achievements []model.FSPAchievement) int {
	sum := 0
	for _, a := range achievements {
		sum += a.Points
	}
	return sum
}

// UploadResume — POST /candidates/profile/resume: загрузка PDF в MinIO,
// парсинг (необязателен), автозаполнение только пустых полей, реиндексация.
func (s *Service) UploadResume(ctx context.Context, userID string, file *multipart.FileHeader) (*ResumeUploadResult, error) {
	if file == nil {
		return nil, common.BadRequest("Файл не передан")
	}
	mimetype := file.Header.Get("Content-Type")
	if mimetype != "application/pdf" {
		return nil, common.BadRequest("Резюме должно быть PDF")
	}
	if file.Size > maxResumeBytes {
		return nil, common.BadRequest("Максимальный размер — 5 МБ")
	}

	rec, err := s.requireProfile(ctx, userID)
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

	// Upload file to storage (сбой → 503, docs 07 §4).
	url, err := s.storage.Upload(ctx, "resumes", rec.P.ID, "resume.pdf", buf, "application/pdf")
	if err != nil {
		return nil, err
	}

	// Parse PDF — сбой парсинга не роняет загрузку.
	parsed, parseErr := ParseResume(buf)
	if parseErr != nil {
		log.Printf("[Candidates] PDF parsing failed for %s: %s", rec.P.ID, parseErr.Error())
	} else {
		log.Printf("[Candidates] Parsed resume for %s: %d skills, %d experience entries",
			rec.P.ID, len(parsed.Skills), len(parsed.Experience))
	}

	sets := []string{"resume_pdf_url = $1", "updated_at = $2"}
	args := []any{url, model.Now()}
	idx := 3

	var autoFilled map[string]any
	var preview *resumePreview

	if parsed != nil {
		raw, err := json.Marshal(parsed)
		if err != nil {
			return nil, err
		}
		sets = append(sets, fmt.Sprintf("resume_parsed = $%d::jsonb", idx))
		args = append(args, raw)
		idx++

		// Автозаполнение ТОЛЬКО пустых полей (!profile.city && parsed.city — истина
		// для null и пустой строки).
		autoFilled = map[string]any{}
		if !jsTruthy(rec.P.City) && parsed.City != nil {
			sets = append(sets, "city = $"+fmt.Sprint(idx))
			args = append(args, *parsed.City)
			idx++
			addAutoFilled(autoFilled, "city", parsed.City)
		} else {
			addAutoFilled(autoFilled, "city", nil, jsTruthy(rec.P.City))
		}
		if !jsTruthy(rec.P.Phone) && parsed.Phone != nil {
			sets = append(sets, "phone = $"+fmt.Sprint(idx))
			args = append(args, *parsed.Phone)
			idx++
			addAutoFilled(autoFilled, "phone", parsed.Phone)
		} else {
			addAutoFilled(autoFilled, "phone", nil, jsTruthy(rec.P.Phone))
		}
		if !jsTruthy(rec.P.FirstName) && parsed.Name != nil {
			// name.split(' ') → parts[0]/[1]/[2]; пустые части не записываются.
			parts := strings.Split(*parsed.Name, " ")
			sets = append(sets, "first_name = $"+fmt.Sprint(idx))
			args = append(args, parts[0])
			idx++
			if len(parts) > 1 && parts[1] != "" {
				sets = append(sets, "last_name = $"+fmt.Sprint(idx))
				args = append(args, parts[1])
				idx++
			}
			if len(parts) > 2 && parts[2] != "" {
				sets = append(sets, "middle_name = $"+fmt.Sprint(idx))
				args = append(args, parts[2])
				idx++
			}
			addAutoFilled(autoFilled, "name", parsed.Name)
		} else {
			addAutoFilled(autoFilled, "name", nil, jsTruthy(rec.P.FirstName))
		}
		// Навыки ДОМЕРЯЮТСЯ к stack (без перезаписи).
		if len(parsed.Skills) > 0 {
			existing := rec.P.Stack
			if existing == nil {
				existing = []string{}
			}
			newSkills := []string{}
			for _, skill := range parsed.Skills {
				dup := false
				for _, e := range existing {
					if strings.EqualFold(e, skill) {
						dup = true
						break
					}
				}
				if !dup {
					newSkills = append(newSkills, skill)
				}
			}
			if len(newSkills) > 0 {
				merged := make([]string, 0, len(existing)+len(newSkills))
				merged = append(merged, existing...)
				merged = append(merged, newSkills...)
				sets = append(sets, "stack = $"+fmt.Sprint(idx))
				args = append(args, merged)
				idx++
			}
			autoFilled["skills"] = true
		} else {
			autoFilled["skills"] = false
		}
		if !jsTruthy(rec.P.Bio) && parsed.Summary != nil {
			sets = append(sets, "bio = $"+fmt.Sprint(idx))
			args = append(args, *parsed.Summary)
			idx++
		}

		preview = &resumePreview{
			Name:            parsed.Name,
			Email:           parsed.Email,
			Phone:           parsed.Phone,
			City:            parsed.City,
			Skills:          parsed.Skills,
			ExperienceCount: len(parsed.Experience),
			EducationCount:  len(parsed.Education),
			Summary:         sliceRunesPtr(parsed.Summary, 200),
		}
	}

	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE candidate_profiles SET `+strings.Join(sets, ", ")+` WHERE id = $`+fmt.Sprint(idx),
		args...); err != nil {
		return nil, err
	}

	// await this.indexing.indexCandidate(profile.id).catch(() => undefined)
	s.swallow(func() error { return s.matching.IndexCandidate(ctx, rec.P.ID) })

	var autoFilledOut any
	if autoFilled != nil {
		autoFilledOut = autoFilled
	}
	return &ResumeUploadResult{
		ResumePDFURL: url,
		Parsed:       preview,
		AutoFilled:   autoFilledOut,
	}, nil
}

// jsTruthy — истинность строки в JS-смысле (null/undefined/"" → false).
func jsTruthy(p *string) bool { return p != nil && *p != "" }

// addAutoFilled — флаг autoFilled.<field>: `!profile.X && parsed.X` возвращает
// строку (заполнено), false (поле уже было заполнено) или пропускает ключ
// (undefined в обеих частях).
func addAutoFilled(out map[string]any, key string, parsed *string, profileHas ...bool) {
	filled := false
	if len(profileHas) > 0 {
		filled = profileHas[0]
	}
	if filled {
		out[key] = false // !profile.X === false
		return
	}
	if parsed == nil {
		return // true && undefined → ключ опускается (JSON.stringify)
	}
	out[key] = *parsed
}

// UploadAvatar — POST /candidates/profile/avatar.
func (s *Service) UploadAvatar(ctx context.Context, userID string, file *multipart.FileHeader) (*AvatarUploadResult, error) {
	if file == nil {
		return nil, common.BadRequest("Файл не передан")
	}
	mimetype := file.Header.Get("Content-Type")
	if !strings.HasPrefix(mimetype, "image/") {
		return nil, common.BadRequest("Нужно изображение")
	}
	if file.Size > maxImageBytes {
		return nil, common.BadRequest("Максимальный размер — 2 МБ")
	}

	rec, err := s.requireProfile(ctx, userID)
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
	ext = strings.Replace(ext, "jpeg", "jpg", 1)

	url, err := s.storage.Upload(ctx, "avatars", rec.P.ID, "avatar."+ext, buf, mimetype)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE candidate_profiles SET avatar_url = $1, updated_at = $2 WHERE id = $3`,
		url, model.Now(), rec.P.ID); err != nil {
		return nil, err
	}
	return &AvatarUploadResult{AvatarURL: url}, nil
}

// swallow — эквивалент .catch(() => undefined): ошибка/паника сайд-эффекта игнорируется.
func (s *Service) swallow(fn func() error) {
	defer func() { _ = recover() }()
	_ = fn()
}
