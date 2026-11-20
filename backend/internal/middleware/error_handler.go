package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel/trace"
)

// ErrorResponse defines the single uniform JSON envelope returned for every API error
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail holds machine-readable error metadata for frontend and API clients
type ErrorDetail struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id,omitempty"`
	TraceID   string            `json:"trace_id,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// HTTPErrorHandler is Echo v5's central error-to-JSON translator.
// Note: Logging is intentionally omitted here because SummaryLogger already logs
// every request's outcome (both errors and successes) in one canonical entry.
func HTTPErrorHandler(c *echo.Context, err error) {
	if err == nil {
		return
	}

	// guard against duplicate writes if an earlier handler already sent a response
	if resp, uErr := echo.UnwrapResponse(c.Response()); uErr == nil && resp.Committed {
		return
	}

	// default to a safe 500 response so unhandled SQL or runtime errors never leak internals
	ctx := c.Request().Context()
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

	var valErrs validator.ValidationErrors
	var httpErr *echo.HTTPError
	var pgErr *pgconn.PgError

	switch {
	// 1. request payload/query validation failures -> 400 Bad Request with field breakdown
	case errors.As(err, &valErrs):
		status = http.StatusBadRequest
		fields := make(map[string]string, len(valErrs))
		for _, fe := range valErrs {
			fields[fe.Field()] = fmt.Sprintf("failed on '%s' validation", fe.Tag())
		}
		detail.Code = "VALIDATION_ERROR"
		detail.Message = "validation failed"
		detail.Fields = fields

	// 2. echo framework & middleware errors (e.g., 401, 403, 404 route not found, 413, 429)
	case echo.StatusCode(err) != 0:
		status = echo.StatusCode(err)
		detail.Code = httpStatusToCode(status)
		if errors.As(err, &httpErr) && httpErr.Message != "" {
			detail.Message = httpErr.Message
		} else {
			detail.Message = err.Error()
		}

	// 3. empty postgreSQL query result -> 404 Not Found
	case errors.Is(err, pgx.ErrNoRows):
		status = http.StatusNotFound
		detail.Code = "NOT_FOUND"
		detail.Message = "resource not found"

	// 4. postgreSQL SQLSTATE constraint and data format errors
	case errors.As(err, &pgErr):
		switch pgErr.Code {
		case "23505": // unique_violation (e.g., duplicate email or SKU)
			status = http.StatusConflict
			detail.Code = "CONFLICT"
			detail.Message = "resource already exists"
		case "23503": // foreign_key_violation (e.g., invalid parent/reference ID)
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
		case "22P02": // invalid_text_representation (e.g., malformed UUID syntax)
			status = http.StatusBadRequest
			detail.Code = "INVALID_FORMAT"
			detail.Message = "invalid identifier format"
			// ... add more PostgreSQL SQLSTATE codes here (e.g., "40001" serialization_failure) as needed
		}

		// ... add future sentinel error checks (e.g., errors.Is(err, context.DeadlineExceeded))
		// or third-party SDK error mappings (e.g., Redis, S3, payment gateway) here
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
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	default:
		if status >= 500 {
			return "INTERNAL_ERROR"
		}
		return "REQUEST_ERROR"
	}
}
