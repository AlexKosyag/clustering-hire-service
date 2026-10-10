// Package candidates — профиль кандидата, категория/статус тестирования,
// загрузка резюме (PDF-парсинг) и аватара (docs 04 §2, docs 07 §4).
package candidates

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// ============ Значения полей DTO ============
//
// Как в auth: тело проходит ValidationPipe({transform, enableImplicitConversion}),
// поэтому «сырые» значения сохраняются и приводятся по design:type DTO-поля.

type jsKind int

const (
	kindUndefined jsKind = iota // поле отсутствует в теле
	kindNull                    // null
	kindString
	kindNumber
	kindBool
	kindArray
	kindObject
)

// jsVal — значение поля DTO в том виде, в котором его валидирует class-validator.
type jsVal struct {
	kind jsKind
	s    string
	n    float64
	b    bool
	arr  []jsVal
}

// toJSVal — json.RawMessage → jsVal.
func toJSVal(raw json.RawMessage) jsVal {
	if raw == nil {
		return jsVal{kind: kindUndefined}
	}
	if string(raw) == "null" {
		return jsVal{kind: kindNull}
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return jsVal{kind: kindObject}
	}
	switch t := v.(type) {
	case string:
		return jsVal{kind: kindString, s: t}
	case float64:
		return jsVal{kind: kindNumber, n: t}
	case bool:
		return jsVal{kind: kindBool, b: t}
	case []any:
		out := make([]jsVal, len(t))
		for i, item := range t {
			out[i] = fromAny(item)
		}
		return jsVal{kind: kindArray, arr: out}
	default:
		return jsVal{kind: kindObject}
	}
}

// fromAny — значение из json.Unmarshal([]any) → jsVal.
func fromAny(v any) jsVal {
	switch t := v.(type) {
	case nil:
		return jsVal{kind: kindNull}
	case string:
		return jsVal{kind: kindString, s: t}
	case float64:
		return jsVal{kind: kindNumber, n: t}
	case bool:
		return jsVal{kind: kindBool, b: t}
	case []any:
		out := make([]jsVal, len(t))
		for i, item := range t {
			out[i] = fromAny(item)
		}
		return jsVal{kind: kindArray, arr: out}
	default:
		return jsVal{kind: kindObject}
	}
}

// jsToString — String(value) class-transformer для поля с design:type String.
// ok=false — значение не приводится (массив/undefined/null).
func jsToString(v jsVal) (string, bool) {
	switch v.kind {
	case kindString:
		return v.s, true
	case kindNumber:
		return jsNumString(v.n), true
	case kindBool:
		if v.b {
			return "true", true
		}
		return "false", true
	case kindObject:
		// JS: String({}) === '[object Object]'
		return "[object Object]", true
	default:
		return "", false
	}
}

// jsNumString — String(number) в стиле JS.
func jsNumString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	abs := math.Abs(f)
	if abs >= 1e21 || abs < 1e-6 {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		if i := strings.IndexByte(s, 'e'); i >= 0 {
			exp, _ := strconv.Atoi(s[i+1:])
			s = s[:i] + "e" + strconv.Itoa(exp)
		}
		return s
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// jsToNumber — Number(value) из @Type(() => Number); NaN для неприводимых значений.
func jsToNumber(v jsVal) float64 {
	switch v.kind {
	case kindNumber:
		return v.n
	case kindString:
		s := strings.TrimSpace(v.s)
		if s == "" {
			return 0
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		return math.NaN()
	case kindBool:
		if v.b {
			return 1
		}
		return 0
	default:
		return math.NaN()
	}
}

// ============ UpdateCandidateProfileDto (dto/candidate.dto.ts) ============

type candFieldKind int

const (
	candString candFieldKind = iota
	candSpecialization
	candGrade
	candStringArray
	candNumber
	candBool
)

type candFieldDef struct {
	Key    string
	Column string
	Kind   candFieldKind
	MaxLen int
}

// candidateProfileFields — порядок как в UpdateCandidateProfileDto
// (verifiedGrade/testStatus в DTO отсутствуют — их пишет только testing-модуль).
var candidateProfileFields = []candFieldDef{
	{"firstName", "first_name", candString, 60},
	{"lastName", "last_name", candString, 60},
	{"middleName", "middle_name", candString, 60},
	{"phone", "phone", candString, 40},
	{"city", "city", candString, 60},
	{"bio", "bio", candString, 4000},
	{"specialization", "specialization", candSpecialization, 0},
	{"declaredGrade", "declared_grade", candGrade, 0},
	{"stack", "stack", candStringArray, 60},
	{"roles", "roles", candStringArray, 60},
	{"softSkills", "soft_skills", candStringArray, 60},
	{"experienceYears", "experience_years", candNumber, 0},
	{"showEmail", "show_email", candBool, 0},
	{"showPhone", "show_phone", candBool, 0},
	{"showFullName", "show_full_name", candBool, 0},
	{"consentToProcessing", "consent_to_processing", candBool, 0},
}

// UpdateCandidateProfileDto — тело PUT /candidates/profile (все поля опциональны).
type UpdateCandidateProfileDto struct {
	fields map[string]jsVal
}

// UnmarshalJSON сохраняет только известные поля; не-объект = пустой DTO (как в NestJS).
func (d *UpdateCandidateProfileDto) UnmarshalJSON(b []byte) error {
	raw := map[string]json.RawMessage{}
	trimmed := strings.TrimSpace(string(b))
	if trimmed != "" && trimmed != "null" {
		if err := json.Unmarshal(b, &raw); err != nil {
			raw = map[string]json.RawMessage{}
		}
	}
	d.fields = map[string]jsVal{}
	for _, f := range candidateProfileFields {
		if v, ok := raw[f.Key]; ok {
			d.fields[f.Key] = toJSVal(v)
		}
	}
	return nil
}

// Validate — ValidationPipe + class-validator для UpdateCandidateProfileDto.
// @IsOptional пропускает undefined и null; порядок сообщений — как в TS.
func (d *UpdateCandidateProfileDto) Validate() error {
	v := &common.V{}
	for _, def := range candidateProfileFields {
		val, ok := d.fields[def.Key]
		if !ok || val.kind == kindUndefined || val.kind == kindNull {
			continue
		}
		switch def.Kind {
		case candString:
			s, ok := jsToString(val)
			if !ok {
				v.Add(common.MsgIsString(def.Key))
				continue
			}
			if len([]rune(s)) > def.MaxLen {
				v.Add(common.MsgMaxLength(def.Key, def.MaxLen))
			}
		case candSpecialization:
			if val.kind != kindString {
				v.Add(common.MsgIsIn(def.Key, model.SpecializationStrings()))
				continue
			}
			if _, ok := model.SpecializationFromString(val.s); !ok {
				v.Add(common.MsgIsIn(def.Key, model.SpecializationStrings()))
			}
		case candGrade:
			if val.kind != kindString {
				v.Add(common.MsgIsIn(def.Key, model.GradeStrings()))
				continue
			}
			if _, ok := model.GradeFromString(val.s); !ok {
				v.Add(common.MsgIsIn(def.Key, model.GradeStrings()))
			}
		case candStringArray:
			if val.kind != kindArray {
				v.Add(common.MsgIsArray(def.Key))
				continue
			}
			// @MaxLength(60, {each}) → @IsString({each}) → @IsArray.
			var lenErrs, strErrs []string
			for _, item := range val.arr {
				is, ok := jsToString(item)
				if !ok {
					strErrs = append(strErrs, common.MsgIsString("each value in "+def.Key))
					continue
				}
				if len([]rune(is)) > def.MaxLen {
					lenErrs = append(lenErrs, common.MsgMaxLength(def.Key, def.MaxLen))
				}
			}
			for _, m := range lenErrs {
				v.Add(m)
			}
			for _, m := range strErrs {
				v.Add(m)
			}
		case candNumber:
			n := jsToNumber(val)
			if math.IsNaN(n) {
				v.Add(common.MsgMin(def.Key, 0))
				v.Add(common.MsgIsNumber(def.Key))
				continue
			}
			if n < 0 {
				v.Add(common.MsgMin(def.Key, 0))
			}
		case candBool:
			// Неявное приведение к boolean всегда успешно — проверок нет.
		}
	}
	return v.Err()
}

// candUpdate — колонка и значение для UPDATE (nil → NULL).
type candUpdate struct {
	Column string
	Value  any
}

// Updates — присланные поля в порядке объявления DTO (null → обнуление колонки).
func (d *UpdateCandidateProfileDto) Updates() []candUpdate {
	out := []candUpdate{}
	for _, def := range candidateProfileFields {
		val, ok := d.fields[def.Key]
		if !ok {
			continue
		}
		if val.kind == kindNull || val.kind == kindUndefined {
			out = append(out, candUpdate{Column: def.Column, Value: nil})
			continue
		}
		switch def.Kind {
		case candString, candSpecialization, candGrade:
			s, _ := jsToString(val)
			out = append(out, candUpdate{Column: def.Column, Value: s})
		case candStringArray:
			arr := make([]string, 0, len(val.arr))
			for _, item := range val.arr {
				s, _ := jsToString(item)
				arr = append(arr, s)
			}
			out = append(out, candUpdate{Column: def.Column, Value: arr})
		case candNumber:
			n := jsToNumber(val)
			out = append(out, candUpdate{Column: def.Column, Value: n})
		case candBool:
			out = append(out, candUpdate{Column: def.Column, Value: jsToBool(val)})
		}
	}
	return out
}

// jsToBool — Boolean(value) class-transformer (design:type Boolean).
func jsToBool(v jsVal) bool {
	switch v.kind {
	case kindBool:
		return v.b
	case kindString:
		return v.s != ""
	case kindNumber:
		return v.n != 0
	default:
		// Массив/объект: Boolean([]) === false, Boolean([x]) === true — считаем true.
		return true
	}
}

// ============ Ответы ============

// resumePreview — `parsed` в ответе POST /candidates/profile/resume.
// Ключи с undefined-значениями в TS опускаются (omitempty).
type resumePreview struct {
	Name            *string  `json:"name,omitempty"`
	Email           *string  `json:"email,omitempty"`
	Phone           *string  `json:"phone,omitempty"`
	City            *string  `json:"city,omitempty"`
	Skills          []string `json:"skills"`
	ExperienceCount int      `json:"experienceCount"`
	EducationCount  int      `json:"educationCount"`
	Summary         *string  `json:"summary,omitempty"`
}

// ResumeUploadResult — ответ POST /candidates/profile/resume.
type ResumeUploadResult struct {
	ResumePDFURL string         `json:"resumePdfUrl"`
	Parsed       *resumePreview `json:"parsed"`
	AutoFilled   any            `json:"autoFilled"`
}

// AvatarUploadResult — ответ POST /candidates/profile/avatar.
type AvatarUploadResult struct {
	AvatarURL string `json:"avatarUrl"`
}
