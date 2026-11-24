package email

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/resend/resend-go/v4"
	"github.com/rs/zerolog"
	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/sharathm18/myapp/internal/config"
)

type EmailHandler struct {
	Config      *config.Config
	EmailClient *resend.Client
}

func NewEmailHandler(config *config.Config) *EmailHandler {
	return &EmailHandler{
		Config:      config,
		EmailClient: resend.NewClient(config.Integration.Email.EmailAPIKey),
	}
}

// HandleWelcomeEmailTask executes the actual background task
func (h *EmailHandler) HandleWelcomeEmailTask(ctx context.Context, t *asynq.Task) error {
	// 1. Decode payload; corrupt bytes skip retries and go straight to DLQ.
	var payload WelcomeEmailPayload
	if err := msgpack.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal welcome email payload: %w: %w", err, asynq.SkipRetry)
	}

	// 1. Add business fields directly onto the context logger.
	zerolog.Ctx(ctx).UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.
			Str("to", payload.To).
			Str("name", payload.Name)
	})

	// 2. Enrich the active Consumer span with business attributes.
	trace.SpanFromContext(ctx).SetAttributes(
		attribute.String("email.to", payload.To),
		attribute.String("email.name", payload.Name),
	)

	// 4.Execute business logic. If this returns an error, ConsumerMiddleware catches it, records it on the span, and logs.
	if err := h.SendWelcome(ctx, payload.To, payload.Name); err != nil {
		return fmt.Errorf("send welcome email: %w", err)
	}

	return nil
}

func (h *EmailHandler) SendWelcome(ctx context.Context, to string, name string) error {
	_, err := h.EmailClient.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
		From:    "onboarding@resend.dev",
		To:      []string{to},
		Subject: "Welcome!",
		Text:    fmt.Sprintf("Welcome, %s!", name),
	})
	return err
}
