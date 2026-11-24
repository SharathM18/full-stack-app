package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/sharathm18/myapp/internal/config"
)

type Client struct {
	Client *redis.Client
	Logger *zerolog.Logger
	Config *config.Config
}

const RedisPingTimeout = 15 * time.Second

// New initializes a Redis connection, attaches OpenTelemetry tracing, and verifies the connection.
func NewClient(config *config.Config, logger *zerolog.Logger) (*Client, error) {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisCache.Address,
		Password: config.RedisCache.Password,
		DB:       config.RedisCache.Database,

		// Connection Pool Settings
		PoolSize:     config.RedisCache.PoolSize,     // Max active connections (default is 10 connections per CPU)
		MinIdleConns: config.RedisCache.MinIdleConns, // Keep at least 2 connections open to avoid latency on sudden spikes
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second, // Max time to wait if all connections are busy
	})

	// Attach OpenTelemetry Hook ONLY if tracing is enabled in the configuration
	if config.Observability.OTel.Enabled && config.Observability.OTel.TracesEnabled {
		if err := redisotel.InstrumentTracing(redisClient); err != nil {
			return nil, fmt.Errorf("failed to instrument redis with OpenTelemetry: %w", err)
		}
	}

	// Test Redis connection with a strict timeout
	ctx, cancel := context.WithTimeout(context.Background(), RedisPingTimeout)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info().Str("address", config.RedisCache.Address).
		Int("DB", config.RedisCache.Database).
		Msg("successfully connected to Redis")

	return &Client{
		Client: redisClient,
		Logger: logger,
		Config: config,
	}, nil
}

// Close Redis client connection
func (c *Client) Close() error {
	if err := c.Client.Close(); err != nil {
		return fmt.Errorf("failed to close redis connection: %w", err)
	}

	c.Logger.Info().Str("address", c.Config.RedisCache.Address).
		Int("DB", c.Config.RedisCache.Database).
		Msg("redis client connection closed successfully")

	return nil
}
