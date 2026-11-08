# go_http

Единый HTTP-слой сервисов SHINPI: формат ошибок, списки с пагинацией, создание,
пустой успех, разбор и валидация тела запроса.

Заменяет ручные `fiber.Map{"message": ..., "error": ...}` и копии расчёта
`pagination` по всем сервисам (на момент выноса — 285 мест и 37 копий).

## Установка

```bash
go get github.com/shinpi-tech/go_http@latest
```

Зависимость — только `github.com/gofiber/fiber/v3`. Валидатор не навязан:
подойдёт любой тип с методом `Validate(any) error` (например, `*go_validate.Valid`).

## Пакеты

| Пакет | Назначение |
|---|---|
| `apierr` | доменная ошибка: HTTP-статус + сообщение для клиента, причина — только для логов |
| `httpx` | помощники ответа и разбора запроса |

## apierr

```go
return apierr.NotFound("бренд не найден")
return apierr.BadRequest("некорректный slug")
return apierr.Conflict("склад уже существует").Wrap(err) // причина уйдёт в лог, не в ответ
return apierr.Internal(err)                              // 500 + «внутренняя ошибка сервера»
```

Доступные конструкторы: `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`,
`Conflict`, `UnprocessableEntity`, `TooManyRequests`, `Internal`, `New(status, msg)`.

`*Error` совместим с `errors.Is/As`: причина доступна через `Unwrap()`.
Хелперы `apierr.Status(err, fallback)` и `apierr.Message(err, fallback)` — когда
ошибка пришла из другого слоя.

## httpx

```go
// ошибка: доменная — как есть, прочие — 500 с нейтральным текстом + лог
func (h *BrandHandler) Get(c fiber.Ctx) error {
    brand, err := h.service.Get(c.Context(), c.Params("id"))
    if err != nil {
        return httpx.Fail(c, err)
    }
    return c.JSON(brand)
}

// список: {items, pagination:{total,limit,page,pages}}
page, limit := httpx.Page(c)                 // 1 / 48, максимум 100
return httpx.List(c, items, total, page, limit)

// создание / обновление / удаление
return httpx.ID(c, id)                       // 200 {"id": "..."}
return httpx.OK(c)                           // 200 без тела

// тело запроса: декодирование + валидация, ошибка — 400
var req dto.CreateBrand
if err := httpx.ParseBody(c, h.validate, &req); err != nil {
    return httpx.Fail(c, err)
}
```

| Функция | Поведение |
|---|---|
| `Fail(c, err)` | `*apierr.Error` → его статус и сообщение; иначе 500 «внутренняя ошибка сервера» + лог |
| `FailStatus(c, status, msg)` | явный статус и сообщение |
| `List(c, items, total, page, limit)` | нормализует `page`/`limit`, считает `pages` |
| `Page(c)` | `page` и `limit` из query: 1/48, максимум 100 |
| `QueryInt(c, key)` | целое из query + признак «параметр был» |
| `ID(c, id)` | 200 `{"id": "..."}` |
| `OK(c)` | 200 без тела |
| `ParseBody(c, validator, dst)` | декодирует и валидирует; ошибка — `400` |
| `BadQuery()` | 400 для некорректных query-параметров |

## Правила

- В ответе об ошибке **одно** поле `message`. Внутренние причины (`error`) клиенту не
  отдаются — они попадают в лог через `Fail`.
- `limit` больше 100 молча приводится к 100: ограничение нельзя забыть в обработчике.
- Пакет не знает про MongoDB, NATS и бизнес-логику — только HTTP-формат.

## Тесты

```bash
GOWORK=off go test ./...
```

Покрыто: формат ошибок и отсутствие утечки причины, расчёт `pages`, нормализация
`page`/`limit`, разбор и валидация тела, пустой успех.
