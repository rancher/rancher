package fleet_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/rancher/rancher/pkg/fleet"
)

func TestGetClusterHost(t *testing.T) {
	testCases := []struct {
		name         string
		config       clientcmdapi.Config
		CAPath       string
		expectedHost []string // The (single) returned host must be an element of this slice.
		expectedCA   []byte
		expectErr    bool
	}{
		// the in-cluster config case is not tested, as it would require a container with environment variables,
		// token and cert files.
		{
			name: "returns host of raw config cluster with current context",
			config: clientcmdapi.Config{
				CurrentContext: "foo",
				Clusters: map[string]*clientcmdapi.Cluster{
					"foo": {
						Server:                   "bar",
						CertificateAuthorityData: []byte("baz"),
					},
				},
			},
			expectedHost: []string{"bar"},
			expectedCA:   []byte("baz"),
			expectErr:    false,
		},
		{
			name: "returns host of raw config cluster with current context with only cert path configured",
			config: clientcmdapi.Config{
				CurrentContext: "foo",
				Clusters: map[string]*clientcmdapi.Cluster{
					"foo": {
						Server:               "bar",
						CertificateAuthority: "/tmp/baz.pem",
					},
				},
			},
			CAPath:       "/tmp/baz.pem",
			expectedHost: []string{"bar"},
			expectedCA:   []byte("baz"),
			expectErr:    false,
		},
		{
			name: "returns host of first found configured cluster when none found with current context",
			config: clientcmdapi.Config{
				CurrentContext: "not-found",
				Clusters: map[string]*clientcmdapi.Cluster{
					"first": {
						Server:                   "first-server",
						CertificateAuthorityData: []byte("baz"),
					},
					"second": {
						Server:                   "second-server",
						CertificateAuthorityData: []byte("baz"),
					},
				},
			},
			expectedHost: []string{"first-server", "second-server"},
			expectedCA:   []byte("baz"),
			expectErr:    false,
		},
		{
			name: "returns error when no cluster found",
			config: clientcmdapi.Config{
				CurrentContext: "not-found",
				Clusters:       map[string]*clientcmdapi.Cluster{},
			},
			expectedHost: []string{""},
			expectedCA:   []byte(""),
			expectErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.CAPath != "" {
				CAFile, err := os.Create(tc.CAPath)
				require.NoError(t, err)

				_, err = CAFile.Write(tc.expectedCA)
				require.NoError(t, err)

				defer os.Remove(tc.CAPath)
			}

			clientConfig := clientcmd.NewDefaultClientConfig(tc.config, nil)
			host, ca, err := fleet.GetClusterHost(clientConfig)

			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Contains(t, tc.expectedHost, host)
			assert.Equal(t, tc.expectedCA, ca)
		})
	}
}
