# `tokens_test.go` Summary

Verifies that authentication tokens are properly managed, including current token tracking, TTL enforcement, and security measures.

## `TestCurrentToken`
**Act:** Lists all tokens via the management API.

**Assert:**
- Checks exactly 1 token is marked as current in the token list.
- Checks the current token's userId matches the admin user.

## `TestWebsocket`
**Arrange:**
- Builds a GET request to a protected endpoint (`/v3/clusters`) with websocket-upgrade headers (`Connection: upgrade`, `Upgrade: websocket`).

**Act:** Sends the request.

**Assert:**
- Checks the request is rejected with 403 Forbidden.

## `TestAPITokenTTL`
**Arrange:**
- Reads the configured max TTL from the `auth-token-max-ttl-minutes` setting.

**Act:** Creates a token with `ttl=0` (unlimited).

**Assert:**
- Checks the created token's TTL (converted from milliseconds to minutes) equals the configured max TTL.

## `TestKubeconfigTokenTTL`
**Arrange:**
- Deletes any existing kubeconfig token for the admin user.
- Saves the original `kubeconfig-generate-token` and `kubeconfig-default-token-ttl-minutes` settings for restoration afterward.
- Sets `kubeconfig-generate-token` to `false` and `kubeconfig-default-token-ttl-minutes` to `0.01` minutes (~600ms).

**Act:** Logs in via the `/v3-public` and `/v1-public` login endpoints, waiting for the previous token to expire before each subsequent login.

**Assert:**
- Checks the `/v3-public` login response contains `token`, `expiresAt`, and `id` fields, with the token longer than the id and response type/baseType `"token"`.
- Checks tokens expire within the configured TTL window, confirmed by polling until the bearer token is rejected with 401.
- Checks a new `/v3-public` login after expiry issues a token different from the previous one.
- Checks the `/v1-public` login also returns `token` and `expiresAt` fields, both before and after expiry.
