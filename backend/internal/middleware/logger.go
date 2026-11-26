package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	echoMiddleware "github.com/labstack/echo/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/sharathm18/myapp/internal/infrastructure/observability/logging"
	"github.com/sharathm18/myapp/internal/server"
)

type loggerCtxKey struct{}

// RequestLoggerMiddleware builds the initial request-scoped logger with correlation IDs and HTTP request details,
// wraps it in a new Go context, and updates *http.Request and *echo.Context to point to it
func RequestLoggerMiddleware(appServer *server.APP) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()

			// build base request logger with metadata
			logBuilder := appServer.Logger.With().
				Str("method", c.Request().Method).
				Str("path", c.Path()).
				Str("ip", c.RealIP())

			if requestID := GetRequestID(req.Context()); requestID != "" {
				logBuilder = logBuilder.Str("request_id", requestID)
			}

			// extract OTel trace_id and span_id from go context and attach to logger
			contextLogger := logging.WithTraceContext(c.Request().Context(), logBuilder.Logger())

			// wrap the logger in a new child context and re-attach it to *http.Request and *echo.Context
			ctx := context.WithValue(req.Context(), loggerCtxKey{}, &contextLogger)
			c.SetRequest(req.WithContext(ctx))

			return next(c)
		}
	}
}

// EnhancedRequestLoggerMiddleware pulls the Zerolog instance created earlier by RequestLogger,
// attaches authenticated user_id and user_role fields, and returns a new child context.
func EnhancedRequestLoggerMiddleware(ctx context.Context, userID uuid.UUID, userRole string) context.Context {
	enrichedLogger := GetLogger(ctx).With().
		Str(UserIDKey, userID.String()).
		Str(UserRoleKey, userRole).
		Logger()

	return context.WithValue(ctx, loggerCtxKey{}, &enrichedLogger)
}

// SummaryLoggerMiddleware measures total request latency, resolves the final HTTP status code,
// and emits one canonical summary log entry (for both successes and errors) using the
// pre-enriched Zerolog instance stored in go context.
func SummaryRequestLoggerMiddleware(appServer *server.APP) echo.MiddlewareFunc {
	return echoMiddleware.RequestLoggerWithConfig(echoMiddleware.RequestLoggerConfig{
		LogURI:          true,
		LogStatus:       true,
		LogLatency:      true,
		LogHost:         true,
		LogUserAgent:    true,
		LogResponseSize: true,

		LogValuesFunc: func(c *echo.Context, v echoMiddleware.RequestLoggerValues) error {
			ctx := c.Request().Context()
			statusCode := v.Status

			// Resolve the exact HTTP status code via the shared error translator so log
			// severity (WRN vs ERR) always matches the JSON response sent by HTTPErrorHandler.
			if v.Error != nil {
				statusCode, _ = resolveError(ctx, v.Error)
			}

			logger := GetLogger(ctx)

			var event *zerolog.Event
			switch {
			case statusCode >= http.StatusInternalServerError:
				event = logger.Error().Err(v.Error)
			case statusCode >= http.StatusBadRequest:
				event = logger.Warn().Err(v.Error)
			default:
				event = logger.Info()
			}

			// Emit the single summary log line for the entire request lifecycle.
			event.
				Str("server", "app").
				Dur("latency", v.Latency).
				Int("status", statusCode).
				Str("uri", v.URI).
				Str("host", v.Host).
				Int64("response_size", v.ResponseSize).
				Str("user_agent", v.UserAgent).
				Msg("api request processed")

			return nil
		},
	})
}

// Utils functions

func GetLogger(ctx context.Context) *zerolog.Logger {
	if logger, ok := ctx.Value(loggerCtxKey{}).(*zerolog.Logger); ok && logger != nil {
		return logger
	}
	logger := zerolog.Nop()
	return &logger
}

// c                   ──►  *echo.Context    (Outer Box)
// c.Request()         ──►  *http.Request    (Middle Box)
// c.Request().Context() ──► context.Context  (Innermost Box — Go's standard context!)
