# `default_roles_test.go` Summary

Verifies that default RoleTemplates/GlobalRoles are automatically bound when clusters, projects, and users are created — including locked-role handling and built-in project roles.

## `TestClusterCreateDefaultRole`
Sets 3 RoleTemplates as cluster-creator defaults, then creates a cluster.
- Checks the cluster reaches `InitialRolesPopulated`.
- Checks exactly 3 CRTBs are created, one per default role.
- Checks each CRTB is bound to a real user whose principal matches.

## `TestClusterCreateRoleLocked`
Sets 3 cluster-creator defaults, locks one of them, then creates a cluster.
- Checks the cluster reaches `InitialRolesPopulated`.
- Checks only the 2 unlocked roles produce CRTBs (locked role is skipped, others still bound).

## `TestProjectCreateDefaultRole`
Sets 3 RoleTemplates as project-creator defaults, then creates a project.
- Checks the project reaches `InitialRolesPopulated`.
- Checks exactly 3 PRTBs are created, one per default role.
- Checks each PRTB is bound to a real user whose principal matches.

## `TestProjectCreateRoleLocked`
Sets 3 project-creator defaults, locks one of them, waits for the lock to take effect, then creates a project.
- Checks the project reaches `InitialRolesPopulated`.
- Checks only the 2 unlocked roles produce PRTBs (locked role is skipped, others still bound).

## `TestUserCreateDefaultRole`
Sets 2 GlobalRoles as new-user defaults, then creates a CRTB with a fake principal to trigger new-user creation.
- Checks the new user reaches `InitialRolesPopulated`.
- Checks the user receives exactly 2 GlobalRoleBindings, one per default role.

## `TestDefaultSystemProjectRole`
Lists the built-in projects in the local cluster.
- Checks the Default and System projects exist with their expected labels.
- Checks every role binding in those projects is `project-owner`.
