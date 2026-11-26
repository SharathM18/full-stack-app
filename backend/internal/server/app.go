package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/sharathm18/myapp/internal/config"
	"github.com/sharathm18/myapp/internal/infrastructure/cache/redis"
	"github.com/sharathm18/myapp/internal/infrastructure/database/postgres"
	"github.com/sharathm18/myapp/internal/infrastructure/taskqueue/asynq"
)

// Central core dependency Injection container
type APP struct {
	Config     *config.Config
	Logger     *zerolog.Logger
	Database   *postgres.Database
	Redis      *redis.Client
	HTTPServer *http.Server
	TaskQueue  *asynq.ProducerClient
}

func NewAPP(config *config.Config, logger *zerolog.Logger) (*APP, error) {
	// Initialize database
	db, err := postgres.NewClient(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize redis client for cache
	redis, err := redis.NewClient(config, logger)
	if err != nil {
		var shutdownErrors []error

		// Should be in reverse order

		if dbErr := db.Close(); dbErr != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("database close error: %w", dbErr))
		}

		shutdownErrors = append(shutdownErrors, fmt.Errorf("failed to initialize redis: %w", err))

		return nil, errors.Join(shutdownErrors...)
	}

	// Initialize background asynq client (Producer)
	// We only need the client to push tasks to redis. no worker is started here.
	taskQueue, err := asynq.NewProducerClient(config, logger)
	if err != nil {
		var shutdownErrors []error

		// should be in reverse order

		if redisErr := redis.Close(); redisErr != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("redis close error: %w", redisErr))
		}

		if dbErr := db.Close(); dbErr != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("database close error: %w", dbErr))
		}

		shutdownErrors = append(shutdownErrors, fmt.Errorf("failed to initialize task queue: %w", err))

		return nil, errors.Join(shutdownErrors...)
	}

	// Return the constructed server pointer
	return &APP{
		Config:    config,
		Logger:    logger,
		Database:  db,
		Redis:     redis,
		TaskQueue: taskQueue,
	}, nil
}

// Setup the HTTP Server
func (s *APP) SetupHTTPServer(handler http.Handler) {
	s.HTTPServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.Config.AppServer.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       time.Duration(s.Config.AppServer.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(s.Config.AppServer.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(s.Config.AppServer.IdleTimeout) * time.Second,
	}
}

// Start HTTP server
func (s *APP) Start() error {
	s.Logger.Info().
		Int("port", s.Config.AppServer.Port).
		Msg("starting app http server...")

	if err := s.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start app http server: %w", err)
	}

	return nil
}

// Shutdown the HTTP Server, DB, Redis, Taskqueue
func (s *APP) Shutdown(ctx context.Context) error {
	var shutdownErrors []error

	// 1. Stop taking new HTTP requests
	if s.HTTPServer != nil {
		if err := s.HTTPServer.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("app http server shutdown error: %w", err))
		} else {
			s.Logger.Info().Msg("app http server shutdown successfully")
		}
	}

	// 2. Close TaskQueue Producer client
	if s.TaskQueue != nil {
		if err := s.TaskQueue.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("task queue close error: %w", err))
		}
	}

	// 3. Close Redis client connection
	if s.Redis != nil {
		if err := s.Redis.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("redis close error: %w", err))
		}
	}

	// 4. Close PostgreSQL connection pool
	if s.Database != nil {
		if err := s.Database.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("database close error: %w", err))
		}
	}

	// Combine any errors that occurred during shutdown into a single error
	return errors.Join(shutdownErrors...)
}
