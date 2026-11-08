package httpx_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/shinpi-tech/go_http/apierr"
	"github.com/shinpi-tech/go_http/httpx"
)

// stubValidator повторяет поведение go_validate.Valid в тестах.
type stubValidator struct {
	err error
}

func (s stubValidator) Validate(any) error { return s.err }

func call(t *testing.T, app *fiber.App, method, target string, body string) (int, string) {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("запрос %s %s: %v", method, target, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("чтение ответа: %v", err)
	}

	return resp.StatusCode, string(raw)
}

func TestFail_DomainError(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return httpx.Fail(c, apierr.NotFound("бренд не найден"))
	})

	status, body := call(t, app, http.MethodGet, "/", "")
	if status != http.StatusNotFound {
		t.Fatalf("статус: got %d, want %d", status, http.StatusNotFound)
	}
	if body != `{"message":"бренд не найден"}` {
		t.Fatalf("тело: got %s", body)
	}
}

func TestFail_InternalHidesCause(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return httpx.Fail(c, errors.New("mongo: no documents in result"))
	})

	status, body := call(t, app, http.MethodGet, "/", "")
	if status != http.StatusInternalServerError {
		t.Fatalf("статус: got %d, want 500", status)
	}
	if strings.Contains(body, "mongo") {
		t.Fatalf("внутренняя причина утекла в ответ: %s", body)
	}
	if body != `{"message":"внутренняя ошибка сервера"}` {
		t.Fatalf("тело: got %s", body)
	}
}

func TestFail_WrappedDomainError(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		err := apierr.Conflict("склад уже существует").Wrap(errors.New("duplicate key"))
		return httpx.Fail(c, err)
	})

	status, body := call(t, app, http.MethodGet, "/", "")
	if status != http.StatusConflict {
		t.Fatalf("статус: got %d, want 409", status)
	}
	if strings.Contains(body, "duplicate key") {
		t.Fatalf("причина утекла в ответ: %s", body)
	}
}

func TestFail_Nil(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return httpx.Fail(c, nil)
	})

	status, _ := call(t, app, http.MethodGet, "/", "")
	if status != http.StatusOK {
		t.Fatalf("статус: got %d, want 200", status)
	}
}

func TestFailStatus(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return httpx.FailStatus(c, http.StatusForbidden, "недостаточно прав")
	})

	status, body := call(t, app, http.MethodGet, "/", "")
	if status != http.StatusForbidden {
		t.Fatalf("статус: got %d", status)
	}
	if body != `{"message":"недостаточно прав"}` {
		t.Fatalf("тело: got %s", body)
	}
}

func TestList_ComputesPages(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return httpx.List(c, []string{"a", "b"}, 101, 2, 48)
	})

	status, body := call(t, app, http.MethodGet, "/", "")
	if status != http.StatusOK {
		t.Fatalf("статус: got %d", status)
	}
	want := `{"items":["a","b"],"pagination":{"total":101,"limit":48,"page":2,"pages":3}}`
	if body != want {
		t.Fatalf("тело:\n got %s\nwant %s", body, want)
	}
}

func TestList_NormalizesAndClamps(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return httpx.List(c, []string{}, 0, 0, 500)
	})

	_, body := call(t, app, http.MethodGet, "/", "")
	want := `{"items":[],"pagination":{"total":0,"limit":100,"page":1,"pages":0}}`
	if body != want {
		t.Fatalf("тело:\n got %s\nwant %s", body, want)
	}
}

func TestPage(t *testing.T) {
	cases := []struct {
		query     string
		wantPage  int
		wantLimit int
	}{
		{"", 1, 48},
		{"?page=3&limit=10", 3, 10},
		{"?page=0&limit=0", 1, 48},
		{"?page=-2&limit=-5", 1, 48},
		{"?page=abc&limit=xyz", 1, 48},
		{"?limit=5000", 1, 100},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			app := fiber.New()
			var (
				page  int
				limit int
			)
			app.Get("/", func(c fiber.Ctx) error {
				page, limit = httpx.Page(c)
				return c.SendStatus(http.StatusOK)
			})

			call(t, app, http.MethodGet, "/"+tc.query, "")
			if page != tc.wantPage || limit != tc.wantLimit {
				t.Fatalf("got page=%d limit=%d, want page=%d limit=%d", page, limit, tc.wantPage, tc.wantLimit)
			}
		})
	}
}

func TestID(t *testing.T) {
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		return httpx.ID(c, "64f0c2")
	})

	status, body := call(t, app, http.MethodPost, "/", "")
	if status != http.StatusOK {
		t.Fatalf("статус: got %d", status)
	}
	if body != `{"id":"64f0c2"}` {
		t.Fatalf("тело: got %s", body)
	}
}

func TestOK(t *testing.T) {
	app := fiber.New()
	app.Delete("/", func(c fiber.Ctx) error {
		return httpx.OK(c)
	})

	status, body := call(t, app, http.MethodDelete, "/", "")
	if status != http.StatusOK || body != "" {
		t.Fatalf("got %d %q, want 200 с пустым телом", status, body)
	}
}

func TestParseBody_Ok(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		var req request
		if err := httpx.ParseBody(c, stubValidator{}, &req); err != nil {
			return httpx.Fail(c, err)
		}
		return httpx.ID(c, req.Name)
	})

	status, body := call(t, app, http.MethodPost, "/", `{"name":"brand"}`)
	if status != http.StatusOK || body != `{"id":"brand"}` {
		t.Fatalf("got %d %s", status, body)
	}
}

func TestParseBody_BrokenJSON(t *testing.T) {
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		var req map[string]any
		if err := httpx.ParseBody(c, stubValidator{}, &req); err != nil {
			return httpx.Fail(c, err)
		}
		return httpx.OK(c)
	})

	status, body := call(t, app, http.MethodPost, "/", `{`)
	if status != http.StatusBadRequest {
		t.Fatalf("статус: got %d, want 400", status)
	}
	if body != `{"message":"некорректное тело запроса"}` {
		t.Fatalf("тело: got %s", body)
	}
}

func TestParseBody_ValidationError(t *testing.T) {
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		var req map[string]any
		err := httpx.ParseBody(c, stubValidator{err: errors.New("Имя: обязательное поле")}, &req)
		if err != nil {
			return httpx.Fail(c, err)
		}
		return httpx.OK(c)
	})

	status, body := call(t, app, http.MethodPost, "/", `{"name":""}`)
	if status != http.StatusBadRequest {
		t.Fatalf("статус: got %d, want 400", status)
	}
	if !strings.Contains(body, "Имя: обязательное поле") {
		t.Fatalf("тело: got %s", body)
	}
}

func TestParseBody_NilValidator(t *testing.T) {
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		var req map[string]any
		if err := httpx.ParseBody(c, nil, &req); err != nil {
			return httpx.Fail(c, err)
		}
		return httpx.OK(c)
	})

	status, _ := call(t, app, http.MethodPost, "/", `{"name":"x"}`)
	if status != http.StatusOK {
		t.Fatalf("статус: got %d, want 200", status)
	}
}

func TestQueryInt(t *testing.T) {
	app := fiber.New()
	var (
		value   int
		present bool
	)
	app.Get("/", func(c fiber.Ctx) error {
		value, present = httpx.QueryInt(c, "warehouse")
		return c.SendStatus(http.StatusOK)
	})

	cases := []struct {
		query       string
		wantValue   int
		wantPresent bool
	}{
		{"?warehouse=12", 12, true},
		{"", 0, false},
		{"?warehouse=abc", 0, false},
	}

	for _, tc := range cases {
		call(t, app, http.MethodGet, "/"+tc.query, "")
		if value != tc.wantValue || present != tc.wantPresent {
			t.Fatalf("%s: got %d %v", tc.query, value, present)
		}
	}
}

func TestFailCause_HidesCause(t *testing.T) {
	app := fiber.New()
	app.Post("/brand", func(c fiber.Ctx) error {
		return httpx.FailCause(c, http.StatusBadRequest, "Товар передан не верно", errors.New("mongo: no documents"))
	})

	status, body := call(t, app, http.MethodPost, "/brand", "")
	if status != http.StatusBadRequest {
		t.Fatalf("статус: got %d, want 400", status)
	}
	if body != `{"message":"Товар передан не верно"}` {
		t.Fatalf("тело: got %s", body)
	}
	if strings.Contains(body, "mongo") {
		t.Fatalf("внутренняя причина утекла в ответ: %s", body)
	}
}

func TestFailCause_NilCause(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return httpx.FailCause(c, http.StatusForbidden, "нет доступа", nil)
	})

	status, body := call(t, app, http.MethodGet, "/", "")
	if status != http.StatusForbidden || body != `{"message":"нет доступа"}` {
		t.Fatalf("got %d %s", status, body)
	}
}

func TestFailCause_StringCause(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return httpx.FailCause(c, http.StatusBadGateway, "ошибка поставщика", "timeout 5s")
	})

	status, body := call(t, app, http.MethodGet, "/", "")
	if status != http.StatusBadGateway || body != `{"message":"ошибка поставщика"}` {
		t.Fatalf("got %d %s", status, body)
	}
}

func TestCreated(t *testing.T) {
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		return httpx.Created(c, "64f0c2")
	})

	status, body := call(t, app, http.MethodPost, "/", "")
	if status != http.StatusCreated || body != `{"id":"64f0c2"}` {
		t.Fatalf("got %d %s", status, body)
	}
}

func TestItems(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return httpx.Items(c, []string{"a", "b"})
	})

	status, body := call(t, app, http.MethodGet, "/", "")
	if status != http.StatusOK || body != `{"items":["a","b"]}` {
		t.Fatalf("got %d %s", status, body)
	}
}
