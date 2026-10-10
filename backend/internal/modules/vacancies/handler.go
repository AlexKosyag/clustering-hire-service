package vacancies

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// Handler — HTTP-слой (vacancies.controller.ts: @Public на GET, @Roles(EMPLOYER) на записи).
type Handler struct {
	service *Service
}

// NewHandler — конструктор для интегратора.
func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// Register — маршруты /api/vacancies (docs 04 §4).
func (h *Handler) Register(r *common.Router) {
	employer := common.RouteOpts{Roles: []model.UserRole{model.RoleEmployer}}
	r.GET("/vacancies", common.RouteOpts{Public: true}, h.list)
	r.GET("/vacancies/:id", common.RouteOpts{Public: true}, h.get)
	r.POST("/vacancies", employer, h.create)
	r.PUT("/vacancies/:id", employer, h.update)
	r.DELETE("/vacancies/:id", employer, h.remove)
}

// list — GET /vacancies.
//
//	@Summary		Список вакансий (фильтры: специализация, грейд, поиск)
//	@Tags			Vacancies
//	@Produce		json
//	@Param			specialization	query		string	false	"Специализация"
//	@Param			grade			query		string	false	"Грейд"
//	@Param			employerId		query		string	false	"Работодатель"
//	@Param			search			query		string	false	"Поиск по title/description"
//	@Param			onlyActive		query		string	false	"false — включать неактивные"
//	@Param			limit			query		int		false	"Лимит (1-100, default 20)"
//	@Param			offset			query		int		false	"Смещение (default 0)"
//	@Success		200				{object}	vacancies.ListResult
//	@Router			/vacancies [get]
func (h *Handler) list(c *gin.Context) error {
	query := ParseListQuery(c)
	body, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// get — GET /vacancies/:id.
//
//	@Summary		Вакансия по ID
//	@Tags			Vacancies
//	@Produce		json
//	@Param			id	path		string	true	"ID вакансии"
//	@Success		200	{object}	vacancies.VacancyWithEmployerFull
//	@Failure		404	{object}	map[string]any	"Вакансия не найдена"
//	@Router			/vacancies/{id} [get]
func (h *Handler) get(c *gin.Context) error {
	body, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// create — POST /vacancies.
//
//	@Summary		Создать вакансию
//	@Tags			Vacancies
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateVacancyDto	true	"Вакансия"
//	@Success		201		{object}	vacancies.VacancyWithEmployerShort
//	@Failure		400		{object}	map[string]any	"salaryFrom не может быть больше salaryTo"
//	@Failure		404		{object}	map[string]any	"Профиль работодателя не найден"
//	@Security		bearer
//	@Router			/vacancies [post]
func (h *Handler) create(c *gin.Context) error {
	u := common.MustUser(c)
	var dto CreateVacancyDto
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

// update — PUT /vacancies/:id.
//
//	@Summary		Обновить вакансию
//	@Tags			Vacancies
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string			true	"ID вакансии"
//	@Param			body	body	UpdateVacancyDto	true	"Поля для обновления"
//	@Success		200		{object}	model.Vacancy
//	@Failure		400		{object}	map[string]any
//	@Failure		403		{object}	map[string]any	"Это не ваша вакансия"
//	@Failure		404		{object}	map[string]any	"Профиль работодателя не найден"
//	@Security		bearer
//	@Router			/vacancies/{id} [put]
func (h *Handler) update(c *gin.Context) error {
	u := common.MustUser(c)
	var dto UpdateVacancyDto
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}
	body, err := h.service.Update(c.Request.Context(), u.ID, c.Param("id"), &dto)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// remove — DELETE /vacancies/:id (@HttpCode(200) в TS).
//
//	@Summary		Удалить вакансию
//	@Tags			Vacancies
//	@Produce		json
//	@Param			id	path	string	true	"ID вакансии"
//	@Success		200	{object}	vacancies.DeleteVacancyResult
//	@Failure		403	{object}	map[string]any	"Это не ваша вакансия"
//	@Failure		404	{object}	map[string]any	"Профиль работодателя не найден"
//	@Security		bearer
//	@Router			/vacancies/{id} [delete]
func (h *Handler) remove(c *gin.Context) error {
	u := common.MustUser(c)
	body, err := h.service.Remove(c.Request.Context(), u.ID, c.Param("id"))
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}
