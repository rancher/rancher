package auth

import "context"

// SAAuthenticatedContextKey is the context key for the SAAuthenticated flag.
var SAAuthenticatedContextKey = struct{}{}

// IsSAAuthenticated returns true if the SAAuthenticated flag is set in the context.
func IsSAAuthenticated(ctx context.Context) bool {
	authed, ok := ctx.Value(SAAuthenticatedContextKey).(bool)
	if !ok {
		return false
	}

	return authed
}

// SetSAAuthenticated sets the SAAuthenticated flag in the context.
func SetSAAuthenticated(ctx context.Context) context.Context {
	return context.WithValue(ctx, SAAuthenticatedContextKey, true)
}
