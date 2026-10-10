package matching

import (
	"strconv"
	"strings"
	"time"

	"fsp-backend/internal/model"
)

// Порт backend/src/modules/matching/explanation.service.ts (as-is, RU-строки дословно).

// ExplanationDetail — деталь объяснения счёта.
type ExplanationDetail struct {
	Factor      string    `json:"factor"`
	Match       any       `json:"match"` // boolean | string
	Weight      float64   `json:"weight"`
	Explanation string    `json:"explanation"`
	Overlap     *[]string `json:"overlap,omitempty"` // всегда у детали «Стек» (в т.ч. [])
	Detail      *string   `json:"detail,omitempty"`
}

// MatchExplanation — объяснение счёта {overall, details}.
type MatchExplanation struct {
	Overall string              `json:"overall"`
	Details []ExplanationDetail `json:"details"`
}

// ExplanationCandidate — кандидат для объяснения: ScoreableCandidate + поля
// из «...candidate» в TS (firstName/lastName в generate не используются).
type ExplanationCandidate struct {
	ScoreableCandidate
	LastTestDate     *time.Time
	AchievementNames []string
}

// explanationService — ExplanationService.
type explanationService struct{}

// generate — детали по всем весомым факторам (порядок как в TS).
func (s explanationService) generate(candidate ExplanationCandidate, need Need, score MatchScore) MatchExplanation {
	details := []ExplanationDetail{}

	// Специализация
	if need.Specialization == "" {
		details = append(details, ExplanationDetail{
			Factor:      "Специализация",
			Match:       "не указана",
			Weight:      WEIGHTS.Category,
			Explanation: "В потребности специализация не задана — нейтральный балл",
		})
	} else if candidate.Specialization == need.Specialization {
		details = append(details, ExplanationDetail{
			Factor:      "Специализация",
			Match:       true,
			Weight:      WEIGHTS.Category,
			Explanation: "Совпадает: " + string(candidate.Specialization),
		})
	} else {
		spec := string(candidate.Specialization)
		if candidate.Specialization == "" {
			spec = "не указана"
		}
		details = append(details, ExplanationDetail{
			Factor:      "Специализация",
			Match:       false,
			Weight:      WEIGHTS.Category,
			Explanation: "Кандидат — " + spec + ", требуется " + string(need.Specialization),
		})
	}

	// Стек (overlap считается всегда, деталь — только при непустом need.stack)
	overlap := []string{}
	needNorm := make([]string, 0, len(need.Stack))
	for _, n := range need.Stack {
		needNorm = append(needNorm, normalizeTech(n))
	}
	for _, s := range candidate.Stack {
		ns := normalizeTech(s)
		for _, nn := range needNorm {
			if nn == ns {
				overlap = append(overlap, s)
				break
			}
		}
	}
	if len(need.Stack) > 0 {
		joined := strings.Join(overlap, ", ")
		if joined == "" {
			joined = "нет пересечений"
		}
		ov := overlap
		details = append(details, ExplanationDetail{
			Factor:      "Стек",
			Match:       len(overlap) > 0,
			Weight:      WEIGHTS.Stack,
			Overlap:     &ov,
			Explanation: "Совпадение " + strconv.Itoa(len(overlap)) + "/" + strconv.Itoa(len(need.Stack)) + ": " + joined,
		})
	}

	// Грейд
	candidateGrade := candidate.VerifiedGrade
	if candidateGrade == "" {
		candidateGrade = candidate.DeclaredGrade
	}
	if need.Grade != "" {
		idxDiff := gradeIndexOrNil(candidateGrade) - gradeIndexOrNil(need.Grade)
		explanation := ""
		switch {
		case candidateGrade == need.Grade:
			verifier := "заявлен кандидатом"
			if candidate.VerifiedGrade != "" {
				verifier = "подтверждён тестом"
			}
			explanation = "Грейд " + string(need.Grade) + " " + verifier
		case idxDiff == 1:
			explanation = "Грейд выше запросленного (" + string(candidateGrade) + " vs " + string(need.Grade) + ") — допустимо"
		case idxDiff == -1:
			cg := string(candidateGrade)
			if candidateGrade == "" {
				cg = "не указан"
			}
			explanation = "Грейд ниже запросленного (" + cg + " vs " + string(need.Grade) + ") — допустимо"
		case idxDiff > 1:
			explanation = "Избыточно высокий грейд: " + string(candidateGrade) + " при запросе " + string(need.Grade)
		default:
			cg := string(candidateGrade)
			if candidateGrade == "" {
				cg = "не указан"
			}
			explanation = "Грейд ниже запросленного: " + cg + " при запросе " + string(need.Grade)
		}

		details = append(details, ExplanationDetail{
			Factor:      "Грейд",
			Match:       candidateGrade == need.Grade,
			Weight:      WEIGHTS.Grade,
			Explanation: explanation,
		})
	}

	// Семантика
	semanticExplanations := "Описание потребности слабо коррелирует с профилем"
	if score.Components.SemanticSimilarity >= 50 {
		semanticExplanations = "Описание потребности близко к описанию профиля"
	}
	detail := strconv.FormatFloat(score.Components.SemanticSimilarity, 'f', -1, 64) + "%"
	details = append(details, ExplanationDetail{
		Factor:      "Семантическое совпадение",
		Match:       score.Components.SemanticSimilarity >= 50,
		Weight:      WEIGHTS.Semantic,
		Detail:      &detail,
		Explanation: semanticExplanations,
	})

	// Достижения ФСП
	if len(candidate.AchievementNames) > 0 {
		top := candidate.AchievementNames
		if len(top) > 2 {
			top = top[:2]
		}
		joined := strings.Join(top, ", ")
		details = append(details, ExplanationDetail{
			Factor:      "Достижения ФСП",
			Match:       true,
			Weight:      WEIGHTS.Achievement,
			Detail:      &joined,
			Explanation: "Подтверждённые достижения: " + joined,
		})
	}

	// Активность
	if candidate.LastTestDate != nil {
		// Math.max(0, Math.round((Date.now() - lastTestDate) / 86_400_000))
		days := int(jsRound(float64(time.Since(*candidate.LastTestDate).Milliseconds()) / 86400000))
		if days < 0 {
			days = 0
		}
		explanation := "Тестирование было давно"
		if days <= 90 {
			explanation = "Кандидат недавно проходил тестирование"
		}
		d := "тест пройден " + strconv.Itoa(days) + " дн. назад"
		details = append(details, ExplanationDetail{
			Factor:      "Активность",
			Match:       days <= 90,
			Weight:      0,
			Detail:      &d,
			Explanation: explanation,
		})
	}

	return MatchExplanation{Overall: s.summary(score.Total), Details: details}
}

// summary — итоговая строка по суммарному счёту.
func (explanationService) summary(total float64) string {
	if total >= 80 {
		return "Сильное совпадение по ключевым факторам"
	}
	if total >= 60 {
		return "Хорошее совпадение с некоторыми расхождениями"
	}
	if total >= 40 {
		return "Частичное совпадение"
	}
	return "Слабое совпадение"
}

// gradeIndexOrNil — gradeIndex(grade): "" → -1, иначе позиция в GRADE_ORDER (-1 если неизвестен).
func gradeIndexOrNil(grade model.Grade) int {
	if grade == "" {
		return -1
	}
	return gradeIndexOf(grade)
}
