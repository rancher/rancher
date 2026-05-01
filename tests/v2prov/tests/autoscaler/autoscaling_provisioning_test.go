package autoscaler

import (
	"context"
	"fmt"
	"testing"
	"time"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
)

// Test_Autoscaling_Provisioning_ScaleUp provisions a cluster with autoscaling enabled on the worker pool,
// deploys resource-intensive pods to the downstream cluster to trigger pending pods, and verifies the
// cluster-autoscaler adds new worker nodes.
func Test_Autoscaling_Provisioning_ScaleUp(t *testing.T) {
	client, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	c, err := cluster.New(client, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-autoscaling-scale-up",
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
						Quantity:         ptr.To[int32](1),
					},
					{
						Name:               "autoscale-workers",
						WorkerRole:         true,
						Quantity:           ptr.To[int32](1),
						AutoscalingMinSize: ptr.To[int32](1),
						AutoscalingMaxSize: ptr.To[int32](3),
					},
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

	// Wait for the cluster-autoscaler HelmOp to deploy successfully before creating load.
	err = waitForAutoscalerDeployment(client, c)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("cluster-autoscaler deployment is ready")

	initialMachines, err := cluster.Machines(client, c)
	if err != nil {
		t.Fatal(err)
	}
	initialCount := len(initialMachines.Items)
	t.Logf("initial machine count: %d", initialCount)
	assert.Equal(t, 2, initialCount, "expected 2 initial machines (1 cp + 1 worker)")

	kc, err := operations.GetAndVerifyDownstreamClientset(client, c)
	if err != nil {
		t.Fatal(err)
	}

	deployLoadPods(t, kc)

	// Wait for the autoscaler to scale up (new machines should appear).
	err = waitForMachineCountCondition(t, client, c, func(count int) bool {
		return count > initialCount
	}, 20*time.Minute)
	if err != nil {
		t.Fatalf("autoscaler did not scale up: %v", err)
	}

	finalMachines, err := cluster.Machines(client, c)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("final machine count after scale-up: %d", len(finalMachines.Items))
	assert.Greater(t, len(finalMachines.Items), initialCount, "expected machine count to increase after scale-up")

	err = cluster.EnsureMinimalConflictsWithThreshold(client, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

// Test_Autoscaling_Provisioning_ScaleDown provisions a cluster with an oversized worker pool
// (Quantity=3 with min=1), then waits for the autoscaler to detect underutilization and scale
// the worker pool down. Sleeps 15 minutes to exceed the default 10-minute scale-down-unneeded-time.
func Test_Autoscaling_Provisioning_ScaleDown(t *testing.T) {
	client, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	c, err := cluster.New(client, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-autoscaling-scale-down",
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
						Quantity:         ptr.To[int32](1),
					},
					{
						Name:               "autoscale-workers",
						WorkerRole:         true,
						Quantity:           ptr.To[int32](3),
						AutoscalingMinSize: ptr.To[int32](1),
						AutoscalingMaxSize: ptr.To[int32](3),
					},
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

	// Wait for the cluster-autoscaler HelmOp to deploy successfully before measuring scale-down.
	err = waitForAutoscalerDeployment(client, c)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("cluster-autoscaler deployment is ready")

	initialMachines, err := cluster.Machines(client, c)
	if err != nil {
		t.Fatal(err)
	}
	initialCount := len(initialMachines.Items)
	t.Logf("initial machine count: %d", initialCount)
	assert.Equal(t, 4, initialCount, "expected 4 initial machines (1 cp + 3 workers)")

	// Sleep 15 minutes — the cluster-autoscaler default scale-down-unneeded-time is 10 minutes.
	// The extra buffer accounts for detection and execution lag.
	t.Log("sleeping 15 minutes to allow autoscaler scale-down...")
	time.Sleep(15 * time.Minute)

	// After the sleep, poll for the machine count to have decreased.
	err = waitForMachineCountCondition(t, client, c, func(count int) bool {
		return count < initialCount
	}, 10*time.Minute)
	if err != nil {
		t.Fatalf("autoscaler did not scale down: %v", err)
	}

	finalMachines, err := cluster.Machines(client, c)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("final machine count after scale-down: %d", len(finalMachines.Items))
	assert.Less(t, len(finalMachines.Items), initialCount, "expected machine count to decrease after scale-down")

	err = cluster.EnsureMinimalConflictsWithThreshold(client, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

// waitForMachineCountCondition polls the machine count for a cluster until the given condition
// function returns true or the timeout expires.
func waitForMachineCountCondition(t *testing.T, client *clients.Clients, c *provisioningv1.Cluster, condition func(int) bool, timeout time.Duration) error {
	t.Helper()
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

// waitForAutoscalerDeployment watches the provisioning cluster until the ClusterAutoscalerDeploymentReady
// condition is True, indicating the cluster-autoscaler HelmOp has been successfully deployed to the
// downstream cluster.
func waitForAutoscalerDeployment(client *clients.Clients, c *provisioningv1.Cluster) error {
	return wait.Object(client.Ctx, client.Provisioning.Cluster().Watch, c, func(obj runtime.Object) (bool, error) {
		cluster := obj.(*provisioningv1.Cluster)
		return capr.ClusterAutoscalerDeploymentReady.IsTrue(cluster), nil
	})
}

// deployLoadPods creates a deployment with resource-intensive pods in the downstream cluster.
// The pods request enough resources that they cannot all fit on a single worker node, causing
// some to enter Pending state and triggering the cluster-autoscaler to scale up.
func deployLoadPods(t *testing.T, kc *kubernetes.Clientset) {
	t.Helper()
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
