package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/sharathm18/myapp/internal/config"
	"github.com/sharathm18/myapp/internal/infrastructure/database/postgres"
	"github.com/sharathm18/myapp/internal/infrastructure/taskqueue/asynq"
	"github.com/sharathm18/myapp/internal/worker/email"
)

// Central core dependency Injection container
type Worker struct {
	Config    *config.Config
	Logger    *zerolog.Logger
	Database  *postgres.Database
	TaskQueue *asynq.ConsumerClient
}

func NewWorker(config *config.Config, logger *zerolog.Logger) (*Worker, error) {
	// 1. Initialize Database
	db, err := postgres.NewClient(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// 2. Initialize background asynq client (Consumer)
	taskQueue, err := asynq.NewConsumerClient(config, logger)
	if err != nil {
		var shutdownErrors []error

		if dbErr := db.Close(); dbErr != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("database close error: %w", dbErr))
		}

		shutdownErrors = append(shutdownErrors, fmt.Errorf("failed to initialize task worker: %w", err))

		return nil, errors.Join(shutdownErrors...)
	}

	// 3. Initialize the task handlers, and passing dependencies via DI
	emailHandler := email.NewEmailHandler(config)

	// 4. Register your task handlers
	taskQueue.Register(email.TypeWelcomeEmail, emailHandler.HandleWelcomeEmailTask) // taskType, taskHandler

	// Return the constructed server pointer
	return &Worker{
		Config:    config,
		Logger:    logger,
		Database:  db,
		TaskQueue: taskQueue,
	}, nil

}

// Start boots up the task consumer to begin pulling tasks from asynq
func (s *Worker) Start() error {
	s.Logger.Info().Msg("starting task background worker...")
	return s.TaskQueue.Start()
}

// Shutdown gracefully stops the Task worker and database
func (s *Worker) Shutdown(ctx context.Context) error {
	var shutdownErrors []error

	// 1. Stop taking new tasks and wait for active ones to finish
	if s.TaskQueue != nil {
		s.TaskQueue.Shutdown()
	}

	// 2. Close PostgreSQL connection pool
	if s.Database != nil {
		if err := s.Database.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("database close error: %w", err))
		}
	}

	return errors.Join(shutdownErrors...)
}
