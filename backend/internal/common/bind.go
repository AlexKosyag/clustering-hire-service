package common

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// BindJSON читает JSON-тело запроса (аналог ValidationPipe: неизвестные поля
// отбрасываются — json.Decoder игнорирует их по умолчанию).
func BindJSON(c *gin.Context, out any) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return BadRequest(err.Error())
	}
	if len(body) == 0 {
		// Пустое тело = пустой DTO (валидация полей — в Validate()).
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return BadRequest(err.Error())
	}
	return nil
}

// NoContent — 204.
func NoContent(c *gin.Context) { c.Status(http.StatusNoContent) }
