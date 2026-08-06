# Quick Reference: ProxyEndpoint Route Fields

This document is a comprehensive reference for ProxyEndpoint
configuration and the `ProxyEndpointRoute` spec in
`pkg/apis/management.cattle.io/v3/proxy_types.go`.


This is the primary reference for ProxyEndpoint configuration, including:
- ProxyEndpointRoute field definitions
- TLS and certificate options
- Validation rules
- Common configuration examples


## Documentation Overview

For detailed guides on specific topics, see:

| Topic | File | Purpose |
|-------|------|---------|
| **TLS Configuration** | [tls-configuration.md](tls-configuration.md) | Setup guides for CA bundles, SNI, mTLS |
| **Security Controls** | [security-controls.md](security-controls.md) | Runtime safeguards for certificate data |
| **API Reference** | [api-reference.md](api_reference.md) | Complete field-level documentation |
| **Implementation** | [implementation-summary.md](implementation_summary.md) | Technical implementation details |
| **Developer Guide** | [cert-management-developer.md](cert_management_developer.md) | Architecture and testing strategy |

## Common Tasks

**Configuring TLS for a route?**
→ See [tls-configuration.md](tls-configuration.md)

**Worried about certificate security?**
→ See [security-controls.md](security-controls.md)

**Implementing mTLS?**
→ See [tls-configuration.md#mutual-tls-mtls](tls-configuration.md#mutual-tls-mtls)

**Handling self-signed certs?**
→ See [tls-configuration.md#self-signed-endpoint-with-custom-ca](tls-configuration.md#self-signed-endpoint-with-custom-ca)

## Field Reference

## Core route field

- `domain` (`string`, required)
  - Domain or wildcard entry to add to the proxy allowlist.
  - Examples: `example.com`, `*.example.com`, `%.example.com`
  - Must not include a URL scheme such as `https://`

## TLS / certificate options

- `insecureSkipTLSVerify` (`bool`, optional)
  - Disables TLS certificate verification for the route.
  - Intended only for development or self-signed endpoints.

- `caBundle` (`string`, optional)
  - PEM-encoded CA bundle used to verify the endpoint certificate.
  - Maximum length: `100000` characters.
  - Cannot be set when `insecureSkipTLSVerify: true`.

- `clientCertificate` (`SecretReference`, optional)
  - References a Kubernetes Secret in the same namespace.
  - Secret must contain `tls.crt` and `tls.key`.
  - Type is defined, but runtime mTLS wiring is not implemented yet.

- `serverName` (`string`, optional)
  - SNI hostname used during the TLS handshake.
  - Maximum length: `253` characters.
  - Useful when the request host differs from the backend certificate name.

- `tlsVerificationOptions` (`TLSVerificationSpec`, optional)
  - `verifyHostname` (`bool`, default: `true`)
  - `verifyExpiration` (`bool`, default: `true`)

## Request credential injection

- `credentialInjection` (`CredentialInjectionSpec`, optional)
  - Server-side rules for applying credential secret values to proxied requests.
  - Supported modes:
    - `bearer`
    - `basic`
    - `headerinject`
    - `bodyinject`

## Validation rules

- `domain` is required and validated as a hostname / wildcard domain.
- `caBundle` and `insecureSkipTLSVerify: true` are mutually exclusive.
- `clientCertificate.name` must refer to a Secret in the same namespace.
- `clientCertificate` is currently not consumed by `pkg/httpproxy/proxy.go` transport logic.
- `verifyHostname` and `verifyExpiration` default to `true` when omitted.

## Common examples

### Self-signed endpoint with custom CA

```yaml
spec:
  routes:
    - domain: api.internal.example.com
      caBundle: |
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
```

### Planned mTLS endpoint with SNI (field defined)

```yaml
spec:
  routes:
    - domain: secure-api.example.com
      serverName: backend.internal.example.com
      clientCertificate:
        name: client-credentials
```

### Development-only insecure route

```yaml
spec:
  routes:
    - domain: dev-api.local
      insecureSkipTLSVerify: true
```

## Regenerate CRD schema

When the Go types change, regenerate the CRD manifests:

```bash
make generate-manifests
```


