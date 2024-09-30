package auth

import "context"

// saAuthenticatedContextKey is the context key for the SAAuthenticated flag.
type saAuthenticatedContextKey struct{}

// saImpersonationContextKey is the context key for the SA impersonation.
type saImpersonationContextKey struct{}

var saContextKey = saAuthenticatedContextKey{}
var saImpersonationKey = saImpersonationContextKey{}

// IsSAAuthenticated returns true if the SAAuthenticated flag is set in the context.
func IsSAAuthenticated(ctx context.Context) bool {
	authed, _ := ctx.Value(saContextKey).(bool)
	return authed
}

// SetSAAuthenticated sets the SAAuthenticated flag in the context.
func SetSAAuthenticated(ctx context.Context) context.Context {
	return context.WithValue(ctx, saContextKey, true)
}

// SetSAImpersonation sets the impersonated SA in the context.
func SetSAImpersonation(ctx context.Context, impSA string) context.Context {
	return context.WithValue(ctx, saImpersonationKey, impSA)
}

// GetSAImpersonation returns the impersonated SA from the context.
func GetSAImpersonation(ctx context.Context) string {
	impSA, _ := ctx.Value(saImpersonationKey).(string)
	return impSA
}
