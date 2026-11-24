package middleware

import (
	"context"

	"github.com/google/uuid"
	echootel "github.com/labstack/echo-otel/v5"
	"github.com/labstack/echo/v5"
	"github.com/sharathm18/myapp/internal/server"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func SetupTracingMiddleware(appServer *server.APP) echo.MiddlewareFunc {
	// Skip creating spans and allocating tracing memory when OTel is disabled in config.
	if !appServer.Config.Observability.OTel.Enabled || !appServer.Config.Observability.OTel.TracesEnabled {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	// Extract incoming traceparent headers or start a new root trace, inject the span
	// into go context, and record HTTP metrics and span status on exit.
	return echootel.NewMiddlewareWithConfig(echootel.Config{
		// leave empty so echo-otel resolves server.address and server.port from Request.Host
		ServerName: "",

		// Forward any returned error to Echo's HTTPErrorHandler first so the span
		// records the final mapped HTTP status code instead of a generic 500.
		OnNextError: func(c *echo.Context, err error) {
			c.Echo().HTTPErrorHandler(c, err)
		},
	})
}

// EnhanceTracingMiddleware attaches authenticated user identity attributes to the active OTel span.
// Called inside AuthenticateMiddleware right after JWT/JWKS verification succeeds.
func EnhanceTracingMiddleware(ctx context.Context, userID uuid.UUID, userRole string) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	span.SetAttributes(
		attribute.String(UserIDKey, userID.String()),
		attribute.String(UserRoleKey, userRole),
	)
}
