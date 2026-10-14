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
//   - After user impersonation, Name identifies the impersonated user, but
//     Extra still describes the original caller. SCIM replaces Extra.
//   - After service-account impersonation, Name still identifies the real
//     caller, because the request user is not replaced. The meaning of Name
//     therefore depends on the kind of impersonation.
//   - The real caller of a user impersonation is not recorded as an identity.
//     RequestUser and RequestGroups only hold the raw Impersonate-* headers,
//     and they are recorded before authorization, so they are also set when
//     the impersonation is denied.
//   - Routes that do not run requests.NewAuthenticatedFilter retain the
//     initially captured identity, which may be anonymous or an error user.
//   - Authentication failures are captured as "system:cattle:error" or
//     "system:unauthenticated".
//
// # Suggested fixes
//
// These are proposals, not current behavior. They are written down so that
// changes to the [User] are made against one agreed model.
//
// Model: Name, Username, Group and Extra always describe the effective
// identity, meaning the identity the request is authorized as. A new optional
// field, impersonatedBy, describes the real authenticated caller. It is
// present only when an impersonation succeeded, so the meaning of the main
// fields is the same on every route.
//
// Shape of impersonatedBy:
//   - Use a dedicated small struct, not a nested [User]. Reusing [User] makes
//     the type recursive and drags in RequestUser and RequestGroups, which mean
//     nothing for the real caller.
//   - It holds name (the Rancher user ID), username (human-readable and
//     best-effort, with the same source and gaps as the top-level Username)
//     and group.
//   - It does not hold extra for now. Add it later as an additive field if a
//     use case appears, for example the request token ID to tie an action to
//     the real caller's token.
//
// Where it is set:
//   - In the impersonation filter, at the point where the request user is
//     replaced, using the pre-replacement user info that the filter already
//     holds. No lookup is needed for name and group.
//   - Through one helper on [User] that changes Name, Username, Group, Extra
//     and impersonatedBy together, so they cannot drift apart.
//   - requests.NewAuthenticatedFilter and the SCIM authenticator should call
//     the same helper instead of assigning fields one by one.
//
// Service-account impersonation follows the same rule: the effective identity
// is the service account and the real caller goes in impersonatedBy. This
// changes the shape of existing entries, so it needs a release note.
//
// Denied impersonation keeps today's behavior. RequestUser and RequestGroups
// still record the attempted target, the main identity stays the real caller,
// and impersonatedBy is absent. Keep RequestUser and RequestGroups for
// compatibility with existing log consumers.
//
// ExcludeGroups must also clear the groups inside impersonatedBy, or the new
// field would expose groups that the operator chose to exclude.
//
// Effective Username after a user impersonation: the replacement user is built
// only from the Impersonate-Group and Impersonate-Extra-* headers, so the
// effective Username is empty unless the client sent it. Filling it needs a
// lookup of the target user. Do not add fallback usernames to the extras built
// by the token authenticator: those extras are forwarded as impersonation
// headers and the per-user impersonator ClusterRole (pkg/impersonation) only
// allows the username values stored in the user's attributes, so an unlisted
// value is rejected with "not allowed to impersonate extras".
//
// Tests: nothing currently pins the audit fields set during impersonation.
// Cover at least no impersonation, successful user impersonation, successful
// service-account impersonation, denied impersonation, SCIM, and
// ExcludeGroups.
//
// Other known limitations:
//   - Anonymous and error identities are expected on unauthenticated routes.
//     No change is proposed; consumers should treat "system:unauthenticated"
//     and "system:cattle:error" as "no identity".
//   - Routes without requests.NewAuthenticatedFilter keep the captured
//     identity. That is acceptable once the helper is the only way to change
//     the identity and every identity-changing filter calls it.
//   - SCIM replacing Extra moves to the shared helper.
package audit
