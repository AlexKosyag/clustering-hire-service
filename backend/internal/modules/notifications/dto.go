// Package notifications — in-app уведомления (read-model без таблицы), docs/07 §3.
package notifications

import (
	"fsp-backend/internal/model"
)

// NotificationItem — элемент ленты (NotificationItem в notifications.service.ts).
// BUG-REF: G7 — статус «прочитано» не хранится: read выводится по правилам.
type NotificationItem struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"` // INVITATION_RECEIVED | INVITATION_STATUS | TEST_RESULT
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	CreatedAt model.Time `json:"createdAt"`
	Read      bool       `json:"read"`
}

// ListResult — ответ GET /api/notifications.
type ListResult struct {
	Items []NotificationItem `json:"items"`
	Total int                `json:"total"`
}
