package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())

	// Report JSON field names (e.g., "phoneNumber") instead of Go struct names ("PhoneNumber").
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})

	return v
}

// BindAndValidate binds JSON/form/query/param data into T and validates struct tags.
func BindAndValidate[T any](c *echo.Context) (T, error) {
	var payload T

	if err := c.Bind(&payload); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			return payload, echo.NewHTTPError(
				http.StatusBadRequest,
				fmt.Sprintf("invalid data type for field '%s'", typeErr.Field),
			)
		}
		return payload, echo.NewHTTPError(http.StatusBadRequest, "invalid or malformed request payload")
	}

	if err := validate.Struct(payload); err != nil {
		return payload, err
	}

	return payload, nil
}

// ParseUUIDParam extracts and validates a UUID path parameter (e.g., /user/:id).
func ParseUUIDParam(c *echo.Context, paramName string) (uuid.UUID, error) {
	raw := c.Param(paramName)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, echo.NewHTTPError(
			http.StatusBadRequest,
			fmt.Sprintf("invalid UUID format for parameter '%s'", paramName),
		)
	}
	return id, nil
}

// FormatFieldError maps validator.FieldError tags to clean human-readable messages.
func FormatFieldError(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "required_if", "required_unless", "required_with", "required_without":
		return "is required for this request"
	case "email":
		return "must be a valid email address"
	case "url", "uri":
		return "must be a valid URL"
	case "uuid", "uuid4":
		return "must be a valid UUID"
	case "e164":
		return "must be a valid phone number in E.164 format (e.g., +919876543210)"
	case "datetime":
		return fmt.Sprintf("must match datetime format %s", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", fe.Param())
	case "eqfield":
		return fmt.Sprintf("must match %s", fe.Param())
	case "alpha":
		return "must contain only letters"
	case "alphanum":
		return "must contain only letters and numbers"
	case "numeric":
		return "must be a valid numeric value"
	case "len":
		if fe.Type().Kind() == reflect.String {
			return fmt.Sprintf("must be exactly %s characters", fe.Param())
		}
		return fmt.Sprintf("must contain exactly %s items", fe.Param())
	case "min":
		if fe.Type().Kind() == reflect.String {
			return fmt.Sprintf("must be at least %s characters", fe.Param())
		}
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		if fe.Type().Kind() == reflect.String {
			return fmt.Sprintf("must not exceed %s characters", fe.Param())
		}
		return fmt.Sprintf("must not exceed %s", fe.Param())
	case "gte":
		return fmt.Sprintf("must be greater than or equal to %s", fe.Param())
	case "lte":
		return fmt.Sprintf("must be less than or equal to %s", fe.Param())
	case "gt":
		return fmt.Sprintf("must be greater than %s", fe.Param())
	case "lt":
		return fmt.Sprintf("must be less than %s", fe.Param())
	case "dive":
		return "contains invalid items"
	default:
		if fe.Param() != "" {
			return fmt.Sprintf("failed on '%s:%s' validation", fe.Tag(), fe.Param())
		}
		return fmt.Sprintf("failed on '%s' validation", fe.Tag())
	}
}
