package middleware

import (
	"net/http"

	"github.com/labstack/echo/v5"
	echoMiddleware "github.com/labstack/echo/v5/middleware"
	"github.com/sharathm18/myapp/internal/server"
)

// SecurityHeaders injects standard browser protection headers (X-Frame-Options,
// X-Content-Type-Options, HSTS) onto every outgoing HTTP response
func SecurityHeadersMiddleware() echo.MiddlewareFunc {
	return echoMiddleware.Secure()
}

// CORS validates incoming browser origins, handles preflight OPTIONS requests,
// and exposes X-Request-ID so frontend clients can read correlation IDs
func CORSMiddleware(appServer *server.APP) echo.MiddlewareFunc {
	return echoMiddleware.CORSWithConfig(echoMiddleware.CORSConfig{
		AllowOrigins:     appServer.Config.AppServer.CORSAllowedOrigins,
		AllowCredentials: true,
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
			echo.HeaderXRequestID,
		},
		ExposeHeaders: []string{
			echo.HeaderXRequestID,
		},
	})
}

// BodyLimit rejects incoming request payloads larger than 2 MB (2 << 20 bytes)
// with 413 Request Entity Too Large to protect server memory
func BodyLimitMiddleware() echo.MiddlewareFunc {
	return echoMiddleware.BodyLimit(2 << 20)
}
