package cred

import (
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestGetHarvesterCloudCredentialExpirationFromKubeconfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		kubeconfig  string
		tokenFunc   func(string) (*apimgmtv3.Token, error)
		expected    string
		expectErr   bool
		validateErr func(err error) bool
	}{
		{
			name:       "empty kubeconfig",
			kubeconfig: "",
			expected:   "",
		},
		{
			name:       "invalid kubeconfig",
			kubeconfig: "invalid",
			expectErr:  true,
		},
		{
			name: "kubeconfig with incompatible token",
			kubeconfig: `users:
- user:
    token: non-kubeconfig-user
`,
		},
		{
			name: "valid kubeconfig but no token",
			kubeconfig: `users:
- user:
    token: kubeconfig-user-test
`,
			tokenFunc: func(token string) (*apimgmtv3.Token, error) {
				return nil, apierrors.NewNotFound(apimgmtv3.Resource("token"), "kubeconfig-user-test")
			},
			expectErr: true,
			validateErr: func(err error) bool {
				return apierrors.IsNotFound(err)
			},
		},
		{
			name: "bad timestamp on token",
			kubeconfig: `users:
- user:
    token: kubeconfig-user-test
`,
			tokenFunc: func(token string) (*apimgmtv3.Token, error) {
				return &apimgmtv3.Token{
					ExpiresAt: "notatimestamp",
				}, nil
			},
			expectErr: true,
		},
		{
			name: "no timestamp on token",
			kubeconfig: `users:
- user:
    token: kubeconfig-user-test
`,
			tokenFunc: func(token string) (*apimgmtv3.Token, error) {
				return &apimgmtv3.Token{}, nil
			},
		},
		{
			name: "valid timestamp on token",
			kubeconfig: `users:
- user:
    token: kubeconfig-user-test
`,
			tokenFunc: func(token string) (*apimgmtv3.Token, error) {
				return &apimgmtv3.Token{
					ExpiresAt: "2006-01-02T15:04:05Z",
				}, nil
			},
			expected: "1136214245000",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tokenName, err := GetHarvesterCloudCredentialExpirationFromKubeconfig(tt.kubeconfig, tt.tokenFunc)
			if tt.expectErr {
				assert.Error(t, err)
				if tt.validateErr != nil {
					assert.True(t, tt.validateErr(err))
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, tokenName)
		})
	}
}
