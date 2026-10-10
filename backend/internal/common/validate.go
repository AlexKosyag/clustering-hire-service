package common

import (
	"fmt"
	"regexp"
	"strings"
)

// V — накопитель ошибок валидации с сообщениями в стиле class-validator
// (глобальный ValidationPipe: whitelist + transform + enableImplicitConversion).
// Итоговое сообщение — склейка через ", ".
type V struct {
	errs []string
}

func (v *V) Err() error {
	if len(v.errs) == 0 {
		return nil
	}
	return &ValidationErrors{Messages: v.errs}
}

func (v *V) Add(msg string) { v.errs = append(v.errs, msg) }

// ============ Канонические сообщения class-validator ============

func MsgIsString(field string) string   { return field + " must be a string" }
func MsgIsEmail(field string) string    { return field + " must be an email" }
func MsgIsInt(field string) string      { return field + " must be an integer number" }
func MsgIsNumber(field string) string   { return field + " must be a number" }
func MsgIsBoolean(field string) string  { return field + " must be a boolean" }
func MsgIsArray(field string) string    { return field + " must be an array" }
func MsgIsNotEmpty(field string) string { return field + " should not be empty" }
func MsgMinLength(field string, n int) string {
	return fmt.Sprintf("%s must be longer than or equal to %d characters", field, n)
}
func MsgMaxLength(field string, n int) string {
	return fmt.Sprintf("%s must be shorter than or equal to %d characters", field, n)
}
func MsgMin(field string, n float64) string {
	return fmt.Sprintf("%s must not be less than %s", field, trimNum(n))
}
func MsgMax(field string, n float64) string {
	return fmt.Sprintf("%s must not be greater than %s", field, trimNum(n))
}
func MsgIsIn(field string, values []string) string {
	return fmt.Sprintf("%s must be one of the following values: %s", field, strings.Join(values, ", "))
}
func MsgArrayMaxSize(field string, n int) string {
	return fmt.Sprintf("%s must contain no more than %d elements", field, n)
}

func trimNum(n float64) string {
	s := fmt.Sprintf("%g", n)
	return s
}

// ============ Проверки ============

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// IsEmail — приблизительно как class-validator isEmail.
func IsEmail(s string) bool { return emailRe.MatchString(s) }

// CheckString проверяет строку на min/max длину; пустая опциональная строка пропускается снаружи.
func (v *V) CheckString(field, value string, minLen, maxLen int) {
	if minLen > 0 && len([]rune(value)) < minLen {
		v.Add(MsgMinLength(field, minLen))
	}
	if maxLen > 0 && len([]rune(value)) > maxLen {
		v.Add(MsgMaxLength(field, maxLen))
	}
}

// CheckStringArray проверяет массив строк (каждый элемент ≤ itemMax, всего ≤ maxItems).
func (v *V) CheckStringArray(field string, values []string, itemMax, maxItems int) {
	if maxItems > 0 && len(values) > maxItems {
		v.Add(MsgArrayMaxSize(field, maxItems))
	}
	for _, s := range values {
		if itemMax > 0 && len([]rune(s)) > itemMax {
			v.Add(MsgMaxLength(field, itemMax))
		}
	}
}

// CheckIn — @IsIn.
func (v *V) CheckIn(field, value string, allowed []string) {
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	v.Add(MsgIsIn(field, allowed))
}

// CheckMatches — @Matches с кастомным сообщением.
func (v *V) CheckMatches(field, value string, re *regexp.Regexp, message string) {
	if !re.MatchString(value) {
		v.Add(message)
	}
}

// ============ Вспомогательные для DTO ============

// CoerceInt — неявное преобразование query-параметра (enableImplicitConversion).
// ok=false — параметр задан, но не число.
func CoerceInt(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	n := 0
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil {
		return 0, true
	}
	return n, true
}

// CoerceBool — неявное преобразование bool из query.
func CoerceBool(raw string) (bool, bool) {
	switch strings.ToLower(raw) {
	case "true", "1", "yes":
		return true, true
	case "false", "0", "no":
		return false, true
	}
	return false, raw != ""
}

// SplitStack — трансформация строки "Node.js,PostgreSQL" в массив (EmployerSearchQueryDto).
func SplitStack(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
