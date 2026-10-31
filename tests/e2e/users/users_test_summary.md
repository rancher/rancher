# `users_test.go` Summary

Verifies that user account security constraints are enforced for self-modification and password policies.

## `TestUserCantDeleteSelf`
**Arrange:**
- Retrieves the current admin user (the one the client is authenticated as).

**Act:** Attempts to delete that user.

**Assert:**
- Checks the deletion is rejected with 422 Unprocessable Entity.

## `TestUserCantDeactivateSelf`
**Arrange:**
- Retrieves the current admin user (the one the client is authenticated as).

**Act:** Attempts to update that user with `enabled=false`.

**Assert:**
- Checks the update is rejected with 422 Unprocessable Entity.

## `TestUserCantUseUsernameAsPassword`
**Act:** Attempts to create a user with username "administrator" and password "administrator".

**Assert:**
- Checks the creation is rejected with 422 Unprocessable Entity.

## `TestPasswordTooShort`
**Act:** Attempts to create a user with an 8-character password ("tooshort").

**Assert:**
- Checks the creation is rejected with 422 Unprocessable Entity.
</content>
</invoke>
