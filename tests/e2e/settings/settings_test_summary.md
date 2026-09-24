# `settings_test.go` Summary

Verifies that settings can be created, read, updated, and deleted according to their access constraints, and that read-only settings are properly protected.

## `TestCreateReadOnly`
Attempts to create a read-only "cacerts" setting with a value.
- Checks the operation is rejected with 405 Method Not Allowed.
- Checks the error message contains "readOnly".

## `TestUpdateReadOnly`
Retrieves the read-only "cacerts" setting and attempts to update its value.
- Checks the operation is rejected with 405 Method Not Allowed.
- Checks the error message contains "readOnly".

## `TestGetReadOnly`
Retrieves the read-only "cacerts" setting by ID.
- Checks the setting can be successfully retrieved.

## `TestDeleteReadOnly`
Retrieves the read-only "cacerts" setting and attempts to delete it.
- Checks the operation is rejected with 405 Method Not Allowed.
- Checks the error message contains "readOnly".

## `TestCreate`
Creates a new setting with name "samplesetting-*" and value "a".
- Checks the setting is successfully created.
- Checks the returned setting has value "a".

## `TestCreateExisting`
Creates a setting with a specific name, then attempts to create another setting with the same name.
- Checks the second creation fails with 409 Conflict.
- Checks the error message contains "AlreadyExists".

## `TestUpdate`
Creates a setting with value "a", then updates it to value "b".
- Checks the update succeeds.
- Checks the returned setting has value "b".

## `TestUpdateNonExisting`
Sends a PUT request to update a setting that does not exist.
- Checks the response status is 404 Not Found.

## `TestUpdateLink`
Creates a setting and verifies the "update" action link is visible to an admin but not to a standard user.
- Checks the admin user sees the update link on the setting.
- Checks a standard user without admin privileges does not see the update link.
