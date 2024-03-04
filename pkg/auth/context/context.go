package auth

import "context"

// saAuthenticatedContextKey is the context key for the SAAuthenticated flag.
type saAuthenticatedContextKey struct{}

var saContextKey = saAuthenticatedContextKey{}

// IsSAAuthenticated returns true if the SAAuthenticated flag is set in the context.
func IsSAAuthenticated(ctx context.Context) bool {
	authed, _ := ctx.Value(saContextKey).(bool)
	return authed
}

// SetSAAuthenticated sets the SAAuthenticated flag in the context.
func SetSAAuthenticated(ctx context.Context) context.Context {
	return context.WithValue(ctx, saContextKey, true)
}
