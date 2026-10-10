// Package seed — данные демо-сида (порт prisma/seed.ts + prisma/seed/*.ts).
//
// JSON-файлы в data/ сгенерированы скриптом backend/prisma/dump-seed.ts
// (прямой импорт TS-модулей, т.е. данные — точная копия эталона). Даты
// сериализованы смещением от момента дампа ({"__rel": мс}) и пересчитываются
// от time.Now() при каждом запуске — как Date.now() в seed.ts (баг F3).
package seed

import (
	"embed"
	"encoding/json"
	"fmt"
	"time"
)

//go:embed data/*.json
var dataFS embed.FS

// ============ relDate: дата сида как смещение от «сейчас» ============

// relDate — значение поля вида { "__rel": <мс от сейчас>} либо null.
type relDate struct {
	ok  bool
	rel time.Duration
}

func (d *relDate) UnmarshalJSON(b []byte) error {
	d.ok = false
	d.rel = 0
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var v struct {
		Rel int64 `json:"__rel"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	d.ok = true
	d.rel = time.Duration(v.Rel) * time.Millisecond
	return nil
}

// At — момент времени = now + смещение; nil, если поле null.
func (d *relDate) At(now time.Time) *time.Time {
	if !d.ok {
		return nil
	}
	t := now.Add(d.rel)
	return &t
}

// ============ Структуры данных ============

// Achievement — запись справочника FSP_ACHIEVEMENTS.
type Achievement struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Category    string  `json:"category"`
	Year        *int    `json:"year"`
	Place       *string `json:"place"`
	Points      int     `json:"points"`
}

// Category — категория теста (buildQuestionSeeds).
type Category struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Specialization string `json:"specialization"`
}

// QuestionOption — вариант ответа multiple_choice.
type QuestionOption struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	IsCorrect bool   `json:"isCorrect"`
}

// Question — вопрос банка.
type Question struct {
	Key            string          `json:"key"`
	QuestionType   string          `json:"questionType"`
	Text           string          `json:"text"`
	CodeSnippet    *string         `json:"codeSnippet"`
	Options        json.RawMessage `json:"options"`
	CorrectAnswer  *string         `json:"correctAnswer"`
	Explanation    *string         `json:"explanation"`
	Difficulty     float64         `json:"difficulty"`
	Discrimination float64         `json:"discrimination"`
	Guessing       float64         `json:"guessing"`
	IsUnique       bool            `json:"isUnique"`
	PoolTag        *string         `json:"poolTag"`
}

// OptionsValue — options/correctAnswer: null → SQL NULL (как ?? null в TS).
func (q *Question) OptionsValue() any {
	raw := q.Options
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return string(raw)
}

// QuestionSeeds — содержимое questions.json.
type QuestionSeeds struct {
	Categories []Category `json:"categories"`
	Questions  []Question `json:"questions"`
}

// Candidate — кандидат (CandidateSeed из seed/people.ts).
type Candidate struct {
	Email             string   `json:"email"`
	FirstName         string   `json:"firstName"`
	LastName          string   `json:"lastName"`
	MiddleName        *string  `json:"middleName"`
	Phone             string   `json:"phone"`
	City              string   `json:"city"`
	Bio               string   `json:"bio"`
	Specialization    string   `json:"specialization"`
	DeclaredGrade     string   `json:"declaredGrade"`
	VerifiedGrade     *string  `json:"verifiedGrade"`
	Stack             []string `json:"stack"`
	Roles             []string `json:"roles"`
	SoftSkills        []string `json:"softSkills"`
	ExperienceYears   float64  `json:"experienceYears"`
	ShowEmail         bool     `json:"showEmail"`
	ShowPhone         bool     `json:"showPhone"`
	ShowFullName      bool     `json:"showFullName"`
	ConsentToProcess  bool     `json:"consentToProcessing"`
	TestStatus        string   `json:"testStatus"`
	LastTestDate      relDate  `json:"lastTestDate"`
	TestCooldownUntil relDate  `json:"testCooldownUntil"`
	LastTestScore     *float64 `json:"lastTestScore"`
	FspID             *string  `json:"fspId"`
	AchievementNames  []string `json:"achievementNames"`
	IsDemo            bool     `json:"isDemo"`
}

// Employer — работодатель (EMPLOYERS из seed/companies.ts).
type Employer struct {
	Email              string  `json:"email"`
	CompanyName        string  `json:"companyName"`
	CompanyDescription *string `json:"companyDescription"`
	Industry           *string `json:"industry"`
	Website            *string `json:"website"`
	ContactPerson      *string `json:"contactPerson"`
	ContactEmail       *string `json:"contactEmail"`
	ContactPhone       *string `json:"contactPhone"`
	IsDemo             bool    `json:"isDemo"`
}

// CompanySeeds — companies.json.
type CompanySeeds struct {
	Employers []Employer `json:"employers"`
}

// Vacancy — вакансия (buildVacancies).
type Vacancy struct {
	EmployerIndex  int      `json:"employerIndex"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Specialization string   `json:"specialization"`
	Grade          string   `json:"grade"`
	Stack          []string `json:"stack"`
	SalaryFrom     int      `json:"salaryFrom"`
	SalaryTo       int      `json:"salaryTo"`
	IsActive       bool     `json:"isActive"`
}

// Invitation — приглашение (buildInvitations).
type Invitation struct {
	EmployerIndex  int     `json:"employerIndex"`
	CandidateEmail string  `json:"candidateEmail"`
	VacancyIndex   *int    `json:"vacancyIndex"`
	Message        *string `json:"message"`
	SalaryFrom     int     `json:"salaryFrom"`
	SalaryTo       int     `json:"salaryTo"`
	Status         string  `json:"status"`
}

// Need — потребность работодателя (buildNeeds).
type Need struct {
	EmployerIndex  int      `json:"employerIndex"`
	Description    string   `json:"description"`
	Specialization *string  `json:"specialization"`
	Grade          *string  `json:"grade"`
	Stack          []string `json:"stack"`
}

// ============ Загрузка ============

func loadJSON(name string, out any) error {
	b, err := dataFS.ReadFile("data/" + name)
	if err != nil {
		return fmt.Errorf("seed data %s: %w", name, err)
	}
	return json.Unmarshal(b, out)
}

// LoadAchievements — справочник достижений ФСП.
func LoadAchievements() ([]Achievement, error) {
	var out []Achievement
	return out, loadJSON("achievements.json", &out)
}

// LoadQuestions — категории и вопросы (858).
func LoadQuestions() (*QuestionSeeds, error) {
	out := &QuestionSeeds{}
	return out, loadJSON("questions.json", out)
}

// LoadCandidates — 204 кандидата (3 демо + 201 сгенерированный).
func LoadCandidates() ([]Candidate, error) {
	var out []Candidate
	return out, loadJSON("candidates.json", &out)
}

// LoadCompanies — 8 работодателей.
func LoadCompanies() (*CompanySeeds, error) {
	out := &CompanySeeds{}
	return out, loadJSON("companies.json", out)
}

// LoadVacancies — 30 вакансий.
func LoadVacancies() ([]Vacancy, error) {
	var out []Vacancy
	return out, loadJSON("vacancies.json", &out)
}

// LoadInvitations — 15 приглашений.
func LoadInvitations() ([]Invitation, error) {
	var out []Invitation
	return out, loadJSON("invitations.json", &out)
}

// LoadNeeds — 6 потребностей работодателей.
func LoadNeeds() ([]Need, error) {
	var out []Need
	return out, loadJSON("needs.json", &out)
}
