package asynq

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/sharathm18/myapp/internal/config"
	"github.com/sharathm18/myapp/internal/infrastructure/observability/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type ProducerClient struct {
	Client *asynq.Client
	Config *config.Config
	Logger *zerolog.Logger
	Tracer trace.Tracer
}

const TaskQueuePingTimeout = 15 * time.Second

// NewProducerClient sets up the Asynq client that pushes tasks to Redis.
func NewProducerClient(config *config.Config, logger *zerolog.Logger) (*ProducerClient, error) {
	redisOpt := asynq.RedisClientOpt{
		Addr:     config.RedisAsynq.Address,
		Password: config.RedisAsynq.Password,
		DB:       config.RedisAsynq.Database,
	}

	rdb, ok := redisOpt.MakeRedisClient().(redis.UniversalClient)
	if !ok {
		return nil, fmt.Errorf("could not create Redis client for %s", config.RedisAsynq.Address)
	}
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), TaskQueuePingTimeout)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("could not connect to Redis at %s: %w", config.RedisAsynq.Address, err)
	}

	client := asynq.NewClient(redisOpt)

	logger.Info().
		Str("address", config.RedisAsynq.Address).
		Int("DB", config.RedisAsynq.Database).
		Msg("successfully connected task producer")

	return &ProducerClient{
		Config: config,
		Logger: logger,
		Client: client,
		Tracer: otel.Tracer("task.producer"),
	}, nil
}

// Close gracefully disconnects from the Redis broker connection pool.
func (c *ProducerClient) Close() error {
	if err := c.Client.Close(); err != nil {
		return fmt.Errorf("failed to close task client: %w", err)
	}

	c.Logger.Info().
		Str("address", c.Config.RedisAsynq.Address).
		Int("DB", c.Config.RedisAsynq.Database).
		Msg("task client closed successfully")

	return nil
}

// EnqueueTask links the producer OTel span into task headers and publishes the task to Redis.
func (c *ProducerClient) EnqueueTask(ctx context.Context, task *asynq.Task) (*asynq.TaskInfo, error) {
	// 1. This automatically creates and link Trace ID, Parent Span ID, and Child Span ID for producer.
	ctx, span := c.Tracer.Start(ctx, "task.producer/"+task.Type(),
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "asynq"),
			attribute.String("messaging.destination.name", task.Type()),
			attribute.String("messaging.operation", "producer"),
		),
	)
	defer span.End()

	// 2. Inject the active Producer SpanContext into Asynq headers.
	if task.Headers() != nil {
		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(task.Headers()))
	}

	// 3. Push task to Redis.
	info, err := c.Client.EnqueueContext(ctx, task)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("redis enqueue failed for task %s: %w", task.Type(), err)
	}

	// 4. Attach consistent Asynq attributes to the span.
	span.SetAttributes(
		attribute.String("messaging.message.id", info.ID), // task_a8f9c2...
		attribute.String("task.type", task.Type()),        // email:welcome
		attribute.String("task.queue", info.Queue),        // critical, default, low
	)

	// 5. Emit canonical producer log with identical keys as the consumer.
	log := logging.WithTraceContext(ctx, *c.Logger)
	log.Info().
		Str("component", "producer").
		Str("task_id", info.ID).       // task_a8f9c2...
		Str("task_type", task.Type()). // email:welcome
		Str("task_queue", info.Queue). // critical, default, low
		Msg("task enqueued successfully")

	return info, nil
}
