# `features_test.go` Summary

Verifies that only admins can mutate Feature resources and that standard users can list them once RBAC propagates.

## `TestCannotCreateFeature`
Creates a standard user and attempts to create a Feature resource as both admin and the standard user.
- Checks that both the admin and standard user receive a 405 Method Not Allowed response.

## `TestCanListFeatures`
Creates a standard user and waits for the "user" role's RBAC permissions to propagate.
- Checks that the standard user can list features once RBAC propagates.
- Checks that the admin can list features.
