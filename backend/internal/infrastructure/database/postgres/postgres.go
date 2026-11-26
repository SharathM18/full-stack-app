package postgres

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/sharathm18/myapp/internal/config"
)

// Psql is the shared Squirrel builder configured for PostgreSQL ($1, $2, ...) placeholders
var Psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// DBTX represents a database executor. Both *pgxpool.Pool (normal queries)
// and pgx.Tx (single locked transaction connection) satisfy this interface
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txContextKey struct{}

// database wraps the PostgreSQL connection pool and logger
type Database struct {
	Pool   *pgxpool.Pool
	Logger *zerolog.Logger
	Config *config.Config
}

const DatabasePingTimeout = 10 * time.Second

// NewClient is a constructor that sets up a PostgreSQL connection pool with connection pooling limits
func NewClient(config *config.Config, logger *zerolog.Logger) (*Database, error) {
	// 1. combine host and port safely using net.JoinHostPort
	hostPort := net.JoinHostPort(config.Database.Host, strconv.Itoa(config.Database.Port))

	// 2. URL-encode the password to prevent crashing if the password contains special characters (like '@', '#', etc.)
	encodedPassword := url.QueryEscape(config.Database.Password)
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
		config.Database.User,
		encodedPassword,
		hostPort,
		config.Database.Name,
		config.Database.SSLMode,
	)

	// 3. parse DSN into pgxpool configuration struct
	pgxPoolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pgx pool config: %w", err)
	}

	// 4. map connection pool sizing configurations from your config files
	pgxPoolConfig.MaxConns = int32(config.Database.MaxOpenConns)
	pgxPoolConfig.MinConns = int32(config.Database.MaxIdleConns)
	pgxPoolConfig.MaxConnLifetime = config.Database.ConnMaxLifetime
	pgxPoolConfig.MaxConnIdleTime = config.Database.ConnMaxIdleTime

	// 5. attach OpenTelemetry Tracer
	// This silently handles all Trace ID linking, Span creation, and error recording.
	if config.Observability.OTel.Enabled && config.Observability.OTel.TracesEnabled {
		pgxPoolConfig.ConnConfig.Tracer = otelpgx.NewTracer()
	}

	// 6. initialize the connection pool
	ctx, cancel := context.WithTimeout(context.Background(), DatabasePingTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, pgxPoolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgx pool: %w", err)
	}

	// 7. register Metrics (Pool Health)
	if config.Observability.OTel.Enabled && config.Observability.OTel.MetricsEnabled {
		if err := otelpgx.RecordStats(pool); err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to record database pool stats: %w", err)
		}
	}

	// 8. verify Connection
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info().
		Str("host", config.Database.Host).
		Str("database", config.Database.Name).
		Msg("successfully connected to postgreSQL database")

	return &Database{
		Pool:   pool,
		Logger: logger,
		Config: config,
	}, nil
}

// Executor inspects ctx for an active transaction:
//   - Inside RunInTx: returns the single locked transaction connection (pgx.Tx)
//   - Normal request: falls back to db.Pool (uses any available pooled connection)
func (db *Database) Executor(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return db.Pool
}

// RunInTx executes multiple queries atomically on ONE connection:
//  1. Reserves a single connection from db.Pool and issues BEGIN
//  2. Stores that connection inside txCtx so every repository call using txCtx shares it
//  3. Automatically issues ROLLBACK if any step returns an error or panics
//  4. Issues COMMIT and releases the connection back to db.Pool if fn returns nil
func (db *Database) RunInTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin postgres tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	txCtx := context.WithValue(ctx, txContextKey{}, tx)
	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			// db.Logger.Error().Err(rbErr).Msg("failed to rollback PostgreSQL transaction")
			return fmt.Errorf("tx error: %w (rollback failed: %v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		// db.Logger.Error().Err(err).Msg("failed to commit PostgreSQL transaction")
		return fmt.Errorf("commit postgres tx: %w", err)
	}

	return nil
}

// Close gracefully shuts down the database connection pool
func (db *Database) Close() error {
	db.Pool.Close()
	db.Logger.Info().
		Str("host", db.Config.Database.Host).
		Str("database", db.Config.Database.Name).
		Msg("database connection closed successfully")
	return nil
}
