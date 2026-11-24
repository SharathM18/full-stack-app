package email

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Task type
// naming convention for name is domain:action
const (
	TypeWelcomeEmail = "email:welcome"
	// TypeResetPassword = "email:reset-password"
)

// Contract JSON Payload
// WelcomeEmailPayload defines the exact JSON structure stored in Redis
type WelcomeEmailPayload struct {
	To   string `msgpack:"to"`
	Name string `msgpack:"name"`
}

func NewWelcomeEmailTask(ctx context.Context, taskPayload WelcomeEmailPayload) (*asynq.Task, error) {
	// Serialize using msgpack instead of json
	payload, err := msgpack.Marshal(taskPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal welcome email payload: %w", err)
	}

	// Initialize an empty header map so Enqueue can inject traceparent after starting the producer span
	headers := make(map[string]string)
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))

	// Pass the headers directly into Asynq using NewTaskWithHeaders
	return asynq.NewTaskWithHeaders(
		TypeWelcomeEmail,
		payload,
		headers,
		asynq.MaxRetry(3), // 1 initial attempt + 3 retries = 4 total attempts before DLQ
	), nil
}
