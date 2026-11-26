package apperror

import "net/http"

// AppError represents a domain/business error decoupled from HTTP frameworks.
type AppError struct {
	Status  int
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func BadRequest(code, msg string) *AppError {
	return &AppError{Status: http.StatusBadRequest, Code: code, Message: msg}
}

func Unauthorized(msg string) *AppError {
	return &AppError{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: msg}
}

func Forbidden(msg string) *AppError {
	return &AppError{Status: http.StatusForbidden, Code: "FORBIDDEN", Message: msg}
}

func NotFound(msg string) *AppError {
	return &AppError{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: msg}
}

func Conflict(code, msg string) *AppError {
	return &AppError{Status: http.StatusConflict, Code: code, Message: msg}
} // apperror.Conflict("INSUFFICIENT_STOCK", "requested quantity exceeds available stock")

func Unprocessable(code, msg string) *AppError {
	return &AppError{Status: http.StatusUnprocessableEntity, Code: code, Message: msg}
} // apperror.Unprocessable("COUPON_EXPIRED", "this promo code has expired")

func ServiceUnavailable(msg string) *AppError {
	return &AppError{Status: http.StatusServiceUnavailable, Code: "SERVICE_UNAVAILABLE", Message: msg}
}
