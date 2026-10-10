package common

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/model"
)

// H — обработчик маршрута с возвратом ошибки (аналог контроллера NestJS:
// исключение → глобальный HttpExceptionFilter).
type H func(c *gin.Context) error

// RouteOpts — метаданные маршрута (аналог @Public()/@Roles()).
// По умолчанию маршрут закрыт (требуется JWT) — «всё закрыто по умолчанию».
type RouteOpts struct {
	Public bool
	Roles  []model.UserRole
}

// AuthUser — request.user в NestJS.
type AuthUser struct {
	ID    string
	Email string
	Role  model.UserRole
}

const userContextKey = "authUser"

// SetUser кладёт аутентифицированного пользователя в контекст (используется guard-ом).
func SetUser(c *gin.Context, u AuthUser) {
	c.Set(userContextKey, u)
}

// CurrentUser возвращает аутентифицированного пользователя.
func CurrentUser(c *gin.Context) (AuthUser, bool) {
	v, ok := c.Get(userContextKey)
	if !ok {
		return AuthUser{}, false
	}
	u, ok := v.(AuthUser)
	return u, ok
}

// MustUser возвращает пользователя или паникует с 401 (как guard — недостижимо после middleware).
func MustUser(c *gin.Context) AuthUser {
	u, ok := CurrentUser(c)
	if !ok {
		panic(ErrUnauthorized())
	}
	return u
}

// Router — реестр маршрутов с guard-логикой JwtAuthGuard/RolesGuard.
type Router struct {
	Engine *gin.Engine
	// Authenticate проверяет Bearer JWT и кладёт AuthUser в контекст.
	// Возвращает ошибку 401 при отсутствии/невалидности токена.
	Authenticate func(c *gin.Context) error
}

// Handle регистрирует маршрут. opts.Public отключает JWT-проверку; opts.Roles
// включает проверку роли (RolesGuard). Без opts.Roles достаточно JWT.
func (r *Router) Handle(method, path string, opts RouteOpts, handlers ...H) {
	chain := make([]gin.HandlerFunc, 0, len(handlers)+2)

	// JwtAuthGuard + RolesGuard (глобальные в NestJS)
	chain = append(chain, func(c *gin.Context) {
		if !opts.Public {
			if err := r.Authenticate(c); err != nil {
				AbortError(c, err)
				return
			}
		}
		if len(opts.Roles) > 0 {
			u, ok := CurrentUser(c)
			if !ok {
				AbortError(c, ErrUnauthorized())
				return
			}
			allowed := false
			for _, role := range opts.Roles {
				if u.Role == role {
					allowed = true
					break
				}
			}
			if !allowed {
				AbortError(c, ErrForbidden())
				return
			}
		}
		c.Next()
	})

	for _, h := range handlers {
		h := h
		chain = append(chain, func(c *gin.Context) {
			if c.IsAborted() {
				return
			}
			if err := h(c); err != nil {
				AbortError(c, err)
			}
		})
	}

	r.Engine.Handle(method, "/api"+path, chain...)
}

// GET/POST/PUT/DELETE/PATCH — синтаксический сахар.
func (r *Router) GET(path string, opts RouteOpts, handlers ...H) {
	r.Handle(http.MethodGet, path, opts, handlers...)
}
func (r *Router) POST(path string, opts RouteOpts, handlers ...H) {
	r.Handle(http.MethodPost, path, opts, handlers...)
}
func (r *Router) PUT(path string, opts RouteOpts, handlers ...H) {
	r.Handle(http.MethodPut, path, opts, handlers...)
}
func (r *Router) DELETE(path string, opts RouteOpts, handlers ...H) {
	r.Handle(http.MethodDelete, path, opts, handlers...)
}

// AbortError нормализует ошибку в формат HttpExceptionFilter и завершает запрос.
func AbortError(c *gin.Context, err error) {
	status, message := NormalizeError(err)
	if status >= 500 {
		log.Printf("[HttpException] %s %s -> %s", c.Request.Method, c.Request.URL.String(), message)
	}
	c.AbortWithStatusJSON(status, gin.H{
		"statusCode": status,
		"timestamp":  time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"path":       c.Request.URL.String(),
		"message":    message,
	})
}

// NormalizeError — аналог catch-блока HttpExceptionFilter:
// HttpException → его статус/сообщение (массив сообщений склеивается через ", "),
// Error → 500 + exception.message, прочее → 500 + 'Internal server error'.
func NormalizeError(err error) (int, string) {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Status, httpErr.Message
	}
	var validErr *ValidationErrors
	if errors.As(err, &validErr) {
		return 400, validErr.Error()
	}
	var generic interface{ Error() string }
	if errors.As(err, &generic) {
		return 500, err.Error()
	}
	return 500, "Internal server error"
}

// JSON отдаёт успешный ответ (аналог res.json).
func JSON(c *gin.Context, status int, body any) {
	c.JSON(status, body)
}

// LoggingMiddleware — аналог LoggingInterceptor: строка "METHOD /url status Xms" (Logger 'HTTP').
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("[HTTP] %s %s %d %dms", c.Request.Method, c.Request.URL.String(), c.Writer.Status(),
			time.Since(start).Milliseconds())
	}
}

// RecoveryMiddleware — перехват паники (нормализованный 500, как HttpExceptionFilter).
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				var err error
				switch v := rec.(type) {
				case error:
					err = v
				case string:
					err = errors.New(v)
				default:
					err = fmt.Errorf("%v", v)
				}
				AbortError(c, err)
			}
		}()
		c.Next()
	}
}

// CORSMiddleware — CORS из main.ts (credentials: true, список origin).
func CORSMiddleware(origins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
