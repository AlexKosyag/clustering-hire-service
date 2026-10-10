// Package vacancies — публичные вакансии и их управление работодателем (docs 04 §4).
package vacancies

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// ============ Значения полей DTO ============
//
// Как в auth/candidates: тело проходит ValidationPipe({transform, enableImplicitConversion}).

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
	return fromAny(v)
}

// fromAny — разобранное JSON-значение → jsVal.
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

// jsToString — String(value) class-transformer для design:type String.
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

// jsToNumber — Number(value) из @Type(() => Number).
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

// jsIsInt — @IsInt: конечное целое число.
func jsIsInt(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) && math.Trunc(n) == n }

// ============ Таблица полей вакансии ============

type vacFieldKind int

const (
	vacString vacFieldKind = iota
	vacSpecialization
	vacGrade
	vacStringArray
	vacInt
	vacBool
)

type vacFieldDef struct {
	Key      string
	Column   string
	Kind     vacFieldKind
	MinLen   int
	MaxLen   int
	Required bool // @IsOptional отсутствует → отсутствие/null — ошибка валидации
}

// createVacancyFields — CreateVacancyDto (vacancy.dto.ts), порядок объявления.
var createVacancyFields = []vacFieldDef{
	{"title", "title", vacString, 3, 160, true},
	{"description", "description", vacString, 20, 10000, true},
	{"specialization", "specialization", vacSpecialization, 0, 0, true},
	{"grade", "grade", vacGrade, 0, 0, true},
	{"stack", "stack", vacStringArray, 0, 60, true},
	{"salaryFrom", "salary_from", vacInt, 0, 0, true},
	{"salaryTo", "salary_to", vacInt, 0, 0, true},
	{"isActive", "is_active", vacBool, 0, 0, false},
}

// updateVacancyFields — UpdateVacancyDto: те же поля, все опциональные.
var updateVacancyFields = []vacFieldDef{
	{"title", "title", vacString, 3, 160, false},
	{"description", "description", vacString, 20, 10000, false},
	{"specialization", "specialization", vacSpecialization, 0, 0, false},
	{"grade", "grade", vacGrade, 0, 0, false},
	{"stack", "stack", vacStringArray, 0, 60, false},
	{"salaryFrom", "salary_from", vacInt, 0, 0, false},
	{"salaryTo", "salary_to", vacInt, 0, 0, false},
	{"isActive", "is_active", vacBool, 0, 0, false},
}

// parseDTO — разбор тела в плоский map известных полей.
func parseDTO(b []byte, defs []vacFieldDef) map[string]jsVal {
	raw := map[string]json.RawMessage{}
	trimmed := strings.TrimSpace(string(b))
	if trimmed != "" && trimmed != "null" {
		if err := json.Unmarshal(b, &raw); err != nil {
			raw = map[string]json.RawMessage{}
		}
	}
	fields := map[string]jsVal{}
	for _, def := range defs {
		if v, ok := raw[def.Key]; ok {
			fields[def.Key] = toJSVal(v)
		}
	}
	return fields
}

// validateField — class-validator для одного поля (порядок сообщений — снизу вверх).
func validateField(v *common.V, def vacFieldDef, val jsVal) {
	required := def.Required
	absentOrNull := val.kind == kindUndefined || val.kind == kindNull
	if !required && absentOrNull {
		return // @IsOptional
	}

	switch def.Kind {
	case vacString:
		s, ok := jsToString(val)
		if !ok {
			// undefined/null/массив: MaxLength, MinLength, IsString — все падают.
			if def.MaxLen > 0 {
				v.Add(common.MsgMaxLength(def.Key, def.MaxLen))
			}
			if def.MinLen > 0 {
				v.Add(common.MsgMinLength(def.Key, def.MinLen))
			}
			v.Add(common.MsgIsString(def.Key))
			break
		}
		if def.MaxLen > 0 && len([]rune(s)) > def.MaxLen {
			v.Add(common.MsgMaxLength(def.Key, def.MaxLen))
		}
		if def.MinLen > 0 && len([]rune(s)) < def.MinLen {
			v.Add(common.MsgMinLength(def.Key, def.MinLen))
		}
	case vacSpecialization:
		if val.kind != kindString {
			v.Add(common.MsgIsIn(def.Key, model.SpecializationStrings()))
			break
		}
		if _, ok := model.SpecializationFromString(val.s); !ok {
			v.Add(common.MsgIsIn(def.Key, model.SpecializationStrings()))
		}
	case vacGrade:
		if val.kind != kindString {
			v.Add(common.MsgIsIn(def.Key, model.GradeStrings()))
			break
		}
		if _, ok := model.GradeFromString(val.s); !ok {
			v.Add(common.MsgIsIn(def.Key, model.GradeStrings()))
		}
	case vacStringArray:
		// @MaxLength(60, {each}) → @IsString({each}) → @IsArray.
		if val.kind == kindArray {
			var lenErrs, strErrs []string
			for _, item := range val.arr {
				is, ok := jsToString(item)
				if !ok {
					strErrs = append(strErrs, common.MsgIsString("each value in "+def.Key))
					continue
				}
				if def.MaxLen > 0 && len([]rune(is)) > def.MaxLen {
					lenErrs = append(lenErrs, common.MsgMaxLength(def.Key, def.MaxLen))
				}
			}
			for _, m := range lenErrs {
				v.Add(m)
			}
			for _, m := range strErrs {
				v.Add(m)
			}
			break
		}
		v.Add(common.MsgIsArray(def.Key))
	case vacInt:
		n := jsToNumber(val)
		if math.IsNaN(n) || math.IsInf(n, 0) {
			v.Add(common.MsgMin(def.Key, 0))
			v.Add(common.MsgIsInt(def.Key))
			break
		}
		if n < 0 {
			v.Add(common.MsgMin(def.Key, 0))
		}
		if math.Trunc(n) != n {
			v.Add(common.MsgIsInt(def.Key))
		}
	case vacBool:
		// Неявное приведение к boolean (@IsBoolean после transform) всегда успешно.
	}
}

// ============ CreateVacancyDto ============

// CreateVacancyDto — тело POST /vacancies.
type CreateVacancyDto struct {
	fields map[string]jsVal
}

func (d *CreateVacancyDto) UnmarshalJSON(b []byte) error {
	d.fields = parseDTO(b, createVacancyFields)
	return nil
}

// Validate — ValidationPipe + class-validator для CreateVacancyDto.
func (d *CreateVacancyDto) Validate() error {
	v := &common.V{}
	for _, def := range createVacancyFields {
		validateField(v, def, d.fields[def.Key]) // отсутствующее поле → jsVal{kindUndefined}
	}
	return v.Err()
}

// createVacancyData — нормализованные значения для INSERT.
type createVacancyData struct {
	Title          string
	Description    string
	Specialization model.Specialization
	Grade          model.Grade
	Stack          []string
	SalaryFrom     int
	SalaryTo       int
	IsActive       bool // default true
}

// Value — значения валидированного DTO (вызывается после Validate).
func (d *CreateVacancyDto) Value() createVacancyData {
	out := createVacancyData{IsActive: true}
	if s, ok := jsToString(d.fields["title"]); ok {
		out.Title = s
	}
	if s, ok := jsToString(d.fields["description"]); ok {
		out.Description = s
	}
	if val := d.fields["specialization"]; val.kind == kindString {
		out.Specialization = model.Specialization(val.s)
	}
	if val := d.fields["grade"]; val.kind == kindString {
		out.Grade = model.Grade(val.s)
	}
	out.Stack = toArray(d.fields["stack"])
	if val := d.fields["salaryFrom"]; val.kind != kindUndefined && val.kind != kindNull {
		out.SalaryFrom = int(jsToNumber(val))
	}
	if val := d.fields["salaryTo"]; val.kind != kindUndefined && val.kind != kindNull {
		out.SalaryTo = int(jsToNumber(val))
	}
	if val := d.fields["isActive"]; val.kind == kindBool {
		out.IsActive = val.b
	}
	return out
}

// toArray — []string из массива (элементы приводятся к строке).
func toArray(v jsVal) []string {
	if v.kind != kindArray {
		return []string{}
	}
	out := make([]string, 0, len(v.arr))
	for _, item := range v.arr {
		s, _ := jsToString(item)
		out = append(out, s)
	}
	return out
}

// ============ UpdateVacancyDto ============

// UpdateVacancyDto — тело PUT /vacancies/:id (все поля опциональны).
type UpdateVacancyDto struct {
	fields map[string]jsVal
}

func (d *UpdateVacancyDto) UnmarshalJSON(b []byte) error {
	d.fields = parseDTO(b, updateVacancyFields)
	return nil
}

// Validate — ValidationPipe + class-validator для UpdateVacancyDto.
func (d *UpdateVacancyDto) Validate() error {
	v := &common.V{}
	for _, def := range updateVacancyFields {
		validateField(v, def, d.fields[def.Key])
	}
	return v.Err()
}

// vacUpdate — колонка и значение для UPDATE (nil → NULL).
type vacUpdate struct {
	Column string
	Value  any
}

// Updates — присланные поля в порядке объявления DTO (null → обнуление колонки).
func (d *UpdateVacancyDto) Updates() []vacUpdate {
	out := []vacUpdate{}
	for _, def := range updateVacancyFields {
		val, ok := d.fields[def.Key]
		if !ok {
			continue
		}
		if val.kind == kindNull || val.kind == kindUndefined {
			out = append(out, vacUpdate{Column: def.Column, Value: nil})
			continue
		}
		switch def.Kind {
		case vacString:
			s, _ := jsToString(val)
			out = append(out, vacUpdate{Column: def.Column, Value: s})
		case vacSpecialization:
			out = append(out, vacUpdate{Column: def.Column, Value: string(specOrEmpty(val))})
		case vacGrade:
			out = append(out, vacUpdate{Column: def.Column, Value: string(gradeOrEmpty(val))})
		case vacStringArray:
			out = append(out, vacUpdate{Column: def.Column, Value: toArray(val)})
		case vacInt:
			out = append(out, vacUpdate{Column: def.Column, Value: int(jsToNumber(val))})
		case vacBool:
			out = append(out, vacUpdate{Column: def.Column, Value: val.b})
		}
	}
	return out
}

func specOrEmpty(v jsVal) model.Specialization {
	if v.kind == kindString {
		return model.Specialization(v.s)
	}
	return ""
}

func gradeOrEmpty(v jsVal) model.Grade {
	if v.kind == kindString {
		return model.Grade(v.s)
	}
	return ""
}

// ============ Query GET /vacancies ============

// ListQuery — query-параметры публичного списка (Record<string, string> в TS).
type ListQuery struct {
	Specialization *string
	Grade          *string
	EmployerID     *string
	Search         *string
	OnlyActive     bool
	Limit          int
	Offset         int
}

// ParseListQuery — разбор query без валидации (в TS её нет):
//
//	limit  = Math.min(Math.max(Number(limit) || 20, 1), 100)
//	offset = Math.max(Number(offset) || 0, 0)
//	onlyActive !== 'false' → только активные вакансии
func ParseListQuery(c *gin.Context) *ListQuery {
	q := &ListQuery{OnlyActive: true, Limit: 20, Offset: 0}
	if raw := c.Query("specialization"); raw != "" {
		q.Specialization = &raw
	}
	if raw := c.Query("grade"); raw != "" {
		q.Grade = &raw
	}
	if raw := c.Query("employerId"); raw != "" {
		q.EmployerID = &raw
	}
	if raw := c.Query("search"); raw != "" {
		q.Search = &raw
	}
	if raw := c.Query("onlyActive"); raw == "false" {
		q.OnlyActive = false
	}
	q.Limit = clampLimit(c.Query("limit"))
	q.Offset = clampOffset(c.Query("offset"))
	return q
}

// clampLimit — Math.min(Math.max(Number(raw) || 20, 1), 100).
func clampLimit(raw string) int {
	f := jsNumberOr(raw, 20)
	f = math.Max(f, 1)
	f = math.Min(f, 100)
	return int(f) // Array.prototype.slice-подобное отсечение дробной части
}

// clampOffset — Math.max(Number(raw) || 0, 0).
func clampOffset(raw string) int {
	f := jsNumberOr(raw, 0)
	f = math.Max(f, 0)
	return int(f)
}

// jsNumberOr — Number(raw) c фолбэком `|| fallback` (NaN/0/пусто → fallback).
func jsNumberOr(raw string, fallback float64) float64 {
	s := strings.TrimSpace(raw)
	if s == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || f == 0 {
		return fallback
	}
	return f
}

// ============ Ответы ============

// employerCard — employer {select: {id, companyName, logoUrl, industry}} (список).
type employerCard struct {
	ID          string  `json:"id"`
	CompanyName string  `json:"companyName"`
	LogoURL     *string `json:"logoUrl"`
	Industry    *string `json:"industry"`
}

// employerCardFull — + website (GET /vacancies/:id).
type employerCardFull struct {
	ID          string  `json:"id"`
	CompanyName string  `json:"companyName"`
	LogoURL     *string `json:"logoUrl"`
	Industry    *string `json:"industry"`
	Website     *string `json:"website"`
}

// employerCardShort — {id, companyName, logoUrl} (ответ POST /vacancies).
type employerCardShort struct {
	ID          string  `json:"id"`
	CompanyName string  `json:"companyName"`
	LogoURL     *string `json:"logoUrl"`
}

// VacancyWithEmployer — элемент GET /vacancies.
type VacancyWithEmployer struct {
	model.Vacancy
	Employer employerCard `json:"employer"`
}

// VacancyWithEmployerFull — ответ GET /vacancies/:id.
type VacancyWithEmployerFull struct {
	model.Vacancy
	Employer employerCardFull `json:"employer"`
}

// VacancyWithEmployerShort — ответ POST /vacancies.
type VacancyWithEmployerShort struct {
	model.Vacancy
	Employer employerCardShort `json:"employer"`
}

// ListResult — ответ GET /vacancies.
type ListResult struct {
	Total     int                   `json:"total"`
	Vacancies []VacancyWithEmployer `json:"vacancies"`
}

// DeleteVacancyResult — ответ DELETE /vacancies/:id (@HttpCode(200)).
type DeleteVacancyResult struct {
	Deleted bool `json:"deleted"`
}
