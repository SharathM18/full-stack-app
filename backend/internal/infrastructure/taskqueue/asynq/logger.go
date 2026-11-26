package asynq

import (
	"fmt"

	"github.com/rs/zerolog"
)

type AsynqLogger struct {
	log zerolog.Logger
}

func NewAsynqLogger(logger *zerolog.Logger) *AsynqLogger {
	return &AsynqLogger{
		log: logger.With().Str("component", "asynq").Logger(),
	}
}

func (l *AsynqLogger) Debug(args ...any) {
	l.log.Debug().Msg(fmt.Sprint(args...))
}

func (l *AsynqLogger) Info(args ...any) {
	l.log.Info().Msg(fmt.Sprint(args...))
}

func (l *AsynqLogger) Warn(args ...any) {
	l.log.Warn().Msg(fmt.Sprint(args...))
}

func (l *AsynqLogger) Error(args ...any) {
	l.log.Error().Msg(fmt.Sprint(args...))
}

func (l *AsynqLogger) Fatal(args ...any) {
	l.log.Fatal().Msg(fmt.Sprint(args...))
}
