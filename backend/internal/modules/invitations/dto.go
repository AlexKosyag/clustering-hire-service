// Package invitations — приглашения EMPLOYER-first: создание, список, детали,
// принятие/отклонение/отзыв (docs 04 §5, docs 02 §6).
package invitations

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"fsp-backend/internal/common"
)

// CreateInvitationDto — POST /api/invitations.
// Разбор ведётся из «сырой» карты: присутствие ключа, null и типы должны
// воспроизводить ValidationPipe (whitelist + transform + implicitConversion).
type CreateInvitationDto struct {
	raw map[string]json.RawMessage
}

func (d *CreateInvitationDto) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	d.raw = raw
	return nil
}

// ============ помощники разбора ============

// jsonString — class-transformer: строка как есть, примитивы → строка, иначе nil.
func jsonString(r json.RawMessage) (string, bool) {
	if len(r) == 0 || string(r) == "null" {
		return "", false
	}
	var s string
	if err := json.Unmarshal(r, &s); err == nil {
		return s, true
	}
	var f float64
	if err := json.Unmarshal(r, &f); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64), true
	}
	var b bool
	if err := json.Unmarshal(r, &b); err == nil {
		return strconv.FormatBool(b), true
	}
	return "", false
}

// jsonInt — @Type(Number): число как есть, строка-число → число; NaN/дробь/null/иное → nil.
func jsonInt(r json.RawMessage) (int, bool) {
	if len(r) == 0 || string(r) == "null" {
		return 0, false
	}
	var f float64
	if err := json.Unmarshal(r, &f); err == nil {
		if math.Trunc(f) != f {
			return 0, false // @IsInt не пройдёт
		}
		return int(f), true
	}
	var s string
	if err := json.Unmarshal(r, &s); err == nil {
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil || math.Trunc(f) != f {
			return 0, false
		}
		return int(f), true
	}
	return 0, false
}

// checkSalary — @Type(Number) @IsInt @Min(0) @Max(10_000_000).
func checkSalary(v *common.V, field string, r json.RawMessage) {
	n, ok := jsonInt(r)
	if !ok {
		v.Add(common.MsgIsInt(field))
		return
	}
	if n < 0 {
		v.Add(common.MsgMin(field, 0))
	}
	if n > 10_000_000 {
		v.Add(common.MsgMax(field, 10000000))
	}
}

// ============ Validate ============

// Validate — ValidationPipe для CreateInvitationDto (порядок полей как в DTO).
func (d *CreateInvitationDto) Validate() error {
	v := &common.V{}

	// candidateId: @IsString (отсутствие и null → одна и та же ошибка)
	if r, ok := d.raw["candidateId"]; !ok || string(r) == "null" {
		v.Add(common.MsgIsString("candidateId"))
	} else if _, isStr := jsonString(r); !isStr {
		v.Add(common.MsgIsString("candidateId"))
	}

	// vacancyId: @IsOptional @IsString
	if r, ok := d.raw["vacancyId"]; ok && string(r) != "null" {
		if _, isStr := jsonString(r); !isStr {
			v.Add(common.MsgIsString("vacancyId"))
		}
	}

	// message: @IsOptional @IsString @MaxLength(4000)
	if r, ok := d.raw["message"]; ok && string(r) != "null" {
		if s, isStr := jsonString(r); !isStr {
			v.Add(common.MsgIsString("message"))
		} else if len([]rune(s)) > 4000 {
			v.Add(common.MsgMaxLength("message", 4000))
		}
	}

	// salaryFrom: @IsInt @Min(0) @Max(10_000_000)
	checkSalary(v, "salaryFrom", d.raw["salaryFrom"])
	// salaryTo: @IsInt @Min(0) @Max(10_000_000)
	checkSalary(v, "salaryTo", d.raw["salaryTo"])

	return v.Err()
}

// ============ Геттеры ============

// CandidateID — обязательный ID кандидата.
func (d *CreateInvitationDto) CandidateID() string {
	if r, ok := d.raw["candidateId"]; ok {
		if s, isStr := jsonString(r); isStr {
			return s
		}
	}
	return ""
}

// VacancyID — опциональная вакансия.
func (d *CreateInvitationDto) VacancyID() *string {
	if r, ok := d.raw["vacancyId"]; ok && string(r) != "null" {
		if s, isStr := jsonString(r); isStr && s != "" {
			return &s
		}
	}
	return nil
}

// Message — опциональное сообщение.
func (d *CreateInvitationDto) Message() *string {
	if r, ok := d.raw["message"]; ok && string(r) != "null" {
		if s, isStr := jsonString(r); isStr {
			return &s
		}
	}
	return nil
}

// Salary — salaryFrom/salaryTo (0 при ошибке разбора — валидация уже отсеяла).
func (d *CreateInvitationDto) Salary() (int, int) {
	from, _ := jsonInt(d.raw["salaryFrom"])
	to, _ := jsonInt(d.raw["salaryTo"])
	return from, to
}
