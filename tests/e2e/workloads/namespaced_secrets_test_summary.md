# `namespaced_secrets_test.go` Summary

Verifies namespaced secret resources in the Norman API, covering create/update/list/delete for Opaque secrets, TLS certificates, docker credentials, basic-auth secrets, and SSH auth secrets scoped to a namespace.

## `TestNamespacedSecrets`
**Arrange:**
- Creates a project and a namespace.

**Act:** Creates a namespaced Opaque secret with stringData `{"foo": "bar"}`.

**Assert:**
- Checks the secret is created with `baseType` "namespacedSecret", `kind` "Opaque", and `data` base64-encoded.
- Checks a second data key ("baz") can be added via update and persists.
- Checks both `projectId` and `namespaceId` are stored.
- Checks the secret appears in the list and can be deleted.

## `TestNamespacedCertificates`
**Arrange:**
- Creates a project and a namespace.

**Act:** Creates a namespaced TLS certificate secret with certificate and key PEM data.

**Assert:**
- Checks the secret is created with `baseType` "namespacedSecret" and `type` "namespacedCertificate".
- Checks the certificate field can be updated to a different value and persists.
- Checks the secret appears in the list and can be deleted.

## `TestNamespacedDockerCredential`
**Arrange:**
- Creates a project and a namespace.

**Act:** Creates a namespaced docker credential with registry credentials for index.docker.io.

**Assert:**
- Checks the credential is created with `type` "namespacedDockerCredential" and the provided registry data.
- Checks a second registry can be added via update.
- Checks the `password` field is write-only (absent after reload).
- Checks the credential appears in the list and can be deleted.

## `TestNamespacedBasicAuth`
**Arrange:**
- Creates a project and a namespace.

**Act:** Creates a namespaced basic-auth secret with username "foo" and a password.

**Assert:**
- Checks the secret is created with `type` "namespacedBasicAuth" and the provided username.
- Checks the username can be updated to "foo2" and persists.
- Checks the `password` field is write-only (absent after reload).
- Checks the secret appears in the list and can be deleted.

## `TestNamespacedSSHAuth`
**Arrange:**
- Creates a project and a namespace.

**Act:** Creates a namespaced SSH auth secret with a private key.

**Assert:**
- Checks the secret is created with `type` "namespacedSshAuth".
- Checks the private key can be updated to a different value.
- Checks the `privateKey` field is write-only (absent after reload).
- Checks the secret appears in the list and can be deleted.
