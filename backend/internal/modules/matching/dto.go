// Package matching — matching engine V1/V2, эмбеддинги, pgvector, ML-клиент
// (порт backend/src/modules/matching/**, docs 05-matching-engine.md).
package matching

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// DTO — порт dto/matching.dto.ts. Декодирование повторяет семантику
// class-transformer + class-validator: ValidationPipe({transform, enableImplicitConversion}).
// Порядок сообщений — «снизу вверх» (обратный порядку декораторов, как в auth/candidates).
// Сообщения валидации — дословно из TS (common.Msg*).

// ============ Значения полей DTO (как в candidates/auth) ============

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

// jsToString — String(value) class-transformer для поля с design:type String
// (enableImplicitConversion). ok=false — значение не приводится (массив/null/undefined).
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

// jsToBool — Boolean(value) для поля с design:type Boolean: неявное приведение
// всегда успешно (как в candidates), ложны только "" / 0 / NaN.
func jsToBool(v jsVal) bool {
	switch v.kind {
	case kindBool:
		return v.b
	case kindString:
		return v.s != ""
	case kindNumber:
		return v.n != 0 && !math.IsNaN(v.n)
	default: // массив/объект — truthy
		return true
	}
}

// jsLen — String.length из JS (UTF-16 code units): MaxLength у class-validator.
func jsLen(s string) int {
	units := 0
	for _, r := range s {
		if r > 0xFFFF {
			units += 2
		} else {
			units++
		}
	}
	return units
}

// jsSlice — s.slice(0, n) по UTF-16 code units.
func jsSlice(s string, n int) string {
	if n <= 0 {
		return ""
	}
	units := 0
	for i, r := range s {
		w := 1
		if r > 0xFFFF {
			w = 2
		}
		if units+w > n {
			return s[:i]
		}
		units += w
	}
	return s
}

// decodeObject — тело запроса → поля DTO. Не-объект/null — пустой DTO (как в NestJS:
// plainToInstance на массиве/строке даёт экземпляр без полей).
func decodeObject(b []byte) map[string]jsVal {
	out := map[string]jsVal{}
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" || trimmed == "null" {
		return out
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return out
	}
	for k, v := range raw {
		out[k] = toJSVal(v)
	}
	return out
}

// fieldValue — поле присутствует и не равно null/undefined (@IsOptional пропускает оба).
func fieldValue(fields map[string]jsVal, key string) (jsVal, bool) {
	v, ok := fields[key]
	if !ok || v.kind == kindUndefined || v.kind == kindNull {
		return jsVal{}, false
	}
	return v, true
}

// ============ SearchCandidatesDto (dto/matching.dto.ts) ============

// SearchCandidatesDTO — POST /api/matching/search и /v2/search (docs 04 §6).
type SearchCandidatesDTO struct {
	Description     *string  `json:"description"`
	Specialization  *string  `json:"specialization"`
	Grade           *string  `json:"grade"`
	Stack           []string `json:"stack"`
	HasAchievements *bool    `json:"hasAchievements"`
	Limit           int      `json:"limit"`
	Offset          int      `json:"offset"`

	fields  map[string]jsVal
	decoded bool
}

// UnmarshalJSON — class-transformer + class-property-инициализаторы (limit=20, offset=0).
func (d *SearchCandidatesDTO) UnmarshalJSON(b []byte) error {
	d.fields = decodeObject(b)
	d.decoded = true
	d.assign()
	return nil
}

func (d *SearchCandidatesDTO) assign() {
	// Значения по умолчанию — как инициализаторы свойств класса (применяются при отсутствии ключа;
	// explicit null по решению трактуется как отсутствие — см. отчёт об амбивалентностях).
	d.Limit, d.Offset = 20, 0

	if v, ok := fieldValue(d.fields, "description"); ok {
		if s, ok := jsToString(v); ok {
			d.Description = &s
		}
	}
	if v, ok := fieldValue(d.fields, "specialization"); ok {
		if v.kind == kindString {
			d.Specialization = &v.s
		}
	}
	if v, ok := fieldValue(d.fields, "grade"); ok {
		if v.kind == kindString {
			d.Grade = &v.s
		}
	}
	if v, ok := fieldValue(d.fields, "stack"); ok && v.kind == kindArray {
		out := make([]string, 0, len(v.arr))
		for _, item := range v.arr {
			if s, ok := jsToString(item); ok {
				out = append(out, s)
			}
		}
		d.Stack = out
	}
	if v, ok := fieldValue(d.fields, "hasAchievements"); ok {
		b := jsToBool(v)
		d.HasAchievements = &b
	}
	if v, ok := fieldValue(d.fields, "limit"); ok {
		if n := jsToNumber(v); !math.IsNaN(n) && !math.IsInf(n, 0) {
			d.Limit = int(n)
		}
	}
	if v, ok := fieldValue(d.fields, "offset"); ok {
		if n := jsToNumber(v); !math.IsNaN(n) && !math.IsInf(n, 0) {
			d.Offset = int(n)
		}
	}
}

// Validate — аналог ValidationPipe для SearchCandidatesDto (все поля опциональны).
func (d *SearchCandidatesDTO) Validate() error {
	if !d.decoded {
		// Пустое тело (UnmarshalJSON не вызывался) — все поля опциональны, дефолты как в TS.
		d.Limit, d.Offset = 20, 0
		return nil
	}
	v := &common.V{}

	// description: @IsOptional @IsString @MaxLength(4000)
	if val, ok := fieldValue(d.fields, "description"); ok {
		s, ok := jsToString(val)
		if !ok {
			v.Add(common.MsgIsString("description"))
		} else if jsLen(s) > 4000 {
			v.Add(common.MsgMaxLength("description", 4000))
		}
	}

	// specialization: @IsOptional @IsEnum(Specialization) — design:type Object → без конверсии.
	if val, ok := fieldValue(d.fields, "specialization"); ok {
		if val.kind != kindString {
			v.Add(common.MsgIsIn("specialization", model.SpecializationStrings()))
		} else if _, ok := model.SpecializationFromString(val.s); !ok {
			v.Add(common.MsgIsIn("specialization", model.SpecializationStrings()))
		}
	}

	// grade: @IsOptional @IsEnum(Grade).
	if val, ok := fieldValue(d.fields, "grade"); ok {
		if val.kind != kindString {
			v.Add(common.MsgIsIn("grade", model.GradeStrings()))
		} else if _, ok := model.GradeFromString(val.s); !ok {
			v.Add(common.MsgIsIn("grade", model.GradeStrings()))
		}
	}

	// stack: @IsOptional @IsArray @IsString({each}) @MaxLength(60, {each});
	// порядок сообщений — bottom-up: @MaxLength → @IsString → @IsArray.
	if val, ok := fieldValue(d.fields, "stack"); ok {
		if val.kind != kindArray {
			v.Add(common.MsgIsArray("stack"))
		} else {
			var lenErrs, strErrs []string
			for _, item := range val.arr {
				s, ok := jsToString(item)
				if !ok {
					strErrs = append(strErrs, common.MsgIsString("each value in stack"))
					continue
				}
				if jsLen(s) > 60 {
					lenErrs = append(lenErrs, common.MsgMaxLength("stack", 60))
				}
			}
			for _, m := range lenErrs {
				v.Add(m)
			}
			for _, m := range strErrs {
				v.Add(m)
			}
		}
	}

	// hasAchievements: @IsOptional @IsBoolean — неявное приведение к boolean всегда успешно.
	// (значение уже приведено в assign())

	// limit: @IsOptional @Type(Number) @IsInt @Min(1) @Max(100).
	if val, ok := fieldValue(d.fields, "limit"); ok {
		n := jsToNumber(val)
		switch {
		case math.IsNaN(n) || math.IsInf(n, 0) || math.Trunc(n) != n:
			v.Add(common.MsgIsInt("limit"))
		default:
			if n < 1 {
				v.Add(common.MsgMin("limit", 1))
			}
			if n > 100 {
				v.Add(common.MsgMax("limit", 100))
			}
		}
	}

	// offset: @IsOptional @Type(Number) @IsInt @Min(0).
	if val, ok := fieldValue(d.fields, "offset"); ok {
		n := jsToNumber(val)
		switch {
		case math.IsNaN(n) || math.IsInf(n, 0) || math.Trunc(n) != n:
			v.Add(common.MsgIsInt("offset"))
		default:
			if n < 0 {
				v.Add(common.MsgMin("offset", 0))
			}
		}
	}

	return v.Err()
}

// ============ EmployerNeedDto (dto/matching.dto.ts) ============

// EmployerNeedDTO — POST /api/employers/need и /api/matching/v2/need (docs 04 §3, §6).
type EmployerNeedDTO struct {
	Description    string   `json:"description" binding:"required"`
	Specialization *string  `json:"specialization"`
	Grade          *string  `json:"grade"`
	Stack          []string `json:"stack"`

	fields  map[string]jsVal
	decoded bool
}

// UnmarshalJSON — class-transformer для EmployerNeedDto.
func (d *EmployerNeedDTO) UnmarshalJSON(b []byte) error {
	d.fields = decodeObject(b)
	d.decoded = true
	d.assign()
	return nil
}

func (d *EmployerNeedDTO) assign() {
	if v, ok := fieldValue(d.fields, "description"); ok {
		if s, ok := jsToString(v); ok {
			d.Description = s
		}
	}
	if v, ok := fieldValue(d.fields, "specialization"); ok {
		if v.kind == kindString {
			d.Specialization = &v.s
		}
	}
	if v, ok := fieldValue(d.fields, "grade"); ok {
		if v.kind == kindString {
			d.Grade = &v.s
		}
	}
	if v, ok := fieldValue(d.fields, "stack"); ok && v.kind == kindArray {
		out := make([]string, 0, len(v.arr))
		for _, item := range v.arr {
			if s, ok := jsToString(item); ok {
				out = append(out, s)
			}
		}
		d.Stack = out
	}
}

// Validate — аналог ValidationPipe для EmployerNeedDto.
func (d *EmployerNeedDTO) Validate() error {
	v := &common.V{}
	if !d.decoded {
		// Пустое тело — обязательное поле отсутствует.
		v.Add(common.MsgIsString("description"))
		return v.Err()
	}

	// description: @IsString @MinLength(10) @MaxLength(4000) (без @IsOptional).
	if val, ok := fieldValue(d.fields, "description"); ok {
		s, ok := jsToString(val)
		if !ok {
			v.Add(common.MsgIsString("description"))
		} else {
			if jsLen(s) > 4000 {
				v.Add(common.MsgMaxLength("description", 4000))
			}
			if jsLen(s) < 10 {
				v.Add(common.MsgMinLength("description", 10))
			}
		}
	} else {
		v.Add(common.MsgIsString("description"))
	}

	// specialization: @IsOptional @IsEnum(Specialization).
	if val, ok := fieldValue(d.fields, "specialization"); ok {
		if val.kind != kindString {
			v.Add(common.MsgIsIn("specialization", model.SpecializationStrings()))
		} else if _, ok := model.SpecializationFromString(val.s); !ok {
			v.Add(common.MsgIsIn("specialization", model.SpecializationStrings()))
		}
	}

	// grade: @IsOptional @IsEnum(Grade).
	if val, ok := fieldValue(d.fields, "grade"); ok {
		if val.kind != kindString {
			v.Add(common.MsgIsIn("grade", model.GradeStrings()))
		} else if _, ok := model.GradeFromString(val.s); !ok {
			v.Add(common.MsgIsIn("grade", model.GradeStrings()))
		}
	}

	// stack: @IsOptional @IsArray @IsString({each}) — без MaxLength({each}).
	if val, ok := fieldValue(d.fields, "stack"); ok {
		if val.kind != kindArray {
			v.Add(common.MsgIsArray("stack"))
		} else {
			for _, item := range val.arr {
				if _, ok := jsToString(item); !ok {
					v.Add(common.MsgIsString("each value in stack"))
				}
			}
		}
	}

	return v.Err()
}

// ============ SemanticQueryDto (dto/matching.dto.ts) ============

// SemanticQueryDTO — POST /matching/semantic и /matching/v2/semantic.
type SemanticQueryDTO struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`

	fields  map[string]jsVal
	decoded bool
}

// UnmarshalJSON — class-transformer + инициализатор limit = 10.
func (d *SemanticQueryDTO) UnmarshalJSON(b []byte) error {
	d.fields = decodeObject(b)
	d.decoded = true
	d.assign()
	return nil
}

func (d *SemanticQueryDTO) assign() {
	d.Limit = 10
	if v, ok := fieldValue(d.fields, "query"); ok {
		if s, ok := jsToString(v); ok {
			d.Query = s
		}
	}
	if v, ok := fieldValue(d.fields, "limit"); ok {
		if n := jsToNumber(v); !math.IsNaN(n) && !math.IsInf(n, 0) {
			d.Limit = int(n)
		}
	}
}

// Validate — аналог ValidationPipe для SemanticQueryDto.
func (d *SemanticQueryDTO) Validate() error {
	v := &common.V{}
	if !d.decoded {
		d.Limit = 10
		v.Add(common.MsgIsString("query"))
		return v.Err()
	}

	// query: @IsString @MinLength(3) @MaxLength(2000) (без @IsOptional).
	if val, ok := fieldValue(d.fields, "query"); ok {
		s, ok := jsToString(val)
		if !ok {
			v.Add(common.MsgIsString("query"))
		} else {
			if jsLen(s) > 2000 {
				v.Add(common.MsgMaxLength("query", 2000))
			}
			if jsLen(s) < 3 {
				v.Add(common.MsgMinLength("query", 3))
			}
		}
	} else {
		v.Add(common.MsgIsString("query"))
	}

	// limit: @IsOptional @Type(Number) @IsInt @Min(1) @Max(50).
	if val, ok := fieldValue(d.fields, "limit"); ok {
		n := jsToNumber(val)
		switch {
		case math.IsNaN(n) || math.IsInf(n, 0) || math.Trunc(n) != n:
			v.Add(common.MsgIsInt("limit"))
		default:
			if n < 1 {
				v.Add(common.MsgMin("limit", 1))
			}
			if n > 50 {
				v.Add(common.MsgMax("limit", 50))
			}
		}
	}

	return v.Err()
}
