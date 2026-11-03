# `default_roles_test.go` Summary

Verifies that default RoleTemplates/GlobalRoles are automatically bound when clusters, projects, and users are created — including locked-role handling and built-in project roles.

## `TestClusterCreateDefaultRole`
**Arrange:**
- Ensures the ClusterRoles "monitoring-ui-view", "navlinks-view", and "navlinks-manage" exist in the local cluster.
- Sets 3 RoleTemplates ("projects-create", "storage-manage", "nodes-view") as cluster-creator defaults.

**Act:** Creates a cluster.

**Assert:**
- Checks the cluster reaches `InitialRolesPopulated`.
- Checks exactly 3 CRTBs are created, one per default role.
- Checks each CRTB is bound to a real user whose principal matches.

## `TestClusterCreateRoleLocked`
**Arrange:**
- Ensures the ClusterRoles "monitoring-ui-view", "navlinks-view", and "navlinks-manage" exist.
- Sets 3 RoleTemplates ("projects-create", "storage-manage", "nodes-view") as cluster-creator defaults.
- Locks the last of the 3 roles ("nodes-view").

**Act:** Creates a cluster.

**Assert:**
- Checks the cluster reaches `InitialRolesPopulated`.
- Checks only the 2 unlocked roles produce CRTBs (the locked role is skipped, others still bound).

## `TestProjectCreateDefaultRole`
**Arrange:**
- Ensures the ClusterRoles "monitoring-ui-view", "navlinks-view", and "navlinks-manage" exist.
- Sets 3 RoleTemplates ("project-member", "workloads-view", "secrets-view") as project-creator defaults.

**Act:** Creates a project on the local cluster.

**Assert:**
- Checks the project reaches `InitialRolesPopulated`.
- Checks exactly 3 PRTBs are created, one per default role.
- Checks each PRTB is bound to a real user whose principal matches.

## `TestProjectCreateRoleLocked`
**Arrange:**
- Ensures the ClusterRoles "monitoring-ui-view", "navlinks-view", and "navlinks-manage" exist.
- Sets 3 RoleTemplates ("project-member", "workloads-view", "secrets-view") as project-creator defaults.
- Locks the last of the 3 roles ("secrets-view") and waits for the lock to take effect.

**Act:** Creates a project on the local cluster.

**Assert:**
- Checks the project reaches `InitialRolesPopulated`.
- Checks only the 2 unlocked roles produce PRTBs (the locked role is skipped, others still bound).

## `TestUserCreateDefaultRole`
**Arrange:**
- Ensures the ClusterRoles "monitoring-ui-view", "navlinks-view", and "navlinks-manage" exist.
- Sets 2 GlobalRoles ("user-base", "settings-manage") as new-user defaults.

**Act:** Creates a CRTB on the local cluster with a fake principal ("local://fakeuser") and RoleTemplate "cluster-owner", which triggers new-user creation.

**Assert:**
- Checks the new user reaches `InitialRolesPopulated`.
- Checks the user receives exactly 2 GlobalRoleBindings, one per default role.

## `TestDefaultSystemProjectRole`
**Act:** Lists the projects in the local cluster.

**Assert:**
- Checks the Default and System projects exist with their expected labels (`authz.management.cattle.io/default-project` and `authz.management.cattle.io/system-project`, both "true").
- Checks every role binding in those projects is `project-owner`.
