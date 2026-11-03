# `steve_api_test.go` Summary

Verifies that the Steve API (Rancher's REST API wrapper) provides access to extension API servers with proper authentication and authorization, and that listing, filtering, sorting, and CRUD operations on secrets respect user permissions across projects and namespaces.

This file uses a shared-base-struct embedding pattern: the unexported `steveAPITestSuite` holds common test methods (`TestList`, `TestLinks`, `TestCRUD`), while `LocalSteveAPITestSuite` and `DownstreamSteveAPITestSuite` each embed it with their own `SetupSuite` (one against the local cluster, one against a real downstream cluster). A test method defined on the shared base runs once per concrete suite.

## `TestExtensionAPIServer` (LocalSteveAPITestSuite)
**Arrange:**
- Builds a discovery client against the extension API server using the admin token.
- Builds a second discovery client against the same server with no auth token.

**Act:** Queries server groups, the OpenAPI v2 schema, and the OpenAPI v3 paths with both the authenticated and unauthenticated clients.

**Assert:**
- Checks the authenticated client retrieves server groups, a non-nil OpenAPI v2 schema, and OpenAPI v3 paths.
- Checks the unauthenticated client's requests to all three endpoints return forbidden errors.

## `TestExtensionAPIServerAuthorization`
**Arrange:**
- Builds an authenticated HTTP client against the extension API server.

**Act:** Sends GET requests to each of 8 endpoints (`/openapi/v2`, `/openapi/v3`, `/openapi/v3/version`, `/metrics`, `/healthz`, `/readyz`, `/livez`, `/version`).

**Assert:**
- Checks `/openapi/v2`, `/openapi/v3`, and `/openapi/v3/version` return 200 OK.
- Checks `/metrics`, `/healthz`, `/readyz`, `/livez`, and `/version` return 403 Forbidden.

## `TestExtensionAPIServerCreateRequests`
**Arrange:**
- Builds an authenticated HTTP client against the extension API server.

**Act:** Posts a JSON payload to create a kubeconfig (name, clusters, description, TTL) and a JSON payload to create a selfuser resource.

**Assert:**
- Checks creating the kubeconfig returns 201 Created.
- Checks creating the selfuser resource returns 201 Created.

## `TestExtensionAPIServerUpdateRequests`
**Arrange:**
- Creates a test kubeconfig via the extension API.

**Act:** Sends PUT requests updating the existing kubeconfig's description and updating a non-existent kubeconfig.

**Assert:**
- Checks updating the existing kubeconfig with a modified description returns 200 OK.
- Checks updating a non-existent kubeconfig returns 404 Not Found.

## `TestExtensionAPIServerDeleteRequests`
**Arrange:**
- Creates a test kubeconfig via the extension API.

**Act:** Sends DELETE requests for the existing kubeconfig and for a non-existent kubeconfig.

**Assert:**
- Checks deleting the existing kubeconfig returns 204 No Content.
- Checks deleting a non-existent kubeconfig returns 404 Not Found.

## `TestExtensionAPIServer` (DownstreamSteveAPITestSuite)
**Arrange:**
- Builds a discovery client against the extension API server on the downstream cluster using the admin token.

**Act:** Queries server groups, the OpenAPI v2 schema, and the OpenAPI v3 paths.

**Assert:**
- Checks all three requests return 404 Not Found, since the extension API is not served on downstream clusters.

## `TestList`
**Arrange:**
- Creates 2 projects and 9 namespaces (7 assigned across the 2 projects, 2 unassigned), each seeded with up to 5 test secrets carrying shared and per-secret labels (used for filter/selector tests).
- Creates 5 test users with varying access scopes: `user-a` (project-owner on 1 project), `user-b` (namespace-scoped role binding granting get/list on secrets in 1 namespace), `user-c` (namespace-scoped role binding with a resource-name restriction across 3 namespaces), `user-d` (project-owner on 2 projects plus namespace-scoped bindings in 2 more namespaces), and `user-e` (cluster-owner).

**Act:** Runs 139 table-driven subtests, each issuing a Steve API list request for secrets as a given user, optionally scoped to a namespace, with a given query string.

**Assert:**
- Checks each user sees only secrets in namespaces they have access to via project membership or role bindings.
- Checks label and field selectors correctly filter results across multiple namespaces or within a single namespace.
- Checks filter queries combining AND (multiple `filter` params), OR (comma-separated values), and NOT (`!=`) operators return the correct set of secrets.
- Checks sorting by `metadata.name` and `metadata.namespace`, ascending and descending, returns results in the expected order.
- Checks pagination with `pagesize` returns the correct first page, and subsequent pages fetched with `page` plus a `continue`/`revision` token return the correct remaining results.
- Checks `projectsornamespaces` restricts results to the named projects or namespaces, and `projectsornamespaces!=` excludes them.
- Checks `summary` queries return aggregated counts per property value (e.g. `metadata.name`, `metadata.namespace`, `metadata.state.name`) reflecting only the current page.

## `TestLinks`
**Arrange:**
- Creates a secret via the Steve API, then reads it back by ID.

**Act:** Deletes the secret.

**Assert:**
- Checks the secret's `id` field is formatted as `namespace/name`.
- Checks the returned `links` map's `self`, `view`, `update`, `patch`, and `remove` entries point to the correct endpoints: `/v1/secrets/{namespace}/{name}` for Steve API operations, and `/api/v1/namespaces/{namespace}/secrets/{name}` for the Kubernetes view link.

## `TestCRUD`
**Arrange:**
- None beyond obtaining a Steve client for the cluster.

**Act:** For both the global (`/v1/secrets`) and namespaced (`/v1/secrets/{namespace}`) endpoints, creates a secret, reads it, updates its data field, deletes it, then reads it again.

**Assert:**
- Checks the created secret can be read back with its original data.
- Checks the updated secret's data reflects the new value on re-read.
- Checks the secret is gone and reading it after deletion returns an error, for both the global and namespaced endpoint variants.
