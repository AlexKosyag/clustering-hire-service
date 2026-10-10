// Package auth — аутентификация и авторизация (docs 03, docs 04 §1):
// register/login/refresh/logout/me + JWT-guard (JwtStrategy+JwtAuthGuard).
package auth

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
// В NestJS тело проходит ValidationPipe({transform, enableImplicitConversion}):
// class-transformer приводит к строке поля с design:type String (числа/bool/объекты
// → String(value)), массивы и null/undefined остаются как есть. Валидирует
// class-validator уже приведённые значения — поэтому, например, password: 123
// проходит @IsString, но не проходит @MinLength/@Matches.

type jsKind int

const (
	jsUndefined jsKind = iota // поле отсутствует в теле (undefined)
	jsNull                    // null
	jsString                  // строка после приведения типов
	jsNonString               // не-строка БЕЗ приведения (design:type Object — например, role)
	jsArray                   // массив — к строке не приводится
)

// jsValue — значение поля DTO ровно в том виде, в котором его валидирует class-validator.
type jsValue struct {
	kind jsKind
	s    string // строковое представление (после String()) для jsString
}

// rawObject разбирает тело запроса в плоский объект.
// Не-объект (массив, примитив) и пустое тело = пустой DTO (все поля undefined) —
// как в NestJS: тело не-объект приводится к «нет полей», валидация падает по-полной.
func rawObject(b []byte) map[string]json.RawMessage {
	m := map[string]json.RawMessage{}
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" || trimmed == "null" {
		return m
	}
	if err := json.Unmarshal(b, &m); err != nil {
		// Не-объект или повреждённое значение: NestJS в этом случае валидирует DTO с undefined-полями.
		return map[string]json.RawMessage{}
	}
	return m
}

// convertValue — аналог class-transformer (enableImplicitConversion) для одного поля.
// implicit=true — design:type String (email/password): число/bool/объект приводятся String().
// implicit=false — design:type Object (role): значение не меняется.
func convertValue(raw json.RawMessage, implicit bool) jsValue {
	if raw == nil {
		return jsValue{kind: jsUndefined}
	}
	if string(raw) == "null" {
		return jsValue{kind: jsNull}
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return jsValue{kind: jsNonString}
	}
	switch t := v.(type) {
	case string:
		return jsValue{kind: jsString, s: t}
	case float64:
		if implicit {
			return jsValue{kind: jsString, s: jsNumberString(t)}
		}
		return jsValue{kind: jsNonString}
	case bool:
		if implicit {
			return jsValue{kind: jsString, s: strconv.FormatBool(t)}
		}
		return jsValue{kind: jsNonString}
	case map[string]any:
		if implicit {
			// JS: String({}) === '[object Object]'
			return jsValue{kind: jsString, s: "[object Object]"}
		}
		return jsValue{kind: jsNonString}
	case []any:
		return jsValue{kind: jsArray}
	default:
		return jsValue{kind: jsNonString}
	}
}

// jsNumberString — String(number) в стиле JS.
func jsNumberString(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	if f == 0 {
		return "0" // включая -0 → "0"
	}
	abs := math.Abs(f)
	if abs >= 1e21 || abs < 1e-6 {
		// Экспоненциальная запись как в JS: 1e+21, 1e-7 (без ведущих нулей в экспоненте).
		s := strconv.FormatFloat(f, 'e', -1, 64) // 1e+21 / 1e-07
		if i := strings.IndexByte(s, 'e'); i >= 0 {
			exp, _ := strconv.Atoi(s[i+1:])
			s = s[:i] + "e" + strconv.Itoa(exp)
		}
		return s
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// isString — class-validator @IsString (после приведения типов).
func (v jsValue) isString() bool { return v.kind == jsString }

// minLen — class-validator @MinLength (validator.isLength учитывает суррогатные пары
// = количество code points).
func (v jsValue) minLen(n int) bool {
	return v.kind == jsString && len([]rune(v.s)) >= n
}

// firstLine — обрезка до первого «перевода строки» (JS `.` не пропускает
// \n, \r, \u2028, \u2029 — lookaheads в @Matches смотрят только первую строку).
func firstLine(s string) string {
	if i := strings.IndexAny(s, "\n\r\u2028\u2029"); i >= 0 {
		return s[:i]
	}
	return s
}

// matchesPassword — @Matches(/^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/):
// в строке (в её первой «строке») есть строчная буква, заглавная буква и цифра.
func (v jsValue) matchesPassword() bool {
	if v.kind != jsString {
		return false // matches() требует typeof value === 'string'
	}
	line := firstLine(v.s)
	return strings.ContainsAny(line, "abcdefghijklmnopqrstuvwxyz") &&
		strings.ContainsAny(line, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") &&
		strings.ContainsAny(line, "0123456789")
}

// isEmail — @IsEmail (проверка только строк; common.IsEmail — санкционированная
// аппроксимация class-validator isEmail).
func (v jsValue) isEmail() bool {
	return v.kind == jsString && common.IsEmail(v.s)
}

// isEnumValue — строгое равенство со значениями enum (@IsEnum/@IsIn без приведения типов).
func (v jsValue) isEnumValue(values ...string) bool {
	if v.kind != jsString {
		return false
	}
	for _, value := range values {
		if v.s == value {
			return true
		}
	}
	return false
}

// ============ RegisterDto (dto/auth.dto.ts) ============

// PasswordRuleMessage — кастомное сообщение @Matches из RegisterDto.
const PasswordRuleMessage = "Пароль должен содержать заглавную букву, строчную букву и цифру"

// RegisterDto — тело POST /auth/register.
// ВНИМАНИЕ: порядок сообщений валидации повторяет class-validator:
// декораторы применяются снизу вверх (@Matches → @MinLength → @IsString),
// свойства — в порядке объявления (email → password → role).
type RegisterDto struct {
	Email    jsValue
	Password jsValue
	Role     jsValue // design:type Object — БЕЗ приведения типов
}

// UnmarshalJSON читает тело (не-объект = пустой DTO, как в NestJS).
func (d *RegisterDto) UnmarshalJSON(b []byte) error {
	m := rawObject(b)
	d.Email = convertValue(m["email"], true)
	d.Password = convertValue(m["password"], true)
	d.Role = convertValue(m["role"], false)
	return nil
}

// Validate — class-validator для RegisterDto (все ошибки склеиваются через ", ").
func (d *RegisterDto) Validate() error {
	v := &common.V{}

	// email: @IsEmail
	if !d.Email.isEmail() {
		v.Add(common.MsgIsEmail("email"))
	}

	// password: @Matches(...) | @MinLength(8) | @IsString (порядок — снизу вверх)
	if !d.Password.matchesPassword() {
		v.Add(PasswordRuleMessage)
	}
	if !d.Password.minLen(8) {
		v.Add(common.MsgMinLength("password", 8))
	}
	if !d.Password.isString() {
		v.Add(common.MsgIsString("password"))
	}

	// role: @IsIn([CANDIDATE, EMPLOYER]) | @IsEnum(UserRole) (порядок — снизу вверх)
	if !d.Role.isEnumValue(string(model.RoleCandidate), string(model.RoleEmployer)) {
		v.Add(common.MsgIsIn("role", []string{string(model.RoleCandidate), string(model.RoleEmployer)}))
	}
	if !d.Role.isEnumValue(string(model.RoleCandidate), string(model.RoleEmployer), string(model.RoleAdmin)) {
		v.Add(common.MsgIsIn("role", []string{string(model.RoleCandidate), string(model.RoleEmployer), string(model.RoleAdmin)}))
	}

	return v.Err()
}

// ============ LoginDto (dto/auth.dto.ts) ============

// LoginDto — тело POST /auth/login.
type LoginDto struct {
	Email    jsValue
	Password jsValue
}

// UnmarshalJSON читает тело (не-объект = пустой DTO, как в NestJS).
func (d *LoginDto) UnmarshalJSON(b []byte) error {
	m := rawObject(b)
	d.Email = convertValue(m["email"], true)
	d.Password = convertValue(m["password"], true)
	return nil
}

// Validate — class-validator для LoginDto.
func (d *LoginDto) Validate() error {
	v := &common.V{}
	if !d.Email.isEmail() {
		v.Add(common.MsgIsEmail("email"))
	}
	if !d.Password.isString() {
		v.Add(common.MsgIsString("password"))
	}
	return v.Err()
}

// ============ Ответы (auth.service.ts) ============

// SafeUser — sanitize(user): без passwordHash/refreshToken/emailVerifyToken.
type SafeUser struct {
	ID              string         `json:"id"`
	Email           string         `json:"email"`
	Role            model.UserRole `json:"role"`
	IsEmailVerified bool           `json:"isEmailVerified"`
	CreatedAt       model.Time     `json:"createdAt"`
}

// AuthResponse — ответ register/login: { ...tokens, user: SafeUser }.
type AuthResponse struct {
	AccessToken  string   `json:"accessToken"`
	RefreshToken string   `json:"refreshToken"`
	User         SafeUser `json:"user"`
}

// TokensResponse — ответ refresh: пара токенов.
type TokensResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// MeResponse — ответ GET /auth/me: пользователь + candidateProfile/employerProfile
// без passwordHash/refreshToken/emailVerifyToken (me() в auth.service.ts).
type MeResponse struct {
	ID               string                  `json:"id"`
	Email            string                  `json:"email"`
	Role             model.UserRole          `json:"role"`
	IsEmailVerified  bool                    `json:"isEmailVerified"`
	CreatedAt        model.Time              `json:"createdAt"`
	UpdatedAt        model.Time              `json:"updatedAt"`
	CandidateProfile *model.CandidateProfile `json:"candidateProfile"`
	EmployerProfile  *model.EmployerProfile  `json:"employerProfile"`
}

// LogoutResponse — ответ POST /auth/logout.
type LogoutResponse struct {
	Message string `json:"message"`
}
