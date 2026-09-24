# `tokens_test.go` Summary

Verifies that authentication tokens are properly managed, including current token tracking, TTL enforcement, and security measures.

## `TestCurrentToken`
Lists all tokens and identifies the current admin token, then verifies exactly one token is marked as current and its userId matches the admin.
- Checks exactly 1 token is marked as current in the token list.
- Checks the current token's userId matches the admin user.

## `TestWebsocket`
Sends an HTTP GET request with websocket upgrade headers (Connection: upgrade, Upgrade: websocket) to a protected endpoint.
- Checks the request is rejected with 403 Forbidden.

## `TestAPITokenTTL`
Creates a token with ttl=0 (unlimited) and verifies it is capped to the auth-token-max-ttl-minutes setting.
- Checks the token TTL is set to the max TTL configured in the auth-token-max-ttl-minutes setting.

## `TestKubeconfigTokenTTL`
Sets kubeconfig-generate-token to false and kubeconfig-default-token-ttl-minutes to 0.01 minutes (~600ms), then logs in via /v3-public and /v1-public endpoints, verifies tokens are returned with expiresAt, waits for expiry, logs in again to verify a new token is issued, and repeats for the /v1-public endpoint.
- Checks login responses contain token, expiresAt, and id fields.
- Checks token values are longer than the id field.
- Checks response type is "token".
- Checks tokens expire as predicted by the TTL setting.
- Checks a new login after token expiry generates a different token.
- Checks both /v3-public and /v1-public endpoints respect the kubeconfig TTL setting.
