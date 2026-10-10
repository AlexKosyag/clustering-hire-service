package matching

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// Handler — matching.controller.ts (роли на уровне маршрутов).
type Handler struct {
	service *Service
}

// NewHandler — конструктор контроллера MatchingController.
func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// Register — маршруты /matching (V1 legacy + V2), Roles(EMPLOYER).
func (h *Handler) Register(r *common.Router) {
	employer := common.RouteOpts{Roles: []model.UserRole{model.RoleEmployer}}

	r.POST("/matching/search", employer, h.search)
	r.POST("/matching/semantic", employer, h.semantic)
	r.GET("/matching/recommendations", employer, h.recommendations)
	r.POST("/matching/v2/search", employer, h.searchV2)
	r.POST("/matching/v2/semantic", employer, h.semanticV2)
	r.POST("/matching/v2/need", employer, h.createNeed)
}

// search — POST /matching/search.
//
//	@Summary		Подбор кандидатов (V1 — legacy)
//	@Tags			Matching
//	@Accept			json
//	@Produce		json
//	@Param			body	body	SearchCandidatesDTO	true	"Фильтры подбора"
//	@Success		201		{object}	map[string]any
//	@Failure		400		{object}	map[string]any
//	@Security		bearer
//	@Router			/matching/search [post]
func (h *Handler) search(c *gin.Context) error {
	var dto SearchCandidatesDTO
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}
	body, err := h.service.Search(c.Request.Context(), dto)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusCreated, body)
	return nil
}

// semantic — POST /matching/semantic.
//
//	@Summary		Семантический поиск (V1 — legacy)
//	@Tags			Matching
//	@Accept			json
//	@Produce		json
//	@Param			body	body	SemanticQueryDTO	true	"Запрос"
//	@Success		201		{object}	map[string]any
//	@Failure		400		{object}	map[string]any
//	@Security		bearer
//	@Router			/matching/semantic [post]
func (h *Handler) semantic(c *gin.Context) error {
	var dto SemanticQueryDTO
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}
	body, err := h.service.SemanticSearch(c.Request.Context(), dto)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusCreated, body)
	return nil
}

// recommendations — GET /matching/recommendations.
//
//	@Summary		Рекомендации кандидатов (V1 — legacy)
//	@Tags			Matching
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Security		bearer
//	@Router			/matching/recommendations [get]
func (h *Handler) recommendations(c *gin.Context) error {
	u := common.MustUser(c)
	body, err := h.service.Recommendations(c.Request.Context(), u.ID)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// searchV2 — POST /matching/v2/search.
//
//	@Summary		Подбор кандидатов V2 (ML query understanding + expanded scoring + re-ranking)
//	@Description	Использует ML для понимания запроса, расширенный scoring (7 features), опциональный cross-encoder re-ranking, детальную объяснимость.
//	@Tags			Matching
//	@Accept			json
//	@Produce		json
//	@Param			body	body	SearchCandidatesDTO	true	"Фильтры подбора"
//	@Success		201		{object}	map[string]any
//	@Failure		400		{object}	map[string]any
//	@Security		bearer
//	@Router			/matching/v2/search [post]
func (h *Handler) searchV2(c *gin.Context) error {
	var dto SearchCandidatesDTO
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}
	body, err := h.service.SearchV2(c.Request.Context(), dto)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusCreated, body)
	return nil
}

// semanticV2 — POST /matching/v2/semantic.
//
//	@Summary		Семантический поиск V2 (ML embeddings + query understanding)
//	@Tags			Matching
//	@Accept			json
//	@Produce		json
//	@Param			body	body	SemanticQueryDTO	true	"Запрос"
//	@Success		201		{object}	map[string]any
//	@Failure		400		{object}	map[string]any
//	@Security		bearer
//	@Router			/matching/v2/semantic [post]
func (h *Handler) semanticV2(c *gin.Context) error {
	var dto SemanticQueryDTO
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}
	body, err := h.service.SemanticSearchV2(c.Request.Context(), dto)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusCreated, body)
	return nil
}

// createNeed — POST /matching/v2/need.
//
//	@Summary		Создать потребность V2 (query understanding + подборка)
//	@Description	Анализирует описание потребности через ML и формирует подборку.
//	@Tags			Matching
//	@Accept			json
//	@Produce		json
//	@Param			body	body	EmployerNeedDTO	true	"Потребность"
//	@Success		201		{object}	map[string]any
//	@Failure		400		{object}	map[string]any
//	@Security		bearer
//	@Router			/matching/v2/need [post]
func (h *Handler) createNeed(c *gin.Context) error {
	u := common.MustUser(c)
	var dto EmployerNeedDTO
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}
	body, err := h.service.CreateNeed(c.Request.Context(), u.ID, dto)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusCreated, body)
	return nil
}
