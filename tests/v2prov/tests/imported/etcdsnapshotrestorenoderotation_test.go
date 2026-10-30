package imported

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/managementuser/snapshotbackpopulate"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/objectstore"
	"github.com/rancher/rancher/tests/v2prov/registry"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

// Test_Imported_Operation_SetE_ImportedETCDSnapshotRestore3NodesAllRolesS3AfterNodeRotation restores
// an S3 snapshot onto an imported cluster none of whose nodes existed when the snapshot was taken.
//
// The cluster starts as three nodes holding every role, with the distro uploading its etcd snapshots
// to S3. After the snapshot is taken every node is replaced, one at a time: a new node joins, and once
// it is a voting etcd member and registered with Rancher, an old node is stopped and deleted, which
// also removes its etcd member. By the time the restore runs, the S3 copy is the only record of the
// snapshot left, and what it holds describes a cluster of nodes that are gone — its Node objects, and
// an etcd membership made of their members.
//
// So the restore has to rebuild the cluster on the new nodes alone: reset etcd on one of them, bring
// the other two back in as members of it, and clear out the Node objects of the old nodes the
// snapshot brought back. The test asserts that the snapshot's data is back, that all three new servers
// agree on one learner-free membership made of themselves, and that the downstream cluster and Rancher
// both list exactly the new nodes.
func Test_Imported_Operation_SetE_ImportedETCDSnapshotRestore3NodesAllRolesS3AfterNodeRotation(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	const displayName = "test-imported-restore-rotated-s3"
	version := defaults.SomeK8sVersion
	pool := cluster.ImportedNodePool{ControlPlane: true, ETCD: true, Worker: true, Quantity: 3}

	osInfo, err := objectstore.GetObjectStore(cs, "default", "imported-store0", "s3snapshots")
	require.NoError(t, err, "setting up object store")
	// One folder per run, so the snapshot this run restores is the only one the nodes can list.
	folder := displayName + "-" + strings.ToLower(name.Hex(time.Now().String(), 8))
	t.Logf("etcd snapshots go to s3://%s/%s at %s", osInfo.Bucket, folder, osInfo.Endpoint)

	// The S3 settings are part of the distro config, not Rancher's, so every replacement node has to
	// be given them too; NewImportedJoinNodePod is handed the same options for that reason.
	opts := importedS3Options(version, osInfo, folder)
	fx := setUpImportedClusterAtVersion(t, cs, displayName, []cluster.ImportedNodePool{pool}, version, opts)
	t.Logf("created imported cluster %s", fx.mgmtCluster.Name)

	originals := importedNodeNames(importedNodes(t, cs, fx))
	require.Len(t, originals, 3)

	proof := newConfigMapProof()
	proof.create(t, fx)

	snapshotsValidAfter := time.Now().Add(-30 * time.Second)
	RunETCDSnapshotSaveOperationTest(t, cs, fx.mgmtCluster.Name, fx.clusterRef)
	snapshot := waitForBackpopulatedSnapshotForStorage(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, snapshotbackpopulate.S3, snapshotsValidAfter)
	snapshotFile := snapshot.SnapshotFile.Name
	require.NotEmpty(t, snapshotFile, "back-populated snapshot %s has no file name", snapshot.Name)

	registryCACert, err := registry.EnsureRegistryCache(cs)
	require.NoError(t, err)

	rotation := newImportedNodeRotation(t, cs, fx, version, pool, registryCACert, opts)
	for i, old := range originals {
		rotation.replace(t, old, fmt.Sprintf("imported-rotated-%d", i))
	}
	replacements := importedNodeNames(importedNodes(t, cs, fx))
	t.Logf("replaced nodes %v with %v", originals, replacements)
	for _, old := range originals {
		require.NotContains(t, replacements, old)
	}

	// The CR the snapshot was found under may have belonged to a node that is now gone; the file in
	// the bucket is what survives, so find the CR that describes it now.
	restoreName := waitForImportedS3SnapshotFile(t, cs, fx.mgmtCluster.Name, snapshotFile)

	proof.delete(t, fx)

	RunETCDSnapshotRestoreOperationTest(t, cs, fx.mgmtCluster.Name, restoreName, fx.clusterRef)

	proof.assertRestored(t, fx)
	assertImportedServersConverged(t, cs, fx, importedETCDPods(t, cs, fx), version)
	waitForDownstreamNodeNames(t, fx, replacements)
	waitForMgmtNodeNames(t, cs, fx.mgmtCluster.Name, replacements)
}

// importedNodeRotation replaces the nodes of an imported cluster one at a time, keeping
// importedClusterFixture.pods — and so importedNodes — in step with the nodes that are running.
type importedNodeRotation struct {
	cs             *clients.Clients
	fx             *importedClusterFixture
	version        string
	pool           cluster.ImportedNodePool
	registryCACert []byte
	opts           cluster.ImportedClusterOptions

	dataDir        string
	supervisorPort int
}

func newImportedNodeRotation(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, version string, pool cluster.ImportedNodePool, registryCACert []byte, opts cluster.ImportedClusterOptions) *importedNodeRotation {
	t.Helper()

	require.Empty(t, pool.DistroDataDir, "node rotation assumes the default distro data directory")
	return &importedNodeRotation{
		cs:             cs,
		fx:             fx,
		version:        version,
		pool:           pool,
		registryCACert: registryCACert,
		opts:           opts,
		dataDir:        fmt.Sprintf("/var/lib/rancher/%s", capr.GetRuntime(version)),
		supervisorPort: capr.GetRuntimeSupervisorPort(version),
	}
}

// replace adds a node named newName, waits until it is a voting etcd member, Ready, and registered
// with Rancher, and only then takes oldName away, so the cluster never drops below three etcd members
// and keeps quorum throughout.
func (r *importedNodeRotation) replace(t *testing.T, oldName, newName string) {
	t.Helper()

	nodes := importedNodes(t, r.cs, r.fx)
	idx := slices.IndexFunc(nodes, func(n importedNode) bool { return n.name == oldName })
	require.GreaterOrEqual(t, idx, 0, "no node %s to replace", oldName)

	// Join through, and from here on drive the cluster from, a node that stays. The newest one is
	// furthest from being rotated out.
	var via importedNode
	for _, n := range nodes {
		if n.name != oldName {
			via = n
		}
	}
	require.NotEmpty(t, via.name, "no node besides %s to join %s through", oldName, newName)
	r.useForKubectl(via.pod)

	t.Logf("replacing %s with %s, joining through %s", oldName, newName, via.name)

	token, err := cluster.ImportedServerToken(r.cs, r.fx.ns.Name, via.pod.Name, r.dataDir)
	require.NoError(t, err)
	// The fixture's nodes run on their pods' primary addresses; see cluster.ImportedClusterOptions.Network.
	serverURL := "https://" + net.JoinHostPort(via.pod.Status.PodIP, strconv.Itoa(r.supervisorPort))

	pod, err := cluster.NewImportedJoinNodePod(r.cs, r.fx.ns.Name, r.version, newName, serverURL, token, r.pool, nil, r.registryCACert, r.opts)
	require.NoError(t, err)
	r.fx.pods = append(r.fx.pods, pod)

	withNew := append(importedNodeNames(nodes), newName)
	waitForImportedNodesReady(t, r.cs, r.fx.ns.Name, via.pod.Name, r.fx.kubectlEnv, withNew)
	r.waitForETCDMembers(t, via.pod, withNew)
	waitForMachinePlanSecret(t, r.cs, r.fx.mgmtCluster.Name, newName, true)

	// Stop the node before deleting its Node object, so its kubelet cannot register it again. Deleting
	// the Node is what makes the distro remove the node's etcd member, and Rancher its management Node
	// and, with it, the machine-plan secret the management Node owns.
	old := nodes[idx].pod
	r.deletePod(t, old)
	r.fx.pods = slices.DeleteFunc(r.fx.pods, func(p *corev1.Pod) bool { return p.Name == old.Name })

	out, err := r.fx.execKubectl(t, "kubectl delete node "+oldName)
	require.NoError(t, err, "deleting node %s: %s", oldName, out)

	remaining := slices.DeleteFunc(withNew, func(n string) bool { return n == oldName })
	r.waitForETCDMembers(t, via.pod, remaining)
	waitForMachinePlanSecret(t, r.cs, r.fx.mgmtCluster.Name, oldName, false)
}

// useForKubectl points the fixture's kubectl at pod, which must be a server. execKubectl otherwise
// keeps running on the init node, which the rotation takes away.
func (r *importedNodeRotation) useForKubectl(pod *corev1.Pod) {
	cs, ns, env := r.cs, r.fx.ns.Name, r.fx.kubectlEnv
	r.fx.execKubectl = func(t *testing.T, cmd string) (string, error) {
		t.Helper()
		return cluster.ExecOnPod(cs, ns, pod.Name, "sh", "-c", fmt.Sprintf("export %s && %s", env, cmd))
	}
}

func (r *importedNodeRotation) deletePod(t *testing.T, pod *corev1.Pod) {
	t.Helper()

	foreground := metav1.DeletePropagationForeground
	err := r.cs.Core.Pod().Delete(pod.Namespace, pod.Name, &metav1.DeleteOptions{PropagationPolicy: &foreground})
	require.NoError(t, err, "deleting node pod %s/%s", pod.Namespace, pod.Name)

	err = utilwait.PollUntilContextTimeout(r.cs.Ctx, 5*time.Second, 5*time.Minute, true, func(context.Context) (bool, error) {
		_, err := r.cs.Core.Pod().Get(pod.Namespace, pod.Name, metav1.GetOptions{})
		return apierrors.IsNotFound(err), nil
	})
	require.NoError(t, err, "node pod %s/%s was never deleted", pod.Namespace, pod.Name)
}

// waitForETCDMembers waits until the etcd that pod runs lists exactly one voting member for each of
// nodeNames, and nothing else.
func (r *importedNodeRotation) waitForETCDMembers(t *testing.T, pod *corev1.Pod, nodeNames []string) {
	t.Helper()

	want := slices.Sorted(slices.Values(nodeNames))
	var last string
	err := utilwait.PollUntilContextTimeout(r.cs.Ctx, 5*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
		members, err := readETCDMembers(t, r.cs, r.fx, pod, r.dataDir, r.supervisorPort)
		if err != nil {
			last = err.Error()
			return false, nil
		}
		last = fmt.Sprintf("%+v", members)

		var got []string
		for _, m := range members {
			if m.IsLearner {
				return false, nil
			}
			got = append(got, m.Name)
		}
		if len(got) != len(want) {
			return false, nil
		}
		for _, n := range want {
			if !slices.ContainsFunc(got, func(member string) bool { return strings.HasPrefix(member, n+"-") }) {
				return false, nil
			}
		}
		return true, nil
	})
	require.NoError(t, err, "etcd on %s never listed exactly the voting members %v (last read %s)", pod.Name, want, last)
}

// importedNodeNames returns the downstream names of nodes.
func importedNodeNames(nodes []importedNode) []string {
	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		names = append(names, n.name)
	}
	return names
}

// waitForMachinePlanSecret waits until Rancher has a machine-plan secret for the downstream node
// nodeName, if present, or no longer has one, if not. A secret means the node's system-agent has
// checked in, so the restore can deliver plans to it.
func waitForMachinePlanSecret(t *testing.T, cs *clients.Clients, clusterName, nodeName string, present bool) {
	t.Helper()

	err := utilwait.PollUntilContextTimeout(cs.Ctx, 5*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
		secrets, err := cs.Core.Secret().List(clusterName, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s,%s=%s", capr.ClusterNameLabel, clusterName, capr.NodeNameLabel, nodeName),
			FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan),
		})
		if err != nil {
			return false, nil
		}
		return (len(secrets.Items) > 0) == present, nil
	})
	require.NoError(t, err, "machine-plan secret for node %s in %s: want present=%v", nodeName, clusterName, present)
}

// waitForImportedS3SnapshotFile waits for an S3 ETCDSnapshot in the cluster's namespace that
// describes snapshotFile, and returns its name.
func waitForImportedS3SnapshotFile(t *testing.T, cs *clients.Clients, clusterName, snapshotFile string) string {
	t.Helper()

	var resolved string
	err := utilwait.PollUntilContextTimeout(cs.Ctx, 10*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
		list, err := cs.RKE.ETCDSnapshot().List(clusterName, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, clusterName),
		})
		if err != nil {
			return false, nil
		}
		for i := range list.Items {
			if list.Items[i].SnapshotFile.S3 != nil && list.Items[i].SnapshotFile.Name == snapshotFile {
				resolved = list.Items[i].Name
				return true, nil
			}
		}
		return false, nil
	})
	require.NoError(t, err, "no S3 ETCDSnapshot in %s describes snapshot file %s after the nodes were replaced", clusterName, snapshotFile)

	t.Logf("snapshot file %s is described by %s/%s", snapshotFile, clusterName, resolved)
	return resolved
}

// waitForDownstreamNodeNames waits until the downstream cluster lists exactly the nodes in want. After
// the restore, any other name is a Node object of a replaced node that the snapshot brought back and
// the restore failed to clean up.
func waitForDownstreamNodeNames(t *testing.T, fx *importedClusterFixture, want []string) {
	t.Helper()

	want = slices.Sorted(slices.Values(want))
	var last []string
	err := utilwait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 5*time.Minute, true, func(context.Context) (bool, error) {
		out, err := fx.execKubectl(t, "kubectl get nodes -o jsonpath='{.items[*].metadata.name}'")
		if err != nil {
			return false, nil
		}
		last = slices.Sorted(slices.Values(strings.Fields(out)))
		return slices.Equal(last, want), nil
	})
	require.NoError(t, err, "downstream cluster lists nodes %v, want %v", last, want)
}

// waitForMgmtNodeNames waits until Rancher's management Nodes for the cluster are exactly the
// downstream nodes in want.
func waitForMgmtNodeNames(t *testing.T, cs *clients.Clients, clusterName string, want []string) {
	t.Helper()

	want = slices.Sorted(slices.Values(want))
	var last []string
	err := utilwait.PollUntilContextTimeout(cs.Ctx, 5*time.Second, 5*time.Minute, true, func(context.Context) (bool, error) {
		nodes, err := cs.Mgmt.Node().List(clusterName, metav1.ListOptions{})
		if err != nil {
			return false, nil
		}
		last = last[:0]
		for _, n := range nodes.Items {
			last = append(last, n.Status.NodeName)
		}
		slices.Sort(last)
		return slices.Equal(last, want), nil
	})
	require.NoError(t, err, "Rancher lists management Nodes %v for %s, want %v", last, clusterName, want)
}
