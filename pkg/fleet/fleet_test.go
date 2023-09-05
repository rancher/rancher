package fleet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/rancher/rancher/pkg/fleet"
)

func TestGetClusterHost(t *testing.T) {
	testCases := []struct {
		name         string
		config       clientcmdapi.Config
		expectedHost string
		expectErr    bool
	}{
		// the in-cluster config case is not tested, as it would require a container with environment variables,
		// token and cert files.
		{
			name: "returns host of raw config cluster with current context",
			config: clientcmdapi.Config{
				CurrentContext: "foo",
				Clusters: map[string]*clientcmdapi.Cluster{
					"foo": &clientcmdapi.Cluster{
						Server: "bar",
					},
				},
			},
			expectedHost: "bar",
			expectErr:    false,
		},
		{
			name: "returns host of first found configured cluster when none found with current context",
			config: clientcmdapi.Config{
				CurrentContext: "not-found",
				Clusters: map[string]*clientcmdapi.Cluster{
					"first": &clientcmdapi.Cluster{
						Server: "first-server",
					},
					"second": &clientcmdapi.Cluster{
						Server: "second-server",
					},
				},
			},
			expectedHost: "first-server",
			expectErr:    false,
		},
		{
			name: "returns error when no cluster found",
			config: clientcmdapi.Config{
				CurrentContext: "not-found",
				Clusters:       map[string]*clientcmdapi.Cluster{},
			},
			expectedHost: "",
			expectErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientConfig := clientcmd.NewDefaultClientConfig(tc.config, nil)
			host, err := fleet.GetClusterHost(clientConfig)

			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedHost, host)
		})
	}
}
