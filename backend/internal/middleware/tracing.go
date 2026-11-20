package middleware

import (
	echootel "github.com/labstack/echo-otel/v5"
	"github.com/labstack/echo/v5"
	"github.com/sharathm18/myapp/internal/server"
)

func SetupTracingMiddleware(appSrv *server.APP) echo.MiddlewareFunc {
	// skip creating spans and allocating tracing memory when OTel is disabled in config.
	if !appSrv.Config.Observability.OTel.Enabled || !appSrv.Config.Observability.OTel.TracesEnabled {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	// extract incoming traceparent headers or start a new root trace, inject the span
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

// EnhanceTracing by extracing span ID from Go context and attaches custom request metadata and errors to active OTel spans
// func EnhanceTracing() echo.MiddlewareFunc {
// 	return func(next echo.HandlerFunc) echo.HandlerFunc {
// 		return func(c *echo.Context) error {
// 			req := c.Request()
// 			span := trace.SpanFromContext(req.Context())

// 			if span.IsRecording() {
// 				span.SetAttributes(
// 					attribute.String("http.real_ip", c.RealIP()),
// 					// UserAgent is already captured by OTel middleware, so no need to duplicate
// 				)

// 				// Add User ID from echo context to otel span if present
// 				if userID := c.Get("user_id"); userID != nil {
// 					if userIDStr, ok := userID.(string); ok {
// 						span.SetAttributes(attribute.String("user.id", userIDStr))
// 					}
// 				}

// 				// Add User Role from echo context to otel span if present
// 				if userRole := c.Get("user_role"); userRole != nil {
// 					if userRoleStr, ok := userRole.(string); ok {
// 						span.SetAttributes(attribute.String("user.role", userRoleStr))
// 					}
// 				}
// 			}

// 			// execute handler pipeline
// 			err := next(c)

// 			if span.IsRecording() {
// 				// record error status and message if handler returned an error
// 				if err != nil {
// 					span.RecordError(err)
// 					span.SetStatus(codes.Error, err.Error())
// 				}

// 				span.SetAttributes(attribute.Int("http.status_code", c.Response().Status))
// 			}

// 			return err
// 		}
// 	}
// }
