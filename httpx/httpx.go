// Package httpx — единый HTTP-слой ответов сервиса: формат ошибок, списки с
// пагинацией, создание, пустой успех, разбор и валидация тела запроса.
//
// Пакет зависит только от fiber и не навязывает сервису конкретный валидатор:
// подойдёт любой тип с методом Validate(any) error (например, *go_validate.Valid).
package httpx

import (
	"errors"
	"log"
	"math"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/shinpi-tech/go_http/apierr"
)

const (
	// DefaultPage — страница по умолчанию.
	DefaultPage = 1
	// DefaultLimit — размер страницы по умолчанию.
	DefaultLimit = 48
	// MaxLimit — жёсткий максимум страницы.
	MaxLimit = 100

	internalMessage = "внутренняя ошибка сервера"
	badBodyMessage  = "некорректное тело запроса"
	badQueryMessage = "некорректные параметры запроса"
)

// ErrorBody — единый формат ошибки API: {"message": "..."}.
type ErrorBody struct {
	Message string `json:"message"`
}

// Pagination — блок пагинации в ответе списка.
type Pagination struct {
	Total int `json:"total"`
	Limit int `json:"limit"`
	Page  int `json:"page"`
	Pages int `json:"pages"`
}

// ListBody — единый формат ответа списка.
type ListBody struct {
	Items      any        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// ItemsBody — список без пагинации.
type ItemsBody struct {
	Items any `json:"items"`
}

// IDBody — единый ответ на создание: {"id": "..."}.
type IDBody struct {
	ID string `json:"id"`
}

// Validator — минимальный интерфейс валидатора (удовлетворяет *go_validate.Valid).
type Validator interface {
	Validate(any) error
}

// Fail приводит любую ошибку к единому формату ответа.
// Доменные (*apierr.Error) отдаются как есть; остальные — 500 с нейтральным
// текстом, внутренняя причина уходит в лог.
func Fail(c fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	if apiErr, ok := errors.AsType[*apierr.Error](err); ok {
		return c.Status(apiErr.Status).JSON(ErrorBody{Message: apiErr.Message})
	}

	log.Printf("httpx: %s %s: %v", c.Method(), c.Path(), err)
	return c.Status(fiber.StatusInternalServerError).JSON(ErrorBody{Message: internalMessage})
}

// FailStatus пишет ошибку с явно заданными статусом и сообщением.
func FailStatus(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(ErrorBody{Message: message})
}

// FailCause — ошибка с явным статусом и сообщением; внутренняя причина пишется в
// лог и клиенту не отдаётся. Заменяет ручные `{"message", "error"}` в обработчиках.
func FailCause(c fiber.Ctx, status int, message string, cause any) error {
	if cause != nil {
		log.Printf("httpx: %s %s: %d %s: %v", c.Method(), c.Path(), status, message, cause)
	}
	return c.Status(status).JSON(ErrorBody{Message: message})
}

// OK отвечает 200 без тела (обновление, удаление).
func OK(c fiber.Ctx) error {
	return c.Send(nil)
}

// ID отвечает 200 и идентификатором созданной записи.
func ID(c fiber.Ctx, id string) error {
	return c.Status(fiber.StatusOK).JSON(IDBody{ID: id})
}

// Accepted отвечает 202 и идентификатором принятой в работу записи.
func Accepted(c fiber.Ctx, id string) error {
	return c.Status(fiber.StatusAccepted).JSON(IDBody{ID: id})
}

// Created отвечает 201 и идентификатором созданной записи.
func Created(c fiber.Ctx, id string) error {
	return c.Status(fiber.StatusCreated).JSON(IDBody{ID: id})
}

// Items отвечает списком без пагинации: {items}.
// Для постраничных списков — List.
func Items(c fiber.Ctx, items any) error {
	return c.Status(fiber.StatusOK).JSON(ItemsBody{Items: items})
}

// List отвечает единым списком: {items, pagination}. Значения page и limit
// нормализуются, pages считается по total.
func List(c fiber.Ctx, items any, total, page, limit int) error {
	page, limit = normalize(page, limit)

	pages := 0
	if limit > 0 {
		pages = int(math.Ceil(float64(total) / float64(limit)))
	}

	return c.Status(fiber.StatusOK).JSON(ListBody{
		Items: items,
		Pagination: Pagination{
			Total: total,
			Limit: limit,
			Page:  page,
			Pages: pages,
		},
	})
}

// Page читает page и limit из query: 1 / 48 по умолчанию, максимум 100.
func Page(c fiber.Ctx) (page, limit int) {
	return normalize(parseInt(c.Query("page")), parseInt(c.Query("limit")))
}

// ParseBody декодирует и валидирует тело запроса. Ошибка декодирования или
// валидации — 400 (возвращается доменная ошибка, отдать её через Fail).
func ParseBody(c fiber.Ctx, validator Validator, dst any) error {
	if err := c.Bind().Body(dst); err != nil {
		return apierr.BadRequest(badBodyMessage)
	}
	if validator == nil {
		return nil
	}
	if err := validator.Validate(dst); err != nil {
		return apierr.BadRequest(err.Error())
	}
	return nil
}

// QueryInt разбирает целочисленный query-параметр. ok=false, если параметра нет
// или он не число — вызывающий сам решает, что это значит.
func QueryInt(c fiber.Ctx, key string) (int, bool) {
	raw := c.Query(key)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

// BadQuery — 400 для некорректных query-параметров.
func BadQuery() error {
	return apierr.BadRequest(badQueryMessage)
}

func parseInt(raw string) int {
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}

func normalize(page, limit int) (int, int) {
	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	return page, limit
}
