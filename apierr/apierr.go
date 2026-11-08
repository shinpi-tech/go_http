// Package apierr описывает доменную ошибку сервиса: HTTP-статус и сообщение,
// которое безопасно отдавать клиенту.
//
// Внутренняя причина сохраняется для логов и никогда не попадает в ответ.
package apierr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error — доменная ошибка с HTTP-статусом.
type Error struct {
	Status  int    // HTTP-статус ответа
	Message string // сообщение для клиента
	cause   error  // внутренняя причина, в ответ не попадает
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.cause)
	}
	return e.Message
}

// Unwrap возвращает внутреннюю причину (для errors.Is/errors.As).
func (e *Error) Unwrap() error { return e.cause }

// Wrap прикрепляет внутреннюю причину и возвращает саму ошибку.
func (e *Error) Wrap(cause error) *Error {
	e.cause = cause
	return e
}

// New создаёт ошибку с произвольным статусом.
func New(status int, message string) *Error {
	return &Error{Status: status, Message: message}
}

func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, message)
}

func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, message)
}

func Forbidden(message string) *Error {
	return New(http.StatusForbidden, message)
}

func NotFound(message string) *Error {
	return New(http.StatusNotFound, message)
}

func Conflict(message string) *Error {
	return New(http.StatusConflict, message)
}

func UnprocessableEntity(message string) *Error {
	return New(http.StatusUnprocessableEntity, message)
}

func TooManyRequests(message string) *Error {
	return New(http.StatusTooManyRequests, message)
}

// Unavailable — 503: зависимость сервиса недоступна.
func Unavailable(message string) *Error {
	return New(http.StatusServiceUnavailable, message)
}

// Internal — 500 с нейтральным сообщением для клиента.
func Internal(cause error) *Error {
	return New(http.StatusInternalServerError, "внутренняя ошибка сервера").Wrap(cause)
}

// Status возвращает HTTP-статус из ошибки или fallback, если это не *Error.
func Status(err error, fallback int) int {
	if apiErr, ok := errors.AsType[*Error](err); ok {
		return apiErr.Status
	}
	return fallback
}

// Message возвращает сообщение из ошибки или fallback, если это не *Error.
func Message(err error, fallback string) string {
	if apiErr, ok := errors.AsType[*Error](err); ok {
		return apiErr.Message
	}
	return fallback
}
