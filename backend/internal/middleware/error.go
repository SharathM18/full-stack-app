package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v5"
	"github.com/sharathm18/myapp/internal/domain/shared/apperror"
	v "github.com/sharathm18/myapp/internal/domain/shared/validator"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ErrorResponse defines the single uniform JSON envelope returned for every API error.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail holds machine-readable error metadata for frontend and API clients.
type ErrorDetail struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id,omitempty"`
	TraceID   string            `json:"trace_id,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// resolveError is the single source of truth that translates any Go error into an HTTP status
// code and structured ErrorDetail. Shared by HTTPErrorHandler and SummaryLoggerMiddleware.
func resolveError(ctx context.Context, err error) (int, ErrorDetail) {
	var traceID string
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		traceID = sc.TraceID().String()
	}

	status := http.StatusInternalServerError
	detail := ErrorDetail{
		Code:      "INTERNAL_ERROR",
		Message:   "internal server error",
		RequestID: GetRequestID(ctx),
		TraceID:   traceID,
	}

	var appErr *apperror.AppError
	var valErrs validator.ValidationErrors
	var httpErr *echo.HTTPError
	var pgErr *pgconn.PgError

	switch {
	// 1. Domain/business errors returned from services
	case errors.As(err, &appErr):
		status = appErr.Status
		detail.Code = appErr.Code
		detail.Message = appErr.Message

	// 2. DTO struct validation failures -> 400 Bad Request with field breakdown
	case errors.As(err, &valErrs):
		status = http.StatusBadRequest
		fields := make(map[string]string, len(valErrs))
		for _, fe := range valErrs {
			fields[fe.Field()] = v.FormatFieldError(fe)
		}
		detail.Code = "VALIDATION_ERROR"
		detail.Message = "validation failed"
		detail.Fields = fields

	// 3. Echo framework & middleware errors (401, 403, 404 route not found, 405, 413, 415, 429)
	case echo.StatusCode(err) != 0:
		status = echo.StatusCode(err)
		detail.Code = httpStatusToCode(status)
		if errors.As(err, &httpErr) && httpErr.Message != "" {
			detail.Message = httpErr.Message
		} else {
			detail.Message = err.Error()
		}

	// 4. Request timeouts & client disconnects
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
		detail.Code = "GATEWAY_TIMEOUT"
		detail.Message = "request timed out"

	case errors.Is(err, context.Canceled):
		status = 499 // Client Closed Request (nginx/industry convention; logged as 4xx WRN, not 500 ERR)
		detail.Code = "CLIENT_CLOSED_REQUEST"
		detail.Message = "request canceled by client"

	// 5. Empty PostgreSQL query result -> 404 Not Found
	case errors.Is(err, pgx.ErrNoRows):
		status = http.StatusNotFound
		detail.Code = "NOT_FOUND"
		detail.Message = "resource not found"

	// 6. PostgreSQL SQLSTATE constraint, format, and concurrency errors
	case errors.As(err, &pgErr):
		switch pgErr.Code {
		case "23505": // unique_violation
			status = http.StatusConflict
			detail.Code = "CONFLICT"
			detail.Message = "resource already exists"
		case "23503": // foreign_key_violation
			status = http.StatusBadRequest
			detail.Code = "INVALID_REFERENCE"
			detail.Message = "referenced resource does not exist"
		case "23502": // not_null_violation
			status = http.StatusBadRequest
			detail.Code = "MISSING_FIELD"
			detail.Message = "required database field is missing"
		case "23514": // check_violation
			status = http.StatusBadRequest
			detail.Code = "CONSTRAINT_VIOLATION"
			detail.Message = "value violates database constraint"
		case "22P02": // invalid_text_representation (malformed UUID/enum)
			status = http.StatusBadRequest
			detail.Code = "INVALID_FORMAT"
			detail.Message = "invalid identifier format"
		case "22001": // string_data_right_truncation (exceeds VARCHAR(N))
			status = http.StatusBadRequest
			detail.Code = "VALUE_TOO_LONG"
			detail.Message = "input value exceeds maximum allowed length"
		case "22003": // numeric_value_out_of_range
			status = http.StatusBadRequest
			detail.Code = "NUMERIC_OUT_OF_RANGE"
			detail.Message = "numeric value is out of allowed range"
		case "40001", "40P01": // serialization_failure / deadlock_detected
			status = http.StatusConflict
			detail.Code = "CONCURRENCY_CONFLICT"
			detail.Message = "concurrent update conflict, please retry"
		case "57014": // query_canceled (PostgreSQL statement_timeout)
			status = http.StatusGatewayTimeout
			detail.Code = "QUERY_TIMEOUT"
			detail.Message = "database operation timed out"
		}
	}

	return status, detail
}

// HTTPErrorHandler is Echo v5's central error-to-JSON translator.
// Logging is omitted here because SummaryLoggerMiddleware emits one canonical log entry per request.
func HTTPErrorHandler(c *echo.Context, err error) {
	if err == nil {
		return
	}

	// Guard against duplicate writes if an earlier handler already sent a response.
	if resp, uErr := echo.UnwrapResponse(c.Response()); uErr == nil && resp.Committed {
		return
	}

	ctx := c.Request().Context()
	status, detail := resolveError(ctx, err)

	// Mark the active OTel span as failed only for 5xx server errors.
	// 4xx client errors (validation, unauthorized, not found) are normal client outcomes.
	if status >= http.StatusInternalServerError {
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}

	_ = c.JSON(status, ErrorResponse{Error: detail})
}

// httpStatusToCode maps standard HTTP status codes to consistent uppercase error codes.
func httpStatusToCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusMethodNotAllowed:
		return "METHOD_NOT_ALLOWED"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusRequestEntityTooLarge:
		return "PAYLOAD_TOO_LARGE"
	case http.StatusUnsupportedMediaType:
		return "UNSUPPORTED_MEDIA_TYPE"
	case http.StatusUnprocessableEntity:
		return "UNPROCESSABLE_ENTITY"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case http.StatusBadGateway:
		return "BAD_GATEWAY"
	case http.StatusServiceUnavailable:
		return "SERVICE_UNAVAILABLE"
	case http.StatusGatewayTimeout:
		return "GATEWAY_TIMEOUT"
	default:
		if status >= 500 {
			return "INTERNAL_ERROR"
		}
		return "REQUEST_ERROR"
	}
}

// ResolveHTTPStatus exposes resolveError's HTTP status mapping to routers and loggers.
func ResolveHTTPStatus(ctx context.Context, err error) int {
	status, _ := resolveError(ctx, err)
	return status
}
