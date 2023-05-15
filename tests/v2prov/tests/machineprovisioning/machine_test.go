package machineprovisioning

import (
	"os"
	"strings"
	"sync/atomic"
	"testing"

	provisioningv1api "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/nodeconfig"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/stretchr/testify/assert"
	errgroup2 "golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func Test_Provisioning_MP_SingleNodeAllRolesWithDelete(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-single-node-all-roles-with-delete",
		},
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
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

func Test_Provisioning_MP_MachineTemplateClonedAnnotations(t *testing.T) {
	if strings.ToLower(os.Getenv("DIST")) == "rke2" {
		t.Skip()
	}

	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-machine-template-cloned-annotations",
		},
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

	infraMachines, err := cluster.PodInfraMachines(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	for _, infraMachine := range infraMachines.Items {
		templateGroupKind := schema.ParseGroupKind(infraMachine.GetAnnotations()[capi.TemplateClonedFromGroupKindAnnotation])

		machineTemplate, err := clients.Dynamic.
			Resource(schema.GroupVersionResource{Group: templateGroupKind.Group, Version: "v1", Resource: strings.ToLower(templateGroupKind.Kind) + "s"}).
			Namespace(infraMachine.GetNamespace()).
			Get(clients.Ctx, infraMachine.GetAnnotations()[capi.TemplateClonedFromNameAnnotation], metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		gv, err := schema.ParseGroupVersion(machineTemplate.GetAnnotations()[capr.MachineTemplateClonedFromGroupVersionAnn])
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, gv.String(), capr.DefaultMachineConfigAPIVersion)
		assert.Equal(t, machineTemplate.GetAnnotations()[capr.MachineTemplateClonedFromKindAnn], c.Spec.RKEConfig.MachinePools[0].NodeConfig.Kind)
		assert.Equal(t, machineTemplate.GetAnnotations()[capr.MachineTemplateClonedFromNameAnn], c.Spec.RKEConfig.MachinePools[0].NodeConfig.Name)
	}
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

func Test_Provisioning_MP_MachineSetDeletePolicyOldestSet(t *testing.T) {
	if strings.ToLower(os.Getenv("DIST")) == "rke2" {
		t.Skip()
	}

	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-machine-set-delete-policy-oldest-set",
		},
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				MachinePools: []provisioningv1api.RKEMachinePool{
					{
						EtcdRole:         true,
						ControlPlaneRole: true,
						WorkerRole:       true,
						Quantity:         &defaults.One,
					},
					{
						EtcdRole:         true,
						ControlPlaneRole: true,
						WorkerRole:       true,
						Quantity:         &defaults.One,
						RollingUpdate: &provisioningv1api.RKEMachinePoolRollingUpdate{
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "10%",
							},
							MaxSurge: &intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "10%",
							},
						},
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

	machineSets, err := cluster.MachineSets(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	for _, machineSet := range machineSets.Items {
		d, err := data.Convert(machineSet)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, string(capi.OldestMachineSetDeletePolicy), d.String("Object", "spec", "deletePolicy"))
	}
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

func Test_Provisioning_MP_ThreeNodesAllRolesScaledToOneThenDelete(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-three-nodes-all-roles-with-delete",
		},
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

	c, err = operations.Scale(clients, c, 0, 2, true)
	assert.NoError(t, err)
	c, err = operations.Scale(clients, c, 0, 1, true)
	assert.NoError(t, err)

	_, err = operations.GetAndVerifyDownstreamClientset(clients, c)
	assert.NoError(t, err)

	// Delete the cluster and wait for cleanup.
	err = clients.Provisioning.Cluster().Delete(c.Namespace, c.Name, &metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitForDelete(clients, c)
	if err != nil {
		t.Fatal(err)
	}
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

func Test_Provisioning_MP_FiveNodesUniqueRolesWithDelete(t *testing.T) {
	if strings.ToLower(os.Getenv("DIST")) == "rke2" {
		t.Skip()
	}
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-five-nodes-unique-roles-with-delete",
		},
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
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

func Test_Provisioning_MP_FourNodesServerAndWorkerRolesWithDelete(t *testing.T) {
	if strings.ToLower(os.Getenv("DIST")) == "rke2" {
		t.Skip()
	}
	t.Parallel()
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-worker-roles-with-delete",
		},
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
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

func Test_Provisioning_MP_Drain(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	drainOpt := rkev1.DrainOptions{
		IgnoreDaemonSets:                &[]bool{true}[0],
		DeleteEmptyDirData:              true,
		Enabled:                         true,
		Force:                           true,
		SkipWaitForDeleteTimeoutSeconds: 30,
		GracePeriod:                     5,
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
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-drain",
		},
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
		var secret *corev1.Secret
		err = retry.OnError(retry.DefaultBackoff, func(err error) bool { return !apierror.IsNotFound(err) }, func() error {
			bootstrap, err := clients.RKE.RKEBootstrap().Get(machine.Spec.Bootstrap.ConfigRef.Namespace, machine.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			secret, err = clients.Core.Secret().Get(bootstrap.Namespace, capr.PlanSecretFromBootstrapName(bootstrap.Name), metav1.GetOptions{})
			return err
		})
		return wait.Object(clients.Ctx, clients.Core.Secret().Watch, secret, func(obj runtime.Object) (bool, error) {
			secret := obj.(*corev1.Secret)
			if secret.Annotations[capr.PreDrainAnnotation] != "" &&
				secret.Annotations[capr.PreDrainAnnotation] != secret.Annotations["test.io/pre-hook1"] {
				secret.Annotations["test.io/pre-hook1"] = secret.Annotations[capr.PreDrainAnnotation]
				secret.Annotations["test.io/pre-hook2"] = secret.Annotations[capr.PreDrainAnnotation]
				_, err := clients.Core.Secret().Update(secret)
				return false, err
			}
			if secret.Annotations[capr.PostDrainAnnotation] != "" &&
				secret.Annotations[capr.PostDrainAnnotation] != secret.Annotations["test.io/post-hook1"] {
				secret.Annotations["test.io/post-hook1"] = secret.Annotations[capr.PostDrainAnnotation]
				secret.Annotations["test.io/post-hook2"] = secret.Annotations[capr.PostDrainAnnotation]
				_, err := clients.Core.Secret().Update(secret)
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
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

func Test_Provisioning_MP_DrainNoDelete(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-drain-no-delete",
		},
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				MachinePools: []provisioningv1api.RKEMachinePool{
					{
						EtcdRole:          true,
						ControlPlaneRole:  true,
						Quantity:          &defaults.One,
						DrainBeforeDelete: false,
					},
					{
						WorkerRole:        true,
						Quantity:          &defaults.One,
						DrainBeforeDelete: true,
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

	excludeNodeDraining, ok := machines.Items[0].Annotations[capi.ExcludeNodeDrainingAnnotation]
	assert.True(t, ok)
	assert.Equal(t, excludeNodeDraining, "true")

	_, ok = machines.Items[1].Annotations[capi.ExcludeNodeDrainingAnnotation]
	assert.False(t, ok)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}
