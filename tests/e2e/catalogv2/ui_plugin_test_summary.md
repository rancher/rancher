# `ui_plugin_test.go` Summary

Verifies that the `/v1/uiplugins` index and per-plugin file endpoints respect authentication requirements and content types, and that UIPlugin resources correctly fetch, cache, and retry compressed/uncompressed plugin archives from remote endpoints.

## `TestGetIndexAuthenticated`
**Arrange:**
- None (relies on the suite's 4 pre-installed plugins: `uk-locale`, `clock`, `top-level-product`, `homepage`).

**Act:** Requests `/v1/uiplugins` with an authenticated session (valid `R_SESS` cookie).

**Assert:**
- Checks the returned index contains exactly 4 entries.

## `TestGetIndexUnauthenticated`
**Arrange:**
- None.

**Act:** Requests `/v1/uiplugins` without authentication.

**Assert:**
- Checks the returned index contains exactly 1 entry.
- Checks that entry is `uk-locale` (the only plugin that doesn't require authentication).

## `TestCorrectContentType`
**Arrange:**
- None.

**Act:** Requests a specific plugin file (`/v1/uiplugins/top-level-product/0.1.0/plugin/top-level-product-0.1.0.umd.min.1.js`) with an authenticated session.

**Assert:**
- Checks the response status is 200 OK.
- Checks the `Content-Type` header matches the MIME type inferred from the file's `.js` extension.

## `TestGetSingleExtensionAuthenticated`
**Arrange:**
- None.

**Act:** Requests the `clock` plugin's JS file with an authenticated session.

**Assert:**
- Checks the response status is 200 OK.

## `TestGetSingleExtensionUnauthenticated`
**Arrange:**
- None.

**Act:** Requests the `uk-locale` plugin's JS file without authentication.

**Assert:**
- Checks the response status is 200 OK (`uk-locale` doesn't require authentication).

## `TestGetSingleUnauthorizedExtension`
**Arrange:**
- None.

**Act:** Requests the `clock` plugin's JS file without authentication.

**Assert:**
- Checks the response status is 404 Not Found (`clock` requires authentication).

## `TestCompressedEndpoint`
**Arrange:**
- Starts a test server serving a compressed (`.tgz`) plugin archive.

**Act:** Updates the `homepage` UIPlugin to clear `Endpoint` and set `CompressedEndpoint` to the test server's URL.

**Assert:**
- Checks a request for a file served from the compressed archive returns 200 OK or 425 Too Early.
- Checks the plugin eventually reaches `Ready` status with the `CompressedEndpoint` configured.

## `TestExponentialBackoff`
**Arrange:**
- Starts a test server that returns a server error on the first 2 requests, then serves the plugin successfully.

**Act:** Updates the `homepage` UIPlugin's `Endpoint` to the test server's URL (clearing `CompressedEndpoint`).

**Assert:**
- Checks the plugin's `RetryNumber` progresses to 1, then to 2, remaining not-`Ready` at each step.
- Checks that once the server starts succeeding, `RetryNumber` resets to 0 and the plugin becomes `Ready`.

## `TestUnreachableCompressedEndpoint`
**Arrange:**
- Starts a working test server for the plugin's uncompressed endpoint.

**Act:** Sets the `homepage` UIPlugin's `CompressedEndpoint` to an unreachable URL while also setting a working `Endpoint`.

**Assert:**
- Checks the plugin still reaches `Ready` status, falling back to the working `Endpoint` despite the broken `CompressedEndpoint`.
