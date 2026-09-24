# `serviceaccounttoken_test.go` Summary

Verifies that concurrent requests to ensure a service account secret do not create duplicate secrets.

## `TestSingleSecretForServiceAccount`
Creates a namespace and service account, then calls the EnsureSecretForServiceAccount function 10 times concurrently from goroutines, and verifies exactly one secret is created.
- Checks only 1 secret is created in the namespace despite 10 concurrent calls.
