package user

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sharathm18/myapp/internal/infrastructure/database/postgres"
	"github.com/sharathm18/myapp/internal/server"
)

type UserRepository struct {
	AppServer *server.APP
}

func NewUserRepository(appServer *server.APP) *UserRepository {
	return &UserRepository{
		AppServer: appServer,
	}
}

// GetByEmail fetches a single user by their normalized lowercase email address.
// Returns a wrapped pgx.ErrNoRows when no matching user exists.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*UserModel, error) {
	// Step 1: Build `SELECT * FROM users WHERE email = $1`
	query, args, err := postgres.Psql.
		Select("*").
		From("users").
		Where(sq.Eq{"email": email}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("user repo build get by email query: %w", err)
	}

	// Step 2: Run query on the active executor (pool or transaction).
	rows, err := r.AppServer.Database.Executor(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("user repo execute get by email query: %w", err)
	}

	// Step 3: CollectOneRow automatically closes `rows` and maps columns via `db:"..."` tags.
	// Wrapping with %w preserves pgx.ErrNoRows for `errors.Is(err, pgx.ErrNoRows)` in AuthService.
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[UserModel])
	if err != nil {
		return nil, fmt.Errorf("user repo scan user by email: %w", err)
	}

	return &u, nil
}

// GetByID fetches a single user by their UUID primary key.
// Returns a wrapped pgx.ErrNoRows when no matching user exists.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*UserModel, error) {
	// Step 1: Build `SELECT * FROM users WHERE id = $1`
	query, args, err := postgres.Psql.
		Select("*").
		From("users").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("user repo build get by id query: %w", err)
	}

	// Step 2: Run query on the active executor (pool or transaction).
	rows, err := r.AppServer.Database.Executor(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("user repo execute get by id query: %w", err)
	}

	// Step 3: Map row into UserModel via `db:"..."` tags.
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[UserModel])
	if err != nil {
		return nil, fmt.Errorf("user repo scan user by id: %w", err)
	}

	return &u, nil
}

// CreatePasswordUser inserts a new user registered via Name + Email + Password.
// Database defaults populate `id`, `role`, `is_active`, `created_at`, and `updated_at`,
// and `RETURNING *` brings the complete created UserModel back in one query.
func (r *UserRepository) CreatePasswordUser(ctx context.Context, name, email, passwordHash string) (*UserModel, error) {
	// Step 1: Build parameterized INSERT and return all columns of the new row.
	query, args, err := postgres.Psql.
		Insert("users").
		Columns("name", "email", "password_hash").
		Values(name, email, passwordHash).
		Suffix("RETURNING *").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("user repo build create password user query: %w", err)
	}

	// Step 2: Execute via Executor(ctx) so Signup's RunInTx shares the same connection with CreateSession.
	rows, err := r.AppServer.Database.Executor(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("user repo execute create password user query: %w", err)
	}

	// Step 3: Map the returned row directly into UserModel.
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[UserModel])
	if err != nil {
		return nil, fmt.Errorf("user repo scan created user: %w", err)
	}

	return &u, nil
}
