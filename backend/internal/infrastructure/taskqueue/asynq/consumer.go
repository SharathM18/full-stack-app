package asynq

import (
	"context"
	"errors"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/sharathm18/myapp/internal/config"
	"github.com/sharathm18/myapp/internal/infrastructure/observability/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type ConsumerClient struct {
	Client *asynq.Server
	Mux    *asynq.ServeMux
	Logger *zerolog.Logger
	Config *config.Config
	Tracer trace.Tracer
}

// NewConsumerClient strictly pulls and executes payloads from Redis.
func NewConsumerClient(config *config.Config, logger *zerolog.Logger) (*ConsumerClient, error) {
	redisOpt := asynq.RedisClientOpt{
		Addr:     config.RedisAsynq.Address,
		Password: config.RedisAsynq.Password,
		DB:       config.RedisAsynq.Database,
	}

	workerSrv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: config.Worker.Concurrency, // Number of concurrent goroutines workers/ consumer across all tasks
			Logger:      NewAsynqLogger(logger),
			Queues: map[string]int{
				"critical": 6, // 60% of worker time (e.g., OTPs)
				"default":  3, // 30% of worker time (e.g., welcome emails)
				"low":      1, // 10% of worker time (e.g., heavy PDF reports)
			},
			// By default, The global error handler executes on every single failure, not just after the maximum retries are reached
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				meta := ExtractTaskMetadata(ctx, task)

				// Trigger alert ONLY on final attempt before DLQ/Archive or when SkipRetry is returned.
				if meta.Retries >= meta.MaxRetries || errors.Is(err, asynq.SkipRetry) {
					// SendSlackAlert(meta, err)
				}
			}),
		},
	)

	consumer := &ConsumerClient{
		Config: config,
		Client: workerSrv,
		Mux:    asynq.NewServeMux(),
		Logger: logger,
		Tracer: otel.Tracer("task.consumer"),
	}

	// Apply shared OTel tracing + structured retry/DLQ logging across ALL registered task handlers.
	consumer.Mux.Use(consumer.ConsumerMiddleware)

	return consumer, nil
}

func (w *ConsumerClient) Register(pattern string, handler asynq.HandlerFunc) {
	w.Mux.HandleFunc(pattern, handler)
}

func (w *ConsumerClient) Start() error {
	return w.Client.Start(w.Mux)
}

// Stop gracefully shuts down
func (w *ConsumerClient) Shutdown() {
	w.Client.Shutdown()
	w.Logger.Info().Msg("task consumer stopped successfully")
}

// TaskMetadata groups core execution data for logging and alerting
type TaskMetadata struct {
	TaskID     string
	TaskType   string
	Queue      string
	Retries    int
	MaxRetries int
}

func ExtractTaskMetadata(ctx context.Context, task *asynq.Task) TaskMetadata {
	taskID, _ := asynq.GetTaskID(ctx)   // task_a8f9c2...
	taskType := task.Type()             // email:welcome
	queue, _ := asynq.GetQueueName(ctx) // critical, default, low
	retries, _ := asynq.GetRetryCount(ctx)
	maxRetries, _ := asynq.GetMaxRetry(ctx)

	return TaskMetadata{
		TaskID:     taskID,
		TaskType:   taskType,
		Queue:      queue,
		Retries:    retries,
		MaxRetries: maxRetries,
	}
}

func (w *ConsumerClient) ConsumerMiddleware(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		start := time.Now()

		// 1. Extract parent Producer TraceID & SpanID from Asynq task headers.
		if task.Headers() != nil {
			ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(task.Headers()))
		}

		// 2. Start Consumer child span linked to the Producer span.
		ctx, span := w.Tracer.Start(ctx, "task.consumer/"+task.Type(),
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("messaging.system", "asynq"),
				attribute.String("messaging.destination.name", task.Type()),
				attribute.String("messaging.operation", "consumer"),
			),
		)
		defer span.End()

		// 3. Extract standardized Asynq task metadata.
		meta := ExtractTaskMetadata(ctx, task)

		// 4. Attach uniform span attributes.
		span.SetAttributes(
			attribute.String("messaging.message.id", meta.TaskID),
			attribute.String("task.type", meta.TaskType),
			attribute.String("task.queue", meta.Queue),
			attribute.Int("task.attempt", meta.Retries+1),
			attribute.Int("task.max_retries", meta.MaxRetries),
		)

		// 5. Attach trace_id, span_id, and task metadata to a logger and store it in ctx
		// so handlers retrieve it via `zerolog.Ctx(ctx)`.
		baseLog := logging.WithTraceContext(ctx, *w.Logger).With().
			Str("component", "consumer").
			Str("task_id", meta.TaskID).
			Str("task_type", meta.TaskType).
			Str("task_queue", meta.Queue).
			Int("attempt", meta.Retries+1).
			Int("max_retries", meta.MaxRetries).
			Logger()
		ctx = baseLog.WithContext(ctx)

		// 6. Execute the business handler (emits zero logs itself).
		err := next.ProcessTask(ctx, task)
		duration := time.Since(start)

		// 7. Emit ONE canonical summary log for this execution (including handler business fields).
		reqLog := zerolog.Ctx(ctx)
		if err != nil {
			isFinalAttempt := meta.Retries >= meta.MaxRetries || errors.Is(err, asynq.SkipRetry)

			span.SetAttributes(attribute.Bool("task.will_retry", !isFinalAttempt))
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			if isFinalAttempt {
				reqLog.Error().Err(err).Dur("duration", duration).Msg("task permanently failed and moved to DLQ")
			} else {
				reqLog.Warn().Err(err).Dur("duration", duration).Msg("task failed, scheduling retry")
			}

			return err
		}

		span.SetStatus(codes.Ok, "task completed")
		reqLog.Info().Dur("duration", duration).Msg("task completed")
		return nil
	})
}
