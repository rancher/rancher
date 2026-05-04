package autoscaler

import (
	"context"
	"fmt"
	"testing"
	"time"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

func Test_Autoscaling_Provisioning(t *testing.T) {
	// this test only works if autoscaling is enabled and set up properly
	if !features.ClusterAutoscaling.Enabled() {
		t.Skip()
	}

	t.Parallel()

	tests := []struct {
		name             string
		workerPoolConfig provisioningv1.RKEMachinePool
		addLoad          bool
	}{
		{

			// scale-up provisions a cluster with autoscaling enabled on the worker pool,
			// deploys resource-intensive pods to the downstream cluster to trigger pending pods, and verifies the
			// cluster-autoscaler adds new worker nodes.
			name: "scale-up",
			workerPoolConfig: provisioningv1.RKEMachinePool{
				Name:               "scale-up-workers",
				WorkerRole:         true,
				Quantity:           &defaults.One,
				AutoscalingMinSize: &defaults.One,
				AutoscalingMaxSize: &defaults.Three,
			},
			addLoad: true,
		},
		{

			// scale-down provisions a cluster with an oversized worker pool
			// (Quantity=3 with min=1), then waits for the autoscaler to detect underutilization and scale
			// the worker pool down. waits a maximum of 15 minutes to exceed the default 10-minute scale-down-unneeded-time.
			name: "scale-down",
			workerPoolConfig: provisioningv1.RKEMachinePool{
				Name:               "scale-down-workers",
				WorkerRole:         true,
				Quantity:           &defaults.Three,
				AutoscalingMinSize: &defaults.One,
				AutoscalingMaxSize: &defaults.Three,
			},
			addLoad: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := clients.New()
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()

			c, err := cluster.New(client, &provisioningv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "autoscaler-" + tt.name,
				},
				Spec: provisioningv1.ClusterSpec{
					KubernetesVersion: defaults.SomeK8sVersion,
					RKEConfig: &provisioningv1.RKEConfig{
						MachinePools: []provisioningv1.RKEMachinePool{
							{
								Name:             "cp-etcd",
								EtcdRole:         true,
								ControlPlaneRole: true,
								WorkerRole:       true,
								Quantity:         &defaults.One,
							},
							tt.workerPoolConfig,
						},
					},
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			c, err = cluster.WaitForCreate(client, c)
			if err != nil {
				t.Fatal(err)
			}

			initialMachines, err := cluster.Machines(client, c)
			if err != nil {
				t.Fatal(err)
			}

			kc, err := operations.GetAndVerifyDownstreamClientset(client, c)
			if err != nil {
				t.Fatal(err)
			}

			// Wait for the cluster-autoscaler HelmOp to deploy successfully before creating load.
			err = waitForAutoscalerDeployment(client, name.SafeConcatName("autoscaler", c.Namespace, c.Name), c.Namespace)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("cluster-autoscaler deployment is ready")

			if tt.addLoad {
				deployLoadPods(t, kc)
			}

			err = waitForMachinePoolScaling(t, client, c, func(count int) bool {
				// if we added load -> we're expecting machines to be added
				if tt.addLoad {
					return count > len(initialMachines.Items)
				}

				// otherwise, reduced
				return count < len(initialMachines.Items)
			}, 20*time.Minute)
			if err != nil {
				t.Fatalf("autoscaler did not scale in time: %v", err)
			}

			err = cluster.EnsureMinimalConflictsWithThreshold(client, c, cluster.SaneConflictMessageThreshold)
			assert.NoError(t, err)
		})
	}
}

// waitForMachinePoolScaling polls the machine count for a cluster until the given condition
// function returns true or the timeout expires.
func waitForMachinePoolScaling(t *testing.T, client *clients.Clients, c *provisioningv1.Cluster, condition func(int) bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		machines, err := cluster.Machines(client, c)
		if err != nil {
			t.Logf("error listing machines: %v", err)
		} else {
			count := len(machines.Items)
			t.Logf("current machine count: %d", count)
			if condition(count) {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for machine count condition after %v", timeout)
		}
		time.Sleep(30 * time.Second)
	}
}

// waitForAutoscalerDeployment waits for the appropriate downstream autoscaler helmop to be "Ready" before adding load to the cluster
func waitForAutoscalerDeployment(client *clients.Clients, name, namespace string) error {
	// watch helmops - once the expected name becomes ready return
	return wait.Object(client.Ctx, client.Fleet.HelmOp().Watch, &fleet.HelmOp{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}, func(obj runtime.Object) (bool, error) {
		helmop := obj.(*fleet.HelmOp)
		if helmop.Status.Summary.DesiredReady == 0 {
			return false, nil
		}

		return helmop.Status.Summary.Ready == helmop.Status.Summary.DesiredReady, nil
	})
}

// deployLoadPods creates a deployment with resource-intensive pods in the downstream cluster.
// The pods request enough resources that they cannot all fit on a single worker node, causing
// some to enter Pending state and triggering the cluster-autoscaler to scale up.
func deployLoadPods(t *testing.T, kc *kubernetes.Clientset) {
	replicas := int32(1000)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "autoscaler-load-test",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "autoscaler-load-test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "autoscaler-load-test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "busybox",
							Image:   "busybox",
							Command: []string{"sleep", "infinity"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	err := retry.OnError(defaults.DownstreamRetry, func(error) bool { return true }, func() error {
		_, err := kc.AppsV1().Deployments("default").Create(context.TODO(), deploy, metav1.CreateOptions{})
		return err
	})
	if err != nil {
		t.Fatalf("failed to create load deployment: %v", err)
	}
	t.Log("deployed load pods to trigger autoscaler scale-up")
}
