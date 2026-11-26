package router

import (
	"crypto/subtle"
	"net/http"
	_ "net/http/pprof"
	"strings"

	"github.com/labstack/echo/v5"
	echoMiddleware "github.com/labstack/echo/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/sharathm18/myapp/internal/domain/system"
	mw "github.com/sharathm18/myapp/internal/middleware"
	"github.com/sharathm18/myapp/internal/server"
)

// Groups system domain handlers
type SystemHandlers struct {
	HealthHandler   *system.HealthHandler
	AsynqMonHandler *system.AsynqMonHandler
	InfoHandler     *system.InfoHandler
}

func NewSystemRouter(systemServer *server.System, h SystemHandlers) *echo.Echo {
	systemRouter := echo.New()

	// Format all returned errors into a single consistent JSON structure
	systemRouter.HTTPErrorHandler = mw.HTTPErrorHandler

	systemRouter.Use(
		// Catch unexpected runtime panics first so the server never crashes
		mw.RecoverMiddleware(),

		// Add security headers (X-Frame-Options, X-Content-Type-Options, HSTS) to responses
		mw.SecurityHeadersMiddleware(),

		// Log admin panel activity once on exit, skipping noisy health probes & metrics scrapes
		echoMiddleware.RequestLoggerWithConfig(echoMiddleware.RequestLoggerConfig{
			Skipper: func(c *echo.Context) bool {
				path := c.Request().URL.Path
				return strings.HasPrefix(path, "/health/") || path == "/admin/metrics"
			},
			LogURI:     true,
			LogStatus:  true,
			LogLatency: true,
			LogValuesFunc: func(c *echo.Context, v echoMiddleware.RequestLoggerValues) error {
				statusCode := v.Status
				if v.Error != nil {
					statusCode = mw.ResolveHTTPStatus(c.Request().Context(), v.Error)
				}

				var event *zerolog.Event
				switch {
				case statusCode >= http.StatusInternalServerError:
					event = systemServer.Logger.Error().Err(v.Error)
				case statusCode >= http.StatusBadRequest:
					event = systemServer.Logger.Warn().Err(v.Error)
				default:
					event = systemServer.Logger.Info()
				}

				event.
					Str("server", "system").
					Str("method", c.Request().Method).
					Str("uri", v.URI).
					Int("status", statusCode).
					Dur("latency", v.Latency).
					Msg("system request processed")

				return nil
			},
		}),
	)

	//  Unauthenticated Probes (For AWS / Kubernetes)
	systemRouter.GET("/health/live", h.HealthHandler.CheckLiveness)
	systemRouter.GET("/health/ready", h.HealthHandler.CheckReadiness)
	systemRouter.GET("/info", h.InfoHandler.GetInfo)

	// Secured internal admin panel (Echo v5 BasicAuth signature puts c *echo.Context first)
	admin := systemRouter.Group("/admin", echoMiddleware.BasicAuth(func(c *echo.Context, username, password string) (bool, error) {
		expectedUser := systemServer.Config.Auth.SystemUsername
		expectedPass := systemServer.Config.Auth.SystemPassword
		if expectedUser == "" || expectedPass == "" {
			return false, nil
		}

		userMatch := subtle.ConstantTimeCompare([]byte(username), []byte(expectedUser))
		passMatch := subtle.ConstantTimeCompare([]byte(password), []byte(expectedPass))
		return (userMatch & passMatch) == 1, nil
	}))

	// Prometheus Scraper
	admin.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	// Asynq Dashboard (Requires both exact and wildcard paths)
	admin.Any("/asynqmon", h.AsynqMonHandler.ServeAsynqMon)
	admin.Any("/asynqmon/*", h.AsynqMonHandler.ServeAsynqMon)

	// Go Profiler (Strip /admin prefix so DefaultServeMux can match /debug/pprof/)
	admin.Any("/debug/pprof/*", echo.WrapHandler(http.StripPrefix("/admin", http.DefaultServeMux)))

	return systemRouter
}
