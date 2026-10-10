package gamification

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// Handler — HTTP-слой (gamification.controller.ts).
type Handler struct {
	service *Service
}

// NewHandler — конструктор для интегратора.
func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// Register — маршруты /api/gamification (docs 04 §9).
func (h *Handler) Register(r *common.Router) {
	r.GET("/gamification/profile", common.RouteOpts{Roles: []model.UserRole{model.RoleCandidate}}, h.getProfile)
	r.GET("/gamification/badges", common.RouteOpts{Roles: []model.UserRole{model.RoleCandidate}}, h.getBadges)
	r.GET("/gamification/leaderboard", common.RouteOpts{}, h.getLeaderboard)
	r.GET("/gamification/xp-history", common.RouteOpts{Roles: []model.UserRole{model.RoleCandidate}}, h.getXpHistory)
}

// getProfile — GET /api/gamification/profile.
//
// @Summary Профиль геймификации (XP, уровень, стрик)
// @Tags Gamification
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]any "Candidate profile not found for user <id>"
// @Router /api/gamification/profile [get]
func (h *Handler) getProfile(c *gin.Context) error {
	u := common.MustUser(c)

	res, err := h.service.GetProfileForUser(c.Request.Context(), u.ID)
	if err != nil {
		return err
	}
	common.JSON(c, 200, res)
	return nil
}

// getBadges — GET /api/gamification/badges.
//
// @Summary Все бейджи с earned статусом
// @Tags Gamification
// @Security BearerAuth
// @Success 200 {array} BadgeView
// @Router /api/gamification/badges [get]
func (h *Handler) getBadges(c *gin.Context) error {
	u := common.MustUser(c)

	res, err := h.service.GetBadges(c.Request.Context(), u.ID)
	if err != nil {
		return err
	}
	common.JSON(c, 200, res)
	return nil
}

// getLeaderboard — GET /api/gamification/leaderboard (доступен любому авторизованному).
//
// @Summary Рейтинг кандидатов
// @Tags Gamification
// @Security BearerAuth
// @Param type query string false "global | specialization"
// @Param specialization query string false "Специализация (при type=specialization)"
// @Param limit query int false "Лимит (default 20)"
// @Success 200 {array} LeaderboardEntry
// @Router /api/gamification/leaderboard [get]
func (h *Handler) getLeaderboard(c *gin.Context) error {
	typ := c.Query("type")
	if typ == "" {
		typ = "global"
	}
	limit, err := queryLimit(c.Query("limit"), 20) // parseInt(limit || '20', 10)
	if err != nil {
		return err
	}

	res, err := h.service.GetLeaderboard(c.Request.Context(), typ, c.Query("specialization"), limit)
	if err != nil {
		return err
	}
	common.JSON(c, 200, res)
	return nil
}

// getXpHistory — GET /api/gamification/xp-history.
//
// @Summary История начисления XP
// @Tags Gamification
// @Security BearerAuth
// @Param limit query int false "Лимит (default 50)"
// @Success 200 {array} XpEventView
// @Router /api/gamification/xp-history [get]
func (h *Handler) getXpHistory(c *gin.Context) error {
	u := common.MustUser(c)

	limit, err := queryLimit(c.Query("limit"), 50) // parseInt(limit || '50', 10)
	if err != nil {
		return err
	}

	res, err := h.service.GetXpHistory(c.Request.Context(), u.ID, limit)
	if err != nil {
		return err
	}
	common.JSON(c, 200, res)
	return nil
}

// queryLimit — parseInt(raw || def, 10) из контроллера (пустая строка → значение по умолчанию).
// Не-числовой параметр TS превращает в NaN → Prisma отвечает 500; здесь ошибка сохраняется.
func queryLimit(raw string, def int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	n, ok := parseLeadingInt(raw)
	if !ok {
		return 0, common.NewError(500,
			"Invalid `prisma.gamificationProfile.findMany()` invocation: take: NaN, Expected Int, provided NaN")
	}
	return n, nil
}

// parseLeadingInt — parseInt(s, 10): ведущее целое со знаком, хвост игнорируется.
func parseLeadingInt(s string) (int, bool) {
	sign := 1
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		if s[i] == '-' {
			sign = -1
		}
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == start {
		return 0, false
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil {
		// Переполнение: BigInt-путь в parseInt не реализуем, берём крайнее значение.
		if sign < 0 {
			return -1 << 62, true
		}
		return 1<<62 - 1, true
	}
	return n * sign, true
}
