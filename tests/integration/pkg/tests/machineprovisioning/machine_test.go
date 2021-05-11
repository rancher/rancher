package machineprovisioning

import (
	"testing"

	provisioningv1api "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/integration/pkg/clients"
	"github.com/rancher/rancher/tests/integration/pkg/cluster"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/rancher/rancher/tests/integration/pkg/nodeconfig"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSingleNodeAllRoles(t *testing.T) {
	t.Parallel()

	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				NodePools: []provisioningv1api.RKENodePool{{
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

	c, err = cluster.WaitFor(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	machines, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, len(machines.Items), 1)

	clusterClients, err := clients.ForCluster(c.Namespace, c.Name)
	if err != nil {
		t.Fatal(err)
	}

	node, err := clusterClients.Core.Node().Get(machines.Items[0].Status.NodeRef.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	args, err := nodeconfig.FromNode(node)
	if err != nil {
		t.Fatal(err)
	}

	// This shouldn't be one, fix when node args starts returning what is from the config file
	assert.Len(t, args, 1)
}

func TestThreeNodesAllRoles(t *testing.T) {
	t.Parallel()

	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				NodePools: []provisioningv1api.RKENodePool{{
					EtcdRole:         true,
					ControlPlaneRole: true,
					WorkerRole:       true,
					Quantity:         &defaults.Three,
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitFor(clients, c)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSevenNodesUniqueRoles(t *testing.T) {
	t.Parallel()

	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				NodePools: []provisioningv1api.RKENodePool{
					{
						EtcdRole: true,
						Quantity: &defaults.Three,
					},
					{
						ControlPlaneRole: true,
						Quantity:         &defaults.Two,
					},
					{
						WorkerRole: true,
						Quantity:   &defaults.Two,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitFor(clients, c)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFiveNodesServerAndWorkerRoles(t *testing.T) {
	t.Parallel()

	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				NodePools: []provisioningv1api.RKENodePool{
					{
						EtcdRole:         true,
						ControlPlaneRole: true,
						Quantity:         &defaults.Three,
					},
					{
						WorkerRole: true,
						Quantity:   &defaults.Two,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitFor(clients, c)
	if err != nil {
		t.Fatal(err)
	}
}
