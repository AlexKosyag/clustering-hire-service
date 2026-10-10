package fsp

import (
	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// Handler — HTTP-слой (fsp.controller.ts).
type Handler struct {
	service *Service
}

// NewHandler — конструктор для интегратора.
func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// Register — маршруты /api/fsp (docs 04 §8).
func (h *Handler) Register(r *common.Router) {
	r.POST("/fsp/connect", common.RouteOpts{Roles: []model.UserRole{model.RoleCandidate}}, h.connect)
	r.POST("/fsp/disconnect", common.RouteOpts{Roles: []model.UserRole{model.RoleCandidate}}, h.disconnect)
	r.GET("/fsp/achievements", common.RouteOpts{Roles: []model.UserRole{model.RoleCandidate}}, h.achievements)
	r.GET("/fsp/catalog", common.RouteOpts{}, h.catalog)
}

// connect — POST /api/fsp/connect.
//
// @Summary Подключить аккаунт ФСП (mock: возвращает достижения)
// @Tags FSP
// @Security BearerAuth
// @Param body body ConnectFspDto true "Идентификатор в системе ФСП"
// @Success 201 {object} ConnectResult
// @Failure 400 {object} map[string]any "Некорректный формат FSP ID"
// @Failure 404 {object} map[string]any "Профиль кандидата не найден"
// @Failure 409 {object} map[string]any "Этот FSP ID уже привязан к другому аккаунту"
// @Router /api/fsp/connect [post]
func (h *Handler) connect(c *gin.Context) error {
	u := common.MustUser(c)

	var dto ConnectFspDto
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}

	res, err := h.service.Connect(c.Request.Context(), u.ID, dto.Value())
	if err != nil {
		return err
	}
	common.JSON(c, 201, res)
	return nil
}

// disconnect — POST /api/fsp/disconnect (@HttpCode(200) в TS).
//
// @Summary Отключить аккаунт ФСП и удалить достижения
// @Tags FSP
// @Security BearerAuth
// @Success 200 {object} DisconnectResult
// @Failure 404 {object} map[string]any "Профиль кандидата не найден"
// @Router /api/fsp/disconnect [post]
func (h *Handler) disconnect(c *gin.Context) error {
	u := common.MustUser(c)

	res, err := h.service.Disconnect(c.Request.Context(), u.ID)
	if err != nil {
		return err
	}
	common.JSON(c, 200, res)
	return nil
}

// achievements — GET /api/fsp/achievements.
//
// @Summary Достижения ФСП кандидата
// @Tags FSP
// @Security BearerAuth
// @Success 200 {object} AchievementsResult
// @Failure 404 {object} map[string]any "Профиль кандидата не найден"
// @Router /api/fsp/achievements [get]
func (h *Handler) achievements(c *gin.Context) error {
	u := common.MustUser(c)

	res, err := h.service.Achievements(c.Request.Context(), u.ID)
	if err != nil {
		return err
	}
	common.JSON(c, 200, res)
	return nil
}

// catalog — GET /api/fsp/catalog (доступен любому авторизованному).
//
// @Summary Справочник достижений ФСП (mock-каталог из seed)
// @Tags FSP
// @Security BearerAuth
// @Success 200 {object} CatalogResult
// @Router /api/fsp/catalog [get]
func (h *Handler) catalog(c *gin.Context) error {
	res, err := h.service.Catalog(c.Request.Context())
	if err != nil {
		return err
	}
	common.JSON(c, 200, res)
	return nil
}
