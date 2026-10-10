package candidates

import (
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// Handler — HTTP-слой (candidates.controller.ts, класс @Roles(CANDIDATE)).
type Handler struct {
	service *Service
}

// NewHandler — конструктор для интегратора.
func NewHandler(service *Service) *Handler { return &Handler{service: service} }

// Register — маршруты /api/candidates (docs 04 §2).
func (h *Handler) Register(r *common.Router) {
	opts := common.RouteOpts{Roles: []model.UserRole{model.RoleCandidate}}
	r.GET("/candidates/profile", opts, h.getProfile)
	r.PUT("/candidates/profile", opts, h.updateProfile)
	r.GET("/candidates/category", opts, h.category)
	r.POST("/candidates/profile/resume", opts, h.uploadResume)
	r.POST("/candidates/profile/avatar", opts, h.uploadAvatar)
}

// getProfile — GET /candidates/profile.
//
//	@Summary		Профиль кандидата
//	@Tags			Candidates
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Failure		401	{object}	map[string]any
//	@Failure		404	{object}	map[string]any	"Профиль не найден"
//	@Security		bearer
//	@Router			/candidates/profile [get]
func (h *Handler) getProfile(c *gin.Context) error {
	u := common.MustUser(c)
	body, err := h.service.Profile(c.Request.Context(), u.ID)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// updateProfile — PUT /candidates/profile.
//
//	@Summary		Обновить профиль (переиндексация для matching)
//	@Tags			Candidates
//	@Accept			json
//	@Produce		json
//	@Param			body	body		UpdateCandidateProfileDto	true	"Поля для обновления"
//	@Success		200		{object}	map[string]any
//	@Failure		400		{object}	map[string]any
//	@Failure		404		{object}	map[string]any
//	@Security		bearer
//	@Router			/candidates/profile [put]
func (h *Handler) updateProfile(c *gin.Context) error {
	u := common.MustUser(c)
	var dto UpdateCandidateProfileDto
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

// category — GET /candidates/category.
//
//	@Summary		Текущая категория/грейд и статус тестирования
//	@Tags			Candidates
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Failure		404	{object}	map[string]any
//	@Security		bearer
//	@Router			/candidates/category [get]
func (h *Handler) category(c *gin.Context) error {
	u := common.MustUser(c)
	body, err := h.service.Category(c.Request.Context(), u.ID)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusOK, body)
	return nil
}

// uploadResume — POST /candidates/profile/resume.
//
//	@Summary		Загрузить резюме (PDF)
//	@Tags			Candidates
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			file	formData	file	true	"Резюме (PDF, ≤5 МБ)"
//	@Success		201		{object}	candidates.ResumeUploadResult
//	@Failure		400		{object}	map[string]any
//	@Failure		404		{object}	map[string]any
//	@Failure		503		{object}	map[string]any
//	@Security		bearer
//	@Router			/candidates/profile/resume [post]
func (h *Handler) uploadResume(c *gin.Context) error {
	u := common.MustUser(c)
	file, err := readFileForm(c)
	if err != nil {
		return err
	}
	body, err := h.service.UploadResume(c.Request.Context(), u.ID, file)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusCreated, body)
	return nil
}

// uploadAvatar — POST /candidates/profile/avatar.
//
//	@Summary		Загрузить аватар
//	@Tags			Candidates
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			file	formData	file	true	"Аватар (image/*, ≤2 МБ)"
//	@Success		201		{object}	candidates.AvatarUploadResult
//	@Failure		400		{object}	map[string]any
//	@Failure		404		{object}	map[string]any
//	@Failure		503		{object}	map[string]any
//	@Security		bearer
//	@Router			/candidates/profile/avatar [post]
func (h *Handler) uploadAvatar(c *gin.Context) error {
	u := common.MustUser(c)
	file, err := readFileForm(c)
	if err != nil {
		return err
	}
	body, err := h.service.UploadAvatar(c.Request.Context(), u.ID, file)
	if err != nil {
		return err
	}
	common.JSON(c, http.StatusCreated, body)
	return nil
}

// readFileForm — FileInterceptor('file'): если файла нет (или тело вообще не
// multipart — multer оставит file undefined), сервис вернёт «Файл не передан».
func readFileForm(c *gin.Context) (*multipart.FileHeader, error) {
	file, err := c.FormFile("file")
	if err != nil {
		return nil, nil
	}
	return file, nil
}
