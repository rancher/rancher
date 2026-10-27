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
// middleware may change it, and the final entry reflects those changes.
//
// Model: Name, Identity and Group describe the effective identity, meaning who
// the request is authorized as. ImpersonatedBy describes the real authenticated
// caller and is set only when a user impersonation succeeded. Extra is captured
// when the entry is created and is not refreshed, see the compatibility rule
// below. RequestUser and RequestGroups hold the raw Impersonate-* headers,
// and are recorded before authorization, so a denied impersonation still
// shows the attempted target while the identity stays the real caller.
//
// The identity changes in three ways, and each has one owner:
//
//   - Authentication substitution: the service-account and token-review
//     authenticators replace the error user. There is no real caller, so only
//     the effective identity changes. requests.NewAuthenticatedFilter applies
//     it with [User.SetIdentity].
//   - User impersonation: the impersonation filter replaces the request user
//     and calls [User.Impersonate] with the user before and after the swap.
//   - SCIM: the SCIM token authenticator sets the identity of the SCIM caller
//     with [User.SetIdentity].
//
// Preserve these invariants when changing the [User]:
//
//   - Change the identity only through [User.SetIdentity] and
//     [User.Impersonate], so Name, Identity, Group and ImpersonatedBy cannot
//     drift apart.
//   - Do not change the content or format of existing fields for requests that
//     are not affected by a bug. Log consumers parse them. New information
//     goes in new optional fields that are omitted when empty, as Identity and
//     ImpersonatedBy are.
//   - Group and Extra initially alias memory owned by the request context's
//     user info. Replace them rather than modifying them in place.
//   - Name is the stable, unique Rancher user ID, not a human-readable name.
//     Who the user is, as a person and a provider, goes in Identity.
//   - ExcludeGroups clears the groups of the user and of ImpersonatedBy on the
//     shared [User]. This is safe only because it happens after the request has
//     completed.
//
// # Identity
//
// [User].Identity and ImpersonatedBy are opt-in, through [WithIncludeUsername], which Rancher
// ties to the audit-log-username feature. It is read once when the entry is created. When it is off,
// nothing is derived or assigned: no identity or impersonator is set and no user lookup is made, so
// entries keep their previous format for consumers that parse them strictly. The flag is meant to
// become the default later.
//
// An [Identity] holds the human-readable Username, the Provider ID that authenticated the session
// and the PrincipalID. A username alone is ambiguous, because the same name can exist on several
// providers and can change, so it is meant to be read with its provider. Name stays the key for
// correlation. Each field is optional and the whole Identity is omitted when empty. The same type
// is used for ImpersonatedBy.
//
// The Identity of a user is chosen in this order:
//
//  1. The authoritative identity the authenticator attached to the request user
//     with [WithIdentity], read through [IdentityInfo]. It is used as a whole and
//     is not mixed with the other sources, because that could pair a username
//     with the wrong provider. It is not part of the user extras, because
//     extras are forwarded as impersonation headers and checked against the
//     impersonator role.
//  2. The "username" user extra (see [UsernameFromExtra]). It gives only the
//     Username, because the extras of a user linked to several providers do not
//     say which provider a value belongs to.
//  3. When the entry is written and it is still empty, the user record that the
//     respective Name refers to: its username, then its display name (see
//     [UserResolver] and [WithUserResolver]). It gives only the Username.
//     System identities have no record and keep an empty Identity.
//
// The authoritative identity is attached to the request user object itself, so
// it is lost whenever that object is replaced. After a user impersonation the
// replacement user has only the extras sent in the Impersonate-Extra-* headers,
// so the effective Identity usually comes from the lookup and has no provider,
// and it is never carried over from the real caller. ImpersonatedBy.Identity is
// read from the caller before the replacement, so it keeps the authoritative
// identity.
//
// Rules:
//   - Do not add fallback usernames to the extras built by the token
//     authenticator. They are forwarded as impersonation headers, and the
//     per-user impersonator ClusterRole (pkg/impersonation) only allows the
//     username values stored in the user's attributes, so an unlisted value is
//     rejected with "not allowed to impersonate extras".
//   - No identity source may depend on data that the verbosity level drops
//     (request or response headers and bodies). The login name from a login
//     request body is read regardless of level for this reason.
//   - Authentication providers decide who a user is. The audit package only
//     reads it. The token authenticator reports the login name, the provider
//     and the principal ID of the token the session was authenticated with.
//
// # Known limitations
//
//   - Service-account impersonation is not modeled. The request user is not
//     replaced there (the downstream token is created for the service account
//     in the cluster proxy), so Name stays the real caller and ImpersonatedBy
//     is unset. The meaning of Name therefore depends on the kind of
//     impersonation. This needs its own design.
//   - After a user impersonation, Name and Group are the impersonated user's,
//     but Extra still holds the extras of the authenticated caller, for example
//     its request token ID. This is kept on purpose, because log consumers
//     may rely on it. ImpersonatedBy has the caller's name, username and groups,
//     so the caller does not have to be inferred from Extra. If a use case needs
//     more of the caller, add it to Impersonator as an additive field.
//   - Routes that do not run requests.NewAuthenticatedFilter retain the
//     initially captured identity. Every route that runs the impersonation
//     filter also runs NewAuthenticatedFilter, but a new route that does not
//     would need to call [User.SetIdentity] itself.
//   - Authentication failures are captured as "system:cattle:error" or
//     "system:unauthenticated". Consumers should treat these as "no identity".
package audit
