# `global_roles_test.go` Summary

Verifies access control on global roles, global role bindings, and the visibility differences between the "user" and "user-base" global roles.

## `TestUserVsUserBaseGlobalRoleVisibility`
Creates two users—one with the "user" role and one with "user-base"—plus two extra users to pad the count, and compares their visibility of users and role templates.
- Checks the admin sees at least 5 users.
- Checks the "user" role user sees only themselves.
- Checks the "user" role user can list all role templates once RBAC propagates.
- Checks the "user-base" role user sees only themselves.
- Checks the "user-base" role user cannot see any role templates.

## `TestKontainerDriverVisibilityByGlobalRole`
Creates users with "user", "clusters-create", "kontainerdrivers-manage", and "settings-manage" roles, checking their visibility of kontainer drivers.
- Checks users with "user", "clusters-create", and "kontainerdrivers-manage" roles see 3 kontainer drivers each.
- Checks users with "settings-manage" role see 0 kontainer drivers.

## `TestBuiltinGlobalRoleOnlyNewUserDefaultEditable`
Retrieves the "admin" builtin global role and attempts to update multiple fields.
- Checks the builtin role has no remove link.
- Checks only newUserDefault changes; name, rules, and builtin flag remain unchanged.

## `TestOnlyAdminCanCRUDGlobalRoles`
Creates a non-builtin global role and attempts CRUD operations as both admin and standard user.
- Checks admin can create, update, list, and delete non-builtin global roles.
- Checks standard user receives 403 Forbidden for all CRUD operations.
- Checks standard user sees no global roles when listing.

## `TestAdminCannotDeleteBuiltinGlobalRole`
Attempts to delete a builtin global role and update the role itself.
- Checks builtin role has no remove link.
- Checks a newly created global role ignores builtin=true (the created role is not builtin).
- Checks admin can update the builtin role.
- Checks admin receives 403 Forbidden when attempting to delete the builtin role.
- Checks the error message contains "cannot delete builtin global roles".

## `TestGRBCannotUpdateGlobalRoleID`
Creates a GlobalRoleBinding with globalRoleId "nodedrivers-manage" and attempts to change it to "settings-manage".
- Checks the globalRoleId remains "nodedrivers-manage" after update.

## `TestGRBGlobalRoleMustExist`
Attempts to create a GlobalRoleBinding referencing a non-existent global role "somefakerole".
- Checks the request fails with 404 Not Found.

## `TestGRBCannotUpdateSubject`
Creates a GlobalRoleBinding with user1 and attempts to change both userId and groupPrincipalId.
- Checks userId remains unchanged when attempting update to user2.
- Checks groupPrincipalId stays empty when attempting to set it.

## `TestGRBTargetsUserOrGroup`
Attempts to create GlobalRoleBindings with invalid subject combinations: both userId and groupPrincipalId, and neither.
- Checks both requests fail with 422 Unprocessable Entity.
