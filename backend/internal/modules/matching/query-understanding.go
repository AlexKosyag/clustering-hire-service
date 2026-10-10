package matching

import (
	"context"
	"log"
	"strings"

	"fsp-backend/internal/model"
	"fsp-backend/internal/modules/candidateview"
)

// Порт backend/src/modules/matching/query-understanding.service.ts (as-is).

// EmployerNeedParsed — структурированное представление потребности работодателя.
type EmployerNeedParsed struct {
	Description       string
	Specialization    model.Specialization // "" = null
	Grade             model.Grade          // "" = null
	RequiredSkills    []string
	PreferredSkills   []string
	Context           *string
	Embedding         []float64
	EmbeddingProvider string
}

// queryUnderstandingService — V2: Query Understanding Service.
// Преобразует свободный текст потребности в структурированный запрос (ML или fallback).
type queryUnderstandingService struct {
	ml mlAPI
}

func newQueryUnderstandingService(ml mlAPI) queryUnderstandingService {
	return queryUnderstandingService{ml: ml}
}

// parseNeed — разбор описания потребности (explicit-фильтры имеют приоритет).
func (q queryUnderstandingService) parseNeed(
	ctx context.Context,
	description string,
	explicitSpecialization model.Specialization,
	explicitGrade model.Grade,
	explicitStack []string,
) EmployerNeedParsed {
	t0 := nowMs()

	// Get ML parsing
	understanding := q.ml.ParseQuery(ctx, description)
	parsed := understanding.Parsed

	// Merge explicit filters with ML-parsed (explicit takes priority)
	specialization := explicitSpecialization
	if specialization == "" {
		specialization = toSpecialization(parsed.Specialization)
	}
	grade := explicitGrade
	if grade == "" && parsed.Seniority != nil {
		grade = model.Grade(*parsed.Seniority)
	}

	// Merge skills: [...new Set([...mlSkills, ...explicitSkills])] — ml первыми, дедупликация.
	seen := map[string]bool{}
	requiredSkills := []string{}
	for _, s := range parsed.RequiredSkills {
		if !seen[s] {
			seen[s] = true
			requiredSkills = append(requiredSkills, s)
		}
	}
	if explicitStack == nil {
		explicitStack = []string{}
	}
	for _, s := range explicitStack {
		if !seen[s] {
			seen[s] = true
			requiredSkills = append(requiredSkills, s)
		}
	}

	specLog := "null"
	if specialization != "" {
		specLog = string(specialization)
	}
	gradeLog := "null"
	if grade != "" {
		gradeLog = string(grade)
	}
	log.Printf("[QueryUnderstanding] Parsed need in %dms — spec=%s, grade=%s, skills=[%s], provider=%s",
		nowMs()-t0, specLog, gradeLog, strings.Join(requiredSkills, ", "), understanding.EmbeddingProvider)

	return EmployerNeedParsed{
		Description:       description,
		Specialization:    specialization,
		Grade:             grade,
		RequiredSkills:    requiredSkills,
		PreferredSkills:   parsed.PreferredSkills,
		Context:           parsed.Context,
		Embedding:         understanding.Embedding,
		EmbeddingProvider: understanding.EmbeddingProvider,
	}
}

// buildCandidateText — текстовое представление кандидата для re-ranking.
func (q queryUnderstandingService) buildCandidateText(candidate *candidateview.Candidate) string {
	parts := []string{}

	if candidate.Specialization != nil && *candidate.Specialization != "" {
		parts = append(parts, "Specialization: "+*candidate.Specialization)
	}

	grade := candidate.VerifiedGrade
	if grade == nil || *grade == "" {
		grade = candidate.DeclaredGrade
	}
	if grade != nil && *grade != "" {
		parts = append(parts, "Grade: "+*grade)
	}

	if len(candidate.Stack) > 0 {
		parts = append(parts, "Tech stack: "+strings.Join(candidate.Stack, ", "))
	}

	if candidate.ExperienceYears != nil {
		parts = append(parts, "Experience: "+jsNumString(*candidate.ExperienceYears)+" years")
	}

	if len(candidate.Roles) > 0 {
		parts = append(parts, "Roles: "+strings.Join(candidate.Roles, ", "))
	}

	if len(candidate.SoftSkills) > 0 {
		parts = append(parts, "Soft skills: "+strings.Join(candidate.SoftSkills, ", "))
	}

	if candidate.Bio != nil && *candidate.Bio != "" {
		parts = append(parts, "Bio: "+*candidate.Bio)
	}

	return strings.Join(parts, ". ")
}

// toSpecialization — валидация значения enum из ML-выдачи.
func toSpecialization(spec *string) model.Specialization {
	if spec == nil || *spec == "" {
		return ""
	}
	if v, ok := model.SpecializationFromString(*spec); ok {
		return v
	}
	return ""
}
