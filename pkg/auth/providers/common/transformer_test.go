package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransformToAuthProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc       string
		authConfig map[string]interface{}
		provider   map[string]interface{}
	}{
		{
			desc: "Logout fields are set",
			authConfig: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "okta",
				},
				"type":               "oktaConfig",
				"logoutAllSupported": true,
				"logoutAllEnabled":   true,
				"logoutAllForced":    true,
			},
			provider: map[string]interface{}{
				"id":                 "okta",
				"type":               "oktaProvider",
				"logoutAllSupported": true,
				"logoutAllEnabled":   true,
				"logoutAllForced":    true,
			},
		},
		{
			desc: "No logout fields are set",
			authConfig: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "okta",
				},
				"type": "oktaConfig",
			},
			provider: map[string]interface{}{
				"id":                 "okta",
				"type":               "oktaProvider",
				"logoutAllSupported": false,
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
			},
		},
		{
			desc: "Only logoutAllSupported is set",
			authConfig: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "okta",
				},
				"type":               "oktaConfig",
				"logoutAllSupported": true,
			},
			provider: map[string]interface{}{
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
