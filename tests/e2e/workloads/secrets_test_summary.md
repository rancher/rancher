# `secrets_test.go` Summary

Verifies project-scoped secret resources in the Norman API, covering create/update/list/delete for Opaque secrets, certificates, docker credentials, basic-auth secrets, and SSH auth secrets, plus retrieval of TLS secrets created directly via the Kubernetes API.

## `TestSecrets`
**Arrange:**
- Creates a project.

**Act:** Creates a project-scoped Opaque secret with stringData `{"foo": "bar"}`.

**Assert:**
- Checks the secret is created with `type` "secret", `kind` "Opaque", and `data` base64-encoded.
- Checks a second data key ("baz") can be added via update and persists.
- Checks `projectId` is stored and `namespaceId` is nil.
- Checks the secret appears in the list and can be deleted.

## `TestCertificates`
**Arrange:**
- Creates a project.

**Act:** Creates a project-scoped certificate with certificate and key PEM data.

**Assert:**
- Checks the certificate is created with `type` "certificate" and `expiresAt` set to a valid date.
- Checks the certificate appears in the list and can be fetched by ID.
- Checks it can be deleted.

## `TestDockerCredential`
**Arrange:**
- Creates a project.

**Act:** Creates a project-scoped docker credential with registry credentials for index.docker.io.

**Assert:**
- Checks the credential is created with `type` "dockerCredential" and registries containing the username plus a computed `auth` field.
- Checks a second registry can be added via update.
- Checks the `password` field is write-only (absent after reload).
- Checks the credential appears in the list and can be deleted.

## `TestBasicAuth`
**Arrange:**
- Creates a project.

**Act:** Creates a project-scoped basic-auth secret with username "foo" and a password.

**Assert:**
- Checks the secret is created with `type` "basicAuth" and the provided username.
- Checks the username can be updated to "foo2" and persists.
- Checks the `password` field is write-only (absent after reload).
- Checks the secret appears in the list and can be deleted.

## `TestSSHAuth`
**Arrange:**
- Creates a project.

**Act:** Creates a project-scoped SSH auth secret with a private key.

**Assert:**
- Checks the secret is created with `type` "sshAuth".
- Checks the private key can be updated to a different value.
- Checks the `privateKey` field is write-only (absent after reload).
- Checks the secret appears in the list and can be deleted.

## `TestSecretCreationKubectl`
**Arrange:**
- Creates a project and a namespace.

**Act:** Creates a TLS secret directly via the Kubernetes API.

**Assert:**
- Checks the secret is retrievable through the project API as a namespacedCertificate with ID `<namespace>:<name>`.
- Checks the certificate's `algorithm` contains "RSA" and `expiresAt`/`issuedAt` are set.

## `TestMalformedSecretParse`
**Arrange:**
- Creates a project and a namespace.

**Act:** Creates a TLS secret with a malformed certificate directly via the Kubernetes API.

**Assert:**
- Checks the malformed secret can still be retrieved as a namespacedCertificate through the project API, returning a non-empty response.
