// Package employers — профиль работодателя, логотип, потребность, поиск
// кандидатов, карточка кандидата с гейтингом контактов и дашборд (docs 04 §3).
package employers

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// ============ UpdateEmployerProfileDto ============

// fieldValue — присутствие поля в JSON: в TS сравнивается value !== undefined,
// поэтому ключ с null обнуляет колонку, а отсутствующий ключ не трогает её.
type fieldValue struct {
	present bool
	value   *string // nil при JSON null
	invalid bool    // не строка и не приводимая примитива → "must be a string"
}

var employerProfileFields = []struct {
	key    string
	column string
	maxLen int
}{
	{"companyName", "company_name", 120},
	{"companyDescription", "company_description", 4000},
	{"industry", "industry", 80},
	{"website", "website", 200},
	{"contactPerson", "contact_person", 80},
	{"contactEmail", "contact_email", 120},
	{"contactPhone", "contact_phone", 40},
}

// UpdateEmployerProfileDto — все поля опциональны (@IsOptional @IsString @MaxLength).
type UpdateEmployerProfileDto struct {
	fields map[string]fieldValue
}

func (d *UpdateEmployerProfileDto) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	d.fields = map[string]fieldValue{}
	for _, f := range employerProfileFields {
		v, ok := raw[f.key]
		if !ok {
			continue
		}
		fv := fieldValue{present: true}
		var s string
		switch {
		case string(v) == "null":
			fv.value = nil
		case json.Unmarshal(v, &s) == nil:
			fv.value = &s
		default:
			// неявное преобразование class-transformer (enableImplicitConversion)
			var num float64
			var boolean bool
			if json.Unmarshal(v, &num) == nil {
				s = jsonNumString(num)
				fv.value = &s
			} else if json.Unmarshal(v, &boolean) == nil {
				if boolean {
					s = "true"
				} else {
					s = "false"
				}
				fv.value = &s
			} else {
				fv.invalid = true
			}
		}
		d.fields[f.key] = fv
	}
	return nil
}

func jsonNumString(f float64) string {
	return fmt.Sprintf("%v", f)
}

// Validate — ValidationPipe для UpdateEmployerProfileDto.
func (d *UpdateEmployerProfileDto) Validate() error {
	v := &common.V{}
	for _, f := range employerProfileFields {
		fv, ok := d.fields[f.key]
		if !ok || !fv.present {
			continue
		}
		if fv.invalid {
			v.Add(common.MsgIsString(f.key))
			continue
		}
		if fv.value != nil && len([]rune(*fv.value)) > f.maxLen {
			v.Add(common.MsgMaxLength(f.key, f.maxLen))
		}
	}
	return v.Err()
}

// ============ EmployerSearchQueryDto (extends SearchCandidatesDto) ============

// EmployerSearchQueryDto — GET /employers/candidates/search.
// super: description, specialization, grade, stack, hasAchievements, limit, offset
// + stack приходит строкой "Node.js,PostgreSQL" и трансформируется в массив.
type EmployerSearchQueryDto struct {
	Description     *string
	Specialization  *model.Specialization
	Grade           *model.Grade
	Stack           []string
	HasAchievements *bool
	Limit           int
	Offset          int
}

// ParseEmployerSearchQuery — разбор query с валидацией class-validator
// (enableImplicitConversion: limit/offset → number, hasAchievements → boolean).
func ParseEmployerSearchQuery(c *gin.Context) (*EmployerSearchQueryDto, error) {
	v := &common.V{}
	d := &EmployerSearchQueryDto{Limit: 20, Offset: 0}

	if raw := c.Query("description"); raw != "" {
		if len([]rune(raw)) > 4000 {
			v.Add(common.MsgMaxLength("description", 4000))
		} else {
			d.Description = &raw
		}
	}

	if raw := c.Query("specialization"); raw != "" {
		if spec, ok := model.SpecializationFromString(raw); ok {
			d.Specialization = &spec
		} else {
			v.Add(common.MsgIsIn("specialization", model.SpecializationStrings()))
		}
	}

	if raw := c.Query("grade"); raw != "" {
		if g, ok := model.GradeFromString(raw); ok {
			d.Grade = &g
		} else {
			v.Add(common.MsgIsIn("grade", model.GradeStrings()))
		}
	}

	// stack: строка через запятую → массив (@Transform split(',').filter(Boolean))
	stackValues := c.QueryArray("stack")
	if len(stackValues) == 1 {
		d.Stack = common.SplitStack(stackValues[0])
	} else {
		d.Stack = stackValues
		if d.Stack == nil {
			d.Stack = []string{}
		}
	}
	for _, item := range d.Stack {
		if len([]rune(item)) > 60 {
			v.Add(common.MsgMaxLength("stack", 60))
			break
		}
	}

	if raw := c.Query("hasAchievements"); raw != "" {
		b := coerceBoolQuery(raw)
		d.HasAchievements = &b
	}

	if raw := c.Query("limit"); raw != "" {
		n, err := parseIntStrict(raw)
		switch {
		case err != nil:
			v.Add(common.MsgIsInt("limit"))
		case n < 1:
			v.Add(common.MsgMin("limit", 1))
		case n > 100:
			v.Add(common.MsgMax("limit", 100))
		default:
			d.Limit = n
		}
	}

	if raw := c.Query("offset"); raw != "" {
		n, err := parseIntStrict(raw)
		switch {
		case err != nil:
			v.Add(common.MsgIsInt("offset"))
		case n < 0:
			v.Add(common.MsgMin("offset", 0))
		default:
			d.Offset = n
		}
	}

	if err := v.Err(); err != nil {
		return nil, err
	}
	return d, nil
}

// coerceBoolQuery — class-transformer неявное преобразование в boolean.
func coerceBoolQuery(raw string) bool {
	switch raw {
	case "false", "0", "off":
		return false
	}
	return true
}

// parseIntStrict — как @Type(Number) + @IsInt: не число или дробное → ошибка валидации.
func parseIntStrict(raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("not an integer")
	}
	return n, nil
}

// ============ CreateNeedDto (аналог matching EmployerNeedDto на стороне вызова) ============

// CreateNeedDto — POST /employers/need: валидация как у matching.EmployerNeedDto
// (@IsString @MinLength(10) @MaxLength(4000), специализация/грейд — enum, стек — массив строк).
type CreateNeedDto struct {
	Description    string   `json:"description"`
	Specialization *string  `json:"specialization"`
	Grade          *string  `json:"grade"`
	Stack          []string `json:"stack"`
}

func (d *CreateNeedDto) Validate() error {
	v := &common.V{}
	if len([]rune(d.Description)) < 10 {
		v.Add(common.MsgMinLength("description", 10))
	} else if len([]rune(d.Description)) > 4000 {
		v.Add(common.MsgMaxLength("description", 4000))
	}
	if d.Specialization != nil {
		if _, ok := model.SpecializationFromString(*d.Specialization); !ok {
			v.Add(common.MsgIsIn("specialization", model.SpecializationStrings()))
		}
	}
	if d.Grade != nil {
		if _, ok := model.GradeFromString(*d.Grade); !ok {
			v.Add(common.MsgIsIn("grade", model.GradeStrings()))
		}
	}
	return v.Err()
}
