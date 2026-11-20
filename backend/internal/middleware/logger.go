package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
	echoMiddleware "github.com/labstack/echo/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/sharathm18/myapp/internal/infrastructure/observability/logging"
	"github.com/sharathm18/myapp/internal/server"
)

const (
	UserIDKey   = "user_id"
	UserRoleKey = "user_role"
	LoggerKey   = "logger"
)

// RequestLoggerMiddleware builds the initial request-scoped logger with correlation IDs and HTTP request details,
// wraps it in a new Go context, and updates *http.Request and *echo.Context to point to it
func RequestLoggerMiddleware(appSrv *server.APP) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()

			// build base request logger with metadata
			logBuilder := appSrv.Logger.With().
				Str("method", c.Request().Method).
				Str("path", c.Path()).
				Str("ip", c.RealIP())

			if requestID := GetRequestID(req.Context()); requestID != "" {
				logBuilder = logBuilder.Str("request_id", requestID)
			}

			// extract OTel trace_id and span_id from go context and attach to logger
			contextLogger := logging.WithTraceContext(c.Request().Context(), logBuilder.Logger())

			// wrap the logger in a new child context and re-attach it to *http.Request and *echo.Context
			ctx := context.WithValue(req.Context(), LoggerKey, &contextLogger)
			c.SetRequest(req.WithContext(ctx))

			return next(c)
		}
	}
}

// SummaryLoggerMiddleware measures total request latency, resolves the final HTTP status code,
// and emits one canonical summary log entry (for both successes and errors) using the
// pre-enriched Zerolog instance stored in go context
func SummaryLoggerMiddleware(appSrv *server.APP) echo.MiddlewareFunc {
	return echoMiddleware.RequestLoggerWithConfig(echoMiddleware.RequestLoggerConfig{
		LogURI:          true,
		LogStatus:       true,
		LogLatency:      true,
		LogHost:         true,
		LogUserAgent:    true,
		LogResponseSize: true,

		// executes once per request right after downstream middlewares and handlers finish
		LogValuesFunc: func(c *echo.Context, v echoMiddleware.RequestLoggerValues) error {
			ctx := c.Request().Context()
			statusCode := v.Status

			// resolve the actual HTTP status code when an error was returned, because Echo's
			// global HTTPErrorHandler runs after this hook to write the JSON error response.
			if v.Error != nil {
				switch {
				case echo.StatusCode(v.Error) != 0:
					statusCode = echo.StatusCode(v.Error)
				case errors.Is(v.Error, pgx.ErrNoRows):
					statusCode = http.StatusNotFound
				default:
					statusCode = http.StatusInternalServerError
				}
			}

			// fetch the request-scoped logger created earlier by RequestLogger
			// (already contains method, path, ip, request_id, trace_id, and span_id).
			logger := GetLogger(ctx)

			// select log severity based on the final resolved HTTP status code.
			var event *zerolog.Event
			switch {
			case statusCode >= http.StatusInternalServerError:
				// record 5xx server failures as Error along with the full %w error chain.
				event = logger.Error().Err(v.Error)
			case statusCode >= http.StatusBadRequest:
				// record 4xx client, validation, or auth failures as Warn
				event = logger.Warn().Err(v.Error)
			default:
				// record 2xx/3xx successful requests as Info.
				event = logger.Info()
			}

			// append user_id if the Auth middleware stored it in go context during this request
			if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
				event = event.Str("user_id", userID)
			}

			// Emit the single summary log line for the entire request lifecycle.
			event.
				Dur("latency", v.Latency).
				Int("status", statusCode).
				Str("uri", v.URI).
				Str("host", v.Host).
				Int64("response_size", v.ResponseSize).
				Str("user_agent", v.UserAgent).
				Msg("API Request Processed")

			// Return nil so Echo passes the original v.Error untouched to HTTPErrorHandler and Tracing
			return nil
		},
	})
}

// Utils functions

func GetLogger(ctx context.Context) *zerolog.Logger {
	if logger, ok := ctx.Value(LoggerKey).(*zerolog.Logger); ok && logger != nil {
		return logger
	}
	logger := zerolog.Nop()
	return &logger
}

// c                   ──►  *echo.Context    (Outer Box)
// c.Request()         ──►  *http.Request    (Middle Box)
// c.Request().Context() ──► context.Context  (Innermost Box — Go's standard context!)
