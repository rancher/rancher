package multiclustermanager

import (
	"context"
	"errors"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	managementfakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

var _ config.MultiClusterManager = (*DeferredServer)(nil)

func TestDeferredServerUserContextUnavailable(t *testing.T) {
	for _, server := range []*DeferredServer{{}, {mcm: &mcm{}}} {
		clients, err := server.UserContext("missing")
		require.Nil(t, clients)
		require.EqualError(t, err, "no cluster manager")
	}
	k8s, err := (&DeferredServer{}).K8sClient("missing")
	require.Nil(t, k8s)
	require.NoError(t, err)
}

type managementWithClusters struct {
	managementv3.Interface
	clusters managementv3.ClusterInterface
}

func (m managementWithClusters) Clusters(string) managementv3.ClusterInterface {
	return m.clusters
}

type coreWithSecrets struct {
	corev1.Interface
	secrets corev1.SecretInterface
}

func (c coreWithSecrets) Secrets(string) corev1.SecretInterface {
	return c.secrets
}

func TestDeferredServerDelegatesUserContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scaled, err := config.NewScaledContext(rest.Config{Host: "https://127.0.0.1"}, nil)
	require.NoError(t, err)
	scaled.RunContext = ctx
	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "local", UID: "local-uid"},
		Spec:       apimgmtv3.ClusterSpec{Internal: true},
	}
	expectedError := errors.New("cluster lister failed")
	lister := &managementfakes.ClusterListerMock{
		GetFunc: func(namespace, name string) (*apimgmtv3.Cluster, error) {
			require.Empty(t, namespace)
			if name == "missing" {
				return nil, expectedError
			}
			require.Equal(t, cluster.Name, name)
			return cluster, nil
		},
	}
	scaled.Management = managementWithClusters{Interface: scaled.Management, clusters: &managementfakes.ClusterInterfaceMock{
		ControllerFunc: func() managementv3.ClusterController {
			return &managementfakes.ClusterControllerMock{ListerFunc: func() managementv3.ClusterLister { return lister }}
		},
	}}
	scaled.Core = coreWithSecrets{Interface: scaled.Core, secrets: &corefakes.SecretInterfaceMock{
		ControllerFunc: func() corev1.SecretController {
			return &corefakes.SecretControllerMock{ListerFunc: func() corev1.SecretLister { return &corefakes.SecretListerMock{} }}
		},
	}}
	manager := clustermanager.NewManager(443, scaled, nil)
	server := &DeferredServer{mcm: &mcm{clusterManager: manager}}

	expected, err := manager.UserContext(cluster.Name)
	require.NoError(t, err)
	require.NotNil(t, expected)
	t.Cleanup(func() { manager.Stop(cluster) })
	actual, err := server.UserContext(cluster.Name)
	require.NoError(t, err)
	require.Same(t, expected, actual)
	require.Same(t, actual, mustUserContext(t, server, cluster.Name))

	actual, err = server.UserContext("missing")
	require.Nil(t, actual)
	require.Same(t, expectedError, err)
	k8s, err := server.K8sClient(cluster.Name)
	require.NoError(t, err)
	require.NotNil(t, k8s)
}

func mustUserContext(t *testing.T, server *DeferredServer, name string) *config.UserContext {
	t.Helper()
	clients, err := server.UserContext(name)
	require.NoError(t, err)
	return clients
}
