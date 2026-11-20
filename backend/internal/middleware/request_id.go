package middleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	echoMiddleware "github.com/labstack/echo/v5/middleware"
)

type requestIDKey struct{}

// RequestIDMiddleware ensures every incoming request has a unique X-Request-ID header and copies that ID into go context
func RequestIDMiddleware() echo.MiddlewareFunc {
	return echoMiddleware.RequestIDWithConfig(echoMiddleware.RequestIDConfig{
		// generate a standard UUID when the client or load balancer did not send X-Request-ID
		Generator: uuid.NewString,

		RequestIDHandler: func(c *echo.Context, id string) {
			// echo contect handover request context
			req := c.Request()

			// store the request ID inside new go context and attach current request context
			ctx := context.WithValue(req.Context(), requestIDKey{}, id)

			// req.WithContext(ctx) means it copy the current request context to new request context with attached go context
			// re-attach updated request context to echo context
			c.SetRequest(req.WithContext(ctx))
		},
	})
}

// Utils functions

// GetRequestID extracts the request ID from go context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}
