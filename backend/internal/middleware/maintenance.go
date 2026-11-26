package middleware

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// MaintenanceMiddleware blocks incoming traffic with 503 Service Unavailable when enabled.
// Can be attached globally, to a specific domain group, or to a single route.
func MaintenanceMiddleware(isUnavailable bool, message string) echo.MiddlewareFunc {
	if message == "" {
		message = "this service is temporarily unavailable for maintenance"
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if isUnavailable {
				// Optional hint telling clients/browsers to retry after 5 minutes (300s)
				c.Response().Header().Set("Retry-After", "300")
				return echo.NewHTTPError(http.StatusServiceUnavailable, message)
			}
			return next(c)
		}
	}
}
