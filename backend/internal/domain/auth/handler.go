package auth

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/sharathm18/myapp/internal/domain/shared/response"
	"github.com/sharathm18/myapp/internal/domain/shared/validator"
	"github.com/sharathm18/myapp/internal/server"
)

const refreshCookieName = "refresh_token"

type AuthHandler struct {
	AppServer   *server.APP
	AuthService *AuthService
}

func NewAuthHandler(appServer *server.APP, authSvc *AuthService) *AuthHandler {
	return &AuthHandler{AppServer: appServer, AuthService: authSvc}
}

// main handler functions

func (h *AuthHandler) Signup(c *echo.Context) error {
	req, err := validator.BindAndValidate[SignupRequest](c)
	if err != nil {
		return err
	}

	res, rawRefreshToken, err := h.AuthService.Signup(
		c.Request().Context(),
		req,
		c.Request().UserAgent(),
		c.RealIP(),
	)

	if err != nil {
		return err
	}

	h.setRefreshCookie(c, rawRefreshToken)

	return response.Created(c, res)
}

func (h *AuthHandler) Login(c *echo.Context) error {
	req, err := validator.BindAndValidate[LoginRequest](c)
	if err != nil {
		return err
	}

	res, rawRefreshToken, err := h.AuthService.Login(
		c.Request().Context(),
		req,
		c.Request().UserAgent(),
		c.RealIP(),
	)

	if err != nil {
		return err
	}

	h.setRefreshCookie(c, rawRefreshToken)

	return response.OK(c, res)
}

// helper functions

func (h *AuthHandler) setRefreshCookie(c *echo.Context, token string) {
	ttl := h.AppServer.Config.Auth.RefreshTokenTTL
	c.SetCookie(&http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/api/v1/auth",
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   c.Request().TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}
