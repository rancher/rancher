package ntlm

import (
	"crypto"
	"crypto/x509"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCertificateHashAlgorithm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		alg     x509.SignatureAlgorithm
		want    crypto.Hash
		wantErr bool
	}{
		{name: "sha256 rsa", alg: x509.SHA256WithRSA, want: crypto.SHA256},
		{name: "sha256 rsa pss", alg: x509.SHA256WithRSAPSS, want: crypto.SHA256},
		{name: "sha256 ecdsa", alg: x509.ECDSAWithSHA256, want: crypto.SHA256},
		{name: "sha384 rsa", alg: x509.SHA384WithRSA, want: crypto.SHA384},
		{name: "sha384 ecdsa", alg: x509.ECDSAWithSHA384, want: crypto.SHA384},
		{name: "sha512 rsa", alg: x509.SHA512WithRSA, want: crypto.SHA512},
		{name: "sha512 ecdsa", alg: x509.ECDSAWithSHA512, want: crypto.SHA512},
		{name: "sha1 upgrades to sha256", alg: x509.SHA1WithRSA, want: crypto.SHA256},
		{name: "sha1 ecdsa upgrades to sha256", alg: x509.ECDSAWithSHA1, want: crypto.SHA256},
		{name: "md5 upgrades to sha256", alg: x509.MD5WithRSA, want: crypto.SHA256},
		{name: "unknown is rejected", alg: x509.UnknownSignatureAlgorithm, wantErr: true},
		{name: "ed25519 identifies no hash", alg: x509.PureEd25519, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := certificateHashAlgorithm(test.alg)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestChannelBindingTokenKnownVector(t *testing.T) {
	t.Parallel()

	// A certificate whose DER body is the single byte 0x00, signed with
	// SHA-256. The expected value below was computed independently of this
	// implementation, so a failure means the implementation is wrong:
	//
	//   certHash = sha256(00)
	//            = 6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d
	//   appData  = "tls-server-end-point:" || certHash          (53 bytes)
	//   struct   = <5 little-endian uint32 zeros/length> || appData  (73 bytes)
	//   cbt      = md5(struct)
	//
	// Reproduce with:
	//   python3 -c "import hashlib,struct;d=hashlib.sha256(bytes([0])).digest();
	//   a=b'tls-server-end-point:'+d;print(hashlib.md5(struct.pack('<IIIII',0,0,0,0,len(a))+a).hexdigest())"
	cert := &x509.Certificate{
		Raw:                []byte{0x00},
		SignatureAlgorithm: x509.SHA256WithRSA,
	}

	got, err := ChannelBindingToken(cert)
	require.NoError(t, err)

	want, err := hex.DecodeString("4e70888ec8512aee31c2b2d2253de9fd")
	require.NoError(t, err)
	assert.Equal(t, want, got[:])
}

func TestChannelBindingTokenIsDeterministic(t *testing.T) {
	t.Parallel()

	cert := &x509.Certificate{Raw: []byte("some der bytes"), SignatureAlgorithm: x509.SHA384WithRSA}

	first, err := ChannelBindingToken(cert)
	require.NoError(t, err)
	second, err := ChannelBindingToken(cert)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}

func TestChannelBindingTokenRejectsUnknownSignatureAlgorithm(t *testing.T) {
	t.Parallel()

	cert := &x509.Certificate{Raw: []byte{0x00}, SignatureAlgorithm: x509.UnknownSignatureAlgorithm}

	_, err := ChannelBindingToken(cert)
	require.Error(t, err)
}
