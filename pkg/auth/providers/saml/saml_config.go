package saml

import (
	"fmt"

	"github.com/crewjam/saml"
	dsig "github.com/russellhaering/goxmldsig"
)

// Accepted NameIDFormat config values for the generic SAML provider.
const (
	nameIDFormatUnspecified = "unspecified"
	nameIDFormatEmail       = "emailAddress"
	nameIDFormatTransient   = "transient"
	nameIDFormatPersistent  = "persistent"
)

// Accepted SignatureMethod config values for the generic SAML provider.
const (
	signatureMethodSHA256 = "RSA-SHA256"
	signatureMethodSHA1   = "RSA-SHA1"
	signatureMethodSHA512 = "RSA-SHA512"
)

// mapNameIDFormat converts a config NameIDFormat value into a crewjam/saml NameIDFormat.
// An empty value defaults to unspecified; unknown values are rejected.
func mapNameIDFormat(v string) (saml.NameIDFormat, error) {
	switch v {
	case "", nameIDFormatUnspecified:
		return saml.UnspecifiedNameIDFormat, nil
	case nameIDFormatEmail:
		return saml.EmailAddressNameIDFormat, nil
	case nameIDFormatTransient:
		return saml.TransientNameIDFormat, nil
	case nameIDFormatPersistent:
		return saml.PersistentNameIDFormat, nil
	default:
		return "", fmt.Errorf("unsupported nameIDFormat %q", v)
	}
}

// mapSignatureMethod converts a config SignatureMethod value into a goxmldsig signature method URI.
// An empty value defaults to RSA-SHA256; unknown values are rejected.
func mapSignatureMethod(v string) (string, error) {
	switch v {
	case "", signatureMethodSHA256:
		return dsig.RSASHA256SignatureMethod, nil
	case signatureMethodSHA1:
		return dsig.RSASHA1SignatureMethod, nil
	case signatureMethodSHA512:
		return dsig.RSASHA512SignatureMethod, nil
	default:
		return "", fmt.Errorf("unsupported signatureMethod %q", v)
	}
}
