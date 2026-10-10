package notifications

import (
	"math"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
)

// Handler — HTTP-слой (notifications.controller.ts).
type Handler struct {
	service *Service
}

// NewHandler — конструктор для интегратора.
func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// Register — маршрут /api/notifications (docs 04 §10; доступен любому авторизованному).
func (h *Handler) Register(r *common.Router) {
	r.GET("/notifications", common.RouteOpts{}, h.list)
}

// list — GET /api/notifications.
//
// @Summary Уведомления пользователя (приглашения, отклики, тесты)
// @Tags Notifications
// @Security BearerAuth
// @Param limit query int false "Лимит (1-100, default 20)"
// @Success 200 {object} ListResult
// @Failure 404 {object} map[string]any "Профиль не найден"
// @Router /api/notifications [get]
func (h *Handler) list(c *gin.Context) error {
	u := common.MustUser(c)

	value := listLimit(c.Query("limit"))
	res, err := h.service.List(c.Request.Context(), u.ID, u.Role, value)
	if err != nil {
		return err
	}
	common.JSON(c, 200, res)
	return nil
}

// listLimit — Math.min(Math.max(Number(limit) || 20, 1), 100) из контроллера.
func listLimit(raw string) int {
	value := 20.0
	if s := strings.TrimSpace(raw); s != "" {
		if f, err := strconv.ParseFloat(s, 64); err == nil && f != 0 && !math.IsNaN(f) {
			value = f
		}
	}
	value = math.Max(value, 1)
	value = math.Min(value, 100)
	// Array.prototype.slice отбрасывает дробную часть.
	return int(value)
}
