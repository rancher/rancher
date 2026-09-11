Overview of HTTP Proxy:

- signers & injection
- credentials, secrets, providing passwords
- TLS and cert handling

- With a self-signed certificate (for mTLS?):

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



## Quick Start

1. **New to ProxyEndpoint?** Start with [quick-reference.md](quick-reference.md)
2. **Configuring TLS?** See [tls-configuration.md](tls-configuration.md)
3. **Security concerns?** Review [security-controls.md](security-controls.md)
4. **Implementation details?** Check [cert-management-developer.md](cert_management_developer.md)
| **API Reference** | [api-reference.md](api_reference.md) | Complete field-level documentation |
| **Implementation** | [implementation-summary.md](implementation_summary.md) | Technical implementation details |
