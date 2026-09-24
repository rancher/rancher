# `namespaced_secrets_test.go` Summary

Verifies namespaced secret resources in the Norman API, testing that Opaque secrets, TLS certificates, docker credentials, basic-auth secrets, and SSH auth secrets can be created, updated, listed, and retrieved by ID within a namespace.

## `TestNamespacedSecrets`
Creates a namespaced Opaque secret with stringData key "foo": "bar", updates it by adding a second key, retrieves it by ID and via listing, and deletes it.
- Checks the secret is created with `baseType` "namespacedSecret" and `kind` "Opaque", with data base64-encoded.
- Checks the secret can be updated to add a second data key "baz".
- Checks the secret persists in list and can be deleted.
- Checks projectId and namespaceId are both stored.

## `TestNamespacedCertificates`
Creates a namespaced TLS certificate secret with certificate and key PEM data, updates the certificate field, retrieves it by ID and via listing, and deletes it.
- Checks the secret is created with `baseType` "namespacedSecret" and `type` "namespacedCertificate".
- Checks the certificate field can be updated to a different certificate value and persists.
- Checks the secret appears in the list and can be deleted.

## `TestNamespacedDockerCredential`
Creates a namespaced docker credential with registry credentials for index.docker.io, updates it by adding a second registry entry, and retrieves, lists, and deletes it.
- Checks the credential is created with `type` "namespacedDockerCredential" and the provided registries structure.
- Checks a second registry can be added via update.
- Checks password field is write-only (not present after reload).
- Checks the credential persists in list and can be deleted.

## `TestNamespacedBasicAuth`
Creates a namespaced basic-auth secret with username "foo" and password, updates the username to "foo2", retrieves it by ID and via listing, and deletes it.
- Checks the secret is created with `type` "namespacedBasicAuth" and the provided username.
- Checks username can be updated to "foo2" and persists.
- Checks password is write-only (not present after reload).
- Checks the secret persists in list and can be deleted.

## `TestNamespacedSSHAuth`
Creates a namespaced SSH auth secret with a private key, updates it with a new private key value, retrieves it by ID and via listing, and deletes it.
- Checks the secret is created with `type` "namespacedSshAuth".
- Checks the private key can be updated to a different value.
- Checks privateKey field is write-only (not present after reload).
- Checks the secret persists in list and can be deleted.
