package machineprovisioning

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	provisioningv1api "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	planapi "github.com/rancher/rancher/pkg/plan"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/nodeconfig"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	errgroup2 "golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8swait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util/conditions"
)

func Test_Provisioning_SetA_MP_SingleNodeAllRolesWithDelete(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()
	t.Parallel()

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

	var node *corev1.Node
	backoff := k8swait.Backoff{
		Steps:    5,
		Duration: 2 * time.Second,
		Factor:   1.0,
		Jitter:   0.5,
	}
	err = retry.OnError(backoff, func(e error) bool {
		return apierror.IsNotFound(e) || apierror.IsUnauthorized(e)
	}, func() error {
		var err error
		node, err = clusterClients.Core.Node().Get(machines.Items[0].Status.NodeRef.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return nil
	})

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

func Test_Provisioning_SetA_MP_MultipleEtcdNodesScaledDownThenDelete(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "etcd-scaled-down",
		},
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				MachinePools: []provisioningv1api.RKEMachinePool{
					{
						EtcdRole:         true,
						ControlPlaneRole: false,
						WorkerRole:       false,
						Quantity:         &defaults.Two,
					},
					{
						EtcdRole:         false,
						ControlPlaneRole: true,
						WorkerRole:       true,
						Quantity:         &defaults.One,
					}},
			},
			ClusterAgentDeploymentCustomization: &provisioningv1api.AgentDeploymentCustomization{
				OverrideAffinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "node-role.kubernetes.io/control-plane",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"true"},
										},
									},
								},
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

	kc, err := operations.GetAndVerifyDownstreamClientset(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	// wait for all nodes to be ready
	err = retry.OnError(defaults.DownstreamRetry, func(error) bool { return true }, func() error {
		nodes, err := kc.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		if len(nodes.Items) != 3 {
			return fmt.Errorf("nodes did not match 3: actual: %d", len(nodes.Items))
		}
		for _, n := range nodes.Items {
			if !capr.Ready.IsTrue(n) {
				return fmt.Errorf("node %s was not ready", n.Name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err = operations.Scale(clients, c, 0, 1, true)
	if err != nil {
		t.Fatal(err)
	}

	_, err = operations.GetAndVerifyDownstreamClientset(clients, c)
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

func Test_Provisioning_SetB_MP_DrainNoDelete(t *testing.T) {
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

func Test_Provisioning_SetB_MP_MachineTemplateClonedAnnotations(t *testing.T) {
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

func Test_Provisioning_SetB_MP_MachineSetDeletePolicyOldestSet(t *testing.T) {
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
		assert.Equal(t, capi.OldestMachineSetDeletionOrder, machineSet.Spec.Deletion.Order)
	}
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

func Test_Provisioning_SetB_MP_FiveNodesUniqueRolesWithDelete(t *testing.T) {
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

func Test_Provisioning_SetB_MP_FourNodesServerAndWorkerRolesWithDelete(t *testing.T) {
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

func Test_Provisioning_SetB_MP_Drain(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()
	t.Parallel()

	drainOpt := rkev1.DrainOptions{
		IgnoreDaemonSets:                ptr.To(true),
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
				ClusterConfiguration: rkev1.ClusterConfiguration{
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
			bootstrap, err := clients.RKE.RKEBootstrap().Get(machine.Namespace, machine.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
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

func Test_Provisioning_SetB_Single_Node_All_Roles_Drain(t *testing.T) {
	clients, err := clients.New()
	require.NoError(t, err)
	defer clients.Close()
	t.Parallel()

	ctx := clients.Ctx

	drainOptions := rkev1.DrainOptions{
		Enabled:                         true,
		DeleteEmptyDirData:              true,
		DisableEviction:                 false,
		GracePeriod:                     -1,
		Force:                           false,
		IgnoreDaemonSets:                ptr.To(true),
		SkipWaitForDeleteTimeoutSeconds: 0,
		Timeout:                         120,
	}

	// Single-node (cp+etcd+worker) with drain upgrade strategy option enabled for both CP and worker
	provClusterSchema := &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-single-node-drain",
		},
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				ClusterConfiguration: rkev1.ClusterConfiguration{
					UpgradeStrategy: rkev1.ClusterUpgradeStrategy{
						ControlPlaneDrainOptions: drainOptions,
						ControlPlaneConcurrency:  "1",
						WorkerDrainOptions:       drainOptions,
						WorkerConcurrency:        "1",
					},
				},
				MachinePools: []provisioningv1api.RKEMachinePool{{
					EtcdRole:         true,
					ControlPlaneRole: true,
					WorkerRole:       true,
					Quantity:         &defaults.One,
				}},
			},
		},
	}

	c, err := cluster.New(clients, provClusterSchema)
	require.NoError(t, err)

	c, err = cluster.WaitForCreate(clients, c)
	require.NoError(t, err)

	// Validate that exactly one Machine and capture its template hash
	machines, err := cluster.Machines(clients, c)
	require.NoError(t, err)
	require.Equal(t, 1, len(machines.Items), "expected exactly one machine initially")

	firstMachine := machines.Items[0]
	firstMachineHash := firstMachine.Labels["machine-template-hash"]
	require.NotEmpty(t, firstMachineHash, "firstMachine missing template-hash label")

	// Create a fresh machine config that can be mutated.
	newCfgRef, err := nodeconfig.NewPodConfig(clients, c.Namespace)
	require.NoError(t, err)

	err = retry.OnError(retry.DefaultBackoff, func(err error) bool {
		return true
	}, func() error {
		gvrPodConfig := schema.GroupVersionResource{
			Group: "rke-machine-config.cattle.io", Version: "v1", Resource: "podconfigs",
		}
		newPodConfig, err := clients.Dynamic.Resource(gvrPodConfig).Namespace(c.Namespace).Get(ctx, newCfgRef.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		currentUserData, ok := unstructuredString(newPodConfig.Object, "userdata")
		require.True(t, ok)
		// Force a no-op template diff
		newPodConfig.Object["userdata"] = currentUserData + `# Noop Change`
		_, err = clients.Dynamic.Resource(gvrPodConfig).Namespace(c.Namespace).Update(ctx, newPodConfig, metav1.UpdateOptions{})
		return err
	})

	require.NoError(t, err)

	provCluster, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	require.NoError(t, err)

	require.NotNil(t, provCluster.Spec.RKEConfig)
	require.GreaterOrEqual(t, len(provCluster.Spec.RKEConfig.MachinePools), 1)

	// Point the pool at the new PodConfig.
	provCluster.Spec.RKEConfig.MachinePools[0].NodeConfig = newCfgRef
	_, err = clients.Provisioning.Cluster().Update(provCluster)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		machines, _ = cluster.Machines(clients, c)
		return len(machines.Items) == 2
	}, 5*time.Minute, 2*time.Second, "never saw 2 nodes after config change")

	var secondMachineName string
	require.Eventually(t, func() bool {
		machines, _ = cluster.Machines(clients, c)
		for _, m := range machines.Items {
			hash := m.Labels["machine-template-hash"]
			if hash != "" && hash != firstMachineHash && m.Status.NodeRef.IsDefined() {
				secondMachineName = m.Name
				return true
			}
		}
		return false
	}, 15*time.Minute, 2*time.Second, "no node with new template hash (and NodeRef) appeared")

	// Ensure the second node reaches Ready=True
	var secondMachineNode *corev1.Node
	clusterClients, err := clients.ForCluster(c.Namespace, c.Name)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		m, err := clients.CAPI.Machine().Get(c.Namespace, secondMachineName, metav1.GetOptions{})
		if err != nil || !m.Status.NodeRef.IsDefined() {
			return false
		}

		secondMachineNode, err = clusterClients.Core.Node().Get(m.Status.NodeRef.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		for _, cond := range secondMachineNode.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				return true
			}
		}
		return false
	}, 20*time.Minute, 2*time.Second, "second machine node never reached Ready=True")

	// Sanity: incoming CP should not be cordoned
	require.False(t, secondMachineNode.Spec.Unschedulable, "second machine node was cordoned; incoming controlplane should not be drained")

	require.Eventually(t, func() bool {
		m, err := clients.CAPI.Machine().Get(firstMachine.Namespace, firstMachine.Name, metav1.GetOptions{})
		if err != nil {
			// already deleted is also OK
			return apierror.IsNotFound(err)
		}
		return !m.DeletionTimestamp.IsZero()
	}, 15*time.Minute, 2*time.Second, "first machine node never entered Deleting")

	require.Eventually(t, func() bool {
		ml, _ := cluster.Machines(clients, c)
		return len(ml.Items) == 1
	}, 15*time.Minute, 2*time.Second, "did not converge back to a single node")

	require.Eventually(t, func() bool {
		latest, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		return err == nil && latest.Status.Ready
	}, 10*time.Minute, 10*time.Second, "cluster did not return to Ready after rollout")
}

func Test_Provisioning_SetB_Single_Node_Drain_Unhealthy_Replacement(t *testing.T) {
	if capr.GetRuntime(defaults.SomeK8sVersion) != capr.RuntimeRKE2 {
		t.Skip("this test injects an RKE2 control-plane probe failure")
	}

	clients, err := clients.New()
	require.NoError(t, err)
	defer clients.Close()

	ctx := clients.Ctx
	const (
		preTerminateHook       = capi.PreTerminateDeleteHookAnnotationPrefix + "/rke-bootstrap-cleanup"
		kubeSchedulerProbePort = "10259"
	)

	drainOptions := rkev1.DrainOptions{
		Enabled:                         true,
		DeleteEmptyDirData:              true,
		DisableEviction:                 false,
		GracePeriod:                     -1,
		Force:                           false,
		IgnoreDaemonSets:                ptr.To(true),
		SkipWaitForDeleteTimeoutSeconds: 0,
		Timeout:                         120,
	}

	// Create a single-node all-roles cluster with draining enabled to exercise a control-plane replacement.
	provClusterSchema := &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-single-node-drain",
		},
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: defaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				ClusterConfiguration: rkev1.ClusterConfiguration{
					UpgradeStrategy: rkev1.ClusterUpgradeStrategy{
						ControlPlaneDrainOptions: drainOptions,
						ControlPlaneConcurrency:  "1",
						WorkerDrainOptions:       drainOptions,
						WorkerConcurrency:        "1",
					},
				},
				MachinePools: []provisioningv1api.RKEMachinePool{{
					EtcdRole:         true,
					ControlPlaneRole: true,
					WorkerRole:       true,
					Quantity:         &defaults.One,
				}},
			},
		},
	}

	c, err := cluster.New(clients, provClusterSchema)
	require.NoError(t, err)

	c, err = cluster.WaitForCreate(clients, c)
	require.NoError(t, err)

	// Verify the initial Machine and downstream Node are ready before starting the rollout.
	machines, err := cluster.Machines(clients, c)
	require.NoError(t, err)
	require.Equal(t, 1, len(machines.Items), "expected exactly one machine initially")

	firstMachine := machines.Items[0]
	require.True(t, firstMachine.Status.NodeRef.IsDefined(), "initial machine has no NodeRef")
	require.True(t, conditions.IsTrue(&firstMachine, capi.MachineNodeReadyCondition), "initial machine is not NodeReady")
	firstNodeName := firstMachine.Status.NodeRef.Name
	firstMachineHash := firstMachine.Labels["machine-template-hash"]
	require.NotEmpty(t, firstMachineHash, "initial machine is missing the template-hash label")

	// Confirm the initial Machine is protected before requesting its replacement.
	require.Eventually(t, func() bool {
		m, err := clients.CAPI.Machine().Get(firstMachine.Namespace, firstMachine.Name, metav1.GetOptions{})
		return err == nil && m.Annotations[preTerminateHook] != ""
	}, 2*time.Minute, 2*time.Second, "initial machine never received the pre-terminate hook")

	clusterClients, err := clients.ForCluster(c.Namespace, c.Name)
	require.NoError(t, err)

	// Create a fresh machine config to trigger the replacement rollout. The probe failure is
	// injected later so infrastructure and system-agent initialization can complete normally.
	newCfgRef, err := nodeconfig.NewPodConfig(clients, c.Namespace)
	require.NoError(t, err)

	err = retry.OnError(retry.DefaultBackoff, func(err error) bool {
		return true
	}, func() error {
		gvrPodConfig := schema.GroupVersionResource{
			Group: "rke-machine-config.cattle.io", Version: "v1", Resource: "podconfigs",
		}
		newPodConfig, err := clients.Dynamic.Resource(gvrPodConfig).Namespace(c.Namespace).Get(ctx, newCfgRef.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		currentUserData, ok := unstructuredString(newPodConfig.Object, "userdata")
		require.True(t, ok)
		newPodConfig.Object["userdata"] = currentUserData + "\n# Trigger the replacement rollout."
		_, err = clients.Dynamic.Resource(gvrPodConfig).Namespace(c.Namespace).Update(ctx, newPodConfig, metav1.UpdateOptions{})
		return err
	})

	require.NoError(t, err)

	provCluster, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	require.NoError(t, err)

	require.NotNil(t, provCluster.Spec.RKEConfig)
	require.GreaterOrEqual(t, len(provCluster.Spec.RKEConfig.MachinePools), 1)

	// Update the pool template to start the replacement rollout.
	provCluster.Spec.RKEConfig.MachinePools[0].NodeConfig = newCfgRef
	c, err = clients.Provisioning.Cluster().Update(provCluster)
	require.NoError(t, err)

	// Identify the replacement by its new machine-template hash.
	var secondMachine *capi.Machine
	require.Eventually(t, func() bool {
		machines, err = cluster.Machines(clients, c)
		if err != nil {
			return false
		}
		for i := range machines.Items {
			m := &machines.Items[i]
			hash := m.Labels["machine-template-hash"]
			if hash != "" && hash != firstMachineHash {
				secondMachine = m.DeepCopy()
				return true
			}
		}
		return false
	}, 5*time.Minute, 2*time.Second, "replacement machine never appeared")

	require.NotEqual(t, firstMachine.UID, secondMachine.UID, "replacement reused the initial machine")
	require.Equal(t, "PodMachine", secondMachine.Spec.InfrastructureRef.Kind, "test requires the pod machine driver")
	require.True(t, secondMachine.Spec.Bootstrap.ConfigRef.IsDefined(), "replacement has no bootstrap reference")
	planSecretName := capr.PlanSecretFromBootstrapName(secondMachine.Spec.Bootstrap.ConfigRef.Name)

	// Resolve the PodMachine's backing Pod so the test can inject the probe failure into only the replacement.
	var replacementPodNamespace, replacementPodName string
	require.Eventually(t, func() bool {
		infraMachine, err := external.GetObjectFromContractVersionedRef(ctx, clients.Client, secondMachine.Spec.InfrastructureRef, secondMachine.Namespace)
		if err != nil {
			return false
		}
		replacementPodNamespace = infraMachine.GetNamespace()
		replacementPodName = strings.ReplaceAll(infraMachine.GetName(), ".", "-")
		pod, err := clients.Core.Pod().Get(replacementPodNamespace, replacementPodName, metav1.GetOptions{})
		return err == nil && pod.Status.Phase == corev1.PodRunning
	}, 5*time.Minute, 2*time.Second, "replacement systemd-node pod never became Running")

	// Wait for the system agent so the fault does not interrupt infrastructure or agent initialization.
	var systemAgentOutput string
	var systemAgentErr error
	systemAgentReady := false
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); time.Sleep(2 * time.Second) {
		systemAgentOutput, systemAgentErr = cluster.ExecOnPod(clients, replacementPodNamespace, replacementPodName,
			"systemctl", "is-active", "rancher-system-agent")
		if systemAgentErr == nil && strings.TrimSpace(systemAgentOutput) == "active" {
			systemAgentReady = true
			break
		}
	}
	require.True(t, systemAgentReady, "replacement system agent did not become active: output=%q err=%v", systemAgentOutput, systemAgentErr)

	// Block only the replacement's local kube-scheduler health endpoint. The kubelet can register
	// and become NodeReady, but the machine plan cannot pass until the rule is removed.
	faultOutput, err := cluster.ExecOnPod(clients, replacementPodNamespace, replacementPodName,
		"iptables", "-I", "OUTPUT", "-o", "lo", "-p", "tcp", "--dport", kubeSchedulerProbePort, "-j", "REJECT")
	require.NoError(t, err, "failed to inject the kube-scheduler probe block: %s", faultOutput)
	faultCleared := false
	defer func() {
		if !faultCleared {
			_, _ = cluster.ExecOnPod(clients, replacementPodNamespace, replacementPodName,
				"iptables", "-D", "OUTPUT", "-o", "lo", "-p", "tcp", "--dport", kubeSchedulerProbePort, "-j", "REJECT")
		}
	}()

	checkOutput, err := cluster.ExecOnPod(clients, replacementPodNamespace, replacementPodName,
		"iptables", "-C", "OUTPUT", "-o", "lo", "-p", "tcp", "--dport", kubeSchedulerProbePort, "-j", "REJECT")
	require.NoError(t, err, "kube-scheduler probe block was not present after injection: %s", checkOutput)

	// Confirm the replacement has not reported a healthy plan while the probe is blocked.
	var planSecret *corev1.Secret
	require.Eventually(t, func() bool {
		planSecret, err = clients.Core.Secret().Get(secondMachine.Namespace, planSecretName, metav1.GetOptions{})
		return err == nil
	}, 2*time.Minute, 2*time.Second, "replacement plan secret never appeared")
	require.Empty(t, planSecret.Annotations[planapi.PlanProbesPassedAnnotation], "replacement probes passed before the fault was injected")

	// Verify CAPI sees the replacement Node as ready despite the failed component probe.
	var secondNodeName string
	require.Eventually(t, func() bool {
		m, err := clients.CAPI.Machine().Get(secondMachine.Namespace, secondMachine.Name, metav1.GetOptions{})
		if err != nil || !m.Status.NodeRef.IsDefined() || !conditions.IsTrue(m, capi.MachineNodeReadyCondition) {
			return false
		}
		secondNodeName = m.Status.NodeRef.Name
		return true
	}, 20*time.Minute, 2*time.Second, "replacement never reached NodeReady while its kube-scheduler probe was blocked")

	// Wait for deletion to begin so the protection checks exercise the pre-terminate hook.
	require.Eventually(t, func() bool {
		m, err := clients.CAPI.Machine().Get(firstMachine.Namespace, firstMachine.Name, metav1.GetOptions{})
		return err == nil && !m.DeletionTimestamp.IsZero() && m.Annotations[preTerminateHook] != ""
	}, 15*time.Minute, 2*time.Second, "initial machine never entered protected deletion")

	// Keep the fault active across multiple reconciliation cycles and verify the old Machine and Node remain protected.
	protectionDeadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(protectionDeadline) {
		oldMachine, err := clients.CAPI.Machine().Get(firstMachine.Namespace, firstMachine.Name, metav1.GetOptions{})
		require.NoError(t, err, "initial machine was removed while the replacement probe was failing")
		require.Equal(t, firstMachine.UID, oldMachine.UID)
		require.NotEmpty(t, oldMachine.Annotations[preTerminateHook], "pre-terminate hook was removed while the replacement probe was failing")

		replacement, err := clients.CAPI.Machine().Get(secondMachine.Namespace, secondMachine.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.True(t, conditions.IsTrue(replacement, capi.MachineNodeReadyCondition), "replacement unexpectedly lost NodeReady")
		planSecret, err := clients.Core.Secret().Get(secondMachine.Namespace, planSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, corev1.SecretType(capr.SecretTypeMachinePlan), planSecret.Type)
		require.Equal(t, "true", planSecret.Labels[capr.InitNodeLabel], "replacement is not the elected init node")
		require.Empty(t, planSecret.Annotations[planapi.PlanProbesPassedAnnotation], "replacement probes passed while the kube-scheduler probe was blocked")

		oldNode, err := clusterClients.Core.Node().Get(firstNodeName, metav1.GetOptions{})
		require.NoError(t, err, "initial downstream node disappeared while the replacement probe was failing")
		require.True(t, nodeReady(oldNode), "initial downstream node became NotReady while the replacement probe was failing")
		time.Sleep(2 * time.Second)
	}

	// Remove the fault so the replacement can complete its machine plan.
	_, err = cluster.ExecOnPod(clients, replacementPodNamespace, replacementPodName,
		"iptables", "-D", "OUTPUT", "-o", "lo", "-p", "tcp", "--dport", kubeSchedulerProbePort, "-j", "REJECT")
	require.NoError(t, err, "failed to restore the replacement's kube-scheduler health probe")
	faultCleared = true

	// Wait for the repaired replacement to report a healthy plan.
	require.Eventually(t, func() bool {
		planSecret, err := clients.Core.Secret().Get(secondMachine.Namespace, planSecretName, metav1.GetOptions{})
		return err == nil && planSecret.Annotations[planapi.PlanProbesPassedAnnotation] != ""
	}, 5*time.Minute, 2*time.Second, "replacement plan probes did not recover")

	// Verify the old CAPI Machine is removed after the replacement becomes healthy.
	require.Eventually(t, func() bool {
		_, err := clients.CAPI.Machine().Get(firstMachine.Namespace, firstMachine.Name, metav1.GetOptions{})
		return apierror.IsNotFound(err)
	}, 15*time.Minute, 2*time.Second, "initial machine was not removed after the replacement recovered")

	// Verify the replacement is the only remaining Machine and is ready.
	require.Eventually(t, func() bool {
		ml, err := cluster.Machines(clients, c)
		return err == nil && len(ml.Items) == 1 && ml.Items[0].UID == secondMachine.UID &&
			ml.Items[0].DeletionTimestamp.IsZero() && conditions.IsTrue(&ml.Items[0], capi.MachineNodeReadyCondition)
	}, 15*time.Minute, 2*time.Second, "cluster did not converge to the recovered replacement")

	// Verify the old downstream Node is also removed.
	require.Eventually(t, func() bool {
		_, err := clusterClients.Core.Node().Get(firstNodeName, metav1.GetOptions{})
		return apierror.IsNotFound(err)
	}, 10*time.Minute, 2*time.Second, "initial downstream node was not removed")

	secondNode, err := clusterClients.Core.Node().Get(secondNodeName, metav1.GetOptions{})
	require.NoError(t, err)
	require.True(t, nodeReady(secondNode), "replacement downstream node is not Ready")
	require.False(t, secondNode.Spec.Unschedulable, "replacement downstream node is cordoned")

	// Verify the provisioning Cluster returns to its fully ready state.
	_, err = cluster.WaitForCreate(clients, c)
	require.NoError(t, err, "cluster did not return to Ready after the replacement recovered")
}

func nodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// unstructuredString safely returns a top-level string field from an Unstructured.Object
func unstructuredString(obj map[string]any, key string) (string, bool) {
	raw, ok := obj[key]
	if !ok {
		return "", false
	}
	s, ok2 := raw.(string)
	return s, ok2
}
