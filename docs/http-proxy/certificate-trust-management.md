# Certificate and Trust Management for ProxyEndpoint

This document describes the certificate and trust management features available for ProxyEndpoint routes in Rancher's HTTP proxy.

## Overview

ProxyEndpoint routes now support comprehensive certificate and trust management options, allowing fine-grained control over TLS connections to external services. This is particularly useful for:

- **Private Certificate Authorities (CA):** Using custom CA certificates to verify endpoints with self-signed or non-standard certificates
- **Server Name Indication (SNI):** Specifying the hostname for TLS handshakes when it differs from the request domain
- **Client Certificate Authentication:** Supporting mutual TLS (mTLS) for endpoints requiring client certificates
- **Certificate Verification Control:** Enabling or disabling hostname and expiration verification

## Features

### 1. Custom CA Bundle (`caBundle`)

Provides custom CA certificates to trust when verifying the TLS certificate of the endpoint.

**Use Cases:**
- Private Certificate Authorities
- Self-signed certificates
- Internal PKI environments

**Configuration:**
```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: example-endpoint
spec:
  routes:
  - domain: api.example.com
    caBundle: |
      -----BEGIN CERTIFICATE-----
      MIIDXTCCAkWgAwIBAgIJAJC1/iNAZwqDMA0GCSqGSIb3...
      -----END CERTIFICATE-----
```

**Behavior:**
- The provided certificates are used in addition to system root CAs
- Incompatible with `insecureSkipTLSVerify: true`
- Maximum size: 100,000 characters (PEM-encoded)

### 2. Server Name Indication (SNI) (`serverName`)

Specifies the hostname to use during the TLS handshake via SNI.

**Use Cases:**
- Endpoints where the serving domain differs from the certificate CN/SAN
- Load balancers or proxies that host multiple certificates
- Internal service names vs external hostnames

**Configuration:**
```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: example-endpoint
spec:
  routes:
  - domain: external-lb.example.com
    serverName: internal-svc.internal.svc.cluster.local
```

**Behavior:**
- If not specified, the request hostname is used for SNI
- Allows mismatch between the domain and certificate hostname
- Maximum length: 253 characters

### 3. Client Certificate for mTLS (`clientCertificate`)

References a Kubernetes Secret containing client certificate and key for mutual TLS authentication.

**Use Cases:**
- Endpoints requiring client certificate authentication
- Mutual TLS (mTLS) secured services
- Certificate-based authorization

**Configuration:**
```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: example-endpoint
spec:
  routes:
  - domain: api.example.com
    clientCertificate:
      name: my-client-cert-secret
```

**Secret Format:**
The referenced Kubernetes Secret must contain:
- `tls.crt`: PEM-encoded client certificate
- `tls.key`: PEM-encoded private key (PKCS#8 or PKCS#1 format)

**Example Secret:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-client-cert-secret
type: kubernetes.io/tls
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t...
  tls.key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVkt...
```

**Behavior:**
- Secret must exist in the same namespace as the ProxyEndpoint
- Credentials are refreshed on each request by reading the current secret state
- Private key must not be encrypted (no passphrase)

### 4. TLS Verification Options (`tlsVerificationOptions`)

Controls certificate verification behavior for enhanced flexibility.

**Configuration:**
```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: example-endpoint
spec:
  routes:
  - domain: api.example.com
    tlsVerificationOptions:
      verifyHostname: false          # Skip hostname verification
      verifyExpiration: false        # Skip expiration check
```

**Options:**

#### `verifyHostname` (boolean, default: true)
Controls whether the certificate's hostname matches the connection hostname.

- `true`: Hostname verification is enabled (default)
- `false`: Hostname verification is disabled (only used with self-signed certs in dev environments)

**Note:** Setting to `false` has the same effect as `insecureSkipTLSVerify: true`

#### `verifyExpiration` (boolean, default: true)
Controls whether the certificate's validity period is checked.

- `true`: Expiration verification is enabled (default)
- `false`: Expired certificates are accepted (not recommended for production)

**Note:** Expiration verification is handled by the underlying crypto/tls package and cannot be completely disabled programmatically. This field is provided for future flexibility and explicit configuration.

## Precedence Rules

When multiple TLS options are configured, the following precedence applies:

1. **`insecureSkipTLSVerify`** takes highest priority
   - If `true`, disables all TLS verification regardless of other settings
   - Incompatible with `caBundle` (validation webhook prevents both from being set)

2. **`tlsVerificationOptions`** supersedes individual verification settings
   - Provides fine-grained control over verification behavior

3. **`caBundle`** and **`serverName`** work together
   - Both can be specified simultaneously
   - No conflict between them

## Complete Example

```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: secure-external-api
spec:
  routes:
  - domain: external-api.example.com
    # Use custom CA for self-signed certificate
    caBundle: |
      -----BEGIN CERTIFICATE-----
      MIIDXTCCAkWgAwIBAgIJAJC1/iNAZwqDMA0GCSqGSIb3...
      -----END CERTIFICATE-----
    # Use SNI for load balancer
    serverName: api-internal.example.com
    # Reference client certificate for mTLS
    clientCertificate:
      name: api-client-credentials
    # Control verification behavior
    tlsVerificationOptions:
      verifyHostname: true
      verifyExpiration: true
```

## Security Considerations

### Best Practices

1. **Use CA Bundles for Self-Signed Certificates**
   - Never rely on `insecureSkipTLSVerify` in production
   - Always use `caBundle` with self-signed certificates in non-dev environments

2. **Protect Client Certificates**
   - Use Kubernetes RBAC to limit access to Secrets containing client certificates
   - Consider using encrypted storage backends for Secrets
   - Rotate client certificates regularly

3. **Enable Hostname Verification**
   - Leave `verifyHostname: true` unless required otherwise
   - Only disable in controlled, non-production environments

4. **Keep Certificates Updated**
   - Ensure `verifyExpiration: true` (default)
   - Implement certificate rotation procedures

### Limitations

- Client certificate secrets are read on each request (no caching)
- Private keys must not be encrypted (no passphrases)
- Maximum CA bundle size: 100,000 characters
- All connections use HTTPS (HTTP scheme will be converted)

## Troubleshooting

### Certificate Verification Failures

**Error:** `x509: certificate signed by unknown authority`
- **Solution:** Add the CA certificate to `caBundle`

**Error:** `x509: certificate has expired`
- **Solution:** Update the certificate or set `verifyExpiration: false` (not recommended)

**Error:** `x509: certificate is valid for ... not <hostname>`
- **Solution:** Use `serverName` to specify the correct hostname for SNI

### Client Certificate Issues

**Error:** `error reading client cert`
- **Solution:** Verify the Secret exists and contains `tls.crt` and `tls.key`

**Error:** `failed to load private key`
- **Solution:** Ensure the private key is unencrypted and in PEM format

## Future Enhancements

Planned features for certificate management:
- Client certificate rotation and refresh strategies
- Certificate pinning (pin-to-public-key)
- Certificate chain validation options
- Additional verification modes (mutual TLS certificate validation)

## API Reference

See `pkg/apis/management.cattle.io/v3/proxy_types.go` for detailed field definitions.

## Related Documentation

- [ProxyEndpoint CRD](./proxyendpoint-crd.md)
- [Rancher HTTP Proxy](./httpproxy.md)
- [Kubernetes TLS Secrets](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets)

