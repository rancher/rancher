# ProxyEndpoint Certificate Management - Developer Guide

## Architecture Overview

The certificate management features enable fine-grained TLS control for ProxyEndpoint routes.

### Type Definitions

**ProxyEndpointRoute** fields:
- `CABundle`: PEM-encoded CA certificates (max 100KB)
- `ClientCertificate`: Reference to mTLS client certificate Secret
- `ServerName`: SNI hostname for TLS handshake
- `TLSVerificationOptions`: Control certificate verification

**SecretReference**:
```go
type SecretReference struct {
    Name string  // Secret name in same namespace
}
```

**TLSVerificationSpec**:
```go
type TLSVerificationSpec struct {
    VerifyHostname   *bool  // Skip hostname verification if false
    VerifyExpiration *bool  // For future use
}
```

## Implementation Flow

### Request Processing

```
RoundTrip()
  → findMatchingRoute() - locate best match
  → Check InsecureSkipTLSVerify - highest priority
  → Check custom cert options - build transport
  → Default - use http.DefaultTransport
```

### TLS Configuration Building

```
buildTransportForRoute()
  → buildTLSConfigForRoute()
    → Set root CAs (from CABundle or system)
    → Configure SNI (ServerName)
    → Apply verification options
  → Clone default transport
  → Apply TLSClientConfig
```

## Key Functions

### buildTLSConfigForRoute

Creates crypto/tls.Config with:
- Custom CA certificate pool (if provided)
- SNI hostname (or request hostname)
- Verification settings (VerifyHostname)

### parseCACertificates

Parses PEM-encoded CA bundle, combining:
- User-provided certificates
- System root CAs (supplementary)

### findMatchingRoute

Selects best route by specificity:
- Score 3: Exact domain match
- Score 2: Single-segment wildcard (%)
- Score 1: Prefix wildcard (*)
- Higher score or longer pattern wins

## Testing

Test file: `cert_management_test.go`

Coverage includes:
- TLS config with all option combinations
- Transport creation and cloning
- SNI hostname handling
- Verification options
- Integration with real TLS servers

**Run tests:**
```bash
go test ./pkg/httpproxy -v -run "TestBuild|TestVerify"
```

## Security

- CA bundles provide explicit trust only
- System root CAs always included
- Hostname verification enabled by default
- Private keys never persisted by proxy

## Future Work

1. Client certificate loading from Secrets
2. Certificate and transport caching
3. Certificate pinning support
4. Custom verification callbacks

