# `secrets_test.go` Summary

Verifies project-scoped secret resources in the Norman API, testing that Opaque secrets, certificates, docker credentials, basic-auth secrets, and SSH auth secrets can be created, updated, listed, and retrieved by ID at the project level, and that TLS secrets created directly via Kubernetes API are accessible through the project API.

## `TestSecrets`
Creates a project-scoped Opaque secret with stringData key "foo": "bar", updates it by adding a second key, retrieves it by ID and via listing, and deletes it.
- Checks the secret is created with `type` "secret" and `kind` "Opaque", with data base64-encoded.
- Checks the secret can be updated to add a second key "baz" to data.
- Checks the secret persists in list and can be deleted.
- Checks projectId is stored and namespaceId is nil.

## `TestCertificates`
Creates a project-scoped certificate with certificate and key PEM data, lists it, retrieves it by ID, and deletes it.
- Checks the certificate is created with `type` "certificate" and `expiresAt` set to a valid date.
- Checks the certificate appears in the list with a valid ID.
- Checks it can be deleted.

## `TestDockerCredential`
Creates a project-scoped docker credential with registries for index.docker.io, updates it by adding a second registry, retrieves it by ID and via listing, and deletes it.
- Checks the credential is created with `type` "dockerCredential" and registries containing username and calculated auth fields.
- Checks a second registry can be added via update.
- Checks password is write-only (not present after reload).
- Checks the credential persists in list and can be deleted.

## `TestBasicAuth`
Creates a project-scoped basic-auth secret with username "foo" and password, updates the username to "foo2", retrieves it by ID and via listing, and deletes it.
- Checks the secret is created with `type` "basicAuth" and the provided username.
- Checks username can be updated and persists.
- Checks password is write-only (not present after reload).
- Checks the secret persists in list and can be deleted.

## `TestSSHAuth`
Creates a project-scoped SSH auth secret with a private key, updates it with a new key value, retrieves it by ID and via listing, and deletes it.
- Checks the secret is created with `type` "sshAuth".
- Checks the private key can be updated to a different value.
- Checks privateKey is write-only (not present after reload).
- Checks the secret persists in list and can be deleted.

## `TestSecretCreationKubectl`
Creates a TLS secret directly via the Kubernetes API and retrieves it as a namespacedCertificate through the Rancher project API.
- Checks the secret is accessible via the project API as a namespacedCertificate with correct ID format (namespace:name).
- Checks the certificate has an algorithm containing "RSA" and valid expiresAt and issuedAt fields.

## `TestMalformedSecretParse`
Creates a TLS secret with a malformed certificate directly via the Kubernetes API and verifies it can still be retrieved as a namespacedCertificate through the Rancher project API.
- Checks the malformed secret does not cause errors and returns a non-empty response.
