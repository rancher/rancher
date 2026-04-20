package common

const (
	// ProviderRefreshErrorAnnotation is set on a UserAttribute when a non-transient
	// error occurs during provider refresh. Its presence causes future refreshes
	// to be skipped until the user logs in again (which clears the annotation).
	ProviderRefreshErrorAnnotation = "cattle.io/provider-refresh-error"
)

// NonTransientError wraps an error that will not resolve on retry
// (e.g., user deleted from the identity provider, object too large).
// Providers should return this for errors that are deterministically permanent.
type NonTransientError struct {
	Err error
}

// Error implements [error] interface.
func (e *NonTransientError) Error() string {
	return e.Err.Error()
}

// Unwrap allows getting the original error using [errors.Unwrap] or [errors.As].
func (e *NonTransientError) Unwrap() error {
	return e.Err
}
