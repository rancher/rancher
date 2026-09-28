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
