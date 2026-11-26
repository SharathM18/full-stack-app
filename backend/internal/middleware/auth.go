package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	// Exported string names for Zerolog fields and OpenTelemetry span attributes
	UserIDKey   = "user_id"
	UserRoleKey = "user_role"
)

// Unexported struct types used strictly as context.WithValue keys
type (
	userIDCtxKey   struct{}
	userRoleCtxKey struct{}
)

// AuthClaims holds the identity and authorization data extracted from a verified token.
type AuthClaims struct {
	UserID   uuid.UUID
	UserRole string
}

// TokenVerifier abstracts token validation so Authenticate works identically
// whether tokens are issued by your own Auth service (HS256/EdDSA) or an external
// identity provider like Clerk/Auth0 (RS256 via JWKS)
type TokenVerifier interface {
	VerifyToken(ctx context.Context, tokenString string) (*AuthClaims, error)
}

// Authenticate validates the Authorization Bearer token, injects user_id and user_role
// into go innermost context, and enriches the request-scoped Zerolog logger and tracing
func AuthenticateMiddleware(verifier TokenVerifier) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			ctx := req.Context()

			// 1. Extract Bearer token from the Authorization header.
			authHeader := req.Header.Get(echo.HeaderAuthorization)
			if authHeader == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header format")
			}

			// 2. Verify token signature, expiry, and claims (Custom JWT or External JWKS)
			claims, err := verifier.VerifyToken(ctx, strings.TrimSpace(parts[1]))
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
			}

			// 3. Store identity values in go innermost context
			ctx = context.WithValue(ctx, userIDCtxKey{}, claims.UserID)
			ctx = context.WithValue(ctx, userRoleCtxKey{}, claims.UserRole)

			// 4. Enrich the existing request logger with user_id and user_role
			ctx = EnhancedRequestLoggerMiddleware(ctx, claims.UserID, claims.UserRole)

			// 5. Enhanced trace by adding user_id and user_role
			EnhanceTracingMiddleware(ctx, claims.UserID, claims.UserRole)

			// 6. Point *http.Request and *echo.Context to the updated child context
			c.SetRequest(req.WithContext(ctx))

			return next(c)
		}
	}
}

// RequireRolesMiddleware enforces Role-Based Access Control (RBAC) on a route or route group
// must be registered after Authenticate so user_role is present in context.Context
func RequireRolesMiddleware(allowedRoles ...string) echo.MiddlewareFunc {
	// pre-build a lookup map once at router startup for O(1) role checks per request
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, r := range allowedRoles {
		allowed[r] = struct{}{}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			userRole := GetUserRole(c.Request().Context())
			if userRole == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}

			if _, ok := allowed[userRole]; !ok {
				return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
			}

			return next(c)
		}
	}
}

// utils functions

// GetUserID retrieves the authenticated user's UUID from go context.
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDCtxKey{}).(uuid.UUID)
	return id, ok
}

// GetUserRole retrieves the authenticated user's userRole from go context.
func GetUserRole(ctx context.Context) string {
	if userRole, ok := ctx.Value(userRoleCtxKey{}).(string); ok {
		return userRole
	}
	return ""
}
