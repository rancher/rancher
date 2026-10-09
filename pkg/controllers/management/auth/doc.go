// Package auth contains the management controllers for authentication and
// access control. They reconcile auth provider configs, users, tokens and
// user attributes, including the provider refresh, and create the Kubernetes
// RBAC resources behind ClusterRoleTemplateBindings, ProjectRoleTemplateBindings
// and role templates.
package auth
