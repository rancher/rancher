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
		assert.Equal(t, string(capi.OldestMachineSetDeletePolicy), machineSet.Spec.DeletePolicy)
	}
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

func Test_Provisioning_MP_MultipleEtcdNodesScaledDownThenDelete(t *testing.T) {
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

func Test_Provisioning_Single_Node_All_Roles_Drain(t *testing.T) {
	clients, err := clients.New()
	require.NoError(t, err)
	defer clients.Close()

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
			if hash != "" && hash != firstMachineHash && m.Status.NodeRef != nil {
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
		if err != nil || m.Status.NodeRef == nil {
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

// unstructuredString safely returns a top-level string field from an Unstructured.Object
func unstructuredString(obj map[string]any, key string) (string, bool) {
	raw, ok := obj[key]
	if !ok {
		return "", false
	}
	s, ok2 := raw.(string)
	return s, ok2
}
