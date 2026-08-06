# Security Controls for ProxyEndpoint Certificate Data

This document describes the runtime security safeguards applied to certificate and TLS configuration in Rancher's HTTP proxy.

## Overview

ProxyEndpoint routes support custom certificate configuration for connecting to external services. To prevent accidental or malicious misuse of sensitive certificate data, runtime security controls enforce strict validation on all user-provided PEM data.

## CA Bundle Security

### Size Limits

All CA bundles have a maximum size of **100,000 bytes**. This limit:
- Prevents denial-of-service attacks via unbounded memory allocation
- Aligns with the CRD schema max length constraint
- Allows typical multi-certificate chains while rejecting oversized data

**Enforcement:** Triggered during certificate parsing; requests with oversized bundles return an error with message: `"CA bundle exceeds maximum size of 100000 bytes"`.

### Private Key Rejection

CA bundles **must not contain private key material**. The proxy performs a full PEM decode and rejects any block with type containing `PRIVATE KEY`.

**Rationale:**
- Private keys should never be embedded in CA bundle fields.
- Accidental exposure through logs, audit trails, or API responses is a security risk.
- If mTLS client certificates are needed, use the dedicated `clientCertificate` field with Kubernetes Secrets.

**Enforcement:** Triggered during certificate parsing; requests with private key material return an error with message: `"CA bundle must not contain private key material"`.

### Certificate Block Requirement

A valid CA bundle must contain **at least one PEM `CERTIFICATE` block**. Non-certificate PEM blocks (e.g., `COMMENT`, `PUBLIC KEY`) are rejected.

**Enforcement:** Triggered during certificate parsing; empty or non-certificate bundles return an error with message: `"CA bundle must contain at least one CERTIFICATE PEM block"`.

### PEM Format Validation

All PEM data must be properly formatted. Malformed PEM will be rejected with message: `"failed to parse CA bundle PEM data"`.

## Certificate Parsing

All certificate validation occurs in the `parseCACertificates()` function, which:

1. Validates bundle size
2. Decodes and validates each PEM block
3. Rejects private key material
4. Ensures at least one certificate
5. Appends valid certificates to the system root CA pool

Validation happens once during route TLS transport setup, not on every request.

## Server Name Indication (SNI)

The `serverName` field (max 253 characters, standard DNS hostname length) is validated by the Go TLS library. Invalid hostnames will result in TLS handshake errors but do not block routing.

## Verification Options

The `tlsVerificationOptions` field controls hostname and expiration verification:

- `verifyHostname`: If `false`, disables hostname validation (use only for development/testing)
- `verifyExpiration`: Reserved for future use; currently the Go crypto/tls package always validates expiration

Setting `verifyHostname: false` is equivalent to setting `tlsConfig.InsecureSkipVerify = true` and should only be used in non-production environments.

## Precedence

When multiple certificate options are specified, they are applied in this order:

1. **InsecureSkipTLSVerify** (highest priority): If `true`, all other options are ignored
2. **CABundle + ServerName + TLSVerificationOptions**: Applied together to build a custom TLS config
3. **System defaults** (lowest priority): Standard Go certificate verification with system root CAs

## Testing Security Controls

Security controls are validated in `pkg/httpproxy/cert_management_test.go`:

- `TestBuildTLSConfigForRoute_WithPrivateKeyInCABundle_ReturnsError`
- `TestBuildTLSConfigForRoute_WithOversizedCABundle_ReturnsError`
- `TestBuildTLSConfigForRoute_WithNoCertificatePEMBlock_ReturnsError`

Run all certificate security tests:

```bash
go test ./pkg/httpproxy -run 'TestBuildTLSConfigForRoute'
```

## Best Practices

1. **Never include private keys in CA bundles**. Use `clientCertificate` with a Kubernetes Secret for mTLS.
2. **Use valid PEM format**. Verify certificate encoding before adding to a route.
3. **Minimize bundle size**. Include only the necessary CA certificates.
4. **Keep `verifyHostname` enabled** in production. Only disable for development with self-signed certificates.
5. **Monitor logs** for parsing errors. A rejected certificate indicates misconfiguration or a security issue.

## Future Enhancements

- Metrics tracking for certificate validation failures
- Debug logging of parsing steps for troubleshooting
- Certificate pinning (pin to certificate subject or public key)
- Client certificate rotation and refresh

