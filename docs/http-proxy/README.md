# HTTP Proxy Documentation

This directory contains comprehensive documentation for Rancher's HTTP proxy, certificate management, and TLS configuration for ProxyEndpoint routes.

## Documentation Files

### [quick-reference.md](quick-reference.md)
Quick lookup guide for ProxyEndpointRoute fields, with common patterns and examples.

**Topics:**
- Core route field (`domain`)
- TLS/certificate options
- Request credential injection
- Validation rules
- Common configuration examples

### [security-controls.md](security-controls.md)
Runtime security safeguards for certificate and PEM data handling.

**Topics:**
- CA bundle size limits (100,000 bytes max)
- Private key rejection
- Certificate block requirements
- PEM format validation
- Best practices
- Security control tests

### [tls-configuration.md](tls-configuration.md)
Detailed guide for configuring TLS and certificate options in ProxyEndpoint routes.

**Topics:**
- Self-signed endpoint setup
- Load balancer SNI configuration
- Mutual TLS (mTLS) setup
- Development hostname verification control
- Field reference table
- Validation rules
- Troubleshooting guide

### [api-reference.md](api_reference.md)
Complete API reference for ProxyEndpointRoute certificate and TLS fields.

**Topics:**
- `caBundle` field specification
- `clientCertificate` field specification
- `serverName` field specification
- `tlsVerificationOptions` field specification
- Complete YAML examples
- Validation rules

### [cert-management-developer.md](cert_management_developer.md)
Developer documentation for certificate management implementation.

**Topics:**
- Architecture overview
- Implementation details
- Function flow diagrams
- Testing strategy
- Future enhancements

### [implementation-summary.md](implementation_summary.md)
Summary of certificate management implementation changes.

**Topics:**
- CRD type definitions
- HTTP proxy implementation
- CRD YAML schema updates
- Test suite details
- Backward compatibility

### [certificate-trust-management.md](certificate-trust-management.md)
User-facing documentation on certificate and trust management features.

**Topics:**
- Feature overview
- Configuration examples
- Precedence rules
- Security best practices
- Troubleshooting

## Quick Start

1. **New to ProxyEndpoint?** Start with [quick-reference.md](quick-reference.md)
2. **Configuring TLS?** See [tls-configuration.md](tls-configuration.md)
3. **Security concerns?** Review [security-controls.md](security-controls.md)
4. **Implementation details?** Check [cert-management-developer.md](cert_management_developer.md)

## Common Tasks

### Enable mTLS for a ProxyEndpoint Route

1. Create a Kubernetes Secret with your client certificate:
   ```bash
   kubectl create secret tls client-cert --cert=cert.pem --key=key.pem -n cattle-system
   ```

2. Add client certificate to route:
   ```yaml
   clientCertificate:
     name: client-cert
   ```

See [tls-configuration.md](tls-configuration.md#mutual-tls-mtls) for details.

### Handle Self-Signed Certificates

Add the CA certificate in PEM format to the `caBundle` field:

```yaml
caBundle: |
  -----BEGIN CERTIFICATE-----
  ... certificate content ...
  -----END CERTIFICATE-----
```

See [tls-configuration.md](tls-configuration.md#self-signed-endpoint-with-custom-ca) for details.

### Configure SNI for Load Balancers

Use the `serverName` field to specify the TLS handshake hostname:

```yaml
serverName: backend.internal.example.com
```

See [tls-configuration.md](tls-configuration.md#load-balancer-with-sni) for details.

## Testing

Run certificate security tests:

```bash
go test ./pkg/httpproxy -run 'TestBuildTLSConfigForRoute'
```

Run all proxy tests:

```bash
go test ./pkg/httpproxy
```

## Related Resources

- Rancher ProxyEndpoint API: `pkg/apis/management.cattle.io/v3/proxy_types.go`
- HTTP proxy implementation: `pkg/httpproxy/proxy.go`
- Certificate management tests: `pkg/httpproxy/cert_management_test.go`
- CRD schema: `pkg/crds/yaml/generated/management.cattle.io_proxyendpoints.yaml`

