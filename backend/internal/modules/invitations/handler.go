package invitations

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// Handler — invitations.controller.ts (роли на уровне маршрутов).
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// Register — маршруты /api/invitations (docs 04 §5).
func (h *Handler) Register(r *common.Router) {
	employer := common.RouteOpts{Roles: []model.UserRole{model.RoleEmployer}}
	candidate := common.RouteOpts{Roles: []model.UserRole{model.RoleCandidate}}
	anyUser := common.RouteOpts{}

	r.POST("/invitations", employer, h.create)
	r.GET("/invitations", anyUser, h.list)
	r.GET("/invitations/:id", anyUser, h.get)
	r.POST("/invitations/:id/accept", candidate, h.accept)
	r.POST("/invitations/:id/reject", candidate, h.reject)
	r.POST("/invitations/:id/withdraw", employer, h.withdraw)
}

// create — POST /invitations.
//
//	@Summary		Пригласить кандидата (EMPLOYER-first)
//	@Tags			Invitations
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateInvitationDto	true	"Приглашение"
//	@Success		201		{object}	map[string]any
//	@Failure		400		{object}	map[string]any
//	@Failure		404		{object}	map[string]any
//	@Failure		409		{object}	map[string]any
//	@Security		bearer
//	@Router			/invitations [post]
func (h *Handler) create(c *gin.Context) error {
	u := common.MustUser(c)
	var dto CreateInvitationDto
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}
	body, err := h.service.Create(c.Request.Context(), u.ID, &dto)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusCreated, body)
	return nil
}

// list — GET /invitations.
//
//	@Summary		Мои приглашения (входящие/исходящие по роли)
//	@Tags			Invitations
//	@Produce		json
//	@Param			status	query		string	false	"Фильтр по статусу"
//	@Success		200		{object}	map[string]any
//	@Failure		404		{object}	map[string]any
//	@Security		bearer
//	@Router			/invitations [get]
func (h *Handler) list(c *gin.Context) error {
	u := common.MustUser(c)
	body, err := h.service.List(c.Request.Context(), u.ID, u.Role, c.Query("status"))
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// get — GET /invitations/:id.
//
//	@Summary		Детали приглашения (контакты — после ACCEPTED)
//	@Tags			Invitations
//	@Produce		json
//	@Param			id	path	string	true	"ID приглашения"
//	@Success		200	{object}	map[string]any
//	@Failure		404	{object}	map[string]any
//	@Security		bearer
//	@Router			/invitations/{id} [get]
func (h *Handler) get(c *gin.Context) error {
	u := common.MustUser(c)
	body, err := h.service.Get(c.Request.Context(), u.ID, u.Role, c.Param("id"))
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// accept — POST /invitations/:id/accept.
//
//	@Summary		Принять приглашение (раскрывает контакты работодателю)
//	@Tags			Invitations
//	@Produce		json
//	@Param			id	path	string	true	"ID приглашения"
//	@Success		200	{object}	map[string]any
//	@Failure		404	{object}	map[string]any
//	@Failure		409	{object}	map[string]any
//	@Security		bearer
//	@Router			/invitations/{id}/accept [post]
func (h *Handler) accept(c *gin.Context) error {
	return h.respond(c, true)
}

// reject — POST /invitations/:id/reject.
//
//	@Summary		Отклонить приглашение
//	@Tags			Invitations
//	@Produce		json
//	@Param			id	path	string	true	"ID приглашения"
//	@Success		200	{object}	map[string]any
//	@Failure		404	{object}	map[string]any
//	@Failure		409	{object}	map[string]any
//	@Security		bearer
//	@Router			/invitations/{id}/reject [post]
func (h *Handler) reject(c *gin.Context) error {
	return h.respond(c, false)
}

func (h *Handler) respond(c *gin.Context, accept bool) error {
	u := common.MustUser(c)
	body, err := h.service.Respond(c.Request.Context(), u.ID, c.Param("id"), accept)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// withdraw — POST /invitations/:id/withdraw.
//
//	@Summary		Отозвать приглашение
//	@Tags			Invitations
//	@Produce		json
//	@Param			id	path	string	true	"ID приглашения"
//	@Success		200	{object}	map[string]any
//	@Failure		403	{object}	map[string]any
//	@Failure		409	{object}	map[string]any
//	@Security		bearer
//	@Router			/invitations/{id}/withdraw [post]
func (h *Handler) withdraw(c *gin.Context) error {
	u := common.MustUser(c)
	body, err := h.service.Withdraw(c.Request.Context(), u.ID, c.Param("id"))
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}
