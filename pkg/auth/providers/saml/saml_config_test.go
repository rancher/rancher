package saml

import (
	"testing"

	"github.com/crewjam/saml"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapNameIDFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		want    saml.NameIDFormat
		wantErr bool
	}{
		{name: "empty defaults to unspecified", in: "", want: saml.UnspecifiedNameIDFormat},
		{name: "unspecified", in: "unspecified", want: saml.UnspecifiedNameIDFormat},
		{name: "emailAddress", in: "emailAddress", want: saml.EmailAddressNameIDFormat},
		{name: "transient", in: "transient", want: saml.TransientNameIDFormat},
		{name: "persistent", in: "persistent", want: saml.PersistentNameIDFormat},
		{name: "unknown errors", in: "bogus", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := mapNameIDFormat(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapSignatureMethod(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "empty defaults to sha256", in: "", want: dsig.RSASHA256SignatureMethod},
		{name: "sha256", in: "RSA-SHA256", want: dsig.RSASHA256SignatureMethod},
		{name: "sha1", in: "RSA-SHA1", want: dsig.RSASHA1SignatureMethod},
		{name: "sha512", in: "RSA-SHA512", want: dsig.RSASHA512SignatureMethod},
		{name: "unknown errors", in: "MD5", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := mapSignatureMethod(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
