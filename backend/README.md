### Main components of Backend

- Config management
- Application server startup and shutdown gracefully
- PostgresSQL DB with Connection pool, index, and migrations
- Redis cache (Serialization using MessagePack (msgpack))
- OpenTelemetry with trace, metrics, or logs collector
- Logging
- Asynq for background tasksqueue and Asynqmon for monitoring
- Apache Kafka for distributed event streaming
- Agents/ deamon to collects the logs or metrics

## Configuration

Configuration is managed through environment variables with the `MYAPP_` prefix:

## Development

### Install and Verify the Taskfile locally

```bash
brew install go-task/tap/go-task
task --version
```

### Available Tasks

```bash
$ task help

task: Available tasks for this project:
* build:                    Build both App and Worker binaries
* build-app:                Build the APP binary
* build-worker:             Build the Worker binary
* help:                     Display available tasks
* run-app:                  Run the APP Server
* run-worker:               Run the Worker Server
* test:                     Run all tests
* tidy:                     Format all .go files, and Tidy and verify Go module dependencies
* migrations:down:          Rollback the last migration
* migrations:new:           Create a new database migration file
* migrations:up:            Apply all pending up database migrations
* migrations:version:       Check current database migration version
```

### Go Commands

| Commands                                      | Description                                                                                            |
| --------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| `go mod init github.com/<your-account>/myapp` | Generates the go.mod                                                                                   |
| `go mod tidy`                                 | Downloads missing dependencies and cleans up unused entries in go.mod and go.sum                       |
| `go fmt ./...`                                | Automatically formats all Go files in your project according to official Go style guidelines.          |
| `go vet ./...`                                | Examines your code and reports suspicious constructs (like printf formatting mistakes or mutex locks). |
| `go run cmd/api/main.go`                      | Compiles your code into a temporary directory in memory and executes it immediately.                   |
| `go build -o bin/api cmd/api/main.go`         | Compiles your source code into a standalone executable binary saved at bin/api.                        |
| `go clean -modcache`                          | Delete all Go modules downloaded on your entire machine                                                |

## API Endpoints

### System Server API Endpoints

|                                                                           |                                                         |
| ------------------------------------------------------------------------- | ------------------------------------------------------- |
| `curl -i http://localhost:9090/health/live`                               | Liveness Probe (Checks if the Go binary is running)     |
| `curl -i http://localhost:9090/health/ready`                              | Readiness Probe (Checks Postgres and Redis connections) |
| `curl -s http://localhost:9090/info`                                      | Build Info                                              |
| `curl -u admin:secret -s http://localhost:9090/admin/metrics `            | Prometheus Metrics Scraper                              |
| `curl -u admin:secret -s http://localhost:9090/admin/asynqmon/api/queues` | Asynq Dashboard (API Check)                             |
| `curl -i -u admin:secret http://localhost:9090/admin/debug/pprof/`        | Go Profiler (pprof)                                     |

### App Server API Endpoints

|     |     |
| --- | --- |

## Docs

### Architecture: Pragmatic Modular Monolith

Today, domains (`user`, `auth`, `product`, etc.) live in isolated packages under `internal/domain/` and share a single PostgreSQL database:

- **Strict Table Ownership:** Each domain only queries its own tables (`user` owns `users`; `auth` owns `user_sessions`).
- **Consumer-Defined Interfaces:** Cross-domain calls go through interfaces defined by the caller (e.g., `UserRepository` inside `internal/domain/auth/service.go`).
- **Zero Mapping Boilerplate:** While in a single repo, `auth` references `*user.UserModel` directly instead of maintaining duplicate adapter structs.
- **Use an Aggregator Domain:** When
  - **Bulk Reads:** Paginated lists, search filters, dashboards, or exports spanning multiple tables.
  - **Strictly Read-Only:** Never perform `INSERT`, `UPDATE`, or `DELETE` inside an aggregator.

### Maintaining Error Consistency Across Layers

| Layer                        | Pattern                                            | Example                                                                                 |
| ---------------------------- | -------------------------------------------------- | --------------------------------------------------------------------------------------- |
| Handler (Input)              | `validator.BindAndValidate[T](c) / ParseUUIDParam` | `req, err := validator.BindAndValidate[SignupRequest](c)`                               |
| Handler (Return)             | Return `err` directly (no wrapping)                | `return err`                                                                            |
| Service (4xx Business Error) | `apperror.<Constructor>(...)`                      | `return apperror.Conflict("EMAIL_EXISTS", "an account with this email already exists")` |
| Service (5xx Internal Error) | `fmt.Errorf("<domain> service <method>: %w", err)` | `return fmt.Errorf("auth service hash password: %w", err)`                              |
| Repository                   | `fmt.Errorf("<domain> repo <method>: %w", err)`    | `return fmt.Errorf("user repo get by email: %w", err)`                                  |

Note:

- Need a custom business error in a new domain: You usually don't even need to edit apperror—just call a`pperror.BadRequest("INVALID_COUPON", "...")` or `apperror.Conflict("OUT_OF_STOCK", "...")` directly in your service. Only open `internal/domain/shared/apperror/error.go` if you need a new HTTP status constructor (like TooManyRequests or PaymentRequired).
- Need to map a new PostgreSQL SQLSTATE or third-party SDK error (Redis, S3, Razorpay/Stripe): Open `internal/middleware/error.go` and add one case inside resolveError(). Both HTTPErrorHandler and SummaryLoggerMiddleware use resolveError(), so the JSON response and log severity (WRN vs ERR) update together automatically.
- Need a custom validation message for a new struct tag: Open `internal/domain/shared/validator/validator.go` and add a case inside FormatFieldError().

### Logger Strategy: Summary Middleware

- Do not put logger.Error() inside repositories, services, or handlers.
- Return err to a `SummaryRequestLoggerMiddleware` (known as the Canonical Log Line or Wide Event pattern) is the cleanest production approach
  - Why One Summary Log Per Request Is Better? -> Zero duplicate noise, Complete context in one JSON object, Clean separation of severity

### OpenTelemetry span.RecordError(err) & span.SetStatus(...)

- Do not write those two lines after every if err != nil in your repositories or services
- Only use `span.RecordError` and `span.SetStatus` when you manually create a custom child span (ctx, span := tracer.Start(ctx, "send-otp-sms")) around a critical external dependency (like an SMS gateway or payment provider) that lacks automatic OTel instrumentation

```
span.RecordError(err)
span.SetStatus(codes.Error, err.Error())
```
