# `rtbs_test.go` Summary

Verifies role template binding behavior, including inheritance chains, immutability constraints, and access revocation when bindings are deleted.

## `TestPRTBRoleTemplateInheritance`
**Arrange:**
- Creates a user with the "user" global role.
- Creates a namespace in the suite's shared project (created in `SetupSuite` on the local cluster).
- Creates a secret in that namespace.
- Confirms the user cannot get the secret before any binding exists.
- Creates RoleB (project-scoped) with a rule granting "get" on that specific secret.
- Creates RoleA (project-scoped) that inherits RoleB via `RoleTemplateIDs`.

**Act 1:** Binds the user to RoleA via a PRTB on the suite project.
**Assert 1:**
- Checks the user can get the secret (permission inherited from RoleB through RoleA).

**Act 2:** Deletes the PRTB, then creates RoleC that inherits RoleA (chain: RoleC → RoleA → RoleB) and binds the user to RoleC via a new PRTB.
**Assert 2:**
- Checks the user's secret access is revoked once the first PRTB is deleted.
- Checks the user regains access to the secret via the chained inheritance.
- Checks the user cannot access a second, newly created secret not covered by any rule.

**Act 3:** Updates RoleB's rules to add "get" access to the second secret.
**Assert 3:**
- Checks the user gains access to the second secret too, without any new binding (the change propagates through the inheritance chain).

## `TestCRTBRoleTemplateInheritance`
**Arrange:**
- Creates a user with the "user" global role.
- Creates a namespace in the suite's shared project (local cluster).
- Confirms the user cannot get the namespace before any binding exists.
- Creates RoleB (cluster-scoped, empty context) with a rule granting "get" on that namespace.
- Creates RoleA (cluster-scoped) that inherits RoleB via `RoleTemplateIDs`.

**Act 1:** Binds the user to RoleA via a CRTB on the local cluster.
**Assert 1:**
- Checks the user can get the namespace (permission inherited from RoleB through RoleA).

**Act 2:** Deletes the CRTB, waits for the namespace access to be revoked, then creates RoleC that inherits RoleA (chain: RoleC → RoleA → RoleB), binds the user to RoleC via a new CRTB, and creates a second namespace.
**Assert 2:**
- Checks the user regains access to the first namespace via the chained inheritance.
- Checks the user cannot access the second, newly created namespace.

**Act 3:** Updates RoleB's rules to add "get" access to the second namespace.
**Assert 3:**
- Checks the user gains access to the second namespace too, while retaining access to the first.

## `TestRemovingPRTBRevokesNamespaceAccess`
**Arrange:**
- Creates a user.
- Creates two projects on the local cluster and binds the user as "project-member" to both via PRTBs.
- Creates a namespace in the first project; confirms the user can access it.
- Creates a namespace in the second project; confirms the user can access both namespaces.

**Act:** Deletes the PRTB binding the user to the second project.

**Assert:**
- Checks the user loses access to the second project's namespace.
- Checks the user retains access to the first project's namespace.

## `TestAPIGroupInRoleTemplate`
**Arrange:**
- Skips the test if the admin cannot see any nodes in the local cluster.
- Creates a standard user with the "user" global role; confirms the user cannot see any nodes yet.
- Creates a cluster-scoped RoleTemplate with rules granting get/list/watch on "nodes"/"nodepools" (management.cattle.io) and full access ("*") on "scheduling.k8s.io", and waits for it to become available.

**Act:** Binds the user to the RoleTemplate via a CRTB on the local cluster.

**Assert:**
- Checks the user eventually can list nodes.
- Checks the user cannot delete a node (the role only grants get/list/watch).

## `TestDeletingPRTBRemovesClusterAccess`
**Arrange:**
- Creates a user.
- Admin creates a PRTB granting the user "project-member" on the suite's shared project (local cluster).
- Confirms the user can see the local cluster.
- Confirms a membership ClusterRoleBinding is created, labeled with a key derived from the PRTB's ID.

**Act:** Deletes the PRTB.

**Assert:**
- Checks the membership ClusterRoleBinding is deleted.
- Checks the user loses cluster access entirely (no clusters listed, and a 403 when fetching the local cluster by ID).

## `TestDeletingPRTBCleansUpLegacyMembershipLabels`
**Arrange:**
- Creates a user.
- Admin creates a PRTB granting the user "project-member" on the suite's shared project (local cluster).
- Confirms the user can see the local cluster.
- Confirms a membership ClusterRoleBinding exists, labeled with a key derived from the PRTB's ID.

**Act:** Deletes the PRTB.

**Assert:**
- Checks the membership ClusterRoleBinding is removed.
- Checks the user loses cluster access.

## `TestCRTBCannotTargetUsersAndGroup`
**Arrange:**
- Creates a user.

**Act:** Attempts to create a CRTB on the local cluster targeting both the user (`userId`) and a group (`groupPrincipalId`) simultaneously, using RoleTemplate "clustercatalogs-view".

**Assert:**
- Checks creation fails with 422 Unprocessable Entity.
- Checks the error message states the binding must target a user OR a group, not both.

## `TestCRTBMustHaveTarget`
**Act:** Attempts to create a CRTB on the local cluster with RoleTemplate "clustercatalogs-view" and no subject at all (no `userId`, `userPrincipalId`, `groupId`, or `groupPrincipalId`).

**Assert:**
- Checks creation fails with 422 Unprocessable Entity.
- Checks the error message states the binding must target a user or a group.

## `TestCRTBCannotUpdateSubjectsOrCluster`
**Arrange:**
- Creates a user.
- Creates a CRTB on the local cluster binding the user to RoleTemplate "clustercatalogs-view"; waits for its `userPrincipalId` to populate.

**Act:** Attempts to update the CRTB's `clusterId`, `userId`, `userPrincipalId`, `groupPrincipalId`, and `groupId` fields simultaneously.

**Assert:**
- Checks `clusterId` remains unchanged.
- Checks `userId` remains unchanged.
- Checks `userPrincipalId` remains unchanged.
- Checks `groupPrincipalId` and `groupId` remain unset.
