package system

import (
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/hibiken/asynqmon"
	"github.com/labstack/echo/v5"
	"github.com/sharathm18/myapp/internal/server"
)

type AsynqMonHandler struct {
	SystemServer *server.System
	hmon         http.Handler
}

// Asynq manages its own pool internally, so we just need the connection details
func NewAsynqMonHandler(systemServer *server.System) *AsynqMonHandler {
	hmon := asynqmon.New(asynqmon.Options{
		RootPath: "/admin/asynqmon",
		RedisConnOpt: asynq.RedisClientOpt{
			Addr:     systemServer.Config.RedisAsynq.Address,
			Password: systemServer.Config.RedisAsynq.Password,
			DB:       systemServer.Config.RedisAsynq.Database,
		},
	})

	return &AsynqMonHandler{
		SystemServer: systemServer,
		hmon:         hmon,
	}
}

func (h *AsynqMonHandler) ServeAsynqMon(c *echo.Context) error {
	return echo.WrapHandler(h.hmon)(c)
}
