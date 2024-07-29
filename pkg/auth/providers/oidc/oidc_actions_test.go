package oidc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_validScopes(t *testing.T) {
	tests := []struct {
		name   string
		scopes string
		want   bool
	}{
		{
			name:   "valid single scope",
			scopes: "openid",
			want:   true,
		},
		{
			name:   "valid multiple scopes",
			scopes: "profile openid",
			want:   true,
		},
		{
			name: "no scopes",
		},
		{
			name:   "scopes lacking openid",
			scopes: "profile email",
		},
		{
			name:   "invalid scopes",
			scopes: "profile, email, openid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt := tt
			assert.Equalf(t, tt.want, validateScopes(tt.scopes), "validateScopes(%v)", tt.scopes)
		})
	}
}
