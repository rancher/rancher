package httpproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRequestParams(t *testing.T) {
	tests := []struct {
		name string
		auth string
		want map[string]string
	}{
		{
			name: "parses key value parameters",
			auth: "basic credID=cattle-global-data/my-cred usernameField=user passwordField=pass",
			want: map[string]string{
				"credID":        "cattle-global-data/my-cred",
				"usernameField": "user",
				"passwordField": "pass",
			},
		},
		{
			name: "ignores malformed terms without separator",
			auth: "inject credID=cattle-global-data/my-cred malformed headers=X-Token=token",
			want: map[string]string{
				"credID":  "cattle-global-data/my-cred",
				"headers": "X-Token=token",
			},
		},
		{
			name: "empty auth yields empty params",
			auth: "",
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRequestParams(tt.auth)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCredentialIDFromCattleAuth(t *testing.T) {
	tests := []struct {
		name string
		auth string
		want string
	}{
		{
			name: "extracts credID parameter",
			auth: "headerinject credID=cattle-global-data/my-cred headers=X-Token=tokenField",
			want: "cattle-global-data/my-cred",
		},
		{
			name: "supports bare namespaced credential ID",
			auth: "cattle-global-data/my-cred",
			want: "cattle-global-data/my-cred",
		},
		{
			name: "supports bare provider style credential ID",
			auth: "aws:my-cred",
			want: "aws:my-cred",
		},
		{
			name: "returns empty for non credential token",
			auth: "opaque-token",
			want: "",
		},
		{
			name: "returns empty for malformed key value without credID",
			auth: "inject headers=X-Token=token",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := credentialIDFromCattleAuth(tt.auth)
			assert.Equal(t, tt.want, got)
		})
	}
}

