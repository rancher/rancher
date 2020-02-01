package validation

import (
	"errors"
	"fmt"
)

var (
	Unauthorized     = ErrorCode{"Unauthorized", 401}
	PermissionDenied = ErrorCode{"PermissionDenied", 403}
	NotFound         = ErrorCode{"NotFound", 404}
	MethodNotAllowed = ErrorCode{"MethodNotAllowed", 405}
	Conflict         = ErrorCode{"Conflict", 409}

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

	ErrComplete = errors.New("request completed")
)

type ErrorCode struct {
	Code   string
	Status int
}

func (e ErrorCode) Error() string {
	return fmt.Sprintf("%s %d", e.Code, e.Status)
}
