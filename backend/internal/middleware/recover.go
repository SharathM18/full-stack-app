package middleware

import (
	"github.com/labstack/echo/v5"
	echoMiddleware "github.com/labstack/echo/v5/middleware"
)

// RecoverMiddleware catches unexpected runtime panics in downstream middlewares or handlers,
// converts them into standard errors, and passes them to the global HTTPErrorHandler
func RecoverMiddleware() echo.MiddlewareFunc {
	return echoMiddleware.RecoverWithConfig(echoMiddleware.RecoverConfig{
		// allocate a 4 KB buffer to capture the stack trace of the panicking goroutine
		StackSize: 4 << 10,

		// capture only the crashing goroutine's stack instead of dumping all background goroutines
		DisableStackAll: true,

		// stop echo from printing raw plain-text stack traces to stderr so the
		// global error handler and logger can record it cleanly in JSON
		DisablePrintStack: true,
	})
}
