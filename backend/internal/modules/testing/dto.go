// Package testing — testing engine бэкенда (V1 + V2): порт
// backend/src/modules/testing/** (docs 06-testing-engine.md).
package testing

import (
	"encoding/json"
	"math"
	"strconv"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// DTO — порт dto/testing.dto.ts. Декодирование повторяет семантику
// class-transformer + class-validator: сообщения валидации — дословно из TS
// (common.MsgIsString/MsgMaxLength/MsgIsIn/MsgIsInt/MsgMin), необязательные
// null/отсутствующие поля пропускаются (@IsOptional).

var (
	specializationValues = func() []string {
		out := make([]string, 0, len(model.AllSpecializations))
		for _, s := range model.AllSpecializations {
			out = append(out, string(s))
		}
		return out
	}()

	gradeValues = func() []string {
		out := make([]string, 0, len(model.GradeOrder))
		for _, g := range model.GradeOrder {
			out = append(out, string(g))
		}
		return out
	}()
)

// StartTestDto — POST /testing/start, /testing/v2/start.
type StartTestDto struct {
	Specialization *model.Specialization // @IsEnum(Specialization) — обязательный
	DeclaredGrade  *model.Grade          // @IsEnum(Grade) — опциональный
	errs           []string
	decoded        bool
}

func (d *StartTestDto) UnmarshalJSON(b []byte) error {
	raw, err := decodeObject(b)
	if err != nil {
		return err
	}
	p := &fieldParser{raw: raw}
	if v := p.enum("specialization", specializationValues, true); v != nil {
		spec := model.Specialization(*v)
		d.Specialization = &spec
	}
	if v := p.enum("declaredGrade", gradeValues, false); v != nil {
		grade := model.Grade(*v)
		d.DeclaredGrade = &grade
	}
	d.errs = p.errs
	d.decoded = true
	return nil
}

// Validate — аналог ValidationPipe для StartTestDto.
func (d *StartTestDto) Validate() error {
	v := &common.V{}
	for _, m := range d.errs {
		v.Add(m)
	}
	// Пустое тело (UnmarshalJSON не вызывался) — как DTO без полей в TS.
	if !d.decoded && d.Specialization == nil {
		v.Add(common.MsgIsIn("specialization", specializationValues))
	}
	return v.Err()
}

// RetakeDto — POST /testing/retake.
type RetakeDto struct {
	Specialization *model.Specialization // @IsEnum(Specialization) — обязательный
	// TargetGrade — «целевой грейд»; BUG-REF: T8 — принимается, но игнорируется.
	TargetGrade *model.Grade
	errs        []string
	decoded     bool
}

func (d *RetakeDto) UnmarshalJSON(b []byte) error {
	raw, err := decodeObject(b)
	if err != nil {
		return err
	}
	p := &fieldParser{raw: raw}
	if v := p.enum("specialization", specializationValues, true); v != nil {
		spec := model.Specialization(*v)
		d.Specialization = &spec
	}
	if v := p.enum("targetGrade", gradeValues, false); v != nil {
		grade := model.Grade(*v)
		d.TargetGrade = &grade
	}
	d.errs = p.errs
	d.decoded = true
	return nil
}

// Validate — аналог ValidationPipe для RetakeDto.
func (d *RetakeDto) Validate() error {
	v := &common.V{}
	for _, m := range d.errs {
		v.Add(m)
	}
	if !d.decoded && d.Specialization == nil {
		v.Add(common.MsgIsIn("specialization", specializationValues))
	}
	return v.Err()
}

// SubmitAnswerDto — POST /testing/session/:id/answer (и V2 + behavioral).
type SubmitAnswerDto struct {
	QuestionID        *string        // @IsString — обязательный
	Answer            *string        // @IsString @MaxLength(20)
	SelectedOptionID  *string        // @IsString @MaxLength(40)
	AnswerText        *string        // @IsString @MaxLength(20000)
	Code              *string        // @IsString @MaxLength(20000)
	FixedCode         *string        // @IsString @MaxLength(20000)
	BugsFound         []string       // @IsArray @IsString(each) — nil = null/отсутствует
	DesignDescription *string        // @IsString @MaxLength(20000)
	Diagram           *string        // @IsString @MaxLength(20000)
	TimeSpentSeconds  *int           // @Type(Number) @IsInt @Min(0)
	Behavioral        map[string]any // @IsObject — behavioral-телеметрия V2
	errs              []string
	decoded           bool
}

func (d *SubmitAnswerDto) UnmarshalJSON(b []byte) error {
	raw, err := decodeObject(b)
	if err != nil {
		return err
	}
	p := &fieldParser{raw: raw}
	d.QuestionID = p.str("questionId", true, 0)
	d.Answer = p.str("answer", false, 20)
	d.SelectedOptionID = p.str("selectedOptionId", false, 40)
	d.AnswerText = p.str("answerText", false, 20000)
	d.Code = p.str("code", false, 20000)
	d.FixedCode = p.str("fixedCode", false, 20000)
	d.BugsFound = p.strArray("bugsFound")
	d.DesignDescription = p.str("designDescription", false, 20000)
	d.Diagram = p.str("diagram", false, 20000)
	d.TimeSpentSeconds = p.int("timeSpentSeconds", 0)
	d.Behavioral = p.object("behavioral")
	d.errs = p.errs
	d.decoded = true
	return nil
}

// Validate — аналог ValidationPipe для SubmitAnswerDto.
func (d *SubmitAnswerDto) Validate() error {
	v := &common.V{}
	for _, m := range d.errs {
		v.Add(m)
	}
	if !d.decoded && d.QuestionID == nil {
		v.Add(common.MsgIsString("questionId"))
	}
	return v.Err()
}

// ============ Разбор полей с семантикой class-validator ============

type fieldParser struct {
	raw  map[string]json.RawMessage
	errs []string
}

func decodeObject(b []byte) (map[string]json.RawMessage, error) {
	raw := map[string]json.RawMessage{}
	if len(b) == 0 {
		return raw, nil
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, common.BadRequest(err.Error())
	}
	return raw, nil
}

func (p *fieldParser) add(msg string) { p.errs = append(p.errs, msg) }

// str — @IsString [@MaxLength(n)] (@IsOptional: null/отсутствует — пропуск).
func (p *fieldParser) str(field string, required bool, maxLen int) *string {
	raw, ok := p.raw[field]
	if !ok || string(raw) == "null" {
		if required {
			p.add(common.MsgIsString(field))
		}
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		p.add(common.MsgIsString(field))
		return nil
	}
	if maxLen > 0 && jsLen(s) > maxLen {
		p.add(common.MsgMaxLength(field, maxLen))
	}
	return &s
}

// enum — @IsEnum.
func (p *fieldParser) enum(field string, values []string, required bool) *string {
	raw, ok := p.raw[field]
	if !ok || string(raw) == "null" {
		if required {
			p.add(common.MsgIsIn(field, values))
		}
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		p.add(common.MsgIsIn(field, values))
		return nil
	}
	for _, allowed := range values {
		if s == allowed {
			return &s
		}
	}
	p.add(common.MsgIsIn(field, values))
	return nil
}

// strArray — @IsArray @IsString({each: true}).
func (p *fieldParser) strArray(field string) []string {
	raw, ok := p.raw[field]
	if !ok || string(raw) == "null" {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		p.add(common.MsgIsArray(field))
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		var s string
		if err := json.Unmarshal(item, &s); err != nil {
			p.add(common.MsgIsString(field))
			continue
		}
		out = append(out, s)
	}
	return out
}

// int — @Type(() => Number) @IsInt @Min(min).
func (p *fieldParser) int(field string, min int) *int {
	raw, ok := p.raw[field]
	if !ok || string(raw) == "null" {
		return nil
	}
	// @Type(() => Number): number — как есть, string/bool — через Number(...)
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			if parsed, err := strconv.ParseFloat(s, 64); err == nil {
				value = parsed
			} else {
				value = math.NaN()
			}
		} else {
			var b bool
			if err := json.Unmarshal(raw, &b); err == nil {
				if b {
					value = 1
				} else {
					value = 0
				}
			} else {
				value = math.NaN()
			}
		}
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Trunc(value) != value {
		p.add(common.MsgIsInt(field))
		return nil
	}
	n := int(value)
	if n < min {
		p.add(common.MsgMin(field, float64(min)))
	}
	return &n
}

// object — @IsObject.
func (p *fieldParser) object(field string) map[string]any {
	raw, ok := p.raw[field]
	if !ok || string(raw) == "null" {
		return nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		p.add(field + " must be an object")
		return nil
	}
	return out
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
