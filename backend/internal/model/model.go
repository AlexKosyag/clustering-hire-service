// Package model — сущности БД, повторяющие prisma/schema.prisma (таблицы snake_case,
// JSON — camelCase как у Prisma Client).
package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Time — обёртка time.Time с сериализацией в ISO-8601 c миллисекундами ("2026-10-10T12:00:00.000Z"),
// как это делает Prisma/JSON.stringify(new Date()).
type Time struct {
	time.Time
}

func Now() Time { return Time{time.Now().UTC()} }

func FromTime(t time.Time) Time { return Time{t.UTC()} }

func (t Time) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.UTC().Format("2006-01-02T15:04:05.000Z") + `"`), nil
}

func (t *Time) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		t.Time = time.Time{}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return err
	}
	t.Time = parsed.UTC()
	return nil
}

// Scan реализует sql.Scanner.
func (t *Time) Scan(src interface{}) error {
	if src == nil {
		t.Time = time.Time{}
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		t.Time = v.UTC()
		return nil
	default:
		return fmt.Errorf("cannot scan %T into model.Time", src)
	}
}

// Value реализует driver.Valuer.
func (t Time) Value() (driver.Value, error) {
	if t.Time.IsZero() {
		return nil, nil
	}
	return t.Time.UTC(), nil
}

// ============ ENUM (значения как у Prisma) ============

type UserRole string

const (
	RoleCandidate UserRole = "CANDIDATE"
	RoleEmployer  UserRole = "EMPLOYER"
	RoleAdmin     UserRole = "ADMIN"
)

type Specialization string

const (
	SpecBackend     Specialization = "BACKEND"
	SpecFrontend    Specialization = "FRONTEND"
	SpecFullstack   Specialization = "FULLSTACK"
	SpecMobile      Specialization = "MOBILE"
	SpecDevops      Specialization = "DEVOPS"
	SpecDataScience Specialization = "DATA_SCIENCE"
	SpecQA          Specialization = "QA"
	SpecSecurity    Specialization = "SECURITY"
	SpecEmbedded    Specialization = "EMBEDDED"
	SpecGameDev     Specialization = "GAME_DEV"
	SpecMLAI        Specialization = "ML_AI"
	SpecDesignUX    Specialization = "DESIGN_UX"
	SpecManagement  Specialization = "MANAGEMENT"
)

// AllSpecializations — порядок как в Prisma enum.
var AllSpecializations = []Specialization{
	SpecBackend, SpecFrontend, SpecFullstack, SpecMobile, SpecDevops, SpecDataScience,
	SpecQA, SpecSecurity, SpecEmbedded, SpecGameDev, SpecMLAI, SpecDesignUX, SpecManagement,
}

type Grade string

const (
	GradeJunior     Grade = "JUNIOR"
	GradeMiddle     Grade = "MIDDLE"
	GradeMiddlePlus Grade = "MIDDLE_PLUS"
	GradeSenior     Grade = "SENIOR"
	GradeLead       Grade = "LEAD"
	GradePrincipal  Grade = "PRINCIPAL"
)

// GradeOrder — порядок грейдов значим (matching «±1 грейд», маппинг θ → грейд).
var GradeOrder = []Grade{GradeJunior, GradeMiddle, GradeMiddlePlus, GradeSenior, GradeLead, GradePrincipal}

// GradeIndex возвращает позицию грейда в GradeOrder (0..5); ok=false если грейд неизвестен.
func GradeIndex(g Grade) (int, bool) {
	for i, v := range GradeOrder {
		if v == g {
			return i, true
		}
	}
	return 0, false
}

// SpecializationStrings — значения enum в порядке объявления (для сообщений валидации).
func SpecializationStrings() []string {
	out := make([]string, len(AllSpecializations))
	for i, s := range AllSpecializations {
		out[i] = string(s)
	}
	return out
}

// GradeStrings — значения enum Grade в порядке объявления.
func GradeStrings() []string {
	out := make([]string, len(GradeOrder))
	for i, g := range GradeOrder {
		out[i] = string(g)
	}
	return out
}

// SpecializationFromString — разбор значения enum (ok=false, если неизвестно).
func SpecializationFromString(s string) (Specialization, bool) {
	for _, v := range AllSpecializations {
		if string(v) == s {
			return v, true
		}
	}
	return "", false
}

// GradeFromString — разбор значения enum Grade (ok=false, если неизвестно).
func GradeFromString(s string) (Grade, bool) {
	for _, v := range GradeOrder {
		if string(v) == s {
			return v, true
		}
	}
	return "", false
}

type TestStatus string

const (
	TestNotStarted TestStatus = "NOT_STARTED"
	TestInProgress TestStatus = "IN_PROGRESS"
	TestCompleted  TestStatus = "COMPLETED"
	TestFailed     TestStatus = "FAILED"
	TestPassed     TestStatus = "PASSED"
	TestUpgraded   TestStatus = "UPGRADED"
)

type InvitationStatus string

const (
	InvSent      InvitationStatus = "SENT"
	InvViewed    InvitationStatus = "VIEWED"
	InvAccepted  InvitationStatus = "ACCEPTED"
	InvRejected  InvitationStatus = "REJECTED"
	InvWithdrawn InvitationStatus = "WITHDRAWN"
)

type SessionStatus string

const (
	SessionInProgress SessionStatus = "IN_PROGRESS"
	SessionCompleted  SessionStatus = "COMPLETED"
	SessionTimedOut   SessionStatus = "TIMED_OUT"
	SessionAbandoned  SessionStatus = "ABANDONED" // BUG-REF: F2 — нигде не выставляется
)

type DailyTaskStatus string

const (
	DailyPending    DailyTaskStatus = "PENDING"
	DailyInProgress DailyTaskStatus = "IN_PROGRESS"
	DailyCompleted  DailyTaskStatus = "COMPLETED"
	DailySkipped    DailyTaskStatus = "SKIPPED"
	DailyExpired    DailyTaskStatus = "EXPIRED"
)

// ============ МОДЕЛИ ============

type User struct {
	ID               string   `json:"id"`
	Email            string   `json:"email"`
	PasswordHash     string   `json:"passwordHash"`
	Role             UserRole `json:"role"`
	IsEmailVerified  bool     `json:"isEmailVerified"`
	EmailVerifyToken *string  `json:"emailVerifyToken"`
	RefreshToken     *string  `json:"refreshToken"`
	CreatedAt        Time     `json:"createdAt"`
	UpdatedAt        Time     `json:"updatedAt"`
}

type CandidateProfile struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`

	FirstName  *string `json:"firstName"`
	LastName   *string `json:"lastName"`
	MiddleName *string `json:"middleName"`
	Phone      *string `json:"phone"`
	City       *string `json:"city"`
	Bio        *string `json:"bio"`
	AvatarURL  *string `json:"avatarUrl"`

	Specialization  *Specialization `json:"specialization"`
	DeclaredGrade   *Grade          `json:"declaredGrade"`
	VerifiedGrade   *Grade          `json:"verifiedGrade"`
	Stack           []string        `json:"stack"`
	Roles           []string        `json:"roles"`
	ExperienceYears *float64        `json:"experienceYears"`
	SoftSkills      []string        `json:"softSkills"`

	FspID          *string `json:"fspId"`
	FspConnectedAt *Time   `json:"fspConnectedAt"`

	TestStatus        TestStatus `json:"testStatus"`
	LastTestDate      *Time      `json:"lastTestDate"`
	TestCooldownUntil *Time      `json:"testCooldownUntil"`
	LastTestScore     *float64   `json:"lastTestScore"`

	ResumePDFURL *string         `json:"resumePdfUrl"`
	ResumeParsed json.RawMessage `json:"resumeParsed"`

	ShowEmail           bool `json:"showEmail"`
	ShowPhone           bool `json:"showPhone"`
	ShowFullName        bool `json:"showFullName"`
	ConsentToProcessing bool `json:"consentToProcessing"`

	LastMatchScore *float64 `json:"lastMatchScore"` // BUG-REF: M7 — никогда не заполняется

	CreatedAt Time `json:"createdAt"`
	UpdatedAt Time `json:"updatedAt"`
}

type EmployerProfile struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`

	CompanyName        string  `json:"companyName"`
	CompanyDescription *string `json:"companyDescription"`
	Industry           *string `json:"industry"`
	Website            *string `json:"website"`
	LogoURL            *string `json:"logoUrl"`
	ContactPerson      *string `json:"contactPerson"`
	ContactEmail       *string `json:"contactEmail"`
	ContactPhone       *string `json:"contactPhone"`

	IsVerified bool  `json:"isVerified"` // BUG-REF: F2 — не используется
	VerifiedAt *Time `json:"verifiedAt"`

	CreatedAt Time `json:"createdAt"`
	UpdatedAt Time `json:"updatedAt"`
}

type Vacancy struct {
	ID         string `json:"id"`
	EmployerID string `json:"employerId"`

	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Specialization Specialization `json:"specialization"`
	Grade          Grade          `json:"grade"`
	Stack          []string       `json:"stack"`
	SalaryFrom     int            `json:"salaryFrom"`
	SalaryTo       int            `json:"salaryTo"`

	IsActive  bool `json:"isActive"`
	CreatedAt Time `json:"createdAt"`
	UpdatedAt Time `json:"updatedAt"`
}

type EmployerNeed struct {
	ID         string `json:"id"`
	EmployerID string `json:"employerId"`

	Description    string          `json:"description"`
	Specialization *Specialization `json:"specialization"`
	Grade          *Grade          `json:"grade"`
	Stack          []string        `json:"stack"`

	CreatedAt Time `json:"createdAt"`
}

type Invitation struct {
	ID string `json:"id"`

	EmployerID  string  `json:"employerId"`
	CandidateID string  `json:"candidateId"`
	VacancyID   *string `json:"vacancyId"`

	Message    *string `json:"message"`
	SalaryFrom int     `json:"salaryFrom"`
	SalaryTo   int     `json:"salaryTo"`

	Status          InvitationStatus `json:"status"`
	StatusUpdatedAt Time             `json:"statusUpdatedAt"`

	CreatedAt Time `json:"createdAt"`
	UpdatedAt Time `json:"updatedAt"`
}

type TestCategory struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    *string        `json:"description"`
	Specialization Specialization `json:"specialization"`
}

// QuestionOption — элемент options Json: {id, text, isCorrect}.
type QuestionOption struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	IsCorrect bool   `json:"isCorrect"`
}

type TestQuestion struct {
	ID         string `json:"id"`
	CategoryID string `json:"categoryId"`

	QuestionType string `json:"questionType"` // multiple_choice, coding, open_text, scenario

	Text          string          `json:"text"`
	CodeSnippet   *string         `json:"codeSnippet"`
	Options       json.RawMessage `json:"options"`
	CorrectAnswer *string         `json:"correctAnswer"`
	Explanation   *string         `json:"explanation"`

	Difficulty     float64 `json:"difficulty"`
	Discrimination float64 `json:"discrimination"`
	Guessing       float64 `json:"guessing"`

	IsUnique bool    `json:"isUnique"`
	PoolTag  *string `json:"poolTag"`
}

// ParsedOptions разбирает Options в []QuestionOption (пустой срез при отсутствии).
func (q TestQuestion) ParsedOptions() []QuestionOption {
	if len(q.Options) == 0 {
		return nil
	}
	var out []QuestionOption
	if err := json.Unmarshal(q.Options, &out); err != nil {
		return nil
	}
	return out
}

type TestSession struct {
	ID          string `json:"id"`
	CandidateID string `json:"candidateId"`
	CategoryID  string `json:"categoryId"`

	Status           SessionStatus `json:"status"`
	StartedAt        Time          `json:"startedAt"`
	CompletedAt      *Time         `json:"completedAt"`
	TimeLimitMinutes int           `json:"timeLimitMinutes"`

	Score          *float64 `json:"score"`
	RawScore       *float64 `json:"rawScore"`
	MaxScore       *float64 `json:"maxScore"`
	GradeBefore    *Grade   `json:"gradeBefore"`
	GradeAfter     *Grade   `json:"gradeAfter"`
	Recommendation *string  `json:"recommendation"`

	CheatFlags   []string `json:"cheatFlags"`
	IsSuspicious bool     `json:"isSuspicious"`

	CurrentDifficulty float64  `json:"currentDifficulty"`
	ThetaEstimate     *float64 `json:"thetaEstimate"`
}

type TestAnswer struct {
	ID         string `json:"id"`
	SessionID  string `json:"sessionId"`
	QuestionID string `json:"questionId"`

	AnswerText       *string  `json:"answerText"`
	IsCorrect        *bool    `json:"isCorrect"`
	Score            *float64 `json:"score"`
	TimeSpentSeconds *int     `json:"timeSpentSeconds"`

	AnsweredAt Time `json:"answeredAt"`
}

type FSPAchievement struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Category    string  `json:"category"` // competition, event, certification
	Year        *int    `json:"year"`
	Place       *string `json:"place"`
	Points      int     `json:"points"`
}

type CandidateAchievement struct {
	ID            string `json:"id"`
	CandidateID   string `json:"candidateId"`
	AchievementID string `json:"achievementId"`
	VerifiedAt    Time   `json:"verifiedAt"`
}

type GamificationProfile struct {
	ID          string `json:"id"`
	CandidateID string `json:"candidateId"`

	Xp             int     `json:"xp"`
	Level          int     `json:"level"`
	Streak         int     `json:"streak"`
	LongestStreak  int     `json:"longestStreak"`
	LastActiveDate *Time   `json:"lastActiveDate"`
	Multiplier     float64 `json:"multiplier"`

	CreatedAt Time `json:"createdAt"`
	UpdatedAt Time `json:"updatedAt"`
}

type Badge struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Category    string `json:"category"` // testing, profile, activity, fsp
	Rarity      string `json:"rarity"`   // common, rare, epic, legendary
	XpReward    int    `json:"xpReward"`
}

type CandidateBadge struct {
	ID          string `json:"id"`
	CandidateID string `json:"candidateId"`
	BadgeID     string `json:"badgeId"`
	EarnedAt    Time   `json:"earnedAt"`
}

type XpEvent struct {
	ID          string          `json:"id"`
	CandidateID string          `json:"candidateId"`
	Action      string          `json:"action"`
	Xp          int             `json:"xp"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   Time            `json:"createdAt"`
}

type DailyTask struct {
	ID          string `json:"id"`
	CandidateID string `json:"candidateId"`

	Title               string `json:"title"`
	Description         string `json:"description"`
	Difficulty          string `json:"difficulty"` // easy, medium, hard
	ExpectedTimeMinutes int    `json:"expectedTimeMinutes"`

	Status          DailyTaskStatus `json:"status"`
	CandidateAnswer *string         `json:"candidateAnswer"`
	Score           *float64        `json:"score"`

	AssignedAt  Time  `json:"assignedAt"`
	DueAt       Time  `json:"dueAt"`
	CompletedAt *Time `json:"completedAt"`
}
