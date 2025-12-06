// Package custom contains integration tests for custom (non-machine-provisioned) v2prov clusters.
// Custom clusters use manually created systemd-node pods to simulate node registration
// via the cluster registration token command.
package custom

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/tests/testhelpers"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_Provisioning_Custom_OneNodeWithDelete creates a custom single-node cluster
// with all roles (worker, etcd, controlplane), verifies it provisions correctly,
// and then deletes it to ensure cleanup works properly.
// This test is skipped for RKE2 distributions.
func Test_Provisioning_Custom_OneNodeWithDelete(t *testing.T) {
	if strings.ToLower(os.Getenv("DIST")) == "rke2" {
		t.Skip()
	}
	tc := testhelpers.NewTestClients(t)

	c, err := testhelpers.CreateCustomCluster(t, tc, "test-custom-one-node", nil)
	if err != nil {
		t.Fatal(err)
	}

	command := testhelpers.GetCustomCommand(t, tc, c)

	testhelpers.CreateCustomClusterNode(t, tc, c, command, testhelpers.CustomClusterNodeOptions{
		Worker:       true,
		ControlPlane: true,
		Etcd:         true,
		Labels:       []string{"foo=bar", "ball=life"},
	})

	c = testhelpers.WaitForClusterReady(t, tc, c, 1)

	machines, err := cluster.Machines(tc.Clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, machines.Items[0].Labels[capr.WorkerRoleLabel], "true")
	assert.Equal(t, machines.Items[0].Labels[capr.ControlPlaneRoleLabel], "true")
	assert.Equal(t, machines.Items[0].Labels[capr.EtcdRoleLabel], "true")
	assert.Len(t, machines.Items[0].Status.Addresses, 2)
	assert.NotNil(t, machines.Items[0].Spec.Bootstrap.ConfigRef)

	secret, err := tc.Core.Secret().Get(machines.Items[0].Namespace, capr.PlanSecretFromBootstrapName(machines.Items[0].Spec.Bootstrap.ConfigRef.Name), metav1.GetOptions{})
	assert.NoError(t, err)

	assert.NotEmpty(t, secret.Annotations[capr.LabelsAnnotation])
	var labels map[string]string
	if err := json.Unmarshal([]byte(secret.Annotations[capr.LabelsAnnotation]), &labels); err != nil {
		t.Error(err)
	}
	assert.Equal(t, labels, map[string]string{"cattle.io/os": "linux", "foo": "bar", "ball": "life"})

	// Delete the cluster and wait for cleanup.
	testhelpers.DeleteClusterAndWait(t, tc, c)

	testhelpers.EnsureMinimalConflicts(t, tc, c)
}

// Test_Provisioning_Custom_ThreeNode creates a custom three-node cluster
// with all roles on each node. This tests multi-node custom cluster provisioning
// and verifies that all nodes are properly registered with the correct roles.
func Test_Provisioning_Custom_ThreeNode(t *testing.T) {
	tc := testhelpers.NewTestClients(t)

	c, err := testhelpers.CreateCustomCluster(t, tc, "test-custom-three-node", nil)
	if err != nil {
		t.Fatal(err)
	}

	command := testhelpers.GetCustomCommand(t, tc, c)

	for i := 0; i < 3; i++ {
		testhelpers.CreateCustomClusterNode(t, tc, c, command, testhelpers.CustomClusterNodeOptions{
			Worker:       true,
			ControlPlane: true,
			Etcd:         true,
			Labels:       []string{"rancher=awesome"},
		})
	}

	c = testhelpers.WaitForClusterReady(t, tc, c, 3)

	machines, err := cluster.Machines(tc.Clients, c)
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range machines.Items {
		assert.Equal(t, m.Labels[capr.WorkerRoleLabel], "true")
		assert.Equal(t, m.Labels[capr.ControlPlaneRoleLabel], "true")
		assert.Equal(t, m.Labels[capr.EtcdRoleLabel], "true")
		assert.NotNil(t, machines.Items[0].Spec.Bootstrap.ConfigRef)

		secret, err := tc.Core.Secret().Get(machines.Items[0].Namespace, capr.PlanSecretFromBootstrapName(machines.Items[0].Spec.Bootstrap.ConfigRef.Name), metav1.GetOptions{})
		assert.NoError(t, err)

		assert.NotEmpty(t, secret.Annotations[capr.LabelsAnnotation])
		var labels map[string]string
		if err := json.Unmarshal([]byte(secret.Annotations[capr.LabelsAnnotation]), &labels); err != nil {
			t.Error(err)
		}
		assert.Equal(t, labels, map[string]string{"cattle.io/os": "linux", "rancher": "awesome"})
	}
	testhelpers.EnsureMinimalConflicts(t, tc, c)
}

// Test_Provisioning_Custom_UniqueRoles creates a custom cluster with separate nodes
// for each role: 3 etcd nodes, 1 control plane node, and 1 worker node.
// This tests the ability to run a cluster with dedicated role assignments.
func Test_Provisioning_Custom_UniqueRoles(t *testing.T) {
	tc := testhelpers.NewTestClients(t)

	c, err := testhelpers.CreateCustomCluster(t, tc, "test-custom-unique-roles", nil)
	if err != nil {
		t.Fatal(err)
	}

	command := testhelpers.GetCustomCommand(t, tc, c)

	// Create 3 etcd nodes
	for i := 0; i < 3; i++ {
		testhelpers.CreateCustomClusterNode(t, tc, c, command, testhelpers.CustomClusterNodeOptions{
			Etcd: true,
		})
	}

	// Create 1 controlplane node
	testhelpers.CreateCustomClusterNode(t, tc, c, command, testhelpers.CustomClusterNodeOptions{
		ControlPlane: true,
	})

	// Create 1 worker node
	testhelpers.CreateCustomClusterNode(t, tc, c, command, testhelpers.CustomClusterNodeOptions{
		Worker: true,
	})

	c = testhelpers.WaitForClusterReady(t, tc, c, 5)

	machines, err := cluster.Machines(tc.Clients, c)
	if err != nil {
		t.Fatal(err)
	}

	var (
		worker       = 0
		controlPlane = 0
		etcd         = 0
	)
	for _, m := range machines.Items {
		if m.Labels[capr.WorkerRoleLabel] == "true" {
			worker++
		}
		if m.Labels[capr.ControlPlaneRoleLabel] == "true" {
			controlPlane++
		}
		if m.Labels[capr.EtcdRoleLabel] == "true" {
			etcd++
		}
	}

	assert.Equal(t, worker, 1)
	assert.Equal(t, etcd, 3)
	assert.Equal(t, controlPlane, 1)
	testhelpers.EnsureMinimalConflicts(t, tc, c)
}

// Test_Provisioning_Custom_ThreeNodeWithTaints creates a three-node custom cluster
// and applies a taint to one of the nodes. This tests the ability to pass taints
// via the node registration command.
// This test is skipped for RKE2 distributions.
func Test_Provisioning_Custom_ThreeNodeWithTaints(t *testing.T) {
	if strings.ToLower(os.Getenv("DIST")) == "rke2" {
		t.Skip()
	}
	tc := testhelpers.NewTestClients(t)

	c, err := testhelpers.CreateCustomCluster(t, tc, "test-custom-three-node-with-taints", nil)
	if err != nil {
		t.Fatal(err)
	}

	command := testhelpers.GetCustomCommand(t, tc, c)

	for i := 0; i < 3; i++ {
		opts := testhelpers.CustomClusterNodeOptions{
			Worker:       true,
			ControlPlane: true,
			Etcd:         true,
			Labels:       []string{"rancher=awesome"},
		}
		// Put a taint on one of the nodes.
		if i == 1 {
			opts.Taints = []string{"key=value:NoExecute"}
		}
		testhelpers.CreateCustomClusterNode(t, tc, c, command, opts)
	}

	c = testhelpers.WaitForClusterReady(t, tc, c, 3)

	machines, err := cluster.Machines(tc.Clients, c)
	if err != nil {
		t.Fatal(err)
	}

	var taintFound bool
	for _, m := range machines.Items {
		assert.Equal(t, m.Labels[capr.WorkerRoleLabel], "true")
		assert.Equal(t, m.Labels[capr.ControlPlaneRoleLabel], "true")
		assert.Equal(t, m.Labels[capr.EtcdRoleLabel], "true")
		assert.NotNil(t, m.Spec.Bootstrap.ConfigRef)

		secret, err := tc.Core.Secret().Get(m.Namespace, capr.PlanSecretFromBootstrapName(m.Spec.Bootstrap.ConfigRef.Name), metav1.GetOptions{})
		assert.NoError(t, err)

		assert.NotEmpty(t, secret.Annotations[capr.LabelsAnnotation])
		var labels map[string]string
		if err := json.Unmarshal([]byte(secret.Annotations[capr.LabelsAnnotation]), &labels); err != nil {
			t.Error(err)
		}
		assert.Equal(t, labels, map[string]string{"cattle.io/os": "linux", "rancher": "awesome"})

		if len(secret.Annotations[capr.TaintsAnnotation]) != 0 {
			// Only one node should have the taint
			assert.False(t, taintFound)

			var taints []corev1.Taint
			if err := json.Unmarshal([]byte(secret.Annotations[capr.TaintsAnnotation]), &taints); err != nil {
				t.Error(err)
			}

			assert.Equal(t, len(taints), 1)
			assert.Equal(t, taints[0].Key, "key")
			assert.Equal(t, taints[0].Value, "value")
			assert.Equal(t, taints[0].Effect, corev1.TaintEffect("NoExecute"))
			taintFound = true
		}
	}

	assert.True(t, taintFound)
	testhelpers.EnsureMinimalConflicts(t, tc, c)
}
