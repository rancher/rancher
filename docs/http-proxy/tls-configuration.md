# TLS and Certificate Configuration Guide

This guide covers how to configure TLS and certificate options for ProxyEndpoint routes.

## Quick Examples

### Self-Signed Endpoint with Custom CA

For endpoints using self-signed certificates, provide the CA certificate in PEM format:

```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: self-signed-api
spec:
  routes:
    - domain: api.internal.example.com
      caBundle: |
        -----BEGIN CERTIFICATE-----
        MIIDXTCCAkWgAwIBAgIJAJC1/iNAZwqDMA0GCSqGSIb3DQEBCwUAMEUxCzAJBgNV
        ... (certificate data) ...
        -----END CERTIFICATE-----
```

### Load Balancer with SNI

When the request hostname differs from the backend certificate name:

```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: lb-with-sni
spec:
  routes:
    - domain: api.example.com
      serverName: backend.internal.example.com
```

### Mutual TLS (mTLS)

For endpoints requiring client certificates:

```yaml
apiVersion: management.cattle.io/v3
kind: ProxyEndpoint
metadata:
  name: mtls-endpoint
  namespace: cattle-system
spec:
  routes:
    - domain: secure-api.example.com
      clientCertificate:
        name: client-tls-secret
```

The Secret must be type `kubernetes.io/tls` with `tls.crt` and `tls.key` keys.

### Development with Disabled Hostname Verification

For development environments only:

```yaml
spec:
  routes:
    - domain: dev-api.local
      tlsVerificationOptions:
        verifyHostname: false
```

## Field Reference

| Field | Type | Required | Max Length | Description |
|-------|------|----------|-----------|-------------|
| `caBundle` | string | No | 100,000 | PEM-encoded CA certificates |
| `serverName` | string | No | 253 | SNI hostname for TLS handshake |
| `clientCertificate.name` | string | No | — | Name of Kubernetes Secret with client cert |
| `tlsVerificationOptions.verifyHostname` | bool | No | — | Enable/disable hostname verification (default: true) |
| `tlsVerificationOptions.verifyExpiration` | bool | No | — | Enable/disable expiration checking (default: true) |

## Validation Rules

- `caBundle` and `insecureSkipTLSVerify: true` are mutually exclusive
- `clientCertificate` must reference an existing Secret in the same namespace
- Secret type must be `kubernetes.io/tls` for `clientCertificate`
- `serverName` must be a valid DNS hostname (≤253 chars)
- All CA bundles must contain at least one CERTIFICATE PEM block
- CA bundles must not contain private key material

## Security Considerations

- **Never include private keys in CA bundles** — use `clientCertificate` with Kubernetes Secrets
- **Keep hostname verification enabled in production** — only disable for development
- **Use valid PEM format** — malformed PEM will be rejected
- **Minimize bundle size** — include only necessary CA certificates
- See [Security Controls](security-controls.md) for detailed runtime safeguards

## Troubleshooting

**"Certificate verification failed"** → Endpoint cert not signed by bundle CA; verify CA is in PEM format

**"Hostname verification failed"** → Certificate CN/SAN doesn't match request host; use `serverName` or disable `verifyHostname` (dev only)

**"CA bundle exceeds maximum size"** → Bundle larger than 100,000 bytes; remove unnecessary certs

**"CA bundle must not contain private key material"** → Remove private key; use `clientCertificate` field

**"CA bundle must contain at least one CERTIFICATE PEM block"** → Verify bundle contains CERTIFICATE blocks

## Related

- [Security Controls](security-controls.md) — Runtime safeguards for certificate data
- [Quick Reference](quick-reference.md) — Field summary and patterns

