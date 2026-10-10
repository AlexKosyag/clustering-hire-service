package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"fsp-backend/internal/common"
	"fsp-backend/internal/config"
	"fsp-backend/internal/model"
	"fsp-backend/internal/store"
)

// Service — бизнес-логика auth (аналог auth.service.ts).
type Service struct {
	db  *store.DB
	cfg *config.Config
}

// NewService создаёт сервис аутентификации.
func NewService(db *store.DB, cfg *config.Config) *Service {
	return &Service{db: db, cfg: cfg}
}

// isUniqueViolation — ошибка уникальности (23505) в гонке параллельных регистраций.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// register — POST /auth/register (auth.service.ts → register).
func (s *Service) register(ctx context.Context, dto *RegisterDto) (*AuthResponse, error) {
	email := dto.Email.s
	password := dto.Password.s
	role := model.UserRole(dto.Role.s)

	// 1. Проверка занятости email → 409 ConflictException('Пользователь с таким email уже существует')
	var exists bool
	if err := s.db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, common.Conflict(common.MsgEmailTaken)
	}

	// 2. bcrypt.hash(password, 12)
	passwordHash, err := common.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// 3. Транзакция Prisma: User (+emailVerifyToken) и связанный профиль.
	// BUG-REF: S3 — верификация e-mail не реализована: токен генерируется, но письмо
	// не отправляется, эндпоинта подтверждения нет (isEmailVerified остаётся false).
	now := model.Now()
	userID := uuid.NewString()
	created := SafeUser{}

	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // после Commit — no-op (ErrTxClosed)

	err = tx.QueryRow(ctx, `
        INSERT INTO users (id, email, password_hash, role, email_verify_token, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id, email, role, is_email_verified, created_at`,
		userID, email, passwordHash, role, uuid.NewString(), now, now,
	).Scan(&created.ID, &created.Email, &created.Role, &created.IsEmailVerified, &created.CreatedAt)
	if err != nil {
		// Гонка двух регистраций с одним email (TS даёт 500 Prisma P2002 → здесь 409).
		if isUniqueViolation(err) {
			return nil, common.Conflict(common.MsgEmailTaken)
		}
		return nil, err
	}

	if role == model.RoleCandidate {
		// candidateProfile.create({ data: { userId } }) — без массивов/прочих полей:
		// stack/roles/softSkills остаются NULL (Prisma не подставляет []).
		if _, err = tx.Exec(ctx, `
            INSERT INTO candidate_profiles (id, user_id, created_at, updated_at)
            VALUES ($1, $2, $3, $4)`, uuid.NewString(), userID, now, now); err != nil {
			return nil, err
		}
	} else {
		// employerProfile.create({ data: { userId, companyName: '' } })
		if _, err = tx.Exec(ctx, `
            INSERT INTO employer_profiles (id, user_id, company_name, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5)`, uuid.NewString(), userID, "", now, now); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// 4. Выпуск пары токенов + сохранение bcrypt-хеша refresh-токена.
	access, refresh, err := s.generateTokens(created)
	if err != nil {
		return nil, err
	}
	if err := s.persistRefreshToken(ctx, created.ID, refresh); err != nil {
		return nil, err
	}

	// 5. Ответ: { accessToken, refreshToken, user: SafeUser } + cookie.
	return &AuthResponse{AccessToken: access, RefreshToken: refresh, User: created}, nil
}

// login — POST /auth/login (auth.service.ts → login).
func (s *Service) login(ctx context.Context, dto *LoginDto) (*AuthResponse, error) {
	// Нет пользователя и неверный пароль → одинаковый 401 (без утечки существования аккаунта).
	user, err := s.findAuthUserByEmail(ctx, dto.Email.s)
	if err != nil {
		return nil, err
	}
	if !common.CheckHash(user.passwordHash, dto.Password.s) {
		return nil, common.Unauthorized(common.MsgBadCredentials)
	}

	access, refresh, err := s.generateTokens(user.safe)
	if err != nil {
		return nil, err
	}
	if err := s.persistRefreshToken(ctx, user.safe.ID, refresh); err != nil {
		return nil, err
	}
	return &AuthResponse{AccessToken: access, RefreshToken: refresh, User: user.safe}, nil
}

// refresh — POST /auth/refresh (auth.service.ts → refresh): ротация пары токенов.
func (s *Service) refresh(ctx context.Context, refreshToken string) (*TokensResponse, error) {
	// 1. jwt.verify(token, refreshSecret)
	payload, err := common.ParseToken(s.cfg.JWTRefreshSecret, refreshToken)
	if err != nil {
		return nil, common.Unauthorized(common.MsgBadRefresh)
	}
	// 2. Проверка typ === 'refresh' — access-токен отвергается.
	if payload.Typ != "refresh" {
		return nil, common.Unauthorized(common.MsgBadRefresh)
	}

	// 3. Пользователь должен существовать и иметь сохранённый refreshToken.
	// BUG-REF: S4 — хранится один bcrypt-хеш на пользователя: списка устройств/сессий нет,
	// детекции повторного использования токена нет (replay старого токена → просто 401).
	var (
		id, email string
		role      model.UserRole
		stored    *string
	)
	err = s.db.Pool.QueryRow(ctx,
		`SELECT id, email, role, refresh_token FROM users WHERE id = $1`, payload.Sub,
	).Scan(&id, &email, &role, &stored)
	if errors.Is(err, pgx.ErrNoRows) || stored == nil {
		return nil, common.Unauthorized(common.MsgBadRefresh)
	}
	if err != nil {
		return nil, err
	}

	// 4. bcrypt.compare(token, user.refreshToken)
	if !common.CheckHash(*stored, refreshToken) {
		return nil, common.Unauthorized(common.MsgBadRefresh)
	}

	// 5. Новая пара + перезапись хеша (ротация).
	user := SafeUser{ID: id, Email: email, Role: role}
	access, newRefresh, err := s.generateTokens(user)
	if err != nil {
		return nil, err
	}
	if err := s.persistRefreshToken(ctx, user.ID, newRefresh); err != nil {
		return nil, err
	}
	return &TokensResponse{AccessToken: access, RefreshToken: newRefresh}, nil
}

// logout — POST /auth/logout (auth.service.ts → logout): инвалидация refresh-токена.
// BUG-REF: S4 — logout действует только на refresh: отозванных access-токенов нет,
// они остаются валидными до истечения 15 минут.
func (s *Service) logout(ctx context.Context, userID string) error {
	_, err := s.db.Pool.Exec(ctx,
		`UPDATE users SET refresh_token = NULL, updated_at = $1 WHERE id = $2`, model.Now(), userID)
	return err
}

// me — GET /auth/me (auth.service.ts → me): пользователь + профили без секретов.
func (s *Service) me(ctx context.Context, userID string) (*MeResponse, error) {
	out := &MeResponse{}
	err := s.db.Pool.QueryRow(ctx, `
        SELECT id, email, role, is_email_verified, created_at, updated_at
        FROM users WHERE id = $1`, userID,
	).Scan(&out.ID, &out.Email, &out.Role, &out.IsEmailVerified, &out.CreatedAt, &out.UpdatedAt)
	// UnauthorizedException() без аргумента → 401 "Unauthorized".
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, common.NewError(401, "Unauthorized")
	}
	if err != nil {
		return nil, err
	}

	candidate, err := s.loadCandidateProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	employer, err := s.loadEmployerProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	out.CandidateProfile = candidate
	out.EmployerProfile = employer
	return out, nil
}

// generateTokens — пара access/refresh (auth.service.ts → generateTokens).
func (s *Service) generateTokens(user SafeUser) (string, string, error) {
	u := common.AuthUser{ID: user.ID, Email: user.Email, Role: user.Role}
	access, err := common.IssueAccessToken(s.cfg.JWTSecret, s.cfg.AccessTokenTTL, u)
	if err != nil {
		return "", "", err
	}
	refresh, err := common.IssueRefreshToken(s.cfg.JWTRefreshSecret, s.cfg.RefreshTokenTTL, u)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// persistRefreshToken — bcrypt.hash(refreshToken, 12) → users.refresh_token.
// BUG-REF: S4 — одна активная сессия на пользователя: новый токен затирает хеш старого.
func (s *Service) persistRefreshToken(ctx context.Context, userID, refreshToken string) error {
	hash, err := common.HashRefresh(refreshToken)
	if err != nil {
		return err
	}
	_, err = s.db.Pool.Exec(ctx,
		`UPDATE users SET refresh_token = $1, updated_at = $2 WHERE id = $3`, hash, model.Now(), userID)
	return err
}

// authUser — пользователь вместе с passwordHash (для bcrypt.compare в login).
type authUser struct {
	safe         SafeUser
	passwordHash string
}

// findAuthUserByEmail — prisma.user.findUnique({ where: { email } }).
func (s *Service) findAuthUserByEmail(ctx context.Context, email string) (*authUser, error) {
	u := &authUser{}
	err := s.db.Pool.QueryRow(ctx, `
        SELECT id, email, password_hash, role, is_email_verified, created_at
        FROM users WHERE email = $1`, email,
	).Scan(&u.safe.ID, &u.safe.Email, &u.passwordHash, &u.safe.Role, &u.safe.IsEmailVerified, &u.safe.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, common.Unauthorized(common.MsgBadCredentials)
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// loadCandidateProfile — prisma.candidateProfile.findUnique({ where: { userId } }).
func (s *Service) loadCandidateProfile(ctx context.Context, userID string) (*model.CandidateProfile, error) {
	var (
		p                model.CandidateProfile
		firstName        *string
		lastName         *string
		middleName       *string
		phone            *string
		city             *string
		bio              *string
		avatarURL        *string
		specialization   *model.Specialization
		declaredGrade    *model.Grade
		verifiedGrade    *model.Grade
		stack            *[]string
		roles            *[]string
		experienceYears  *float64
		softSkills       *[]string
		fspID            *string
		fspConnectedAt   *model.Time
		lastTestDate     *model.Time
		testCooldownTill *model.Time
		lastTestScore    *float64
		resumePDFURL     *string
		resumeParsed     *json.RawMessage
		lastMatchScore   *float64
	)
	err := s.db.Pool.QueryRow(ctx, `
        SELECT id, user_id, first_name, last_name, middle_name, phone, city, bio, avatar_url,
               specialization, declared_grade, verified_grade, stack, roles, experience_years, soft_skills,
               fsp_id, fsp_connected_at, test_status, last_test_date, test_cooldown_until, last_test_score,
               resume_pdf_url, resume_parsed, show_email, show_phone, show_full_name, consent_to_processing,
               last_match_score, created_at, updated_at
        FROM candidate_profiles WHERE user_id = $1`, userID,
	).Scan(
		&p.ID, &p.UserID, &firstName, &lastName, &middleName, &phone, &city, &bio, &avatarURL,
		&specialization, &declaredGrade, &verifiedGrade, &stack, &roles, &experienceYears, &softSkills,
		&fspID, &fspConnectedAt, &p.TestStatus, &lastTestDate, &testCooldownTill, &lastTestScore,
		&resumePDFURL, &resumeParsed, &p.ShowEmail, &p.ShowPhone, &p.ShowFullName, &p.ConsentToProcessing,
		&lastMatchScore, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	p.FirstName, p.LastName, p.MiddleName = firstName, lastName, middleName
	p.Phone, p.City, p.Bio, p.AvatarURL = phone, city, bio, avatarURL
	p.Specialization, p.DeclaredGrade, p.VerifiedGrade = specialization, declaredGrade, verifiedGrade
	p.Stack = normalizeArray(stack)
	p.Roles = normalizeArray(roles)
	p.SoftSkills = normalizeArray(softSkills)
	p.ExperienceYears = experienceYears
	p.FspID, p.FspConnectedAt = fspID, fspConnectedAt
	p.LastTestDate, p.TestCooldownUntil, p.LastTestScore = lastTestDate, testCooldownTill, lastTestScore
	p.ResumePDFURL = resumePDFURL
	if resumeParsed != nil {
		p.ResumeParsed = *resumeParsed
	}
	p.LastMatchScore = lastMatchScore
	return &p, nil
}

// loadEmployerProfile — prisma.employerProfile.findUnique({ where: { userId } }).
func (s *Service) loadEmployerProfile(ctx context.Context, userID string) (*model.EmployerProfile, error) {
	var (
		p                  model.EmployerProfile
		companyDescription *string
		industry           *string
		website            *string
		logoURL            *string
		contactPerson      *string
		contactEmail       *string
		contactPhone       *string
		verifiedAt         *model.Time
	)
	err := s.db.Pool.QueryRow(ctx, `
        SELECT id, user_id, company_name, company_description, industry, website, logo_url,
               contact_person, contact_email, contact_phone, is_verified, verified_at, created_at, updated_at
        FROM employer_profiles WHERE user_id = $1`, userID,
	).Scan(
		&p.ID, &p.UserID, &p.CompanyName, &companyDescription, &industry, &website, &logoURL,
		&contactPerson, &contactEmail, &contactPhone, &p.IsVerified, &verifiedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	p.CompanyDescription, p.Industry, p.Website = companyDescription, industry, website
	p.LogoURL, p.ContactPerson, p.ContactEmail, p.ContactPhone = logoURL, contactPerson, contactEmail, contactPhone
	p.VerifiedAt = verifiedAt
	return &p, nil
}

// normalizeArray: NULL из БД → nil (marshal: null, как у Prisma),
// пустой массив → [] (не nil).
func normalizeArray(v *[]string) []string {
	if v == nil {
		return nil
	}
	if *v == nil {
		return []string{}
	}
	return *v
}

// NewAuthGuard — поведение JwtStrategy + JwtAuthGuard (см. docs 03 §7–§8):
// Bearer из Authorization → common.ParseToken → пользователь в БД → common.SetUser.
// Нет токена / токен невалиден / истёк → 401 «Требуется авторизация»;
// пользователя нет → 401 «Пользователь не найден».
func NewAuthGuard(db *store.DB, jwtSecret string) func(c *gin.Context) error {
	return func(c *gin.Context) error {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			return common.Unauthorized(common.MsgUnauthorized)
		}
		payload, err := common.ParseToken(jwtSecret, token)
		if err != nil {
			return common.Unauthorized(common.MsgUnauthorized)
		}

		// validate(): prisma.user.findUnique({ where: { id: payload.sub } }) на каждый запрос.
		var (
			user common.AuthUser
			role model.UserRole
		)
		err = db.Pool.QueryRow(c.Request.Context(),
			`SELECT id, email, role FROM users WHERE id = $1`, payload.Sub,
		).Scan(&user.ID, &user.Email, &role)
		if errors.Is(err, pgx.ErrNoRows) {
			return common.Unauthorized(common.MsgUserNotFound)
		}
		if err != nil {
			return common.Unauthorized(common.MsgUnauthorized)
		}
		user.Role = role
		common.SetUser(c, user)
		return nil
	}
}

// bearerToken — ExtractJwt.fromAuthHeaderAsBearerToken(): "Bearer <token>" (схема без учёта регистра).
func bearerToken(header string) string {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
