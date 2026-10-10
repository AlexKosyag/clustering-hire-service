package app

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/swag"

	// генерируется командой swag init -g cmd/server/main.go (docs/ — committed).
	_ "fsp-backend/docs"
)

// openAPI3Once — ленивая однократная конвертация Swagger 2.0 (swag) в OpenAPI 3.0.
var (
	openAPI3Once sync.Once
	openAPI3Doc  []byte
	openAPI3Err  error
)

// openAPI3Spec — OpenAPI 3.0 JSON для /api/docs-json. NestJS (@nestjs/swagger v10)
// отдаёт OpenAPI 3.0, а swag генерирует Swagger 2.0 — конвертируем через
// openapi2conv.ToV3 один раз при старте/первом запросе; при ошибке конвертации
// отдаём исходную спецификацию как есть.
func openAPI3Spec() ([]byte, error) {
	openAPI3Once.Do(func() {
		raw, err := swag.ReadDoc()
		if err != nil || raw == "" {
			openAPI3Err = err
			return
		}
		var v2 openapi2.T
		if err := json.Unmarshal([]byte(raw), &v2); err != nil {
			log.Printf("[Swagger] разбор swagger2: %s", err)
			openAPI3Doc = []byte(raw)
			return
		}
		v3, err := openapi2conv.ToV3(&v2)
		if err != nil {
			log.Printf("[Swagger] конвертация в OpenAPI 3.0: %s", err)
			openAPI3Doc = []byte(raw)
			return
		}
		if openAPI3Doc, err = json.Marshal(v3); err != nil {
			log.Printf("[Swagger] marshaling OpenAPI 3.0: %s", err)
			openAPI3Doc = []byte(raw)
		}
	})
	return openAPI3Doc, openAPI3Err
}

// registerSwagger — публикация спецификации (аналог SwaggerModule.setup('api/docs', app, document)):
//   - GET /api/docs      — Swagger UI (index.html; spec подтягивается с /api/docs-json);
//   - GET /api/docs-json — OpenAPI 3.0 JSON (swag → openapi2conv).
func registerSwagger(engine *gin.Engine) {
	engine.GET("/api/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/api/docs-json")))
	engine.GET("/api/docs-json", func(c *gin.Context) {
		doc, err := openAPI3Spec()
		if err != nil || len(doc) == 0 {
			log.Printf("[Swagger] спецификация не сгенерирована: %v", err)
			c.JSON(500, gin.H{"message": "спецификация не сгенерирована (swag init)"})
			return
		}
		c.Data(200, "application/json; charset=utf-8", doc)
	})
}
