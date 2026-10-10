package employers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// Handler — employers.controller.ts (класс @Roles(EMPLOYER)).
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// Register — маршруты /api/employers/... (docs 04 §3).
func (h *Handler) Register(r *common.Router) {
	opts := common.RouteOpts{Roles: []model.UserRole{model.RoleEmployer}}
	r.GET("/employers/profile", opts, h.profile)
	r.PUT("/employers/profile", opts, h.updateProfile)
	r.POST("/employers/profile/logo", opts, h.uploadLogo)
	r.POST("/employers/need", opts, h.createNeed)
	r.GET("/employers/candidates/search", opts, h.searchCandidates)
	r.GET("/employers/candidates/:id", opts, h.candidateDetail)
	r.GET("/employers/dashboard", opts, h.dashboard)
}

// profile — GET /employers/profile.
//
//	@Summary		Профиль компании
//	@Tags			Employers
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Failure		401	{object}	map[string]any
//	@Failure		404	{object}	map[string]any
//	@Security		bearer
//	@Router			/employers/profile [get]
func (h *Handler) profile(c *gin.Context) error {
	u := common.MustUser(c)
	body, err := h.service.Profile(c.Request.Context(), u.ID)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// updateProfile — PUT /employers/profile.
//
//	@Summary		Обновить профиль компании
//	@Tags			Employers
//	@Accept			json
//	@Produce		json
//	@Param			body	body		UpdateEmployerProfileDto	true	"Поля для обновления"
//	@Success		200		{object}	map[string]any
//	@Failure		400		{object}	map[string]any
//	@Failure		404		{object}	map[string]any
//	@Security		bearer
//	@Router			/employers/profile [put]
func (h *Handler) updateProfile(c *gin.Context) error {
	u := common.MustUser(c)
	var dto UpdateEmployerProfileDto
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}
	body, err := h.service.UpdateProfile(c.Request.Context(), u.ID, &dto)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// uploadLogo — POST /employers/profile/logo.
//
//	@Summary		Загрузить логотип компании
//	@Tags			Employers
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			file	formData	file	true	"Логотип (image/*, ≤2 МБ)"
//	@Success		201		{object}	map[string]string
//	@Failure		400		{object}	map[string]any
//	@Security		bearer
//	@Router			/employers/profile/logo [post]
func (h *Handler) uploadLogo(c *gin.Context) error {
	u := common.MustUser(c)
	file, err := c.FormFile("file")
	if err != nil {
		if err == http.ErrMissingFile {
			file = nil
		} else {
			return err
		}
	}
	body, err := h.service.UploadLogo(c.Request.Context(), u.ID, file)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusCreated, body)
	return nil
}

// createNeed — POST /employers/need.
//
//	@Summary		Описать потребность и получить подборку кандидатов
//	@Tags			Employers
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateNeedDto	true	"Потребность"
//	@Success		201		{object}	map[string]any
//	@Failure		400		{object}	map[string]any
//	@Security		bearer
//	@Router			/employers/need [post]
func (h *Handler) createNeed(c *gin.Context) error {
	u := common.MustUser(c)
	var dto CreateNeedDto
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}
	body, err := h.service.CreateNeed(c.Request.Context(), u.ID, &dto)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusCreated, body)
	return nil
}

// searchCandidates — GET /employers/candidates/search.
//
//	@Summary		Поиск по базе кандидатов (фильтры + семантика)
//	@Tags			Employers
//	@Produce		json
//	@Param			description		query		string	false	"Описание потребности"
//	@Param			specialization	query		string	false	"Специализация"
//	@Param			grade			query		string	false	"Грейд"
//	@Param			stack			query		string	false	"Стек через запятую"
//	@Param			hasAchievements	query		bool	false	"Только с достижениями ФСП"
//	@Param			limit			query		int		false	"Лимит (1-100)"	default(20)
//	@Param			offset			query		int		false	"Смещение"		default(0)
//	@Success		200				{object}	map[string]any
//	@Failure		400				{object}	map[string]any
//	@Security		bearer
//	@Router			/employers/candidates/search [get]
func (h *Handler) searchCandidates(c *gin.Context) error {
	u := common.MustUser(c)
	query, err := ParseEmployerSearchQuery(c)
	if err != nil {
		return err
	}
	body, err := h.service.SearchCandidates(c.Request.Context(), u.ID, query)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// candidateDetail — GET /employers/candidates/:id.
//
//	@Summary		Профиль кандидата (контакты скрыты до ACCEPTED)
//	@Tags			Employers
//	@Produce		json
//	@Param			id	path	string	true	"ID кандидата"
//	@Success		200	{object}	map[string]any
//	@Failure		404	{object}	map[string]any
//	@Security		bearer
//	@Router			/employers/candidates/{id} [get]
func (h *Handler) candidateDetail(c *gin.Context) error {
	u := common.MustUser(c)
	body, err := h.service.CandidateDetail(c.Request.Context(), u.ID, c.Param("id"))
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// dashboard — GET /employers/dashboard.
//
//	@Summary		Дашборд работодателя
//	@Tags			Employers
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Security		bearer
//	@Router			/employers/dashboard [get]
func (h *Handler) dashboard(c *gin.Context) error {
	u := common.MustUser(c)
	body, err := h.service.Dashboard(c.Request.Context(), u.ID)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}
