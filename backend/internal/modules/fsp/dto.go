// Package fsp — интеграция с ФСП (детерминированный mock), docs/07 §1.
package fsp

import (
	"regexp"
	"strings"

	"fsp-backend/internal/common"
	"fsp-backend/internal/model"
)

// ConnectFspDto — тело POST /api/fsp/connect (ConnectFspDto в fsp.controller.ts).
// FspID типизирован как any, чтобы повторить class-validator различение
// отсутствующего/null/не-строкового значения (два сообщения вместо одного).
type ConnectFspDto struct {
	FspID any `json:"fspId"`
}

// fspIDRe — @Matches(/^[A-Za-z0-9_-]{4,40}$/) в ConnectFspDto.
var fspIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{4,40}$`)

// Validate повторяет сообщения class-validator из ConnectFspDto:
// @IsString → "fspId must be a string", @Matches → "Некорректный формат FSP ID".
// Проверка идёт по «сырому» значению (trim выполняется позже в сервисе).
func (d *ConnectFspDto) Validate() error {
	v := &common.V{}
	s, isString := d.FspID.(string)
	if !isString {
		// Нестроковое/отсутствующее значение: оба декоратора падают.
		v.Add(common.MsgIsString("fspId"))
		s = ""
	}
	v.CheckMatches("fspId", s, fspIDRe, "Некорректный формат FSP ID")
	return v.Err()
}

// Value — нормализованный fspId (trim, как в FspService.connect).
func (d *ConnectFspDto) Value() string {
	s, _ := d.FspID.(string)
	return strings.TrimSpace(s)
}

// FspAchievementDto — элемент ответа mock-генерации (FspAchievementDto в fsp.service.ts).
type FspAchievementDto struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Category    string  `json:"category"`
	Year        int     `json:"year"`
	Place       *string `json:"place"`
	Points      int     `json:"points"`
}

// ConnectResult — ответ POST /api/fsp/connect.
type ConnectResult struct {
	FspID             string              `json:"fspId"`
	Connected         bool                `json:"connected"`
	AchievementsFound int                 `json:"achievementsFound"`
	Achievements      []FspAchievementDto `json:"achievements"`
	Note              string              `json:"note"`
}

// DisconnectResult — ответ POST /api/fsp/disconnect.
type DisconnectResult struct {
	FspID        *string `json:"fspId"`
	Disconnected bool    `json:"disconnected"`
}

// AchievementItem — элемент GET /api/fsp/achievements:
// {...a.achievement, verifiedAt: a.verifiedAt} в fsp.service.ts.
type AchievementItem struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	Category    string     `json:"category"`
	Year        *int       `json:"year"`
	Place       *string    `json:"place"`
	Points      int        `json:"points"`
	VerifiedAt  model.Time `json:"verifiedAt"`
}

// AchievementsResult — ответ GET /api/fsp/achievements.
type AchievementsResult struct {
	FspID          *string           `json:"fspId"`
	FspConnectedAt *model.Time       `json:"fspConnectedAt"`
	Achievements   []AchievementItem `json:"achievements"`
	Points         int               `json:"points"`
}

// CatalogResult — ответ GET /api/fsp/catalog.
type CatalogResult struct {
	Total        int                    `json:"total"`
	Achievements []model.FSPAchievement `json:"achievements"`
}
