package custom

import (
	"context"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/systemdnode"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8swait "k8s.io/apimachinery/pkg/util/wait"
)

const mgmtClusterNameAnn = "provisioning.cattle.io/management-cluster-name"

// Test_Provisioning_Custom_RecreateWithSameManagementClusterName creates a custom provisioning cluster pinned to a
// management cluster name via the provisioning.cattle.io/management-cluster-name annotation, then deletes and
// immediately recreates it with the same name and annotation (the equivalent of `kubectl replace --force`). Custom
// cluster deletion does not remove the node infrastructure, so the original node (and its agent connection) stays
// alive while the replacement is registered with a new node. The replacement cluster must come up with a new
// management cluster that does not carry over state (i.e. the CA cert) from the previous one.
func Test_Provisioning_Custom_RecreateWithSameManagementClusterName(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	rand, err := randomtoken.Generate()
	require.NoError(t, err)
	mgmtClusterName := name.SafeConcatName("c", "m", rand[:8])

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-recreate-same-mgmt-cluster-name",
			Annotations: map[string]string{
				mgmtClusterNameAnn: mgmtClusterName,
			},
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Build the replacement from the object as the server returned it, as `kubectl get -o yaml | kubectl replace
	// --force -f -` would, so that it references the same registries, etc.
	replacement := &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
			Annotations: map[string]string{
				mgmtClusterNameAnn: mgmtClusterName,
			},
		},
		Spec: *c.Spec.DeepCopy(),
	}

	command, err := cluster.CustomCommand(clients, c)
	require.NoError(t, err)
	require.NotEmpty(t, command)

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --etcd --controlplane", map[string]string{"custom-cluster-name": c.Name}, nil)
	require.NoError(t, err)

	c, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, mgmtClusterName, c.Status.ClusterName)

	oldMgmtCluster, err := clients.Mgmt.Cluster().Get(mgmtClusterName, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, oldMgmtCluster.Status.CACert)

	oldMgmtNamespace, err := clients.Core.Namespace().Get(mgmtClusterName, metav1.GetOptions{})
	require.NoError(t, err)

	// Mirror `kubectl replace --force`: delete, poll every second until the object is gone, then create. The original
	// systemd-node pod is intentionally left running.
	err = clients.Provisioning.Cluster().Delete(c.Namespace, c.Name, &metav1.DeleteOptions{})
	require.NoError(t, err)

	err = k8swait.PollUntilContextTimeout(clients.Ctx, time.Second, time.Duration(defaults.WatchTimeoutSeconds)*time.Second, true, func(context.Context) (bool, error) {
		_, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if apierror.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	require.NoError(t, err, "original provisioning cluster was not deleted")

	c, err = clients.Provisioning.Cluster().Create(replacement)
	require.NoError(t, err)

	newMgmtCluster := oldMgmtCluster
	err = wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, oldMgmtCluster, func(obj runtime.Object) (bool, error) {
		newMgmtCluster = obj.(*v3.Cluster)
		return newMgmtCluster.UID != oldMgmtCluster.UID, nil
	})
	require.NoError(t, err, "replacement management cluster was not created")

	// The old management cluster's namespace (and the registration tokens in it) can outlive the old management
	// cluster, so wait for the namespace to be recreated before fetching the registration command; otherwise the old
	// cluster's (soon to be deleted) token can be picked up.
	err = k8swait.PollUntilContextTimeout(clients.Ctx, time.Second, time.Duration(defaults.WatchTimeoutSeconds)*time.Second, true, func(context.Context) (bool, error) {
		ns, err := clients.Core.Namespace().Get(mgmtClusterName, metav1.GetOptions{})
		if apierror.IsNotFound(err) {
			return false, nil
		}
		return err == nil && ns.UID != oldMgmtNamespace.UID && ns.DeletionTimestamp == nil, err
	})
	require.NoError(t, err, "replacement management cluster namespace was not created")

	oldCommand := command
	command, err = cluster.CustomCommand(clients, c)
	require.NoError(t, err)
	require.NotEmpty(t, command)
	require.NotEqual(t, oldCommand, command, "replacement cluster registration command reuses the old cluster's token")

	// No node has been registered to the replacement cluster yet, so it must not be considered connected. The
	// clusterconnected checker runs every 15 seconds; give it a couple of passes. The original node's agent is still
	// running, and its tunnel sessions (keyed only by management cluster name) are never closed when the old
	// management cluster is deleted, which makes the replacement appear connected.
	time.Sleep(35 * time.Second)
	newMgmtCluster, err = clients.Mgmt.Cluster().Get(mgmtClusterName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.False(t, condition.Cond("Connected").IsTrue(newMgmtCluster), "replacement management cluster is connected before any node was registered to it")

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --etcd --controlplane", map[string]string{"custom-cluster-name": c.Name}, nil)
	require.NoError(t, err)

	c, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	// WaitForCreate only checks the management cluster by name, so explicitly check that the replacement is ready.
	err = wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, newMgmtCluster, func(obj runtime.Object) (bool, error) {
		newMgmtCluster = obj.(*v3.Cluster)
		return newMgmtCluster.UID != oldMgmtCluster.UID && condition.Cond("Ready").IsTrue(newMgmtCluster), nil
	})
	require.NoError(t, err, "replacement management cluster did not become ready")

	assert.Equal(t, mgmtClusterName, c.Status.ClusterName)
	assert.NotEmpty(t, newMgmtCluster.Status.CACert)
	assert.NotEqual(t, oldMgmtCluster.Status.CACert, newMgmtCluster.Status.CACert, "replacement management cluster reused the CA cert of the previous management cluster")

	// The replacement provisioning cluster must not have been caught up in the removal of the old management cluster.
	c, err = clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Nil(t, c.DeletionTimestamp, "replacement provisioning cluster is being deleted")

	machines, err := cluster.Machines(clients, c)
	require.NoError(t, err)
	assert.Len(t, machines.Items, 1)

	// Ensure Rancher can actually talk to the new downstream cluster through the cluster proxy.
	clusterClients, err := clients.ForCluster(c.Namespace, c.Name)
	require.NoError(t, err)
	err = k8swait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, 2*time.Minute, true, func(context.Context) (bool, error) {
		nodes, err := clusterClients.Core.Node().List(metav1.ListOptions{})
		if err != nil {
			t.Logf("listing nodes of replacement cluster: %v", err)
			return false, nil
		}
		return len(nodes.Items) == 1, nil
	})
	assert.NoError(t, err, "unable to reach replacement downstream cluster")
}
