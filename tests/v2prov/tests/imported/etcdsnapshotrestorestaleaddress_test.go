package imported

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

// Test_Imported_Operation_SetD_ETCDSnapshotRestoreStaleAddressOutsideLeaderCertificate checks that a
// restore fails loudly, rather than splitting the cluster, when Rancher points the rejoining nodes at
// an address the restored leader's certificate does not cover.
//
// The nodes run IPv4-only — node-ip is IPv4, so that is all their serving certificates are issued
// for — inside dual-stack pods, with their Node addresses published by the test acting as an external
// cloud controller (cluster.ImportedClusterOptions.ExternalCloudProvider). After the snapshot is
// taken, the "cloud controller" starts reporting each node's IPv6 address as its primary InternalIP,
// the way a cloud that has just enabled IPv6 might, without anyone adding it to tls-san. Rancher
// syncs that onto its management Nodes, and ImportedAdapter.GetServerURL takes a node's primary
// InternalIP, so the restore points the rejoining nodes at the leader's IPv6 address.
//
// What makes that address unusable is the restore itself. On K3s/RKE2 an address listed on a server's
// own Node object is one its SAN filter (tls-san-security) allows, and it is certified the first time
// a client connects through it — so outside a restore this setup would quietly heal. But the restore
// rolls the downstream cluster back to the snapshot: the leader comes back with its Node objects, and
// its serving certificate secret, from before the IPv6 addresses existed, while Rancher's management
// Nodes still carry them. The leader's filter then rejects the address Rancher chose, and the first
// node told to rejoin through it fails certificate validation, which fails the restore at
// InitialRestartCluster.
//
// That failure has to be loud. Before a rejoining etcd node is restarted its etcd data is removed, so
// the distro treats the join URL's validation failure as fatal. With its old data still on disk, the
// distro would instead count the node as already initialized, ignore the failed validation, and start
// etcd as a new single-member cluster: a split brain the node's health probes pass, and the restore
// reported as a success. The test asserts the loud failure, and each link of it. A fix that stops
// Rancher acting on stale addresses — preferring addresses the leader itself certifies, say — should
// make the restore succeed here, at which point this test needs turning around.
//
// There is one race. If Rancher re-syncs the leader's management Node from the restored cluster
// before it hands out the rejoin plans, it picks the IPv4 address and the restore succeeds; the test
// then fails saying so, rather than with a misleading assertion. Needs a dual-stack host cluster, and
// skips without one: see dev-scripts/k3d-dual-stack.
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreStaleAddressOutsideLeaderCertificate(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	requireHostIPFamilies(t, cs, []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol})

	const leaderName = "imported-node-1"
	version := defaults.SomeK8sVersion
	runtime := capr.GetRuntime(version)
	dataDir := fmt.Sprintf("/var/lib/rancher/%s", runtime)
	port := strconv.Itoa(capr.GetRuntimeSupervisorPort(version))

	fx := setUpImportedClusterWithHook(t, cs, "rm-stale-address", []cluster.ImportedNodePool{
		{ETCD: true, ControlPlane: true, Quantity: 3},
	}, version, func(fx *importedClusterFixture) {
		// Act as the cloud controller for the first time: report what node-ip says, IPv4 only.
		waitForRegisteredNodes(t, fx, len(fx.pods))
		publishImportedNodeAddresses(t, cs, fx, corev1.IPv4Protocol)
	}, cluster.ImportedClusterOptions{
		ExtractServerToken:    true,
		ExternalCloudProvider: true,
		Network: &cluster.ImportedNetwork{
			IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol},
		},
	})

	var leader importedNode
	for _, node := range importedNodes(t, cs, fx) {
		if node.name == leaderName {
			leader = node
		}
	}
	require.NotEmpty(t, leader.name, "no node %s in the fixture", leaderName)
	leaderV6 := leader.ip(corev1.IPv6Protocol)
	staleURL := "https://" + net.JoinHostPort(leaderV6, port)

	proof := newConfigMapProof()
	proof.create(t, fx)

	before := time.Now()
	RunETCDSnapshotSaveOperationTest(t, cs, fx.mgmtCluster.Name, fx.clusterRef)
	snapshot := waitForBackpopulatedLocalSnapshotOnNode(t, cs, fx.mgmtCluster.Name, leaderName, before)

	// The cloud now reports IPv6 first. The snapshot predates this, so the restore will roll the
	// downstream cluster's view of it back — but not Rancher's.
	publishImportedNodeAddresses(t, cs, fx, corev1.IPv6Protocol, corev1.IPv4Protocol)
	waitForMgmtNodePrimaryAddress(t, cs, fx.mgmtCluster.Name, leaderName, leaderV6)

	proof.delete(t, fx)

	stopWatchingServerArgs := watchRestoreServerArgs(t, cs, fx.mgmtCluster.Name)
	defer stopWatchingServerArgs()

	op := CreateETCDSnapshotRestoreOp(t, cs, fx.mgmtCluster.Name, snapshot.Name, fx.clusterRef,
		WithRestoreMode(rkev1.RestoreRKEConfigNone),
		WithRestoreTTL(-1))
	op = waitForRestoreFinished(t, cs, op)
	seen := stopWatchingServerArgs()
	t.Logf("restore finished %s at step %q (%s); server drop-ins by node: %v",
		op.Status.Phase, op.Status.Step, opv1alpha1.FailedCondition.GetMessage(op), seen)

	// Rancher chose the leader's stale IPv6 address for at least one rejoining node.
	var pointedAtStale []string
	for node, values := range seen {
		for _, value := range values {
			if value == staleURL {
				pointedAtStale = append(pointedAtStale, node)
			}
		}
	}
	if len(pointedAtStale) == 0 {
		t.Fatalf("the restore never pointed a node at the leader's stale address %s (drop-ins %v, restore %s): "+
			"Rancher refreshed %s's management Node from the restored cluster before assigning the rejoin plans, "+
			"so this run did not reproduce the scenario", staleURL, seen, op.Status.Phase, leaderName)
	}
	t.Logf("restore pointed %v at the leader's stale address %s", pointedAtStale, staleURL)

	// The restore broke on it.
	require.Equal(t, opv1alpha1.OperationPhaseFailed, op.Status.Phase,
		"the restore should fail when rejoining nodes are pointed at %s", staleURL)
	assert.Equal(t, opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster, op.Status.Step,
		"the restore should fail at the initial restart, when nodes first rejoin the leader")

	// Because a rejoining node could not validate the leader's certificate for that address.
	unit := capr.GetRuntimeServerUnit(version)
	var rejected string
	err = utilwait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 2*time.Minute, true, func(context.Context) (bool, error) {
		for _, node := range importedNodes(t, cs, fx) {
			if node.name == leaderName {
				continue
			}
			out, err := cluster.ExecOnPod(cs, fx.ns.Name, node.pod.Name, "sh", "-c", fmt.Sprintf(
				"journalctl -u %s --no-pager -o cat 2>/dev/null | grep -F 'x509: certificate is valid for' | grep -F %q | tail -n1", unit, leaderV6))
			if err == nil && strings.TrimSpace(out) != "" {
				rejected = node.name + ": " + strings.TrimSpace(out)
				return true, nil
			}
		}
		return false, nil
	})
	require.NoError(t, err, "no rejoining node logged a certificate rejection for %s", leaderV6)
	t.Logf("certificate rejection as expected: %s", rejected)

	// And the restored leader's certificate really does omit the address: its SAN filter refuses it,
	// since its restored Node objects no longer list it.
	assertServingCertOmits(t, cs, fx.ns.Name, leader.pod.Name, runtime, leaderV6)
	assertProbeRejectedBySAN(t, cs, fx.ns.Name, leader.pod.Name, dataDir, staleURL)
}

// waitForRegisteredNodes waits until the downstream API server lists want nodes.
func waitForRegisteredNodes(t *testing.T, fx *importedClusterFixture, want int) {
	t.Helper()

	var lastOut string
	err := utilwait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
		out, err := fx.execKubectl(t, "kubectl get nodes -o name")
		lastOut = strings.TrimSpace(out)
		return err == nil && lastOut != "" && len(strings.Split(lastOut, "\n")) == want, nil
	})
	require.NoError(t, err, "downstream never listed %d nodes (last read %q)", want, lastOut)
}

// publishImportedNodeAddresses plays the external cloud controller for every node: it sets the
// node's addresses to one InternalIP per family, in the order given — so the first family is the
// node's primary address — plus its hostname, and clears the uninitialized taint the kubelet
// registers it with. It only sticks on nodes started with
// cluster.ImportedClusterOptions.ExternalCloudProvider; otherwise the kubelet overwrites it.
func publishImportedNodeAddresses(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, families ...corev1.IPFamily) {
	t.Helper()

	for _, node := range importedNodes(t, cs, fx) {
		var addresses []corev1.NodeAddress
		for _, family := range families {
			ip := node.ip(family)
			require.NotEmpty(t, ip, "node %s has no %s address to publish: %v", node.name, family, node.pod.Status.PodIPs)
			addresses = append(addresses, corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: ip})
		}
		addresses = append(addresses, corev1.NodeAddress{Type: corev1.NodeHostName, Address: node.name})

		patch, err := json.Marshal(map[string]any{"status": map[string]any{"addresses": addresses}})
		require.NoError(t, err)

		out, err := fx.execKubectl(t, fmt.Sprintf(
			"kubectl patch node %s --subresource=status --type=merge -p '%s' && "+
				"(kubectl taint node %s node.cloudprovider.kubernetes.io/uninitialized- 2>/dev/null || true)",
			node.name, patch, node.name))
		require.NoError(t, err, "publishing addresses %v for %s: %s", addresses, node.name, out)
	}
}

// waitForMgmtNodePrimaryAddress waits until Rancher's management Node for nodeName reports address
// as its first InternalIP, i.e. until the address Rancher would hand out for the node is address.
func waitForMgmtNodePrimaryAddress(t *testing.T, cs *clients.Clients, clusterName, nodeName, address string) {
	t.Helper()

	var last []corev1.NodeAddress
	err := utilwait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 5*time.Minute, true, func(context.Context) (bool, error) {
		nodes, err := cs.Mgmt.Node().List(clusterName, metav1.ListOptions{})
		if err != nil {
			return false, nil
		}
		for _, node := range nodes.Items {
			if node.Status.NodeName != nodeName {
				continue
			}
			last = node.Status.InternalNodeStatus.Addresses
			for _, addr := range last {
				if addr.Type == corev1.NodeInternalIP {
					return addr.Address == address, nil
				}
			}
		}
		return false, nil
	})
	require.NoError(t, err, "Rancher's management Node for %s never reported %s as its primary address (last %v)", nodeName, address, last)
}

// waitForRestoreFinished waits for op to reach Succeeded or Failed, and returns it.
func waitForRestoreFinished(t *testing.T, cs *clients.Clients, op *opv1alpha1.ETCDSnapshotRestore) *opv1alpha1.ETCDSnapshotRestore {
	t.Helper()

	err := wait.ObjectWithTimeout(cs.Ctx, 20*time.Minute, cs.Operation.ETCDSnapshotRestore().Watch, op, func(obj runtime.Object) (bool, error) {
		op = obj.(*opv1alpha1.ETCDSnapshotRestore)
		return op.Status.Phase == opv1alpha1.OperationPhaseSucceeded || op.Status.Phase == opv1alpha1.OperationPhaseFailed, nil
	})
	require.NoError(t, err, "restore %s/%s never finished (phase %q, step %q)", op.Namespace, op.Name, op.Status.Phase, op.Status.Step)
	return op
}
