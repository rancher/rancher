package custom

import (
	"context"
	"testing"
	"time"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
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
// etcd node, creates a configmap, takes a snapshot of the cluster, deletes the configmap, then restores from snapshot.
// This validates that it is possible to restore a snapshot.
func Test_Operation_Custom_EtcdSnapshotCreationRestore(t *testing.T) {
	configmapName := "my-configmap-" + name.Hex(time.Now().String(), 10)
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-etcd-snapshot-operations",
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

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd", map[string]string{"custom-cluster-name": c.Name}, nil)
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

	// The client will return a configmap object but it will not have anything populated.
	assert.Equal(t, "", configMap.Name)

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
