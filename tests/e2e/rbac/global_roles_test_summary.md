# `global_roles_test.go` Summary

Verifies access control on global roles, global role bindings, and the visibility differences between the "user" and "user-base" global roles.

## `TestUserVsUserBaseGlobalRoleVisibility`
**Arrange:**
- Creates user1 with the "user" global role.
- Creates user2 with the "user-base" global role.
- Creates 2 more "user"-role users to pad the total user count.

**Act:** Lists users and RoleTemplates as the admin, user1, and user2.

**Assert:**
- Checks the admin sees at least 5 users.
- Checks user1 (user role) sees only themselves when listing users.
- Checks user1 can eventually list all RoleTemplates once RBAC propagates.
- Checks user2 (user-base role) sees only themselves when listing users.
- Checks user2 cannot see any RoleTemplates.

## `TestKontainerDriverVisibilityByGlobalRole`
**Arrange:**
- Creates 4 users, each with one of the following global roles: "user", "clusters-create", "kontainerdrivers-manage", "settings-manage".

**Act:** Lists KontainerDrivers as each of the 4 users.

**Assert:**
- Checks the users with "user", "clusters-create", and "kontainerdrivers-manage" roles each see 3 kontainer drivers.
- Checks the user with the "settings-manage" role sees 0 kontainer drivers.

## `TestBuiltinGlobalRoleOnlyNewUserDefaultEditable`
**Arrange:**
- Retrieves the builtin "admin" GlobalRole, confirming it is builtin, has no "remove" link, and `newUserDefault` is false.

**Act:** Updates the "admin" GlobalRole, attempting to change `name`, `description`, `rules`, `newUserDefault`, and `builtin` simultaneously.

**Assert:**
- Checks `name` remains unchanged.
- Checks `rules` are not wiped out.
- Checks `builtin` remains true.
- Checks only `newUserDefault` changes, becoming true.

## `TestOnlyAdminCanCRUDGlobalRoles`
**Arrange:**
- Creates a standard user with the "user" global role.

**Act:** Performs create, update, list, and delete operations against GlobalRoles as both the admin and the standard user.

**Assert:**
- Checks the admin can create, update, list, and delete a non-builtin GlobalRole.
- Checks the standard user receives 403 Forbidden when attempting to create or update a GlobalRole.
- Checks the standard user sees no GlobalRoles when listing.
- Checks the standard user receives 403 Forbidden when attempting to delete a GlobalRole.

## `TestAdminCannotDeleteBuiltinGlobalRole`
**Arrange:**
- Retrieves the builtin "admin" GlobalRole, confirming it is builtin and has no "remove" link.

**Act:** Creates a GlobalRole with `builtin: true`, updates the builtin "admin" role, and attempts to delete the builtin "admin" role.

**Assert:**
- Checks the newly created GlobalRole ignores the `builtin: true` field (the created role is not builtin).
- Checks the admin can update the builtin role without error.
- Checks deleting the builtin role fails with 403 Forbidden.
- Checks the error message contains "cannot delete builtin global roles".

## `TestGRBCannotUpdateGlobalRoleID`
**Arrange:**
- Creates a user.
- Creates a GlobalRoleBinding for the user with `globalRoleId` "nodedrivers-manage".

**Act:** Attempts to update the GlobalRoleBinding's `globalRoleId` to "settings-manage".

**Assert:**
- Checks `globalRoleId` remains "nodedrivers-manage" after the update.

## `TestGRBGlobalRoleMustExist`
**Arrange:**
- Creates a user.

**Act:** Attempts to create a GlobalRoleBinding referencing a non-existent GlobalRole ("somefakerole").

**Assert:**
- Checks the creation fails with 404 Not Found.

## `TestGRBCannotUpdateSubject`
**Arrange:**
- Creates two users (user1, user2).
- Creates a GlobalRoleBinding binding user1 to "nodedrivers-manage".

**Act:** Attempts to update the GlobalRoleBinding's `userId` to user2's ID, then attempts to set `groupPrincipalId`.

**Assert:**
- Checks `userId` remains user1's ID after attempting to change it to user2.
- Checks `userId` remains user1's ID and `groupPrincipalId` stays empty after attempting to set `groupPrincipalId`.

## `TestGRBTargetsUserOrGroup`
**Arrange:**
- Creates a user.

**Act:** Attempts to create GlobalRoleBindings with both `userId` and `groupPrincipalId` set, and with neither set.

**Assert:**
- Checks both attempts fail with 422 Unprocessable Entity.
