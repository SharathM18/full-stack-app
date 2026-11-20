package middleware

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	echoMiddleware "github.com/labstack/echo/v5/middleware"
)

// RateLimit enforces per-IP token-bucket rate limiting (requestsPerSecond steady rate
// with short spikes allowed up to burst) and returns 429 Too Many Requests when exceeded
func RateLimitMiddleware(requestsPerSecond float64, burst int) echo.MiddlewareFunc {
	// track each client IP's token bucket in memory and evict idle entries after 3 minutes
	store := echoMiddleware.NewRateLimiterMemoryStoreWithConfig(
		echoMiddleware.RateLimiterMemoryStoreConfig{
			Rate:      requestsPerSecond,
			Burst:     burst,
			ExpiresIn: 3 * time.Minute,
		},
	)

	return echoMiddleware.RateLimiterWithConfig(echoMiddleware.RateLimiterConfig{
		Store: store,

		// group rate-limit buckets by the client's IP address
		IdentifierExtractor: func(c *echo.Context) (string, error) {
			return c.RealIP(), nil
		},

		// return 403 if the client IP cannot be determined
		ErrorHandler: func(c *echo.Context, err error) error {
			return echo.NewHTTPError(http.StatusForbidden, "unable to identify client")
		},

		// return 429 so SummaryLogger logs a Warn and HTTPErrorHandler formats the JSON error
		DenyHandler: func(c *echo.Context, identifier string, err error) error {
			return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
		},
	})
}
