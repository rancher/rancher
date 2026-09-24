# `users_test.go` Summary

Verifies that user account security constraints are enforced for self-modification and password policies.

## `TestUserCantDeleteSelf`
Retrieves the current admin user and attempts to delete it.
- Checks the deletion is rejected with 422 Unprocessable Entity.

## `TestUserCantDeactivateSelf`
Retrieves the current admin user and attempts to update it with enabled=false.
- Checks the update is rejected with 422 Unprocessable Entity.

## `TestUserCantUseUsernameAsPassword`
Attempts to create a user with username "administrator" and password "administrator".
- Checks the creation is rejected with 422 Unprocessable Entity.

## `TestPasswordTooShort`
Attempts to create a user with password "tooshort" (8 characters).
- Checks the creation is rejected with 422 Unprocessable Entity.
