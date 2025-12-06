// Package testhelpers provides common helper functions for v2prov integration tests.
// These helpers reduce code duplication and provide consistent patterns for test setup,
// cluster creation, node management, and common assertions.
package testhelpers

import (
	"fmt"
	"testing"
	"time"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/systemdnode"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestClients wraps clients.Clients to provide test-specific cleanup functionality.
// It ensures that the client connection is properly closed when the test completes.
type TestClients struct {
	*clients.Clients
	t *testing.T
}

// NewTestClients creates a new Clients instance for testing and registers
// cleanup to be called when the test completes. This eliminates the need
// for manual defer statements in each test.
func NewTestClients(t *testing.T) *TestClients {
	t.Helper()
	c, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	tc := &TestClients{Clients: c, t: t}
	t.Cleanup(func() {
		c.Close()
	})
	return tc
}

// NodeRole represents the role(s) a node should have in the cluster.
// Multiple roles can be combined using bitwise OR.
type NodeRole int

const (
	// RoleWorker indicates the node should have the worker role.
	RoleWorker NodeRole = 1 << iota
	// RoleControlPlane indicates the node should have the control plane role.
	RoleControlPlane
	// RoleEtcd indicates the node should have the etcd role.
	RoleEtcd
)

// String returns a command-line flag string for the node roles.
func (r NodeRole) String() string {
	var roles string
	if r&RoleWorker != 0 {
		roles += " --worker"
	}
	if r&RoleControlPlane != 0 {
		roles += " --controlplane"
	}
	if r&RoleEtcd != 0 {
		roles += " --etcd"
	}
	return roles
}

// RoleAllInOne combines all three roles (worker, control plane, and etcd).
const RoleAllInOne = RoleWorker | RoleControlPlane | RoleEtcd

// CustomClusterNodeOptions contains options for creating a custom cluster node.
type CustomClusterNodeOptions struct {
	// Role specifies the role(s) for the node.
	Role NodeRole
	// Labels specifies additional labels to add to the node (format: "key=value").
	Labels []string
	// Taints specifies taints to add to the node (format: "key=value:Effect").
	Taints []string
	// NodeName specifies an explicit node name.
	NodeName string
	// HostPaths specifies volume mounts for the node container.
	HostPaths []string
}

// CreateCustomClusterNode creates a new systemd node for a custom cluster.
// It constructs the appropriate command with roles, labels, taints, and node name.
func CreateCustomClusterNode(t *testing.T, tc *TestClients, c *provisioningv1.Cluster, command string, opts CustomClusterNodeOptions) *corev1.Pod {
	t.Helper()

	cmd := "#!/usr/bin/env sh\n" + command + opts.Role.String()

	// Add labels
	for _, label := range opts.Labels {
		cmd += " --label " + label
	}

	// Add taints
	for _, taint := range opts.Taints {
		cmd += " --taint " + taint
	}

	// Add node name
	if opts.NodeName != "" {
		cmd += " --node-name " + opts.NodeName
	}

	pod, err := systemdnode.New(tc.Clients, c.Namespace, cmd, map[string]string{"custom-cluster-name": c.Name}, opts.HostPaths)
	if err != nil {
		t.Fatal(err)
	}
	return pod
}

// CreateCustomClusterWithNodes creates a custom cluster and adds the specified number
// of nodes with the given role. This is a convenience function for simple test setups.
func CreateCustomClusterWithNodes(t *testing.T, tc *TestClients, clusterName string, nodeCount int, role NodeRole) (*provisioningv1.Cluster, error) {
	t.Helper()

	c, err := cluster.New(tc.Clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{},
		},
	})
	if err != nil {
		return nil, err
	}

	command, err := cluster.CustomCommand(tc.Clients, c)
	if err != nil {
		return nil, err
	}

	assert.NotEmpty(t, command)

	for i := 0; i < nodeCount; i++ {
		CreateCustomClusterNode(t, tc, c, command, CustomClusterNodeOptions{Role: role})
	}

	return c, nil
}

// WaitForClusterReady waits for a cluster to become ready and verifies it has
// the expected number of machines.
func WaitForClusterReady(t *testing.T, tc *TestClients, c *provisioningv1.Cluster, expectedMachines int) *provisioningv1.Cluster {
	t.Helper()

	c, err := cluster.WaitForCreate(tc.Clients, c)
	if err != nil {
		t.Fatal(err)
	}

	if expectedMachines > 0 {
		machines, err := cluster.Machines(tc.Clients, c)
		if err != nil {
			t.Fatal(err)
		}
		assert.Len(t, machines.Items, expectedMachines)
	}

	return c
}

// EnsureMinimalConflicts verifies that the cluster hasn't experienced excessive
// conflict errors during provisioning.
func EnsureMinimalConflicts(t *testing.T, tc *TestClients, c *provisioningv1.Cluster) {
	t.Helper()
	err := cluster.EnsureMinimalConflictsWithThreshold(tc.Clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

// DeleteClusterAndWait deletes a cluster and waits for it to be fully cleaned up.
func DeleteClusterAndWait(t *testing.T, tc *TestClients, c *provisioningv1.Cluster) {
	t.Helper()

	err := tc.Provisioning.Cluster().Delete(c.Namespace, c.Name, &metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForDelete(tc.Clients, c)
	if err != nil {
		t.Fatal(err)
	}
}

// NewTestConfigMap creates a ConfigMap with a unique name for testing.
// The name includes a timestamp hash to ensure uniqueness across test runs.
func NewTestConfigMap(data map[string]string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-configmap-" + name.Hex(time.Now().String(), 10),
		},
		Data: data,
	}
}

// GetCustomCommand retrieves the custom cluster command and asserts it's not empty.
func GetCustomCommand(t *testing.T, tc *TestClients, c *provisioningv1.Cluster) string {
	t.Helper()

	command, err := cluster.CustomCommand(tc.Clients, c)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEmpty(t, command)
	return command
}

// CreateCustomCluster creates a new custom cluster with the given name and optional spec modifications.
func CreateCustomCluster(t *testing.T, tc *TestClients, clusterName string, specModifier func(*provisioningv1.ClusterSpec)) (*provisioningv1.Cluster, error) {
	t.Helper()

	spec := provisioningv1.ClusterSpec{
		RKEConfig: &provisioningv1.RKEConfig{},
	}

	if specModifier != nil {
		specModifier(&spec)
	}

	return cluster.New(tc.Clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
		Spec: spec,
	})
}

// CreateSnapshotTestDir creates a temporary directory path for etcd snapshot storage.
// This is used for tests that need to persist snapshots across node recreation.
func CreateSnapshotTestDir(prefix string, runtime string) string {
	return fmt.Sprintf("%s:/var/lib/rancher/%s/server/db/snapshots", prefix, runtime)
}
