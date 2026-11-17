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

// ExtrasUnavailableError wraps an error from a provider's GetPrincipal when the
// provider can't read the user's own record, for example because the user
// lacks permission to look themselves up. Groups can still be refreshed, but
// the user's extra attributes can't be, so the refresh keeps the stored ones.
type ExtrasUnavailableError struct {
	Err error
}

// Error implements [error] interface.
func (e *ExtrasUnavailableError) Error() string {
	return e.Err.Error()
}

// Unwrap allows getting the original error using [errors.Unwrap] or [errors.As].
func (e *ExtrasUnavailableError) Unwrap() error {
	return e.Err
}
