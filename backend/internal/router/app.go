package router

import (
	"github.com/labstack/echo/v5"
	mw "github.com/sharathm18/myapp/internal/middleware"
	"github.com/sharathm18/myapp/internal/server"
)

// Context (When a request hits your server, Go and Echo build the three boxes from inside to outside)
// 1. innermost context -> go (ctx context.Context)
// 2. middle context -> http (req *http.Request)
// 3. outtermost context -> echo (c *echo.Context)
// context are immutable, You cannot modify an existing context in place,
// so that we do like copy current context into new context with new values and link/ attach to previous context
// flow outtermost -> middle -> innermost

// groups app domain handlers
type AppHandlers struct {
}

func NewAppRouter(appSrv *server.APP, h AppHandlers) *echo.Echo {

	appRouter := echo.New()

	// Format all returned errors into a single consistent JSON structure.
	appRouter.HTTPErrorHandler = mw.HTTPErrorHandler

	appRouter.Use(

		// catch unexpected runtime panics first so the server never crashes
		mw.RecoverMiddleware(),

		// resolve or generate X-Request-ID and store it in go context
		mw.RequestIDMiddleware(),

		// extract incoming traceparent or create a new OTel trace_id and root span_id
		mw.SetupTracingMiddleware(appSrv),

		// attach HTTP metadata, request_id, and OTel IDs to Zerolog in context
		mw.RequestLoggerMiddleware(appSrv),

		// emits one canonical summary log entry (for both successes and errors) when the request finishes using pre-enriched zerolog instance
		mw.SummaryLoggerMiddleware(appSrv),

		// add security headers (X-Frame-Options, X-Content-Type-Options, HSTS) to responses.
		mw.SecurityHeadersMiddleware(),

		// validate allowed browser origins and answer preflight OPTIONS requests.
		mw.CORSMiddleware(appSrv),

		// reject oversized request payloads (> 2 MB) before reading into memory.
		mw.BodyLimitMiddleware(),

		// throttle abusive traffic per client IP (20 req/sec steady, burst up to 40).
		mw.RateLimitMiddleware(20, 40),
	)

	return appRouter
}
