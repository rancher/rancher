package machineprovisioning

import (
	"sync/atomic"
	"testing"

	provisioningv1api "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/tests/integration/pkg/clients"
	"github.com/rancher/rancher/tests/integration/pkg/cluster"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/rancher/rancher/tests/integration/pkg/nodeconfig"
	"github.com/rancher/rancher/tests/integration/pkg/wait"
	"github.com/stretchr/testify/assert"
	errgroup2 "golang.org/x/sync/errgroup"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestSingleNodeAllRolesWithDelete(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				MachinePools: []provisioningv1api.RKEMachinePool{{
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

	machines, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, len(machines.Items), 1)
	machine := machines.Items[0]

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
	assert.Greater(t, len(args), 10)
	assert.Len(t, machine.Status.Addresses, 2)

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

func TestThreeNodesAllRolesWithDelete(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				MachinePools: []provisioningv1api.RKEMachinePool{{
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

	c, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

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

func TestFiveNodesUniqueRolesWithDelete(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				MachinePools: []provisioningv1api.RKEMachinePool{
					{
						EtcdRole: true,
						Quantity: &defaults.Three,
					},
					{
						ControlPlaneRole: true,
						Quantity:         &defaults.One,
					},
					{
						WorkerRole: true,
						Quantity:   &defaults.One,
					},
				},
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

func TestFourNodesServerAndWorkerRolesWithDelete(t *testing.T) {
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
				MachinePools: []provisioningv1api.RKEMachinePool{
					{
						EtcdRole:         true,
						ControlPlaneRole: true,
						Quantity:         &defaults.Three,
					},
					{
						WorkerRole: true,
						Quantity:   &defaults.One,
					},
				},
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

func TestDrain(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	drainOpt := rkev1.DrainOptions{
		IgnoreDaemonSets:   &[]bool{true}[0],
		DeleteEmptyDirData: true,
		Enabled:            true,
		PreDrainHooks: []rkev1.DrainHook{
			{
				Annotation: "test.io/pre-hook1",
			},
			{
				Annotation: "test.io/pre-hook2",
			},
		},
		PostDrainHooks: []rkev1.DrainHook{
			{
				Annotation: "test.io/post-hook1",
			},
			{
				Annotation: "test.io/post-hook2",
			},
		},
	}

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
					UpgradeStrategy: rkev1.ClusterUpgradeStrategy{
						ControlPlaneDrainOptions: drainOpt,
						WorkerDrainOptions:       drainOpt,
					},
				},
				MachinePools: []provisioningv1api.RKEMachinePool{
					{
						EtcdRole:         true,
						ControlPlaneRole: true,
					},
					{
						WorkerRole: true,
					},
				},
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

	machines, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, len(machines.Items), 2)

	for {
		c.Spec.RKEConfig.ProvisionGeneration = 1
		newC, err := clients.Provisioning.Cluster().Update(c)
		if apierror.IsConflict(err) {
			c, _ = clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		} else if err != nil {
			t.Fatal(err)
		} else {
			c = newC
			break
		}
	}

	var doneHooks int32
	runHooks := func(machine *capi.Machine) error {
		return wait.Object(clients.Ctx, clients.CAPI.Machine().Watch, machine, func(obj runtime.Object) (bool, error) {
			machine := obj.(*capi.Machine)
			if machine.Annotations[planner.PreDrainAnnotation] != "" &&
				machine.Annotations[planner.PreDrainAnnotation] != machine.Annotations["test.io/pre-hook1"] {
				machine.Annotations["test.io/pre-hook1"] = machine.Annotations[planner.PreDrainAnnotation]
				machine.Annotations["test.io/pre-hook2"] = machine.Annotations[planner.PreDrainAnnotation]
				_, err := clients.CAPI.Machine().Update(machine)
				return false, err
			}
			if machine.Annotations[planner.PostDrainAnnotation] != "" &&
				machine.Annotations[planner.PostDrainAnnotation] != machine.Annotations["test.io/post-hook1"] {
				machine.Annotations["test.io/post-hook1"] = machine.Annotations[planner.PostDrainAnnotation]
				machine.Annotations["test.io/post-hook2"] = machine.Annotations[planner.PostDrainAnnotation]
				_, err := clients.CAPI.Machine().Update(machine)
				if err != nil {
					return false, err
				}
				atomic.AddInt32(&doneHooks, 1)
				return true, nil
			}
			return false, nil
		})
	}

	errgroup, _ := errgroup2.WithContext(clients.Ctx)
	errgroup.Go(func() error {
		return runHooks(&machines.Items[0])
	})
	errgroup.Go(func() error {
		return runHooks(&machines.Items[1])
	})
	if err := errgroup.Wait(); err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, int32(2), atomic.LoadInt32(&doneHooks))
}
