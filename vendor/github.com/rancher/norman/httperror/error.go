package httperror

import (
	"fmt"
)

var (
	Unauthorized     = ErrorCode{"Unauthorized", 401}
	PermissionDenied = ErrorCode{"PermissionDenied", 403}
	NotFound         = ErrorCode{"NotFound", 404}
	MethodNotAllowed = ErrorCode{"MethodNotAllow", 405}

	InvalidDateFormat  = ErrorCode{"InvalidDateFormat", 422}
	InvalidFormat      = ErrorCode{"InvalidFormat", 422}
	InvalidReference   = ErrorCode{"InvalidReference", 422}
	NotNullable        = ErrorCode{"NotNullable", 422}
	NotUnique          = ErrorCode{"NotUnique", 422}
	MinLimitExceeded   = ErrorCode{"MinLimitExceeded", 422}
	MaxLimitExceeded   = ErrorCode{"MaxLimitExceeded", 422}
	MinLengthExceeded  = ErrorCode{"MinLengthExceeded", 422}
	MaxLengthExceeded  = ErrorCode{"MaxLengthExceeded", 422}
	InvalidOption      = ErrorCode{"InvalidOption", 422}
	InvalidCharacters  = ErrorCode{"InvalidCharacters", 422}
	MissingRequired    = ErrorCode{"MissingRequired", 422}
	InvalidCSRFToken   = ErrorCode{"InvalidCSRFToken", 422}
	InvalidAction      = ErrorCode{"InvalidAction", 422}
	InvalidBodyContent = ErrorCode{"InvalidBodyContent", 422}
	InvalidType        = ErrorCode{"InvalidType", 422}
	ActionNotAvailable = ErrorCode{"ActionNotAvailable", 404}
	InvalidState       = ErrorCode{"InvalidState", 422}

	ServerError        = ErrorCode{"ServerError", 500}
	ClusterUnavailable = ErrorCode{"ClusterUnavailable", 503}
)

type ErrorCode struct {
	code   string
	Status int
}

func (e ErrorCode) String() string {
	return fmt.Sprintf("%s %d", e.code, e.Status)
}

type APIError struct {
	code      ErrorCode
	message   string
	Cause     error
	fieldName string
}

func NewAPIErrorLong(status int, code, message string) error {
	return NewAPIError(ErrorCode{
		code:   code,
		Status: status,
	}, message)
}

func NewAPIError(code ErrorCode, message string) error {
	return &APIError{
		code:    code,
		message: message,
	}
}

func NewFieldAPIError(code ErrorCode, fieldName, message string) error {
	return &APIError{
		code:      code,
		message:   message,
		fieldName: fieldName,
	}
}

// WrapFieldAPIError will cause the API framework to log the underlying err before returning the APIError as a response.
// err WILL NOT be in the API response
func WrapFieldAPIError(err error, code ErrorCode, fieldName, message string) error {
	return &APIError{
		Cause:     err,
		code:      code,
		message:   message,
		fieldName: fieldName,
	}
}

// WrapAPIError will cause the API framework to log the underlying err before returning the APIError as a response.
// err WILL NOT be in the API response
func WrapAPIError(err error, code ErrorCode, message string) error {
	return &APIError{
		code:    code,
		message: message,
		Cause:   err,
	}
}

func (a *APIError) Error() string {
	if a.fieldName != "" {
		return fmt.Sprintf("%s=%s: %s", a.fieldName, a.code, a.message)
	}
	return fmt.Sprintf("%s: %s", a.code, a.message)
}

func IsAPIError(err error) bool {
	_, ok := err.(*APIError)
	return ok
}

func IsConflict(err error) bool {
	if apiError, ok := err.(*APIError); ok {
		return apiError.code.Status == 409
	}

	return false
}
