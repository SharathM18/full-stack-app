package response

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// Envelope defines the uniform JSON structure for all successful API responses.
type Envelope[T any] struct {
	Data T     `json:"data"`
	Meta *Meta `json:"meta,omitempty"`
}

// Meta holds pagination details for list endpoints.
type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

// OK sends a 200 JSON response wrapped in {"data": ...} (Get by ID, Update, Login).
func OK[T any](c *echo.Context, data T) error {
	return c.JSON(http.StatusOK, Envelope[T]{Data: data})
}

// Created sends a 201 JSON response wrapped in {"data": ...} (Create).
func Created[T any](c *echo.Context, data T) error {
	return c.JSON(http.StatusCreated, Envelope[T]{Data: data})
}

// Accepted sends a 202 JSON response wrapped in {"data": ...} (Async background jobs).
func Accepted[T any](c *echo.Context, data T) error {
	return c.JSON(http.StatusAccepted, Envelope[T]{Data: data})
}

// Paginated sends a 200 JSON response with both "data" and "meta" (Paginated list).
func Paginated[T any](c *echo.Context, data T, meta Meta) error {
	return c.JSON(http.StatusOK, Envelope[T]{Data: data, Meta: &meta})
}

// NoContent sends a 204 response with an empty body (Delete).
func NoContent(c *echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
