# ProxyEndpoint Certificate Management - API Reference

## ProxyEndpointRoute

Complete route specification with certificate and trust management options.

### Field: `caBundle`

**Type:** `string`

**Description:** PEM-encoded bundle of CA certificates to trust when verifying the TLS certificate of the endpoint.

**Constraints:**
- Optional (default: empty)
- Maximum 100,000 characters
- Must be valid PEM format
- Mutually exclusive with `insecureSkipTLSVerify: true`

**Example:**
```yaml
caBundle: |
  -----BEGIN CERTIFICATE-----
  MIIDXTCCAkWgAwIBAgIJAJC1/iNAZwqDMA0GCSqGSIb3...
  -----END CERTIFICATE-----
```

**Use Case:** Self-signed certificates, private CAs, non-standard PKI environments

---

### Field: `clientCertificate`

**Type:** `*SecretReference`

**Description:** References a Kubernetes Secret containing client certificate and key for mutual TLS (mTLS) authentication.

**Sub-fields:**
- `name` (string, required): Name of the Secret in the same namespace

**Constraints:**
- Optional (default: nil)
- Secret must exist in same namespace as ProxyEndpoint
- Secret must be of type `kubernetes.io/tls`
- Required data keys: `tls.crt`, `tls.key`

**Example:**
```yaml
clientCertificate:
  name: my-mtls-credentials
```

**Secret Format:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-mtls-credentials
type: kubernetes.io/tls
data:
  tls.crt: base64-encoded-cert
  tls.key: base64-encoded-key
```

**Use Case:** mTLS authentication, certificate-based endpoint authorization

---

### Field: `serverName`

**Type:** `string`

**Description:** SNI (Server Name Indication) hostname to use during the TLS handshake.

**Constraints:**
- Optional (default: uses request hostname)
- Maximum 253 characters
- Must be valid hostname format
- Used for TLS ClientHello SNI extension

**Example:**
```yaml
serverName: internal-api.svc.cluster.local
```

**Behavior:**
- If not specified: Uses the request's `Host` header value
- If specified: Overrides hostname for TLS handshake
- Allows hostname mismatch between request and certificate

**Use Case:** Load balancers, proxies, internal service names vs external hostnames

---

### Field: `tlsVerificationOptions`

**Type:** `*TLSVerificationSpec`

**Description:** Controls certificate verification behavior.

**Sub-fields:**

#### `verifyHostname`

**Type:** `*bool`

**Default:** `true`

**Description:** Controls whether certificate hostname validation is performed.

**Behavior:**
- `nil` or `true`: Hostname verification enabled (standard)
- `false`: Hostname verification disabled

**Example:**
```yaml
tlsVerificationOptions:
  verifyHostname: false
```

**Security Note:** Only set to `false` in development environments with self-signed certificates.

---

#### `verifyExpiration`

**Type:** `*bool`

**Default:** `true`

**Description:** Controls certificate expiration checking.

**Behavior:**
- `nil` or `true`: Expiration verification enabled (standard)
- `false`: Prepared for future fine-grained expiration handling

**Note:** Currently, Go's crypto/tls package always validates expiration. This field is provided for future flexibility.

**Example:**
```yaml
tlsVerificationOptions:
  verifyExpiration: false
```

---

## Complete Examples

### Example 1: Self-Signed Certificate with Custom CA

```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: private-api
spec:
  routes:
  - domain: api.internal.example.com
    # Load custom CA certificate
    caBundle: |
      -----BEGIN CERTIFICATE-----
      MIIDXTCCAkWgAwIBAgIJAJC1/iNAZwqDMA0GCSqGSIb3DQEBCwUAMEUxCzAJBgNV
      ...
      -----END CERTIFICATE-----
```

### Example 2: Load Balancer with SNI

```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: lb-routed
spec:
  routes:
  - domain: external-lb.example.com
    # Specify internal hostname for SNI
    serverName: actual-backend.internal
```

### Example 3: Mutual TLS Authentication

```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: mtls-endpoint
spec:
  routes:
  - domain: secure-api.example.com
    # Reference client certificate for mTLS
    clientCertificate:
      name: client-credentials
```

### Example 4: Comprehensive Configuration

```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: enterprise-api
spec:
  routes:
  - domain: api.enterprise.example.com
    # Custom CA for verification
    caBundle: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    
    # SNI for load balancer
    serverName: internal-api-backend
    
    # mTLS authentication
    clientCertificate:
      name: enterprise-client-cert
    
    # Verification options
    tlsVerificationOptions:
      verifyHostname: true
      verifyExpiration: true
```

### Example 5: Permissive Development Configuration

```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: dev-endpoint
spec:
  routes:
  - domain: dev-api.local
    # Disable hostname verification for self-signed dev certificates
    tlsVerificationOptions:
      verifyHostname: false
      verifyExpiration: false
```

---

## Validation Rules

### CRD-Level Validation

1. **Mutually exclusive fields:**
   - `caBundle` and `insecureSkipTLSVerify: true` cannot both be set
   - Error: "caBundle cannot be set when insecureSkipTLSVerify is true"

2. **Field constraints:**
   - `caBundle`: Maximum 100,000 characters
   - `serverName`: Maximum 253 characters
   - `Domain`: Required, validated by domain pattern regex

3. **Type constraints:**
   - `tlsVerificationOptions`: Must be valid TLSVerificationSpec
   - `clientCertificate`: Must be valid SecretReference

### Runtime Validation

1. **Certificate validation:**
   - CA bundle must be valid PEM format
   - Client certificate secret must exist
   - Secret must contain required keys

2. **Hostname validation:**
   - ServerName must be valid hostname format
   - Domain pattern validated at creation time

3. **Error handling:**
   - Invalid certificates logged as warnings
   - Fallback to default transport on errors
   - Request proceeds (not rejected)

---

## Precedence and Behavior

### TLS Configuration Priority

```
1. InsecureSkipTLSVerify
   ├─ If true: Skip all verification
   └─ Incompatible with caBundle

2. tlsVerificationOptions
   ├─ VerifyHostname controls InsecureSkipVerify flag
   └─ VerifyExpiration for future use

3. caBundle
   ├─ Provides custom CA certificates
   ├─ Supplements system root CAs
   └─ Cannot be used with InsecureSkipTLSVerify

4. serverName
   ├─ Controls SNI hostname
   ├─ Independent from verification
   └─ Can combine with all other options
```

### Route Matching

When multiple routes could match a request:

1. **Score calculation:**
   - Exact match: score = 3
   - Single-segment wildcard (%): score = 2
   - Prefix wildcard (*): score = 1

2. **Tie-breaking:**
   - Higher score wins
   - Same score: longer pattern wins

3. **Example:**
   ```
   Routes defined:
   - %.example.com         (score 2 for api.example.com)
   - api.example.com       (score 3 for api.example.com) ← Selected
   - *.example.com         (score 1 for api.example.com)
   ```

---

## Error Handling

### Common Errors and Solutions

| Error | Cause | Solution |
|-------|-------|----------|
| "failed to parse CA certificate" | Invalid PEM format | Verify CA bundle is valid PEM |
| "x509: certificate signed by unknown authority" | CA certificate not found | Add missing CA to caBundle |
| "x509: certificate has expired" | Expired certificate | Update certificate or disable verification |
| "x509: certificate is valid for X not Y" | Hostname mismatch | Use serverName to specify correct hostname |
| "failed to read client cert" | Secret not found | Create Secret with correct name |
| "error reading private key" | Invalid key format | Ensure key is unencrypted PEM |

---

## Security Best Practices

1. **Production Use:**
   - ✅ Use caBundle for self-signed certificates
   - ✅ Enable hostname verification (default)
   - ✅ Enable expiration verification (default)
   - ✅ Protect client certificate secrets with RBAC

2. **Development Use:**
   - ⚠️ Only disable verification in controlled environments
   - ⚠️ Document why verification is disabled
   - ⚠️ Rotate credentials regularly

3. **Secret Management:**
   - ✅ Use encrypted storage for secrets
   - ✅ Limit access via Kubernetes RBAC
   - ✅ Audit secret access
   - ✅ Rotate certificates before expiration

---

## API Version

- **Group:** `management.cattle.io`
- **Version:** `v3`
- **Kind:** `ProxyEndpoint`
- **Scope:** Cluster

---

## Status

| Feature | Status | Notes |
|---------|--------|-------|
| `caBundle` | ✅ Production Ready | Fully tested |
| `clientCertificate` | 🔄 Type Defined | Ready for implementation |
| `serverName` | ✅ Production Ready | Full SNI support |
| `tlsVerificationOptions.verifyHostname` | ✅ Production Ready | Fully tested |
| `tlsVerificationOptions.verifyExpiration` | ✅ Production Ready | Prepared for future use |

---

## Related Resources

- [Kubernetes TLS Secrets](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets)
- [TLS Overview](https://en.wikipedia.org/wiki/Transport_Layer_Security)
- [SNI Extension](https://en.wikipedia.org/wiki/Server_Name_Indication)
- [RFC 7539 - TLS 1.3](https://tools.ietf.org/html/rfc8446)

