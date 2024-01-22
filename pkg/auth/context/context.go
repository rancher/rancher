package auth

import "context"

// SAAuthenticatedContextKey is the context key for the SAAuthenticated flag.
type SAAuthenticatedContextKey struct{}

var SAContextKey = SAAuthenticatedContextKey{}

// IsSAAuthenticated returns true if the SAAuthenticated flag is set in the context.
func IsSAAuthenticated(ctx context.Context) bool {
	authed, _ := ctx.Value(SAContextKey).(bool)
	return authed
}

// SetSAAuthenticated sets the SAAuthenticated flag in the context.
func SetSAAuthenticated(ctx context.Context) context.Context {
	return context.WithValue(ctx, SAContextKey, true)
}
