# `steve_api_test.go` Summary

Verifies that the Steve API (Rancher's REST API wrapper) provides access to extension API servers with proper authentication and authorization, and that listing, filtering, sorting, and CRUD operations on secrets respect user permissions across projects and namespaces.

## `TestExtensionAPIServer` (LocalSteveAPITestSuite)
Creates a discovery client against the extension API server and verifies server groups, OpenAPI v2 and v3 schemas are available with admin token, then verifies all endpoints return forbidden errors when accessed without authentication.
- Checks the discovery client retrieves server groups.
- Checks OpenAPI v2 schema is available and non-nil.
- Checks OpenAPI v3 paths are available.
- Checks unauthenticated requests to all endpoints return forbidden errors.

## `TestExtensionAPIServerAuthorization`
Makes HTTP requests to various OpenAPI and metrics/health endpoints with an authenticated client and verifies expected status codes for each.
- Checks `/openapi/v2`, `/openapi/v3`, and `/openapi/v3/version` return 200 OK.
- Checks `/metrics`, `/healthz`, `/readyz`, `/livez`, and `/version` return 403 Forbidden.

## `TestExtensionAPIServerCreateRequests`
Posts JSON payloads to create a kubeconfig and a selfuser resource via the extension API and verifies both return 201 Created.
- Checks creating a kubeconfig with name, clusters, and TTL returns 201 Created.
- Checks creating a selfuser resource returns 201 Created.

## `TestExtensionAPIServerUpdateRequests`
Retrieves a test kubeconfig, then updates it via PUT with modified description and verifies 200 OK, and attempts to update a non-existent kubeconfig and verifies 404 Not Found.
- Checks updating an existing kubeconfig with modified description returns 200 OK.
- Checks updating a non-existent kubeconfig returns 404 Not Found.

## `TestExtensionAPIServerDeleteRequests`
Retrieves a test kubeconfig, then deletes it via DELETE and verifies 204 No Content, and attempts to delete a non-existent kubeconfig and verifies 404 Not Found.
- Checks deleting an existing kubeconfig returns 204 No Content.
- Checks deleting a non-existent kubeconfig returns 404 Not Found.

## `TestExtensionAPIServer` (DownstreamSteveAPITestSuite)
Attempts to access extension API server endpoints on a downstream cluster and verifies all requests return 404 Not Found because the extension API is not served on downstream clusters.
- Checks ServerGroups request returns 404 Not Found.
- Checks OpenAPI v2 schema request returns 404 Not Found.
- Checks OpenAPI v3 paths request returns 404 Not Found.

## `TestList`
Sets up 5 test secrets per namespace (9 namespaces total), 2 projects with namespace assignments, 5 test users with varying project-level and namespace-level RBAC bindings, then executes 100+ table-driven test cases that verify listing, filtering, sorting, and pagination behavior for each user across different access scopes.
- Checks each user sees only secrets in namespaces they have access to via project membership or role bindings.
- Checks label and field selectors correctly filter results across multiple namespaces or within a single namespace.
- Checks filter queries with AND, OR, and NOT operators work correctly.
- Checks sort by metadata.name and metadata.namespace in ascending and descending order.
- Checks pagination with pagesize parameter returns correct first page and subsequent pages using continue tokens and revision numbers.
- Checks `projectsornamespaces` parameter restricts results to specific projects or namespaces, and `projectsornamespaces!=` excludes them.
- Checks summary queries return aggregated counts per property value (name, namespace, state) on the current page.

## `TestLinks`
Creates a secret resource via the Steve API and reads it back, then verifies that the returned object has correct self, view, update, patch, and remove links pointing to valid API endpoints.
- Checks the secret ID is formatted as `namespace/name`.
- Checks links reference the correct API endpoints: `/v1/secrets/{namespace}/{name}` for Steve API operations and `/api/v1/namespaces/{namespace}/secrets/{name}` for the Kubernetes view link.

## `TestCRUD`
Tests both global (`/v1/secrets`) and namespaced (`/v1/secrets/{namespace}`) endpoints for secret CRUD operations, with each variant creating, reading, updating, and deleting a secret.
- Checks creating a secret returns a valid resource object.
- Checks reading a secret by ID retrieves the same object.
- Checks updating a secret's data field persists the change.
- Checks deleting a secret removes it and subsequent reads return an error.
