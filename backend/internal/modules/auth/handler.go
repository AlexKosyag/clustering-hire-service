package auth

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
)

// Handler — HTTP-слой (аналог auth.controller.ts).
type Handler struct {
	service *Service
}

// NewHandler создаёт HTTP-обработчики auth.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register — маршруты /auth (docs 04 §1): register/login/refresh — @Public(),
// logout/me — любой авторизованный.
func (h *Handler) Register(r *common.Router) {
	r.POST("/auth/register", common.RouteOpts{Public: true}, h.register)
	r.POST("/auth/login", common.RouteOpts{Public: true}, h.login)
	r.POST("/auth/refresh", common.RouteOpts{Public: true}, h.refresh)
	r.POST("/auth/logout", common.RouteOpts{}, h.logout)
	r.GET("/auth/me", common.RouteOpts{}, h.me)
}

// @Summary Регистрация (CANDIDATE или EMPLOYER)
// @Tags Auth
// @Produce json
// @Param body body auth.RegisterDto true "{email, password, role: CANDIDATE|EMPLOYER}"
// @Success 201 {object} auth.AuthResponse
// @Failure 400 {object} map[string]any
// @Failure 409 {object} map[string]any
// @Router /auth/register [post]
func (h *Handler) register(c *gin.Context) error {
	var dto RegisterDto
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}
	result, err := h.service.register(c.Request.Context(), &dto)
	if err != nil {
		return err
	}
	h.setRefreshCookie(c, result.RefreshToken)
	// NestJS: POST без @HttpCode → 201.
	common.JSON(c, 201, result)
	return nil
}

// @Summary Вход по email и паролю
// @Tags Auth
// @Produce json
// @Param body body auth.LoginDto true "{email, password}"
// @Success 200 {object} auth.AuthResponse
// @Failure 400 {object} map[string]any
// @Failure 401 {object} map[string]any
// @Router /auth/login [post]
func (h *Handler) login(c *gin.Context) error {
	var dto LoginDto
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}
	result, err := h.service.login(c.Request.Context(), &dto)
	if err != nil {
		return err
	}
	h.setRefreshCookie(c, result.RefreshToken)
	common.JSON(c, 200, result) // @HttpCode(200)
	return nil
}

// @Summary Обновление access token по refresh token из cookie
// @Tags Auth
// @Produce json
// @Param body body object true "{refreshToken?} или cookie refreshToken"
// @Success 200 {object} auth.TokensResponse
// @Failure 401 {object} map[string]any
// @Router /auth/refresh [post]
func (h *Handler) refresh(c *gin.Context) error {
	// Токен из body.refreshToken ИЛИ cookie refreshToken (parseCookies).
	token := h.refreshTokenFromRequest(c)
	result, err := h.service.refresh(c.Request.Context(), token)
	if err != nil {
		return err
	}
	h.setRefreshCookie(c, result.RefreshToken)
	common.JSON(c, 200, result) // @HttpCode(200)
	return nil
}

// @Summary Выход: инвалидация refresh token
// @Tags Auth
// @Produce json
// @Security bearer
// @Success 200 {object} auth.LogoutResponse
// @Failure 401 {object} map[string]any
// @Router /auth/logout [post]
func (h *Handler) logout(c *gin.Context) error {
	u := common.MustUser(c)
	if err := h.service.logout(c.Request.Context(), u.ID); err != nil {
		return err
	}
	h.clearRefreshCookie(c)
	// @HttpCode(200)
	common.JSON(c, 200, LogoutResponse{Message: "Выход выполнен"})
	return nil
}

// @Summary Текущий пользователь с профилем
// @Tags Auth
// @Produce json
// @Security bearer
// @Success 200 {object} auth.MeResponse
// @Failure 401 {object} map[string]any
// @Router /auth/me [get]
func (h *Handler) me(c *gin.Context) error {
	u := common.MustUser(c)
	result, err := h.service.me(c.Request.Context(), u.ID)
	if err != nil {
		return err
	}
	common.JSON(c, 200, result)
	return nil
}

// ============ Cookie (auth.controller.ts: res.cookie/clearCookie) ============

const (
	refreshCookieName   = "refreshToken"
	refreshCookieMaxAge = 7 * 24 * 60 * 60 // REFRESH_MAX_AGE = 604800
)

//	setRefreshCookie — res.cookie(REFRESH_COOKIE, token, {
//	  httpOnly: true, sameSite: 'lax', maxAge: 604800,
//	  secure: process.env.NODE_ENV === 'production' })  (Path=/ по умолчанию).
func (h *Handler) setRefreshCookie(c *gin.Context, token string) {
	attrs := []string{
		refreshCookieName + "=" + encodeURIComponent(token),
		"Path=/",
		"HttpOnly",
		"SameSite=Lax",
		"Max-Age=" + strconv.Itoa(refreshCookieMaxAge),
	}
	if h.service.cfg.NodeEnv == "production" {
		attrs = append(attrs, "Secure")
	}
	c.Writer.Header().Add("Set-Cookie", strings.Join(attrs, "; "))
}

// clearRefreshCookie — res.clearCookie(REFRESH_COOKIE, { path: '/' }):
// пустое значение + Expires=epoch, без HttpOnly (как в express).
func (h *Handler) clearRefreshCookie(c *gin.Context) {
	c.Writer.Header().Add("Set-Cookie", refreshCookieName+"=; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Path=/")
}

// refreshTokenFromRequest — refresh-токен из body.refreshToken || cookie (auth.controller.ts):
// const fromCookie = parseCookies(req.headers.cookie)[REFRESH_COOKIE];
// const refreshToken = body?.refreshToken || fromCookie;
// Нестрогое «или» JS: пустая строка/0/null/undefined → берётся cookie;
// truthy не-строка попадает в jwt.verify и даёт 401.
func (h *Handler) refreshTokenFromRequest(c *gin.Context) string {
	var body []byte
	if c.Request != nil && c.Request.Body != nil {
		body, _ = io.ReadAll(c.Request.Body)
	}
	if raw, ok := rawObject(body)["refreshToken"]; ok {
		if s, ok := jsTruthyString(raw); ok {
			return s
		}
	}
	return common.ParseCookies(c.GetHeader("Cookie"))[refreshCookieName]
}

// jsTruthyString — JS-правило `body?.refreshToken || fromCookie`:
// ok=false — значение falsy (пустая строка/0/false/null/отсутствует) → берётся cookie;
// ok=true — значение truthy: строка возвращается как есть, для не-строки возвращается
// «нестрока-маркер» (""), которая уходит в jwt.verify и даёт 401 (cookie не используется).
func jsTruthyString(raw json.RawMessage) (string, bool) {
	if raw == nil || string(raw) == "null" {
		return "", false
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, t != ""
	case float64:
		return jsNumberString(t), t != 0
	case bool:
		return strconv.FormatBool(t), t
	case map[string]any:
		return "", true // объект → jwt.verify получает не-строку → 401
	case []any:
		return "", true // массив → аналогично
	default:
		return "", false
	}
}

// encodeURIComponent — эквивалент encodeURIComponent (res.cookie кодирует значение).
func encodeURIComponent(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case 'a' <= ch && ch <= 'z', 'A' <= ch && ch <= 'Z', '0' <= ch && ch <= '9',
			ch == '-', ch == '_', ch == '.', ch == '!', ch == '~', ch == '*', ch == '\'', ch == '(', ch == ')':
			b.WriteByte(ch)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[ch>>4])
			b.WriteByte(hex[ch&0x0F])
		}
	}
	return b.String()
}
