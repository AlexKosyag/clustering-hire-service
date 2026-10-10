// Handler — HTTP-слой testing engine: V1 /testing/* (legacy) + V2 /testing/v2/*
// (testing.controller.ts, docs 04 §7); класс Roles(CANDIDATE) на каждый маршрут.
package testing

import (
	"github.com/gin-gonic/gin"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// Handler — обработчики 10 маршрутов тестирования (V1 + V2).
type Handler struct {
	v1 *Service
	v2 *ServiceV2
}

// NewHandler — конструктор для интегратора.
func NewHandler(v1 *Service, v2 *ServiceV2) *Handler { return &Handler{v1: v1, v2: v2} }

// Register — маршруты /api/testing (путь здесь БЕЗ /api: common.Router добавляет префикс).
func (h *Handler) Register(r *common.Router) {
	candidate := common.RouteOpts{Roles: []model.UserRole{model.RoleCandidate}}

	// ─────────────────────────── V1 Endpoints (legacy) ───────────────────────────
	r.POST("/testing/start", candidate, h.start)
	r.POST("/testing/retake", candidate, h.retake)
	r.GET("/testing/active", candidate, h.active)
	r.GET("/testing/session/:id/current", candidate, h.current)
	r.POST("/testing/session/:id/answer", candidate, h.answer)
	r.POST("/testing/session/:id/complete", candidate, h.complete)

	// ─────────────────────────── V2 Endpoints ───────────────────────────
	r.POST("/testing/v2/start", candidate, h.startV2)
	r.GET("/testing/v2/session/:id/current", candidate, h.currentV2)
	r.POST("/testing/v2/session/:id/answer", candidate, h.answerV2)
	r.POST("/testing/v2/session/:id/complete", candidate, h.completeV2)
}

// start — POST /api/testing/start.
//
// @Summary Начать тестирование (V1 — legacy)
// @Tags Testing
// @Security BearerAuth
// @Accept json
// @Param body body StartTestDto true "Специализация и заявленный грейд"
// @Success 201 {object} StartResult
// @Failure 400 {object} map[string]any "Пересдача возможна не раньше <ISO> / Для этой специализации нет теста / Банк вопросов неполный — тест временно недоступен"
// @Failure 404 {object} map[string]any "Профиль кандидата не найден"
// @Failure 409 {object} map[string]any "Тестирование уже начато"
// @Router /api/testing/start [post]
func (h *Handler) start(c *gin.Context) error {
	u := common.MustUser(c)

	var dto StartTestDto
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}

	res, err := h.v1.Start(c.Request.Context(), u.ID, dto)
	if err != nil {
		return err
	}
	common.JSON(c, 201, res)
	return nil
}

// retake — POST /api/testing/retake.
//
// @Summary Пересдача (V1 — legacy; targetGrade игнорируется — BUG-REF: T8)
// @Tags Testing
// @Security BearerAuth
// @Accept json
// @Param body body RetakeDto true "Специализация и целевой грейд"
// @Success 201 {object} StartResult
// @Failure 400 {object} map[string]any "Пересдача возможна не раньше <ISO>"
// @Failure 404 {object} map[string]any "Профиль кандидата не найден"
// @Failure 409 {object} map[string]any "Тестирование уже начато"
// @Router /api/testing/retake [post]
func (h *Handler) retake(c *gin.Context) error {
	u := common.MustUser(c)

	var dto RetakeDto
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}

	res, err := h.v1.Retake(c.Request.Context(), u.ID, dto)
	if err != nil {
		return err
	}
	common.JSON(c, 201, res)
	return nil
}

// active — GET /api/testing/active.
//
// @Summary Активная сессия (V1 — legacy; используется фронтендом для восстановления)
// @Tags Testing
// @Security BearerAuth
// @Success 200 {object} ActiveResult
// @Failure 404 {object} map[string]any "Профиль кандидата не найден"
// @Router /api/testing/active [get]
func (h *Handler) active(c *gin.Context) error {
	u := common.MustUser(c)

	res, err := h.v1.Active(c.Request.Context(), u.ID)
	if err != nil {
		return err
	}
	common.JSON(c, 200, res)
	return nil
}

// current — GET /api/testing/session/:id/current.
//
// @Summary Текущий вопрос (V1 — legacy)
// @Tags Testing
// @Security BearerAuth
// @Param id path string true "ID сессии"
// @Success 200 {object} CurrentResult
// @Failure 403 {object} map[string]any "Это чужая сессия"
// @Failure 404 {object} map[string]any "Сессия не найдена"
// @Router /api/testing/session/{id}/current [get]
func (h *Handler) current(c *gin.Context) error {
	u := common.MustUser(c)

	res, err := h.v1.Current(c.Request.Context(), u.ID, c.Param("id"))
	if err != nil {
		return err
	}
	common.JSON(c, 200, res)
	return nil
}

// answer — POST /api/testing/session/:id/answer.
//
// @Summary Отправить ответ (V1 — legacy)
// @Tags Testing
// @Security BearerAuth
// @Accept json
// @Param id path string true "ID сессии"
// @Param body body SubmitAnswerDto true "Ответ на вопрос"
// @Success 201 {object} AnswerResult
// @Failure 400 {object} map[string]any "Сессия тестирования уже завершена / Время тестирования истекло / Ответ пустой"
// @Failure 403 {object} map[string]any "Это чужая сессия"
// @Failure 404 {object} map[string]any "Сессия не найдена / Вопрос не найден в этом тесте"
// @Failure 409 {object} map[string]any "Ответ на этот вопрос уже записан"
// @Router /api/testing/session/{id}/answer [post]
func (h *Handler) answer(c *gin.Context) error {
	u := common.MustUser(c)

	var dto SubmitAnswerDto
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}

	res, err := h.v1.Answer(c.Request.Context(), u.ID, c.Param("id"), dto)
	if err != nil {
		return err
	}
	common.JSON(c, 201, res)
	return nil
}

// complete — POST /api/testing/session/:id/complete.
//
// @Summary Завершить тест (V1 — legacy; идемпотентен для COMPLETED)
// @Tags Testing
// @Security BearerAuth
// @Param id path string true "ID сессии"
// @Success 201 {object} CompleteResult
// @Failure 400 {object} map[string]any "Сессию нельзя завершить / Осталось неотвеченных вопросов: N"
// @Failure 403 {object} map[string]any "Это чужая сессия"
// @Failure 404 {object} map[string]any "Сессия не найдена"
// @Router /api/testing/session/{id}/complete [post]
func (h *Handler) complete(c *gin.Context) error {
	u := common.MustUser(c)

	res, err := h.v1.Complete(c.Request.Context(), u.ID, c.Param("id"))
	if err != nil {
		return err
	}
	common.JSON(c, 201, res)
	return nil
}

// startV2 — POST /api/testing/v2/start.
//
// @Summary Начать тестирование V2 (мультиформатные задания)
// @Description Поддерживает: multiple_choice, code_execution, llm_open, code_review, system_design. Распределение заданий зависит от грейда.
// @Tags Testing
// @Security BearerAuth
// @Accept json
// @Param body body StartTestDto true "Специализация и заявленный грейд"
// @Success 201 {object} StartResultV2
// @Failure 400 {object} map[string]any "Пересдача возможна не раньше <ISO> / Для этой специализации нет теста / Банк вопросов неполный — тест временно недоступен"
// @Failure 404 {object} map[string]any "Профиль кандидата не найден"
// @Failure 409 {object} map[string]any "Тестирование уже начато"
// @Router /api/testing/v2/start [post]
func (h *Handler) startV2(c *gin.Context) error {
	u := common.MustUser(c)

	var dto StartTestDto
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}

	res, err := h.v2.Start(c.Request.Context(), u.ID, dto)
	if err != nil {
		return err
	}
	common.JSON(c, 201, res)
	return nil
}

// currentV2 — GET /api/testing/v2/session/:id/current.
//
// @Summary Текущая задача V2
// @Tags Testing
// @Security BearerAuth
// @Param id path string true "ID сессии"
// @Success 200 {object} CurrentResultV2
// @Failure 403 {object} map[string]any "Это чужая сессия"
// @Failure 404 {object} map[string]any "Сессия не найдена"
// @Router /api/testing/v2/session/{id}/current [get]
func (h *Handler) currentV2(c *gin.Context) error {
	u := common.MustUser(c)

	res, err := h.v2.CurrentTask(c.Request.Context(), u.ID, c.Param("id"))
	if err != nil {
		return err
	}
	common.JSON(c, 200, res)
	return nil
}

// answerV2 — POST /api/testing/v2/session/:id/answer.
//
// @Summary Отправить ответ V2 (с behavioral данными для anti-cheat)
// @Description Поддерживает behavioral data: copy-paste, tab-switch, keystroke count. Повторная отправка идемпотентна (duplicate: true).
// @Tags Testing
// @Security BearerAuth
// @Accept json
// @Param id path string true "ID сессии"
// @Param body body SubmitAnswerDto true "Ответ на задание (поля формата + behavioral)"
// @Success 201 {object} AnswerResultV2
// @Failure 400 {object} map[string]any "Сессия тестирования уже завершена / Время тестирования истекло"
// @Failure 403 {object} map[string]any "Это чужая сессия"
// @Failure 404 {object} map[string]any "Сессия не найдена / Вопрос не найден в этом тесте"
// @Router /api/testing/v2/session/{id}/answer [post]
func (h *Handler) answerV2(c *gin.Context) error {
	u := common.MustUser(c)

	var dto SubmitAnswerDto
	if err := common.BindJSON(c, &dto); err != nil {
		return err
	}
	if err := dto.Validate(); err != nil {
		return err
	}

	res, err := h.v2.SubmitAnswer(c.Request.Context(), u.ID, c.Param("id"), dto)
	if err != nil {
		return err
	}
	common.JSON(c, 201, res)
	return nil
}

// completeV2 — POST /api/testing/v2/session/:id/complete.
//
// @Summary Завершить тест V2 (task breakdown + behavioral anti-cheat + геймификация)
// @Tags Testing
// @Security BearerAuth
// @Param id path string true "ID сессии"
// @Success 201 {object} TestResultV2
// @Failure 400 {object} map[string]any "Сессию нельзя завершить / Осталось неотвеченных задач: N"
// @Failure 403 {object} map[string]any "Это чужая сессия"
// @Failure 404 {object} map[string]any "Сессия не найдена"
// @Router /api/testing/v2/session/{id}/complete [post]
func (h *Handler) completeV2(c *gin.Context) error {
	u := common.MustUser(c)

	res, err := h.v2.Complete(c.Request.Context(), u.ID, c.Param("id"))
	if err != nil {
		return err
	}
	common.JSON(c, 201, res)
	return nil
}
