package common

import (
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"fsp-backend/internal/model"
)

// JwtPayload — payload access/refresh токенов (strategies/jwt.strategy.ts).
type JwtPayload struct {
	Sub   string         `json:"sub"`
	Email string         `json:"email"`
	Role  model.UserRole `json:"role"`
	Typ   string         `json:"typ,omitempty"`
}

// jwtClaims — payload токенов: RegisteredClaims даёт интерфейс jwt.Claims (exp/iat),
// а кастомные поля лежат ПЛОСКО: иначе RegisteredClaims.Subject (json:"sub") и
// JwtPayload.Sub конфликтуют на одном уровне вложенности, и encoding/json выбрасывает
// оба — sub терялся бы из токена (guard не находит пользователя по "").
type jwtClaims struct {
	jwt.RegisteredClaims
	Sub   string         `json:"sub,omitempty"`
	Email string         `json:"email,omitempty"`
	Role  model.UserRole `json:"role,omitempty"`
	Typ   string         `json:"typ,omitempty"`
}

func sign(secret string, ttl time.Duration, p JwtPayload) (string, error) {
	now := time.Now()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Sub:   p.Sub,
		Email: p.Email,
		Role:  p.Role,
		Typ:   p.Typ,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseToken проверяет подпись и срок жизни токена (ignoreExpiration: false).
func ParseToken(secret, token string) (*JwtPayload, error) {
	var claims jwtClaims
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !parsed.Valid {
		return nil, ErrUnauthorized()
	}
	return &JwtPayload{Sub: claims.Sub, Email: claims.Email, Role: claims.Role, Typ: claims.Typ}, nil
}

// IssueAccessToken — access-токен (15 мин, секрет jwt.secret).
func IssueAccessToken(secret string, ttl time.Duration, u AuthUser) (string, error) {
	return sign(secret, ttl, JwtPayload{Sub: u.ID, Email: u.Email, Role: u.Role})
}

// IssueRefreshToken — refresh-токен (7 дней, секрет jwt.refreshSecret, typ='refresh').
func IssueRefreshToken(secret string, ttl time.Duration, u AuthUser) (string, error) {
	return sign(secret, ttl, JwtPayload{Sub: u.ID, Email: u.Email, Role: u.Role, Typ: "refresh"})
}

// bcryptInput — вход для bcrypt (crypt_blowfish обрабатывает максимум 72 байта).
// bcryptjs (TS) лимита не имеет и хеширует длинные строки целиком; чтобы
// hash/compare оставались симметричными, длинные входы (refresh-JWT ~200 байт)
// усекаются до 72 байт на границе UTF-8 руны — как это делает C-bcrypt.
func bcryptInput(password string) []byte {
	b := []byte(password)
	if len(b) <= 72 {
		return b
	}
	end := 72
	for end > 0 && b[end]&0xC0 == 0x80 { // не режем многобайтовую руну
		end--
	}
	return b[:end]
}

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword(bcryptInput(password), 12)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// HashRefresh — bcrypt-хеш refresh-токена (persistRefreshToken, cost 12).
func HashRefresh(refreshToken string) (string, error) {
	return HashPassword(refreshToken)
}

// CheckHash - bcrypt.compare.
func CheckHash(hash, value string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), bcryptInput(value)) == nil
}

// ============ Cookie (common/utils/cookie.util.ts) ============

// ParseCookies — минимальный парсер cookie (без cookie-parser).
func ParseCookies(header string) map[string]string {
	out := map[string]string{}
	if header == "" {
		return out
	}
	for _, part := range strings.Split(header, ";") {
		idx := strings.Index(part, "=")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(part[:idx])
		value := strings.TrimSpace(part[idx+1:])
		if key != "" {
			out[key] = decodeURIComponent(value)
		}
	}
	return out
}

// decodeURIComponent — раскодирование %XX (аналог decodeURIComponent в cookie.util.ts).
func decodeURIComponent(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi, ok1 := fromHex(s[i+1])
			lo, ok2 := fromHex(s[i+2])
			if ok1 && ok2 {
				b.WriteByte(hi<<4 | lo)
				i += 2
				continue
			}
		}
		if s[i] == '+' {
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func fromHex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
