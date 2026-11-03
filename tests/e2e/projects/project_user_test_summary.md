# `project_user_test.go` Summary

Verifies that project members and owners can create namespaces within their assigned projects.

## `TestCreateNamespaceProjectMember`
**Arrange:**
- Creates a project ("TestProject") on the local cluster.
- Creates a test user with global role "user".
- Binds the test user to the project via a ProjectRoleTemplateBinding with role "project-member".
- Waits until the test user is allowed to create namespaces (RBAC propagation).

**Act:** Creates a namespace as the test user.

**Assert:**
- Checks the namespace is created without error.
- Checks the created namespace's name is "testnamespace".

## `TestCreateNamespaceProjectOwner`
**Arrange:**
- Creates a project ("TestProject") on the local cluster.
- Creates a test user with global role "user".
- Binds the test user to the project via a ProjectRoleTemplateBinding with role "project-owner".
- Waits until the test user is allowed to create namespaces (RBAC propagation).

**Act:** Creates a namespace as the test user.

**Assert:**
- Checks the namespace is created without error.
- Checks the created namespace's name is "testnamespace".
