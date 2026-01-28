// Package scim implements SCIM 2.0 (RFC 7644) endpoints for user and group provisioning.
//
// This package provides HTTP handlers for identity provider integration, enabling
// automated user lifecycle management. It supports:
//
//   - User resources: list, get, create, update, delete
//   - Group resources: list, get, create, update (PATCH), delete
//   - Bearer token authentication via Kubernetes secrets
//   - Pagination following RFC 7644 3.4.2.4
//
// Endpoints are registered under /v1-scim/{provider}/ where provider identifies
// the authentication provider (e.g., okta, azure).
package scim
