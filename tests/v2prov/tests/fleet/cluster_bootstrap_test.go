package fleetcluster_test

import (
	"testing"
	"time"

	fleetv1api "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1api "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Fleet_ClusterBootstrap(t *testing.T) {
	const waitFor = 5 * time.Minute
	const tick = 2 * time.Second
	assert := assert.New(t)
	require := require.New(t)
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	t.Run("fleet local cluster and cluster group present", func(t *testing.T) {
		lc := &fleetv1api.Cluster{}
		require.Eventually(func() bool {
			lc, err = clients.Fleet.Cluster().Get("fleet-local", "local", metav1.GetOptions{})
			return err == nil && lc != nil && lc.Status.Summary.Ready > 0
		}, waitFor, tick)
		require.Contains(lc.Labels, "name")
		require.Equal(lc.Labels["name"], "local")
		assert.Equal(lc.Spec.KubeConfigSecret, "local-kubeconfig")

		lcg := &fleetv1api.ClusterGroup{}
		require.Eventually(func() bool {
			lcg, err = clients.Fleet.ClusterGroup().Get("fleet-local", "default", metav1.GetOptions{})
			return err == nil && lcg != nil
		}, waitFor, tick)
		require.Contains(lcg.Spec.Selector.MatchLabels, "name")
		assert.Equal(lcg.Spec.Selector.MatchLabels["name"], "local")
	})

	c, err := cluster.New(clients, &provv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "downstream-cluster",
		},
		Spec: provv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provv1api.RKEConfig{
				MachinePools: []provv1api.RKEMachinePool{{
					EtcdRole:         true,
					ControlPlaneRole: true,
					WorkerRole:       true,
					Quantity:         &defaults.One,
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	mgmtCluster := &apimgmtv3.Cluster{}
	require.Eventually(func() bool {
		mgmtCluster, err = clients.Mgmt.Cluster().Get(c.Status.ClusterName, metav1.GetOptions{})
		return err == nil && mgmtCluster != nil
	}, waitFor, tick)

	t.Run("fleet downstream cluster present", func(t *testing.T) {
		fc := &fleetv1api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: c.Name, Namespace: c.Namespace}}
		require.Eventually(func() bool {
			fc, err = clients.Fleet.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
			return err == nil && fc.Status.Summary.Ready > 0
		}, waitFor, tick)

		require.NotNil(fc)
		assert.Equal(fc.Spec.AgentEnvVars, mgmtCluster.Spec.AgentEnvVars)
		assert.Equal(c.Status.ClientSecretName, fc.Spec.KubeConfigSecret)
		require.Contains(fc.Labels, "management.cattle.io/cluster-name")
		assert.Equal(fc.Labels["management.cattle.io/cluster-name"], mgmtCluster.Name)
		require.Contains(fc.Labels, "management.cattle.io/cluster-display-name")
		assert.Equal(fc.Labels["management.cattle.io/cluster-display-name"], mgmtCluster.Spec.DisplayName)
	})

	// Delete the cluster and wait for cleanup.
	err = clients.Provisioning.Cluster().Delete(c.Namespace, c.Name, &metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitForDelete(clients, c)
	if err != nil {
		t.Fatal(err)
	}
}
