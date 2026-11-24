package auth

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sharathm18/myapp/internal/infrastructure/database/postgres"
	"github.com/sharathm18/myapp/internal/server"
)

type AuthRepository struct {
	AppServer *server.APP
}

func NewAuthRepository(appServer *server.APP) *AuthRepository {
	return &AuthRepository{
		AppServer: appServer,
	}
}

func (r *AuthRepository) CreateSession(ctx context.Context, params *CreateSessionParams) (*UserSessionModel, error) {
	// build
	query, args, err := postgres.Psql.Insert("user_sessions").Columns("user_id",
		"refresh_token_hash",
		"user_agent",
		"ip_address",
		"expires_at").Values(
		params.UserID, params.RefreshTokenHash, params.UserAgent, params.IPAddress, params.ExpiresAt,
	).Suffix("RETURNING *").ToSql()

	if err != nil {
		return nil, fmt.Errorf("auth repo get by email: %w", err)
	}

	// execute
	rows, err := r.AppServer.Database.Executor(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("auth repo get by email: %w", err)
	}

	// map return columns
	session, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[UserSessionModel])
	if err != nil {
		return nil, fmt.Errorf("auth repo get by email: %w", err)
	}

	return &session, nil
}
