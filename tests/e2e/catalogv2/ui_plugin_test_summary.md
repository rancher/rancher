# `ui_plugin_test.go` Summary

Verifies UI plugin installation, authentication requirements, content-type headers, plugin fetching, and endpoint reliability with compressed archives and exponential backoff retry logic.

## `TestGetIndexAuthenticated`
Fetches the UI plugin index with an authenticated session using admin token.
- Checks the index returns 4 installed plugins (uk-locale, clock, top-level-product, homepage).

## `TestGetIndexUnauthenticated`
Fetches the UI plugin index without authentication.
- Checks the index returns only 1 plugin (uk-locale), which is unauthenticated.

## `TestCorrectContentType`
Requests a plugin file (top-level-product-0.1.0.umd.min.1.js) with authenticated session and verifies the response headers.
- Checks the response status is 200 OK.
- Checks the Content-Type header matches the MIME type for .js files.

## `TestGetSingleExtensionAuthenticated`
Requests the clock plugin file (clock-0.2.0.umd.min.js) with an authenticated session.
- Checks the response status is 200 OK.

## `TestGetSingleExtensionUnauthenticated`
Requests the uk-locale plugin file without authentication (uk-locale does not require auth).
- Checks the response status is 200 OK.

## `TestGetSingleUnauthorizedExtension`
Requests the clock plugin file without authentication (clock requires auth).
- Checks the response returns 404 Not Found.

## `TestCompressedEndpoint`
Starts a test server serving a compressed plugin archive, updates the homepage plugin to use the CompressedEndpoint, fetches a file from the compressed archive, and verifies the plugin reaches ready status.
- Checks the compressed endpoint HTTP response is 200 OK or 425 Too Early.
- Checks the plugin eventually reaches ready status with the CompressedEndpoint configured.

## `TestExponentialBackoff`
Starts a test server that fails on the first 2 requests then succeeds, updates the homepage plugin endpoint, and verifies exponential backoff retry logic.
- Checks the plugin initially has RetryNumber 1 and is not ready.
- Checks the plugin then has RetryNumber 2 and is not ready.
- Checks after backoff succeeds, the plugin has RetryNumber 0 and reaches ready status.

## `TestUnreachableCompressedEndpoint`
Sets both an unreachable CompressedEndpoint and a working Endpoint on the homepage plugin.
- Checks the plugin reaches ready status by falling back to the working Endpoint.
