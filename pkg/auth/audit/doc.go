// Package audit implements Rancher's audit log: an access-log style HTTP
// middleware that writes one entry per request after the handler returns.
//
// # Registration
//
// The middleware is registered on Rancher.Auth in pkg/rancher/rancher.go:
//
//	Rancher.Auth = authServer.Authenticator.Chain(auditLogMiddleware)
//
// When audit logging is disabled, a passthrough middleware is used instead.
//
// steve's Middleware.Chain composes as m(middleware(next)), so the
// authenticator runs first and the audit middleware wraps Rancher.Handler.
//
// The audit middleware must remain after the authenticator and outside
// Rancher.Handler. It captures the request user once, before downstream
// handlers can change it, and must observe the final response and mutations
// to the audit [User]. Moving it into Rancher.Handler or a per-route chain
// can change the captured identity or leave routes unaudited.
//
// # Request lifecycle
//
//  1. The authenticator sets the request user, including error and
//     unauthenticated identities when authentication fails or is absent.
//  2. The audit middleware creates a [User] from that context, stores it
//     for downstream mutation via [FromContext], and wraps the request
//     body and response writer as needed.
//  3. Rancher.Handler and its downstream middleware handle the request.
//  4. After the handler returns, the audit middleware builds the entry from
//     the final [User] and response, then writes it.
//
// # Coverage
//
// Every request reaching Rancher.Handler passes through the audit middleware,
// including public login routes and SCIM. No entry is written when:
//
//   - audit logging is disabled;
//   - the resolved policy and verbosity exclude the request URI; or
//   - the response writer is hijacked, as for websockets and tunnels.
//
// # The audit User
//
// The [User] is a shared pointer created before the handler runs. Downstream
// middleware may mutate it, and the final entry reflects those mutations.
//
// Current mutation points include:
//
//   - The impersonation filter sets RequestUser and RequestGroups.
//   - requests.NewAuthenticatedFilter sets Name and Group from the current
//     request user, which may be the impersonated user.
//   - The SCIM token authenticator sets Name, Group, and Extra.
//
// Preserve these invariants when changing the [User] or its mutation points:
//
//   - Group and Extra initially alias memory owned by the request context's
//     user info. Replace them rather than modifying them in place.
//   - Name is the stable, unique Rancher user ID, not a human-readable name.
//     Store human-readable identity separately.
//   - Fields derived from the request user must be refreshed wherever Name
//     is refreshed, unless they intentionally represent request-entry state.
//   - ExcludeGroups clears Group on the shared [User]; preserve the required
//     lifecycle and ownership assumptions when changing this behavior.
//
// # Known limitations
//
//   - After impersonation, Name identifies the impersonated user, but Extra
//     still describes the original caller. SCIM replaces Extra.
//   - Routes that do not run requests.NewAuthenticatedFilter retain the
//     initially captured identity, which may be anonymous or an error user.
//   - Authentication failures are captured as "system:cattle:error" or
//     "system:unauthenticated".
package audit
