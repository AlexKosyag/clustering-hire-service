// Package common — сквозные помощники: формат ошибок (HttpExceptionFilter),
// роутер с JWT/Roles-guard, логирование, cookie, валидация.
package common

import (
	"fmt"
	"strings"
)

// HTTPError — ошибка с HTTP-статусом; нормализуется в
// {statusCode, timestamp, path, message} (см. docs 03 §9).
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string { return e.Message }

func NewError(status int, message string) *HTTPError {
	return &HTTPError{Status: status, Message: message}
}

// ValidationErrors — массив сообщений валидации class-validator;
// при нормализации склеивается через ", ".
type ValidationErrors struct {
	Messages []string
}

func (e *ValidationErrors) Error() string { return strings.Join(e.Messages, ", ") }

func (e *ValidationErrors) Add(msg string) { e.Messages = append(e.Messages, msg) }

func (e *ValidationErrors) Addf(format string, args ...any) {
	e.Messages = append(e.Messages, fmt.Sprintf(format, args...))
}

// Has — есть ли ошибки.
func (e *ValidationErrors) Has() bool { return len(e.Messages) > 0 }

// Err возвращает *ValidationErrors или nil.
func (e *ValidationErrors) Err() error {
	if e.Has() {
		return e
	}
	return nil
}

// ============ Стандартные сообщения (docs 01 §6, 03 §9) ============

const (
	MsgUnauthorized   = "Требуется авторизация"
	MsgForbidden      = "Недостаточно прав для этого действия"
	MsgUserNotFound   = "Пользователь не найден"
	MsgStorageDown    = "Файловое хранилище недоступно"
	MsgBadCredentials = "Неверный email или пароль"
	MsgBadRefresh     = "Refresh token недействителен"
	MsgEmailTaken     = "Пользователь с таким email уже существует"
)

func BadRequest(msg string) *HTTPError         { return NewError(400, msg) }
func Unauthorized(msg string) *HTTPError       { return NewError(401, msg) }
func Forbidden(msg string) *HTTPError          { return NewError(403, msg) }
func NotFound(msg string) *HTTPError           { return NewError(404, msg) }
func Conflict(msg string) *HTTPError           { return NewError(409, msg) }
func Unprocessable(msg string) *HTTPError      { return NewError(422, msg) }
func StorageUnavailable(msg string) *HTTPError { return NewError(503, msg) }

func ErrUnauthorized() *HTTPError { return NewError(401, MsgUnauthorized) }
func ErrForbidden() *HTTPError    { return NewError(403, MsgForbidden) }
