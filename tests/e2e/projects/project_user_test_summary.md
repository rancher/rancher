# `project_user_test.go` Summary

Verifies that project members and owners can create namespaces within their assigned projects.

## `TestCreateNamespaceProjectMember`
Creates a project, assigns the test user the project-member role, impersonates that user, waits for the "create namespaces" permission to be allowed, then creates a namespace.
- Checks the namespace is successfully created.
- Checks the namespace name is "testnamespace".

## `TestCreateNamespaceProjectOwner`
Creates a project, assigns the test user the project-owner role, impersonates that user, waits for the "create namespaces" permission to be allowed, then creates a namespace.
- Checks the namespace is successfully created.
- Checks the namespace name is "testnamespace".
