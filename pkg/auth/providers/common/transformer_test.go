package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransformToAuthProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc       string
		authConfig map[string]any
		provider   map[string]any
	}{
		{
			desc: "Logout fields are set",
			authConfig: map[string]any{
				"metadata": map[string]any{
					"name": "okta",
				},
				"type":               "oktaConfig",
				"logoutAllSupported": true,
				"logoutAllEnabled":   true,
				"logoutAllForced":    true,
			},
			provider: map[string]any{
				"id":                 "okta",
				"type":               "oktaProvider",
				"logoutAllSupported": true,
				"logoutAllEnabled":   true,
				"logoutAllForced":    true,
			},
		},
		{
			desc: "No logout fields are set",
			authConfig: map[string]any{
				"metadata": map[string]any{
					"name": "okta",
				},
				"type": "oktaConfig",
			},
			provider: map[string]any{
				"id":                 "okta",
				"type":               "oktaProvider",
				"logoutAllSupported": false,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
		{
			desc: "Only logoutAllSupported is set",
			authConfig: map[string]any{
				"metadata": map[string]any{
					"name": "okta",
				},
				"type":               "oktaConfig",
				"logoutAllSupported": true,
			},
			provider: map[string]any{
				"id":                 "okta",
				"type":               "oktaProvider",
				"logoutAllSupported": true,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			got := TransformToAuthProvider(test.authConfig)
			require.Equal(t, test.provider, got)
		})
	}
}
