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

pending
err
sqlerr
validator

|                                                                           |                                                         |
| ------------------------------------------------------------------------- | ------------------------------------------------------- |
| `curl -i http://localhost:9090/health/live`                               | Liveness Probe (Checks if the Go binary is running)     |
| `curl -i http://localhost:9090/health/ready`                              | Readiness Probe (Checks Postgres and Redis connections) |
| `curl -s http://localhost:9090/info`                                      | Build Info                                              |
| `curl -u admin:secret -s http://localhost:9090/admin/metrics `            | Prometheus Metrics Scraper                              |
| `curl -u admin:secret -s http://localhost:9090/admin/asynqmon/api/queues` | Asynq Dashboard (API Check)                             |
| `curl -i -u admin:secret http://localhost:9090/admin/debug/pprof/`        | Go Profiler (pprof)                                     |

## Notes

- If struct is used/ passed to other package then use capitalise otherwise small case is better
- Only one error and success log should be there in the entire business req. and also upload to otel
  - Use `fmt.Errrof('Error is %w', err)
- Go with an Aggregator Domain (Direct SQL Joins) when:
  - It's a Bulk Read (List/Search/Export): Endpoints returning arrays of data (pagination, dashboards, search filters) that require columns from multiple domain tables.
  - It's Strictly Read-Only: You just need to format and ship heavy data to the frontend as fast as possible.
- Go with Interfaces (Service-to-Service) when:
  - It's a Write (Mutation): Any action that creates, updates, or deletes data (e.g., adding a product and checking if a category exists).
  - It's a Single-Record Check: Validating one ID, checking a permission, or enriching a single user's profile.

API Endpoints

2. create user
3. update user
4. delete user
5. list user

6. create category
7. update category
8. delete category
9. list category

10. create product
11. update product
12. delete user
13. list user with pagination and filters

14. search products with filter of multiple domains fields

# Go & Observability Engineering Notes

## 1. Visibility & Error Handling

- **Package Visibility:** Capitalize types/functions only when another package needs them; keep internal helpers lowercase. _(Exception: Struct fields used by `json`, `msgpack`, or `validator` must always be capitalized)._
- **Wrap & Bubble Up:** Never log and return the same error. Wrap errors with lowercase context using `fmt.Errorf("action failed: %w", err)` and return them to the root caller.
- **Single Canonical Log:** Emit **one** structured log (success or error) per business request/job at the root boundary (middleware/worker wrapper). Inner layers return `(data, nil)` silently.

## 2. Modular Monolith Domain Boundaries

- **Use Interfaces (Service-to-Service) when:**
  - **Mutating data (Writes):** Creating, updating, or deleting across domains (e.g., `products` asking `categories` to `GetOrCreateByName`).
  - **Single-record checks:** Validating a `user_id` or checking permissions. Define the interface in the _consumer_ domain.
- **Use an Aggregator Domain (Direct SQLC `JOIN`s) when:**
  - **Bulk Reads:** Paginated lists, search filters, dashboards, or exports spanning multiple tables.
  - **Strictly Read-Only:** Never perform `INSERT`, `UPDATE`, or `DELETE` inside an aggregator.

## 3. OpenTelemetry (OTel) & Logging

1. **Correlate Logs & Traces:** Extract `trace_id` and `span_id` from `ctx` in the entry middleware and attach them to the request-scoped Zerolog logger.
2. **Trace Propagation:** Use `tracer.Start(ctx, "span_name")` for in-process child spans. For async jobs (Asynq Producer -> Consumer), explicitly inject trace context into the task payload and extract it in the worker to link parent/child spans.
3. **Record Span Errors Properly:** Whenever a span fails, call both:
   - `span.RecordError(err)`
   - `span.SetStatus(codes.Error, err.Error())`
4. **Enrich Spans with Attributes:** Attach safe debugging metadata (e.g., `user.id`, `product.id`, `db.rows_affected`) via `span.SetAttributes(...)`—never log secrets or PII.
