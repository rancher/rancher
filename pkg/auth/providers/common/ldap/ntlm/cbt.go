// Package ntlm implements the subset of MS-NLMP (NTLMv2) needed to bind to an
// Active Directory domain controller with an RFC 5929 tls-server-end-point
// channel binding token. It depends only on the Go standard library.
//
// The MD4, MD5 and HMAC-MD5 primitives used here are mandated by MS-NLMP and
// RFC 5929. They are protocol conformance requirements, not a cryptographic
// choice made by Rancher.
package ntlm

import (
	"bytes"
	"crypto"
	"crypto/md5" // #nosec G501 -- MD5 is mandated by MS-NLMP for the channel binding token.
	"crypto/x509"
	"encoding/binary"
	"fmt"

	_ "crypto/sha256" // Register SHA-256 for crypto.Hash.New.
	_ "crypto/sha512" // Register SHA-384 and SHA-512 for crypto.Hash.New.
)

// channelBindingPrefix is the RFC 5929 tls-server-end-point prefix that
// precedes the certificate digest in the GSS application data.
const channelBindingPrefix = "tls-server-end-point:"

// certificateHashAlgorithm returns the digest to apply to the certificate
// under RFC 5929 section 4.1: the hash of the certificate's own signature
// algorithm, with MD5 and SHA-1 replaced by SHA-256. Signature algorithms
// that do not identify exactly one hash are rejected, because guessing
// produces an opaque SEC_E_BAD_BINDINGS at the domain controller.
func certificateHashAlgorithm(alg x509.SignatureAlgorithm) (crypto.Hash, error) {
	switch alg {
	case x509.MD5WithRSA,
		x509.SHA1WithRSA,
		x509.DSAWithSHA1,
		x509.ECDSAWithSHA1,
		x509.SHA256WithRSA,
		x509.DSAWithSHA256,
		x509.ECDSAWithSHA256,
		x509.SHA256WithRSAPSS:
		return crypto.SHA256, nil
	case x509.SHA384WithRSA, x509.ECDSAWithSHA384, x509.SHA384WithRSAPSS:
		return crypto.SHA384, nil
	case x509.SHA512WithRSA, x509.ECDSAWithSHA512, x509.SHA512WithRSAPSS:
		return crypto.SHA512, nil
	default:
		return 0, fmt.Errorf("cannot derive a channel binding hash from certificate signature algorithm %v", alg)
	}
}

// ChannelBindingToken returns the 16-byte MsvAvChannelBindings value binding an
// NTLM exchange to the TLS channel terminated by cert.
//
// The all-zero value means "no channel binding" on the wire and is never
// returned: a caller with no peer certificate must fail the bind instead.
func ChannelBindingToken(cert *x509.Certificate) ([16]byte, error) {
	var token [16]byte

	if cert == nil || len(cert.Raw) == 0 {
		return token, fmt.Errorf("channel binding requires a server certificate")
	}

	hash, err := certificateHashAlgorithm(cert.SignatureAlgorithm)
	if err != nil {
		return token, err
	}

	digester := hash.New()
	digester.Write(cert.Raw)
	applicationData := append([]byte(channelBindingPrefix), digester.Sum(nil)...)

	// gss_channel_bindings_struct, RFC 2744 section 3.11, serialized as
	// Microsoft implements it: little-endian, empty initiator and acceptor
	// addresses, application data carrying the prefixed certificate digest.
	var buf bytes.Buffer
	writeUint32 := func(v uint32) {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], v)
		buf.Write(b[:])
	}
	writeUint32(0) // initiator_addtype
	writeUint32(0) // initiator_address length
	writeUint32(0) // acceptor_addrtype
	writeUint32(0) // acceptor_address length
	writeUint32(uint32(len(applicationData)))
	buf.Write(applicationData)

	token = md5.Sum(buf.Bytes()) // #nosec G401 -- MD5 is mandated by MS-NLMP.
	return token, nil
}
