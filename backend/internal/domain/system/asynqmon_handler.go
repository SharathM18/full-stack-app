package system

import (
	"encoding/json"
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/hibiken/asynqmon"
	"github.com/labstack/echo/v5"
	"github.com/sharathm18/myapp/internal/server"
	"github.com/vmihailenco/msgpack/v5"
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
		PayloadFormatter: asynqmon.PayloadFormatterFunc(func(taskType string, payload []byte) string {
			var decoded any
			if err := msgpack.Unmarshal(payload, &decoded); err != nil {
				return string(payload)
			}
			out, err := json.Marshal(decoded)
			if err != nil {
				return string(payload)
			}
			return string(out)
		}),
	})

	return &AsynqMonHandler{
		SystemServer: systemServer,
		hmon:         hmon,
	}
}

func (h *AsynqMonHandler) ServeAsynqMon(c *echo.Context) error {
	return echo.WrapHandler(h.hmon)(c)
}
