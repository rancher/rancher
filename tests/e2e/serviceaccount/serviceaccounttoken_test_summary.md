# `serviceaccounttoken_test.go` Summary

Verifies that concurrent requests to ensure a service account secret do not create duplicate secrets.

## `TestSingleSecretForServiceAccount`
**Arrange:**
- Gets a Kubernetes clientset for the local cluster.
- Creates a namespace.
- Creates a service account in that namespace.

**Act:** Calls `EnsureSecretForServiceAccount` 10 times concurrently for the same service account.

**Assert:**
- Checks only 1 secret is created in the namespace despite the 10 concurrent calls.
