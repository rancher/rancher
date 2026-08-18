# Quick Reference: ProxyEndpoint Route Fields

This document is a concise reference for the `ProxyEndpointRoute` spec in
`pkg/apis/management.cattle.io/v3/proxy_types.go`.

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


