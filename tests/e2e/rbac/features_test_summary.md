# `features_test.go` Summary

Verifies that only admins can mutate Feature resources and that standard users can list them once RBAC propagates.

## `TestCannotCreateFeature`
**Arrange:**
- Creates a standard user with the "user" global role.

**Act:** Attempts to create a Feature resource as both the admin and the standard user.

**Assert:**
- Checks the admin's attempt fails with 405 Method Not Allowed.
- Checks the standard user's attempt also fails with 405 Method Not Allowed.

## `TestCanListFeatures`
**Arrange:**
- Creates a standard user with the "user" global role.

**Act:** Lists Features as both the standard user (waiting for RBAC propagation) and the admin.

**Assert:**
- Checks the standard user can eventually list features once RBAC propagates, and the list is non-empty.
- Checks the admin can list a non-empty set of features.
