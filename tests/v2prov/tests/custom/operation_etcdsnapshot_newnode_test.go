package custom

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/rancher/tests/v2prov/systemdnode"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_Operation_Custom_EtcdSnapshotOperationsOnNewNode creates a custom 2 node cluster with a controlplane+worker and
// etcd node, creates a configmap, takes a snapshot of the cluster, deletes the configmap, then deletes the etcd machine/node
// It then creates a new etcd node and restores from local snapshot file. This validates that it is possible to restore
// a snapshot on a completely new etcd node from file (without a corresponding snapshot file)
func Test_Operation_Custom_EtcdSnapshotOperationsOnNewNode(t *testing.T) {
	configmapName := "my-configmap-" + name.Hex(time.Now().String(), 10)
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-etcd-snapshot-operations-on-new-node",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	command, err := cluster.CustomCommand(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, command)

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --controlplane", map[string]string{"custom-cluster-name": c.Name}, nil)
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := os.TempDir() + "/snapshot-" + name.Hex(time.Now().String(), 5)

	// TODO: defer to remove the temp dir

	// store the snapshots in a universal directory
	etcdSnapshotDir := []string{
		fmt.Sprintf("%s:/var/lib/rancher/%s/server/db/snapshots", tmpDir, capr.GetRuntime(c.Spec.KubernetesVersion)),
	}

	// Taint the etcd node so that we don't have important (namely coredns) workloads running on the node that we are deleting
	var etcdNode *corev1.Pod
	etcdNode, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd", map[string]string{"custom-cluster-name": c.Name}, etcdSnapshotDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	clientset, err := operations.GetDownstreamClientset(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	// Create a configmap called "myspecialconfigmap" that we will delete after taking a snapshot.
	_, err = clientset.CoreV1().ConfigMaps("default").Create(context.TODO(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: configmapName,
		},
		Data: map[string]string{
			"test": "wow",
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := operations.CreateSnapshot(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotNil(t, snapshot)

	err = clientset.CoreV1().ConfigMaps(corev1.NamespaceDefault).Delete(context.TODO(), configmapName, metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the configmap no longer exists
	configMap, expectedErr := clientset.CoreV1().ConfigMaps(corev1.NamespaceDefault).Get(context.TODO(), configmapName, metav1.GetOptions{})
	if !apierrors.IsNotFound(expectedErr) {
		t.Fatal(expectedErr)
	}

	err = clients.Core.Pod().Delete(etcdNode.Namespace, etcdNode.Name, &metav1.DeleteOptions{PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0]})
	if err != nil {
		t.Fatal(err)
	}

	// Delete the machine from the cluster too...
	oldEtcdMachines, err := clients.CAPI.Machine().List(c.Namespace, metav1.ListOptions{LabelSelector: capr.EtcdRoleLabel + "=true"})
	if err != nil {
		t.Fatal(err)
	}

	for _, machine := range oldEtcdMachines.Items {
		err = clients.CAPI.Machine().Delete(machine.Namespace, machine.Name, &metav1.DeleteOptions{PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0]})
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = cluster.WaitForControlPlane(clients, c, "rkecontrolplane ready condition indicating insane cluster", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return strings.Contains(capr.Ready.GetMessage(&rkeControlPlane.Status), "waiting for at least one control plane, etcd, and worker node to be registered"), nil
	})

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd", map[string]string{"custom-cluster-name": c.Name}, etcdSnapshotDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForControlPlane(clients, c, "rkecontrolplane ready condition indicating restoration required", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return strings.Contains(capr.Ready.GetMessage(&rkeControlPlane.Status), "rkecontrolplane was already initialized but no etcd machines exist that have plans, indicating the etcd plane has been entirely replaced. Restoration from etcd snapshot is required."), nil
	})

	err = operations.RestoreSnapshot(clients, c, snapshot.SnapshotFile.Name)
	if err != nil {
		t.Fatal(err)
	}

	// Check for the configmap!
	configMap, err = clientset.CoreV1().ConfigMaps(corev1.NamespaceDefault).Get(context.TODO(), configmapName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	assert.NotNil(t, configMap)
}
