package apierr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/shinpi-tech/go_http/apierr"
)

func TestError_MessageWithoutCause(t *testing.T) {
	err := apierr.NotFound("бренд не найден")
	if got := err.Error(); got != "бренд не найден" {
		t.Fatalf("got %q", got)
	}
}

func TestError_MessageWithCause(t *testing.T) {
	err := apierr.BadRequest("неверный запрос").Wrap(errors.New("mongo: bad query"))
	if got := err.Error(); got != "неверный запрос: mongo: bad query" {
		t.Fatalf("got %q", got)
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := errors.New("duplicate key")
	err := apierr.Conflict("конфликт").Wrap(cause)

	if !errors.Is(err, cause) {
		t.Fatal("errors.Is не находит причину")
	}
}

func TestStatuses(t *testing.T) {
	cases := []struct {
		err  *apierr.Error
		want int
	}{
		{apierr.BadRequest(""), 400},
		{apierr.Unauthorized(""), 401},
		{apierr.Forbidden(""), 403},
		{apierr.NotFound(""), 404},
		{apierr.Conflict(""), 409},
		{apierr.UnprocessableEntity(""), 422},
		{apierr.TooManyRequests(""), 429},
		{apierr.Unavailable(""), 503},
	}

	for _, tc := range cases {
		if tc.err.Status != tc.want {
			t.Fatalf("%s: got %d, want %d", tc.err.Message, tc.err.Status, tc.want)
		}
	}
}

func TestInternal(t *testing.T) {
	cause := errors.New("db is down")
	err := apierr.Internal(cause)

	if err.Status != 500 {
		t.Fatalf("статус: got %d", err.Status)
	}
	if err.Message != "внутренняя ошибка сервера" {
		t.Fatalf("сообщение: got %q", err.Message)
	}
	if !errors.Is(err, cause) {
		t.Fatal("причина не сохраняется")
	}
}

func TestStatusHelper(t *testing.T) {
	if got := apierr.Status(apierr.NotFound(""), 500); got != 404 {
		t.Fatalf("got %d", got)
	}
	if got := apierr.Status(fmt.Errorf("обычная ошибка"), 500); got != 500 {
		t.Fatalf("got %d", got)
	}
	if got := apierr.Status(nil, 500); got != 500 {
		t.Fatalf("got %d", got)
	}
}

func TestMessageHelper(t *testing.T) {
	if got := apierr.Message(apierr.Forbidden("нет доступа"), "fallback"); got != "нет доступа" {
		t.Fatalf("got %q", got)
	}
	if got := apierr.Message(errors.New("x"), "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
}

func TestNew(t *testing.T) {
	err := apierr.New(418, "я чайник")
	if err.Status != 418 || err.Message != "я чайник" {
		t.Fatalf("got %d %q", err.Status, err.Message)
	}
}
