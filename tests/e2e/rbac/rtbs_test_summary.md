# `rtbs_test.go` Summary

Verifies role template binding behavior, including inheritance chains, immutability constraints, and access revocation when bindings are deleted.

## `TestPRTBRoleTemplateInheritance`
Creates RoleB with secret access, RoleA inheriting RoleB, and a PRTB binding the user to RoleA, verifying permission inheritance and permission updates.
- Checks user can access the secret via inherited permissions.
- Checks removing the PRTB revokes the secret access.
- Checks creating a second role RoleC that inherits from RoleA re-grants secret access.
- Checks updating the inherited rule to allow another secret propagates the new permission.

## `TestCRTBRoleTemplateInheritance`
Creates RoleB with namespace access, RoleA inheriting RoleB, and a CRTB binding the user to RoleA, verifying cluster-scoped permission inheritance and chained inheritance.
- Checks user can access the namespace via inherited permissions.
- Checks removing the CRTB revokes namespace access.
- Checks creating a chained inheritance (RoleC → RoleA → RoleB) re-grants access.
- Checks updating the inherited rule to allow another namespace propagates the new permission.

## `TestRemovingPRTBRevokesNamespaceAccess`
Creates two projects, binds user to project-member on both with namespaces in each, then removes the PRTB from one project.
- Checks user can access namespaces in both projects initially.
- Checks removing one PRTB revokes access to only that project's namespace.
- Checks user retains access to the first project's namespace.

## `TestAPIGroupInRoleTemplate`
Creates a role template with rules for "management.cattle.io" (nodes, nodepools) and "scheduling.k8s.io" (*), binds it via CRTB.
- Checks standard user cannot see nodes initially.
- Checks user can see nodes after binding.
- Checks user cannot delete a node (role only grants get/list/watch).

## `TestDeletingPRTBRemovesClusterAccess`
Creates a PRTB giving a user project-member access, then deletes the PRTB.
- Checks user can see the cluster while the PRTB exists.
- Checks the membership ClusterRoleBinding is created with the PRTB ID label.
- Checks deleting the PRTB removes the membership ClusterRoleBinding.
- Checks user loses cluster access.

## `TestDeletingPRTBCleansUpLegacyMembershipLabels`
Creates a PRTB, verifies the membership CRB is created, then deletes the PRTB.
- Checks the membership ClusterRoleBinding exists with the PRTB ID label.
- Checks deleting the PRTB removes the membership ClusterRoleBinding.
- Checks user loses cluster access.

## `TestCRTBCannotTargetUsersAndGroup`
Attempts to create a CRTB with both userId and groupPrincipalId set.
- Checks creation fails with 422 Unprocessable Entity.
- Checks error message states "must target a user [userId]/[userPrincipalId] OR a group".

## `TestCRTBMustHaveTarget`
Attempts to create a CRTB with no subject (no userId, userPrincipalId, groupId, or groupPrincipalId).
- Checks creation fails with 422 Unprocessable Entity.
- Checks error message states "must target a user or a group".

## `TestCRTBCannotUpdateSubjectsOrCluster`
Creates a CRTB, waits for userPrincipalId to be populated, then attempts to change immutable fields.
- Checks clusterId remains unchanged.
- Checks userId remains unchanged.
- Checks userPrincipalId remains unchanged.
- Checks groupPrincipalId and groupId remain empty.
