package imported

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/managementuser/snapshotbackpopulate"
	"github.com/rancher/rancher/pkg/plan"
	"github.com/rancher/rancher/pkg/restoremode"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/objectstore"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

// previousK8sVersion is the version these tests provision on before upgrading. Unlike the v2prov
// restore-mode tests — which use synthetic versions because they only assert the spec field was
// replaced — an imported cluster's upgrade is carried out for real by the downstream
// system-upgrade-controller, so both versions have to be installable.
//
// It must be one minor behind SOME_K8S_VERSION and use the same distro.
var previousK8sVersion = os.Getenv("SOME_K8S_VERSION_PREV")

// requireTwoVersions skips when the test environment has not supplied a second, older version. The
// snapshot-then-upgrade-then-restore scenario is meaningless without one, and silently collapsing to
// a single version would make the test pass while proving nothing.
func requireTwoVersions(t *testing.T) {
	t.Helper()

	if previousK8sVersion == "" {
		t.Skip("SOME_K8S_VERSION_PREV is not set; needed to provision one minor behind SOME_K8S_VERSION")
	}
	if previousK8sVersion == defaults.SomeK8sVersion {
		t.Fatalf("SOME_K8S_VERSION_PREV (%s) must differ from SOME_K8S_VERSION (%s)", previousK8sVersion, defaults.SomeK8sVersion)
	}
	if capr.GetRuntime(previousK8sVersion) != capr.GetRuntime(defaults.SomeK8sVersion) {
		t.Fatalf("SOME_K8S_VERSION_PREV (%s) and SOME_K8S_VERSION (%s) must use the same distro", previousK8sVersion, defaults.SomeK8sVersion)
	}
}

// desiredVersion reads the version the mgmt cluster is configured for, from whichever distro config
// matches the driver — the same choice k3sbasedupgrade makes when deciding what to roll out.
func desiredVersion(c *mgmtv3.Cluster) string {
	if c.Status.Driver == mgmtv3.ClusterDriverK3s {
		if c.Spec.K3sConfig == nil {
			return ""
		}
		return c.Spec.K3sConfig.Version
	}
	if c.Spec.Rke2Config == nil {
		return ""
	}
	return c.Spec.Rke2Config.Version
}

// setDesiredVersion writes the desired version onto the mgmt cluster, which is what drives the
// downstream upgrade. Retried on conflict because the cluster object is written by several
// controllers concurrently.
func setDesiredVersion(t *testing.T, clients *clients.Clients, name, version string) {
	t.Helper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		c, err := clients.Mgmt.Cluster().Get(name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if c.Status.Driver == mgmtv3.ClusterDriverK3s {
			require.NotNil(t, c.Spec.K3sConfig, "k3s cluster %s has no k3sConfig to upgrade", name)
			c.Spec.K3sConfig.Version = version
		} else {
			require.NotNil(t, c.Spec.Rke2Config, "rke2 cluster %s has no rke2Config to upgrade", name)
			c.Spec.Rke2Config.Version = version
		}
		_, err = clients.Mgmt.Cluster().Update(c)
		return err
	})
	require.NoError(t, err, "setting desired version to %s", version)
}

// waitForReportedVersion waits until the cluster reports version and is not upgrading, i.e. the
// nodes have actually converged on it rather than the desired field merely having been written.
func waitForReportedVersion(t *testing.T, clients *clients.Clients, c *mgmtv3.Cluster, version string) {
	t.Helper()

	logrus.Infof("waiting for cluster %s to report version %s", c.Name, version)

	err := wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, c, func(obj runtime.Object) (bool, error) {
		current := obj.(*mgmtv3.Cluster)
		if current.Status.Version == nil {
			return false, nil
		}
		// The cluster must not be mid-rollout either: the reported version flips to the new value as
		// soon as the first node comes back, well before the rest have followed. The upgrader marks a
		// rollout by setting Upgraded to Unknown; a freshly imported cluster that has never been
		// upgraded has no Upgraded condition at all, which counts as not upgrading.
		upgrading := mgmtv3.ClusterConditionUpgraded.IsUnknown(current) || mgmtv3.ClusterConditionUpgraded.IsFalse(current)
		return current.Status.Version.GitVersion == version && !upgrading, nil
	})
	handleError(t, clients, c.Name, err)
}

// waitForImportedNodesOnVersion waits until every node of the cluster runs version, is Ready, and is
// schedulable — the node-level picture of a finished rollout — as seen from both sides:
//
//   - downstream, through the API server, where the kubelet reports its own version; and
//   - upstream, on the management.cattle.io Node objects Rancher syncs from them, which is what the
//     k3s-based upgrader's nodesNeedUpgrade reads to decide whether a rollout is still needed.
//
// Both have to account for every node the fixture created, so a node that has dropped out of the
// cluster — or never made it back after a restart — is a failure rather than silently not checked.
//
// The mgmt cluster's own view is not enough on its own. Its reported version comes from the API
// server and its Upgraded condition from the upgrader's reading of the system-upgrade-controller
// plans, and neither says anything direct about the kubelets. The upgrader cordons each node while it
// swaps the binary, so a node left unschedulable means its plan never finished.
func waitForImportedNodesOnVersion(t *testing.T, clients *clients.Clients, fx *importedClusterFixture, version string) {
	t.Helper()

	want := len(fx.pods)
	logrus.Infof("waiting for all %d nodes of cluster %s to be Ready and schedulable on %s", want, fx.mgmtCluster.Name, version)

	var downstreamReason, upstreamReason string
	err := utilwait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 15*time.Minute, true, func(context.Context) (bool, error) {
		downstreamOK := downstreamNodesOnVersion(t, fx, want, version, &downstreamReason)
		upstreamOK := mgmtNodesOnVersion(clients, fx.mgmtCluster.Name, want, version, &upstreamReason)
		return downstreamOK && upstreamOK, nil
	})
	if err != nil {
		t.Fatalf("not all %d nodes of cluster %s are Ready and schedulable on %s:\n  downstream: %s\n  management: %s",
			want, fx.mgmtCluster.Name, version, downstreamReason, upstreamReason)
	}
}

// downstreamNodesOnVersion reports whether the downstream API server lists exactly want nodes, all
// Ready, schedulable and with kubelet version. reason is set to what was last read.
func downstreamNodesOnVersion(t *testing.T, fx *importedClusterFixture, want int, version string, reason *string) bool {
	t.Helper()

	out, err := fx.execKubectl(t, "kubectl get nodes -o jsonpath='{range .items[*]}"+
		"{.metadata.name}={.status.nodeInfo.kubeletVersion}={range .status.conditions[?(@.type==\"Ready\")]}{.status}{end}={.spec.unschedulable}{\"\\n\"}{end}'")
	if err != nil {
		*reason = fmt.Sprintf("%v: %s", err, out)
		return false
	}
	*reason = fmt.Sprintf("(name=version=ready=unschedulable) %q", out)

	var lines []string
	if trimmed := strings.TrimSpace(out); trimmed != "" {
		lines = strings.Split(trimmed, "\n")
	}
	if len(lines) != want {
		*reason = fmt.Sprintf("%d nodes, want %d: %s", len(lines), want, *reason)
		return false
	}
	for _, line := range lines {
		fields := strings.Split(strings.TrimSpace(line), "=")
		if len(fields) != 4 || fields[1] != version || fields[2] != "True" || fields[3] == "true" {
			return false
		}
	}
	return true
}

// mgmtNodesOnVersion reports whether the cluster's management.cattle.io Nodes number exactly want,
// and each records kubelet version, Ready, and schedulable. These are synced from the downstream
// cluster by the node syncer, so they lag it; the caller polls. reason is set to what was last read.
func mgmtNodesOnVersion(clients *clients.Clients, clusterName string, want int, version string, reason *string) bool {
	nodes, err := clients.Mgmt.Node().List(clusterName, metav1.ListOptions{})
	if err != nil {
		*reason = fmt.Sprintf("list: %v", err)
		return false
	}

	ok := len(nodes.Items) == want
	var seen []string
	for _, node := range nodes.Items {
		kubelet := node.Status.InternalNodeStatus.NodeInfo.KubeletVersion
		ready := mgmtNodeReady(node.Status.InternalNodeStatus.Conditions)
		unschedulable := node.Spec.InternalNodeSpec.Unschedulable
		seen = append(seen, fmt.Sprintf("%s=%s=%t=%t", node.Status.NodeName, kubelet, ready, unschedulable))
		if kubelet != version || !ready || unschedulable {
			ok = false
		}
	}
	slices.Sort(seen)
	*reason = fmt.Sprintf("%d nodes, want %d (name=version=ready=unschedulable) %v", len(nodes.Items), want, seen)
	return ok
}

func mgmtNodeReady(conditions []corev1.NodeCondition) bool {
	for _, c := range conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// waitForDesiredVersion waits until the mgmt cluster's desired version is version. A restore writes
// that field, and the write is what the test is really asserting; the subsequent downstream rollout
// is the upgrade controller's job.
func waitForDesiredVersion(t *testing.T, clients *clients.Clients, c *mgmtv3.Cluster, version string) {
	t.Helper()

	logrus.Infof("waiting for cluster %s desired version to be %s", c.Name, version)

	err := wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, c, func(obj runtime.Object) (bool, error) {
		return desiredVersion(obj.(*mgmtv3.Cluster)) == version, nil
	})
	handleError(t, clients, c.Name, err)
}

// assertSnapshotOffersMode fails unless the snapshot advertises mode. snapshotbackpopulate computes
// that annotation by resolving each mode's selector against the resources the snapshot captured, so
// this is the same gate the restore's preflight step applies — asserting it here turns "the mode was
// never on offer" into a clear failure instead of a canceled operation later.
func assertSnapshotOffersMode(t *testing.T, snapshot *rkev1.ETCDSnapshot, mode string) {
	t.Helper()

	offered := restoremode.AvailableModes(snapshot.Annotations[capr.RestoreModeOptionsAnnotation])
	assert.Contains(t, offered, mode,
		"snapshot %s/%s offers restore modes %v, which does not include %q",
		snapshot.Namespace, snapshot.Name, offered, mode)
}

// The six Test_Imported_Operation_SetD_ETCDSnapshotRestoreMode* tests below are the restore-mode
// matrix for an imported cluster: every mode against both places a snapshot can live.
//
//	                   | local | s3
//	none               |   ✓   | ✓
//	kubernetesVersion  |   ✓   | ✓
//	all                |   ✓   | ✓
//
// The modes only differ in how much of the configuration captured in the snapshot comes back with
// the etcd rollback, so all six cases run the same scenario — provision on the older version,
// capture a snapshot, upgrade and change the cluster agent's customization, then restore — and
// differ only in what they expect of the cluster afterwards. That symmetry is the point: `none`
// leaving the post-upgrade configuration alone is as much an assertion as `all` reverting it.
//
// Storage is a real dimension rather than an implementation detail. Snapshot extra metadata — what
// the restore modes are resolved from — is stamped onto an ETCDSnapshotFile by the distro at the
// moment it registers the resource, and for a local file that metadata is not persisted anywhere,
// while for S3 it is uploaded to the bucket alongside the snapshot and read back on listing. So the
// two paths reach the same feature through genuinely different plumbing. An imported cluster upgrades
// in place, keeping its nodes and their disks, so a local snapshot survives the upgrade here — unlike
// the CAPRKE2 case, where the upgrade replaces the machine and S3 is the only option.

// Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeNoneLocal verifies that a `none` restore from
// a machine-local snapshot rolls etcd back and leaves the cluster's configuration alone.
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeNoneLocal(t *testing.T) {
	runImportedRestoreModeTest(t, "rm-none-local", rkev1.RestoreRKEConfigNone, snapshotbackpopulate.Local)
}

// Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeNoneS3 is the S3 counterpart.
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeNoneS3(t *testing.T) {
	runImportedRestoreModeTest(t, "rm-none-s3", rkev1.RestoreRKEConfigNone, snapshotbackpopulate.S3)
}

// Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeKubernetesVersionLocal verifies the downgrade
// path from a machine-local snapshot: the cluster ends up configured for, and running, the
// snapshot's version, while everything else keeps its post-snapshot value.
//
// This is the mode's whole purpose. The restore has to reinstall the distro at the snapshot's
// version before `--cluster-reset`, because a newer server cannot reset onto etcd data written by an
// older one.
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeKubernetesVersionLocal(t *testing.T) {
	runImportedRestoreModeTest(t, "rm-kv-local", rkev1.RestoreRKEConfigKubernetesVersion, snapshotbackpopulate.Local)
}

// Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeKubernetesVersionS3 is the S3 counterpart.
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeKubernetesVersionS3(t *testing.T) {
	runImportedRestoreModeTest(t, "rm-kv-s3", rkev1.RestoreRKEConfigKubernetesVersion, snapshotbackpopulate.S3)
}

// Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeAllLocal verifies that `all` restores the rest
// of the captured configuration too, not just the Kubernetes version — here, the cluster agent's
// deployment customization, which the other two modes must leave at its post-snapshot value.
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeAllLocal(t *testing.T) {
	runImportedRestoreModeTest(t, "rm-all-local", rkev1.RestoreRKEConfigAll, snapshotbackpopulate.Local)
}

// Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeAllS3 is the S3 counterpart.
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeAllS3(t *testing.T) {
	runImportedRestoreModeTest(t, "rm-all-s3", rkev1.RestoreRKEConfigAll, snapshotbackpopulate.S3)
}

// Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeKubernetesVersionLocalHA is the
// kubernetesVersion/local case on an HA topology — three etcd+control-plane servers and one agent —
// brought up the way clusters usually are outside of tests: the bootstrap server generates its own
// token, which is then read off its disk and handed to the joining nodes, rather than every node
// being pre-seeded with the same shared secret.
//
// This targets a failure seen in the field, where after such a restore the distro version is not
// put back on every node, and any node that *is* on the restored version stays unavailable — the
// signature of a split brain, with the restored server and the rest of the cluster no longer
// agreeing on etcd membership. So beyond the usual matrix assertions, every server is checked
// individually: each must be a voting member of one and the same etcd cluster, and each must see
// the same set of nodes, all Ready on the restored version.
//
// The restore deliberately runs from a snapshot taken on a joining server rather than the init node.
// For a local snapshot the controller resets etcd on the node that holds the file, so this makes a
// non-init server the restore leader: the init node — whose config has no `server:` and which
// bootstrapped the cluster — then has to rejoin through the temporary drop-in like everyone else.
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeKubernetesVersionLocalHA(t *testing.T) {
	runImportedRestoreModeTestOn(t, "rm-kv-local-ha", rkev1.RestoreRKEConfigKubernetesVersion, snapshotbackpopulate.Local,
		[]cluster.ImportedNodePool{
			{ETCD: true, ControlPlane: true, Quantity: 3},
			{Worker: true, Quantity: 1},
		},
		cluster.ImportedClusterOptions{ExtractServerToken: true},
		// cluster.NewImportedClusterPods names joining nodes imported-node-<n>; node 1 is the first
		// server to join the init node.
		"imported-node-1")
}

// Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeKubernetesVersionLocalHAIPv6 is the HA case on
// a dual-stack downstream cluster whose nodes reach each other over IPv6: IPv6 is the primary family,
// so it is what nodes join over and what etcd advertises its peers on, with IPv4 alongside it.
//
// A restore has to keep the cluster on that network. It rebuilds etcd membership from scratch — one
// node resets, every other node is pointed at it through a temporary `server:` drop-in — so the
// address it chooses for the leader decides which family the cluster comes back on. Pointing the
// others at the leader's IPv4 address would rejoin them over the secondary family, and etcd would
// re-advertise its peers there. So on top of the HA assertions, the test checks that every drop-in
// the restore writes names the leader's IPv6 address, and that afterwards the nodes and etcd members
// are all still on IPv6.
//
// Needs a dual-stack host cluster, and skips without one: see dev-scripts/k3d-dual-stack.
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeKubernetesVersionLocalHAIPv6(t *testing.T) {
	runImportedRestoreModeTestOn(t, "rm-kv-local-ha-v6", rkev1.RestoreRKEConfigKubernetesVersion, snapshotbackpopulate.Local,
		[]cluster.ImportedNodePool{
			{ETCD: true, ControlPlane: true, Quantity: 3},
			{Worker: true, Quantity: 1},
		},
		cluster.ImportedClusterOptions{
			ExtractServerToken: true,
			Network: &cluster.ImportedNetwork{
				IPFamilies: []corev1.IPFamily{corev1.IPv6Protocol, corev1.IPv4Protocol},
			},
		},
		"imported-node-1")
}

// defaultRestoreModePools is the split-role topology the matrix runs on: one node per role.
var defaultRestoreModePools = []cluster.ImportedNodePool{
	{ETCD: true, Quantity: 1},
	{ControlPlane: true, Quantity: 1},
	{Worker: true, Quantity: 1},
}

// The two configuration values every case in the matrix moves, so that each mode can be pinned by
// what it does and does not put back.
const (
	capturedToleration = "captured-before-snapshot"
	changedToleration  = "changed-after-snapshot"
)

// runImportedRestoreModeTest runs one cell of the matrix: snapshot on the older version, upgrade and
// change the agent customization, restore selecting mode, then assert what that mode is supposed to
// have restored — plus, in every case, that etcd itself rolled back, that no node was replaced, and
// that the cluster came out healthy.
func runImportedRestoreModeTest(t *testing.T, displayName, mode string, storage snapshotbackpopulate.Storage) {
	t.Helper()

	runImportedRestoreModeTestOn(t, displayName, mode, storage, defaultRestoreModePools, cluster.ImportedClusterOptions{}, "")
}

// runImportedRestoreModeTestOn is runImportedRestoreModeTest on an explicit topology. When the
// topology has more than one etcd node, it additionally asserts that those nodes agree with each
// other both before the snapshot and after the restore — see assertImportedServersConverged.
//
// restoreNode, when set, is the downstream node name to restore from: the restore uses that node's
// local snapshot, which makes it the node etcd is reset on, and the test asserts afterwards that the
// reset really ran there. Empty restores from whichever snapshot is newest. It only applies to local
// storage — an S3 snapshot is not tied to a node.
//
// In every case the test also checks that each server URL the restore hands a node is one the node can
// actually connect to with certificate verification — see assertRestoreServerArgsTrusted.
//
// When opts.Network is set, the test skips unless the host cluster can give pods an address in every
// family it names, and asserts that the cluster runs on its primary family before the snapshot, that
// the restore points rejoining nodes at the leader over that family, and that the cluster is still on
// it afterwards — see assertImportedClusterOnFamily and watchRestoreServerArgs.
func runImportedRestoreModeTestOn(t *testing.T, displayName, mode string, storage snapshotbackpopulate.Storage, pools []cluster.ImportedNodePool, opts cluster.ImportedClusterOptions, restoreNode string) {
	t.Helper()

	requireTwoVersions(t)
	if restoreNode != "" && storage != snapshotbackpopulate.Local {
		t.Fatalf("restoreNode %q requires local snapshot storage, got %s", restoreNode, storage)
	}

	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	// The family the cluster is expected to run on, or "" to leave the network unchecked.
	var primaryFamily corev1.IPFamily
	if opts.Network != nil {
		requireHostIPFamilies(t, clients, opts.Network.IPFamilies)
		if len(opts.Network.IPFamilies) > 0 {
			primaryFamily = opts.Network.IPFamilies[0]
		}
	}

	fx := setUpImportedRestoreModeCluster(t, clients, displayName, storage, pools, opts)
	servers := importedETCDPods(t, clients, fx)

	// The cluster comes up on the older version, so that is what the snapshot captures.
	waitForReportedVersion(t, clients, fx.mgmtCluster, previousK8sVersion)
	waitForImportedNodesOnVersion(t, clients, fx, previousK8sVersion)
	if len(servers) > 1 {
		// Establish that the servers agree before anything happens, so that disagreement after the
		// restore is attributable to the restore.
		assertImportedServersConverged(t, clients, fx, servers, previousK8sVersion)
	}
	if primaryFamily != "" {
		// Without this the post-restore network assertion could pass on a cluster that was never on
		// the family to begin with.
		assertImportedClusterOnFamily(t, clients, fx, primaryFamily)
	}
	waitForImportedSnapshotMetadataReady(t, fx)

	// Tolerations are a convenient probe for the agent customization: a plain list on the mgmt
	// cluster spec that Rancher applies to the cattle-cluster-agent deployment, so a restore either
	// brings the whole value back or it does not.
	setAgentToleration(t, clients, fx.mgmtCluster.Name, capturedToleration)
	waitForAgentToleration(t, clients, fx.mgmtCluster, capturedToleration)

	proof := newConfigMapProof()
	proof.create(t, fx)

	before := time.Now()
	RunETCDSnapshotSaveOperationTest(t, clients, fx.mgmtCluster.Name, fx.clusterRef)
	var snapshot *rkev1.ETCDSnapshot
	if restoreNode != "" {
		snapshot = waitForBackpopulatedLocalSnapshotOnNode(t, clients, fx.mgmtCluster.Name, restoreNode, before)
	} else {
		snapshot = waitForBackpopulatedSnapshotForStorage(t, clients, fx.mgmtCluster.Name, fx.mgmtCluster.Name, storage, before)
	}
	assertSnapshotOffersMode(t, snapshot, mode)
	if mode != rkev1.RestoreRKEConfigNone {
		// `none` declares no selector, so there is nothing to resolve; for the other two, check the
		// snapshot really did capture the pre-upgrade version.
		assertImportedSnapshotCapturedVersion(t, clients, fx.mgmtCluster.Name, snapshot.Name, mode)
	}

	nodesBefore := importedNodeIdentities(t, fx)

	// Move both things a mode could restore.
	logrus.Infof("upgrading cluster %s from %s to %s and changing its agent customization",
		fx.mgmtCluster.Name, previousK8sVersion, defaults.SomeK8sVersion)
	setDesiredVersion(t, clients, fx.mgmtCluster.Name, defaults.SomeK8sVersion)
	waitForReportedVersion(t, clients, fx.mgmtCluster, defaults.SomeK8sVersion)
	// Every node has to have finished upgrading before the restore starts. Otherwise the restore
	// begins on a cluster that is part-way through a rollout, which is a different scenario from the
	// one this test is about, and fails for different reasons.
	waitForImportedNodesOnVersion(t, clients, fx, defaults.SomeK8sVersion)

	// An imported cluster upgrades in place — the system-upgrade-controller drains a node, swaps the
	// distro binary and brings it back — so unlike the CAPRKE2 case the nodes are expected to be the
	// same objects afterwards. Asserting it here is what makes the identical assertion after the
	// restore meaningful: both halves of the test measure the same thing.
	assertImportedNodesUnchanged(t, fx, nodesBefore, "the upgrade should have been carried out in place")

	setAgentToleration(t, clients, fx.mgmtCluster.Name, changedToleration)
	waitForAgentToleration(t, clients, fx.mgmtCluster, changedToleration)

	// Delete the ConfigMap so the etcd rollback is observable independently of any configuration
	// change — which is the only thing a `none` restore leaves behind as evidence.
	proof.delete(t, fx)

	// Record what the restore tells each node to rejoin through; stopping is idempotent, so the defer
	// only matters if the restore fails before the explicit stop below.
	stopWatchingServerArgs := watchRestoreServerArgs(t, clients, fx.mgmtCluster.Name)
	defer stopWatchingServerArgs()

	op := RunETCDSnapshotRestoreOperationTest(t, clients, fx.mgmtCluster.Name, snapshot.Name, fx.clusterRef,
		WithRestoreMode(mode),
		WithRestoreTTL(-1))
	require.NotNil(t, op)

	restoreServerArgs := stopWatchingServerArgs()
	assertRestoreServerArgsTrusted(t, clients, fx, restoreServerArgs)
	if primaryFamily != "" {
		assertRestoreServerArgsOnFamily(t, clients, fx, restoreServerArgs, primaryFamily, restoreNode)
	}

	proof.assertRestored(t, fx)
	if restoreNode != "" {
		assertETCDResetRanOnlyOn(t, clients, fx, restoreNode)
	}

	// What each mode is expected to have put back. Waiting is only appropriate where a change is
	// expected: for the values a mode must *not* restore there is nothing to wait for, and they are
	// asserted below once the cluster has settled.
	expectedVersion := previousK8sVersion
	expectedToleration := changedToleration

	switch mode {
	case rkev1.RestoreRKEConfigNone:
		// No configuration is restored at all, so the cluster stays on the version it was upgraded
		// to, running etcd data from before the upgrade.
		expectedVersion = defaults.SomeK8sVersion
	case rkev1.RestoreRKEConfigKubernetesVersion:
		waitForDesiredVersion(t, clients, fx.mgmtCluster, previousK8sVersion)
		waitForReportedVersion(t, clients, fx.mgmtCluster, previousK8sVersion)
	case rkev1.RestoreRKEConfigAll:
		waitForDesiredVersion(t, clients, fx.mgmtCluster, previousK8sVersion)
		waitForAgentToleration(t, clients, fx.mgmtCluster, capturedToleration)
		waitForReportedVersion(t, clients, fx.mgmtCluster, previousK8sVersion)
		expectedToleration = capturedToleration
	default:
		t.Fatalf("unhandled restore mode %q", mode)
	}

	assertImportedNodesUnchanged(t, fx, nodesBefore, "the restore must not replace any node")
	assertImportedClusterHealthy(t, clients, fx, expectedVersion)
	if len(servers) > 1 {
		assertImportedServersConverged(t, clients, fx, servers, expectedVersion)
	}
	if primaryFamily != "" {
		assertImportedClusterOnFamily(t, clients, fx, primaryFamily)
	}

	// The reported version is the last thing to settle, so wait for it rather than sampling it, then
	// assert the rest off the object that wait last read.
	latest := waitForReportedGitVersion(t, clients, fx.mgmtCluster.Name, expectedVersion)
	assert.Equal(t, expectedVersion, desiredVersion(latest),
		"restore mode %q should leave the cluster configured for %s", mode, expectedVersion)
	assert.Equal(t, expectedToleration, agentToleration(latest),
		"restore mode %q should leave the cluster agent customization at %q", mode, expectedToleration)
}

// waitForReportedGitVersion waits up to five minutes for the cluster to report version in
// status.version.gitVersion, and returns the last cluster it read.
//
// Unlike everything else these tests assert, the reported version is not something Rancher was told —
// it is something Rancher observed, relayed from the downstream API server after the cluster agent
// reconnects. It therefore lags every other signal: the desired version flips the moment the restore
// writes it, and the nodes report their kubelet version sooner, while status.version can still be the
// pre-restore value (or briefly absent) for minutes. Sampling it immediately is a race, and five
// minutes is generous for an observation that normally lands within a sync or two.
func waitForReportedGitVersion(t *testing.T, clients *clients.Clients, name, version string) *mgmtv3.Cluster {
	t.Helper()

	logrus.Infof("waiting for cluster %s to report version %s", name, version)

	var (
		latest     *mgmtv3.Cluster
		lastReason string
	)
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 10*time.Second, 5*time.Minute, true, func(context.Context) (bool, error) {
		cluster, err := clients.Mgmt.Cluster().Get(name, metav1.GetOptions{})
		if err != nil {
			lastReason = fmt.Sprintf("get: %v", err)
			return false, nil
		}
		latest = cluster

		if cluster.Status.Version == nil {
			lastReason = "status.version is not set yet"
			return false, nil
		}
		if cluster.Status.Version.GitVersion != version {
			lastReason = fmt.Sprintf("reports %s", cluster.Status.Version.GitVersion)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for cluster %s to report version %s: %s", name, version, lastReason)
	}

	return latest
}

// setUpImportedRestoreModeCluster brings up an imported cluster with the given topology on the older
// version, configured to keep etcd snapshots wherever the case under test needs them. opts is passed
// through to the node pods, with the object store settings merged in for S3.
//
// For snapshotStorageLocal the cluster is left alone and snapshots stay on the node's disk, which is
// durable enough here because an imported cluster upgrades in place and keeps its nodes. For
// snapshotStorageS3 an object store is stood up and the distro is pointed at it; the snapshot is then
// written locally *and* uploaded, and the test restores from the uploaded copy.
func setUpImportedRestoreModeCluster(t *testing.T, cs *clients.Clients, displayName string, storage snapshotbackpopulate.Storage, pools []cluster.ImportedNodePool, opts cluster.ImportedClusterOptions) *importedClusterFixture {
	t.Helper()

	if storage == snapshotbackpopulate.Local {
		return setUpImportedClusterAtVersion(t, cs, displayName, pools, previousK8sVersion, opts)
	}

	// The nodes are pods in this cluster, so a ClusterIP endpoint is reachable — unlike the CAPRKE2
	// case, which needs objectstore's external flavour. Living in `default` mirrors the shared
	// registry cache.
	osInfo, err := objectstore.GetObjectStore(cs, "default", "imported-store0", "s3snapshots")
	if err != nil {
		t.Fatalf("setting up object store: %v", err)
	}

	// One folder per run, so a rerun never has to reason about a previous run's snapshots — the
	// distro lists the whole folder and back-populates everything it finds there.
	folder := displayName + "-" + strings.ToLower(name.Hex(time.Now().String(), 8))
	t.Logf("etcd snapshots go to s3://%s/%s at %s", osInfo.Bucket, folder, osInfo.Endpoint)

	s3 := importedS3Options(previousK8sVersion, osInfo, folder)
	opts.ServerConfig = mergeStrings(opts.ServerConfig, s3.ServerConfig)
	opts.ServerFiles = mergeStrings(opts.ServerFiles, s3.ServerFiles)

	return setUpImportedClusterAtVersion(t, cs, displayName, pools, previousK8sVersion, opts)
}

// mergeStrings returns a new map holding base overlaid with extra.
func mergeStrings(base, extra map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(extra))
	maps.Copy(merged, base)
	maps.Copy(merged, extra)
	return merged
}

// importedS3Options renders the object store as the etcd-s3* config keys RKE2/K3s read, plus the CA
// bundle one of them points at. Only server nodes get them: the keys are server-only and an agent
// would reject them.
func importedS3Options(k8sVersion string, osInfo objectstore.Info, folder string) cluster.ImportedClusterOptions {
	caPath := fmt.Sprintf("/etc/rancher/%s/etcd-s3-ca.pem", capr.GetRuntime(k8sVersion))

	return cluster.ImportedClusterOptions{
		ServerConfig: map[string]string{
			"etcd-s3":             "true",
			"etcd-s3-endpoint":    osInfo.Endpoint,
			"etcd-s3-bucket":      osInfo.Bucket,
			"etcd-s3-folder":      folder,
			"etcd-s3-access-key":  osInfo.AccessKey,
			"etcd-s3-secret-key":  osInfo.SecretKey,
			"etcd-s3-endpoint-ca": caPath,
		},
		ServerFiles: map[string]string{
			caPath: decodeCert(osInfo.Cert),
		},
	}
}

// decodeCert turns objectstore.Info's base64 PEM bundle back into PEM.
func decodeCert(encoded string) string {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Info.Cert is produced by objectstore itself; a decode failure is a programming error, and
		// returning the raw value keeps this usable in a config file for diagnosis.
		return encoded
	}
	return string(decoded)
}

// waitForImportedSnapshotMetadataReady blocks until a snapshot taken now would actually capture the
// extra metadata that the restore modes are resolved from.
//
// RKE2/K3s will happily take a snapshot and record no metadata at all, silently: a snapshot taken
// before snapshotextrametadata has written the ConfigMap comes out bare, which leaves the upstream
// ETCDSnapshot advertising only the "none" restore mode and cancels any mode-bearing restore during
// preflight. Waiting for the cluster to be Ready is not enough — the publisher runs per downstream
// cluster and is not ordered against anything the test does.
func waitForImportedSnapshotMetadataReady(t *testing.T, fx *importedClusterFixture) {
	t.Helper()

	runtime := capr.GetRuntime(previousK8sVersion)
	configMap := runtime + "-etcd-snapshot-extra-metadata"

	t.Logf("waiting for %s/%s before taking a snapshot", metav1.NamespaceSystem, configMap)

	var lastOut string
	err := utilwait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
		out, err := fx.execKubectl(t, fmt.Sprintf(
			"kubectl get configmap -n %s %s -o jsonpath='{.data.%s}{\",\"}{.data.%s}'",
			metav1.NamespaceSystem, configMap, rkev1.SnapshotMetadataResourcesKey, rkev1.SnapshotMetadataRestoreModesKey))
		if err != nil {
			lastOut = fmt.Sprintf("%v: %s", err, out)
			return false, nil
		}
		lastOut = out
		resources, restoreModes, _ := strings.Cut(strings.TrimSpace(out), ",")
		return resources != "" && restoreModes != "", nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for %s to carry both metadata payloads (last read %q)", configMap, lastOut)
	}
}

// assertImportedSnapshotCapturedVersion fails unless the snapshot's captured resources still describe
// the pre-upgrade Kubernetes version.
//
// The snapshot is read back out of the object store by whichever node lists the bucket, and the
// distro stamps ETCDSnapshotFile.Spec.Metadata from what it finds there. If it were instead to
// re-stamp from the live cluster when re-registering an already-uploaded snapshot, the CR would
// describe the *post-upgrade* cluster, kubernetesVersion would resolve to the version we just
// upgraded to, and the restore would quietly become a no-op that still passes every other assertion.
func assertImportedSnapshotCapturedVersion(t *testing.T, clients *clients.Clients, clusterName, snapshotName, mode string) {
	t.Helper()

	snapshot, err := clients.RKE.ETCDSnapshot().Get(clusterName, snapshotName, metav1.GetOptions{})
	require.NoError(t, err)

	metadata, err := snapshotutil.SnapshotMetadata(snapshot)
	require.NoError(t, err, "snapshot %s/%s carries no decodable metadata", clusterName, snapshotName)

	modes, err := restoremode.Modes(metadata)
	require.NoError(t, err)
	selector, ok := modes[mode]
	require.True(t, ok, "snapshot %s/%s does not declare restore mode %q (declares %v)",
		clusterName, snapshotName, mode, modes)

	resources, err := restoremode.Resources(metadata)
	require.NoError(t, err)

	matches, err := restoremode.Resolve(selector, resources)
	require.NoError(t, err)
	require.NotEmpty(t, matches, "restore mode %q resolves to nothing in snapshot %s/%s",
		mode, clusterName, snapshotName)

	for _, m := range matches {
		if len(m.Path) > 0 && m.Path[len(m.Path)-1] == "kubernetesVersion" {
			assert.Equal(t, previousK8sVersion, m.Value,
				"snapshot %s/%s captured kubernetesVersion %v, want the pre-upgrade %s",
				clusterName, snapshotName, m.Value, previousK8sVersion)
			return
		}
	}
	t.Fatalf("restore mode %q in snapshot %s/%s selects no kubernetesVersion field (selected %v)",
		mode, clusterName, snapshotName, matches)
}

// importedNodeIdentities returns the downstream nodes as name -> UID. The UID is what makes this an
// identity rather than a name check: a replaced node can come back under the same name, and the test
// wants to know whether these are the same objects.
func importedNodeIdentities(t *testing.T, fx *importedClusterFixture) map[string]string {
	t.Helper()

	out, err := fx.execKubectl(t, "kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}={.metadata.uid}{\"\\n\"}{end}'")
	if err != nil {
		t.Fatalf("listing downstream nodes failed: %v\noutput: %s", err, out)
	}

	nodes := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		nodeName, uid, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || nodeName == "" {
			continue
		}
		nodes[nodeName] = uid
	}
	if len(nodes) == 0 {
		t.Fatalf("no downstream nodes found (kubectl output %q)", out)
	}
	return nodes
}

// assertImportedNodesUnchanged fails unless the downstream cluster still consists of exactly the
// nodes in want, same UIDs. It is polled rather than sampled once: a restore rolls etcd back, and the
// node objects take a moment to settle afterwards as the kubelet re-registers and the operation's
// PostRestoreNodeCleanup step removes anything that is no longer part of the cluster.
func assertImportedNodesUnchanged(t *testing.T, fx *importedClusterFixture, want map[string]string, why string) {
	t.Helper()

	var got map[string]string
	err := utilwait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
		got = importedNodeIdentities(t, fx)
		return len(got) == len(want) && mapsEqual(got, want), nil
	})
	if err != nil {
		t.Fatalf("%s: downstream nodes are %v, want %v", why, got, want)
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// assertImportedClusterHealthy is the "the cluster came out of this intact" check: the mgmt v3
// Cluster is Ready, and every node is Ready, schedulable and running the expected version, both
// downstream and on the management Node objects (see waitForImportedNodesOnVersion). Ready on the
// mgmt cluster covers the parts of the downstream cluster Rancher itself watches — the agent is
// connected and the cluster's components are responding.
func assertImportedClusterHealthy(t *testing.T, clients *clients.Clients, fx *importedClusterFixture, version string) {
	t.Helper()

	logrus.Infof("waiting for cluster %s to be Ready after the restore", fx.mgmtCluster.Name)

	err := wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, fx.mgmtCluster, func(obj runtime.Object) (bool, error) {
		return mgmtv3.ClusterConditionReady.IsTrue(obj.(*mgmtv3.Cluster)), nil
	})
	handleError(t, clients, fx.mgmtCluster.Name, err)

	waitForImportedNodesOnVersion(t, clients, fx, version)
}

// agentToleration returns the marker key of the cluster agent's first appended toleration, or "".
func agentToleration(c *mgmtv3.Cluster) string {
	custom := c.Spec.ClusterAgentDeploymentCustomization
	if custom == nil || len(custom.AppendTolerations) == 0 {
		return ""
	}
	return custom.AppendTolerations[0].Key
}

// setAgentToleration stamps a single identifiable toleration onto the cluster agent customization.
func setAgentToleration(t *testing.T, clients *clients.Clients, name, key string) {
	t.Helper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		c, err := clients.Mgmt.Cluster().Get(name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		c.Spec.ClusterAgentDeploymentCustomization = &mgmtv3.AgentDeploymentCustomization{
			AppendTolerations: []corev1.Toleration{
				{
					Key:      key,
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
		}
		_, err = clients.Mgmt.Cluster().Update(c)
		return err
	})
	require.NoError(t, err, "setting cluster agent toleration to %s", key)
}

// waitForAgentToleration waits until the cluster agent customization carries key.
func waitForAgentToleration(t *testing.T, clients *clients.Clients, c *mgmtv3.Cluster, key string) {
	t.Helper()

	logrus.Infof("waiting for cluster %s agent toleration to be %s", c.Name, key)

	err := wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, c, func(obj runtime.Object) (bool, error) {
		return agentToleration(obj.(*mgmtv3.Cluster)) == key, nil
	})
	handleError(t, clients, c.Name, err)
}

// importedNode is one of the fixture's node pods, read back from the API server, together with the
// downstream node it runs.
type importedNode struct {
	pod *corev1.Pod
	// name is the downstream node name.
	name string
	etcd bool
}

// ip returns the node's address in family, or "". The downstream node takes its node-ip from the
// pod's interface, so this is also the address the node registers with.
func (n importedNode) ip(family corev1.IPFamily) string {
	return cluster.PodIPForFamily(n.pod, family)
}

// importedNodes returns every node of the fixture, freshly read so that pod IPs are populated — the
// fixture's own pods are snapshots from creation time, and joining pods are created without waiting
// for an address. Nodes are identified by the labels cluster.NewImportedClusterPods puts on each pod.
func importedNodes(t *testing.T, cs *clients.Clients, fx *importedClusterFixture) []importedNode {
	t.Helper()

	var nodes []importedNode
	for _, p := range fx.pods {
		pod, err := cs.Core.Pod().Get(p.Namespace, p.Name, metav1.GetOptions{})
		require.NoError(t, err, "reading node pod %s/%s", p.Namespace, p.Name)

		name := pod.Labels[cluster.ImportedNodeNameLabel]
		require.NotEmpty(t, name, "node pod %s/%s has no %s label", pod.Namespace, pod.Name, cluster.ImportedNodeNameLabel)
		nodes = append(nodes, importedNode{
			pod:  pod,
			name: name,
			etcd: pod.Labels[cluster.ImportedNodeETCDLabel] == "true",
		})
	}
	return nodes
}

// importedETCDPods returns the fixture's etcd nodes, init node first.
func importedETCDPods(t *testing.T, cs *clients.Clients, fx *importedClusterFixture) []importedNode {
	t.Helper()

	var servers []importedNode
	for _, node := range importedNodes(t, cs, fx) {
		if node.etcd {
			servers = append(servers, node)
		}
	}
	return servers
}

// etcdMemberList is the subset of the supervisor's /db/info response the split-brain check reads.
// The handler answers with the local etcd's member list, JSON-encoded straight from the client's
// protobuf types, so IDs are plain numbers and isLearner is omitted when false.
//
// The supervisor is used rather than etcd's own client port because K3s and RKE2 do not enable
// etcd's gRPC gateway: over TLS, curl negotiates HTTP/2 and lands on the native gRPC server (415,
// wrong content type), and over HTTP/1.1 nothing answers at all.
type etcdMemberList struct {
	Members []etcdMember `json:"members"`
}

type etcdMember struct {
	ID        uint64   `json:"ID"`
	Name      string   `json:"name"`
	PeerURLs  []string `json:"peerURLs"`
	IsLearner bool     `json:"isLearner"`
}

// readETCDMembers returns the member list of pod's local etcd, through its supervisor.
func readETCDMembers(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, pod *corev1.Pod, dataDir string, supervisorPort int) ([]etcdMember, error) {
	t.Helper()

	// /db/info authenticates with the server token's password, i.e. everything after "::server:".
	serverDir := path.Join(dataDir, "server")
	out, err := cluster.ExecOnPod(cs, fx.ns.Name, pod.Name, "sh", "-c", fmt.Sprintf(
		`pw=$(cut -d: -f4- %[1]s/token) && curl -sSf --cacert %[1]s/tls/server-ca.crt -u "server:$pw" https://127.0.0.1:%[2]d/db/info`,
		serverDir, supervisorPort))
	if err != nil {
		return nil, fmt.Errorf("listing etcd members: %w: %s", err, out)
	}

	var list etcdMemberList
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return nil, fmt.Errorf("decoding etcd member list %q: %w", out, err)
	}
	return list.Members, nil
}

// serverView is what a single server reports about the cluster: the etcd members its local etcd
// lists, plus the nodes its own API server returns.
type serverView struct {
	// members are "<name>/<id>", so a member that was removed and re-added under the same name — as
	// happens when a server is reset into a new cluster — does not compare equal to the original.
	members  []string
	learners []string
	nodes    string
}

func (v serverView) String() string {
	return fmt.Sprintf("etcd members %v, learners %v, nodes [%s]",
		v.members, v.learners, strings.ReplaceAll(strings.TrimSpace(v.nodes), "\n", "; "))
}

// readServerView asks one server — its local supervisor and API server, not the init node's — for
// its view of the cluster. Going through each server's own endpoints is the whole point: a split
// brain is precisely the case where the init node's answers are not representative.
func readServerView(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, pod *corev1.Pod, dataDir string, supervisorPort int) (serverView, error) {
	t.Helper()

	members, err := readETCDMembers(t, cs, fx, pod, dataDir, supervisorPort)
	if err != nil {
		return serverView{}, err
	}

	var view serverView
	for _, m := range members {
		view.members = append(view.members, fmt.Sprintf("%s/%d", m.Name, m.ID))
		if m.IsLearner {
			view.learners = append(view.learners, m.Name)
		}
	}
	slices.Sort(view.members)
	slices.Sort(view.learners)

	nodes, err := cluster.ExecOnPod(cs, fx.ns.Name, pod.Name, "sh", "-c", fmt.Sprintf("export %s && %s", fx.kubectlEnv,
		"kubectl get nodes --sort-by=.metadata.name -o jsonpath='{range .items[*]}"+
			"{.metadata.name}={.metadata.uid}={.status.nodeInfo.kubeletVersion}={range .status.conditions[?(@.type==\"Ready\")]}{.status}{end}{\"\\n\"}{end}'"))
	if err != nil {
		return serverView{}, fmt.Errorf("listing nodes: %w: %s", err, nodes)
	}
	view.nodes = nodes

	return view, nil
}

// assertImportedServersConverged fails unless every etcd server in servers agrees on the shape of
// the cluster: each one's local etcd reports the same full, learner-free membership — one voting member per server — and each one's local API server returns the same
// nodes, all Ready and on version. Every server passed must run etcd and the API server.
//
// assertImportedClusterHealthy only asks the init node, which is exactly the wrong place to look for
// a split brain: the init node is the one the restore resets, so it is guaranteed to be on the
// restored side. A server that rejoined the wrong cluster, or was never brought back into the
// restored one, only shows up when it is asked directly.
func assertImportedServersConverged(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, servers []importedNode, version string) {
	t.Helper()

	logrus.Infof("waiting for all %d servers of cluster %s to agree on etcd membership and nodes at %s",
		len(servers), fx.mgmtCluster.Name, version)

	dataDir := fmt.Sprintf("/var/lib/rancher/%s", capr.GetRuntime(version))
	supervisorPort := capr.GetRuntimeSupervisorPort(version)

	var views map[string]string
	err := utilwait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 15*time.Minute, true, func(context.Context) (bool, error) {
		views = map[string]string{}

		var reference *serverView
		converged := true
		for _, server := range servers {
			view, err := readServerView(t, cs, fx, server.pod, dataDir, supervisorPort)
			if err != nil {
				views[server.name] = err.Error()
				converged = false
				continue
			}
			views[server.name] = view.String()

			if !serverViewHealthy(view, len(servers), version) {
				converged = false
			}
			if reference == nil {
				reference = &view
				continue
			}
			if !slices.Equal(view.members, reference.members) || view.nodes != reference.nodes {
				converged = false
			}
		}
		return converged, nil
	})
	if err != nil {
		var report strings.Builder
		for _, server := range servers {
			fmt.Fprintf(&report, "\n  %s: %s", server.name, views[server.name])
		}
		t.Fatalf("servers of cluster %s do not agree on a single healthy cluster at %s (possible split brain):%s",
			fx.mgmtCluster.Name, version, report.String())
	}
}

// serverViewHealthy reports whether a single server's view is, on its own, what a converged cluster
// looks like: every etcd server a voting member, and every node Ready on version.
func serverViewHealthy(view serverView, etcdServers int, version string) bool {
	if len(view.members) != etcdServers || len(view.learners) != 0 {
		return false
	}
	lines := strings.Split(strings.TrimSpace(view.nodes), "\n")
	if len(lines) == 0 || strings.TrimSpace(view.nodes) == "" {
		return false
	}
	for _, line := range lines {
		fields := strings.Split(strings.TrimSpace(line), "=")
		if len(fields) != 4 || fields[2] != version || fields[3] != "True" {
			return false
		}
	}
	return true
}

// waitForBackpopulatedLocalSnapshotOnNode is waitForBackpopulatedSnapshotForStorage narrowed to the
// local snapshot taken on nodeName: an on-demand save snapshots every etcd node, and the newest of
// those is whichever node happened to finish last.
func waitForBackpopulatedLocalSnapshotOnNode(t *testing.T, clients *clients.Clients, clusterName, nodeName string, createdAfter time.Time) *rkev1.ETCDSnapshot {
	t.Helper()

	var (
		picked *rkev1.ETCDSnapshot
		seen   []string
	)
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
		list, err := clients.RKE.ETCDSnapshot().List(clusterName, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, clusterName),
		})
		if err != nil {
			return false, err
		}
		seen = seen[:0]
		for i := range list.Items {
			s := &list.Items[i]
			if s.SnapshotFile.Name == "" || s.SnapshotFile.S3 != nil {
				continue
			}
			if s.SnapshotFile.CreatedAt == nil || !s.SnapshotFile.CreatedAt.Time.After(createdAfter) {
				continue
			}
			seen = append(seen, s.SnapshotFile.NodeName)
			if s.SnapshotFile.NodeName != nodeName {
				continue
			}
			if picked == nil || s.SnapshotFile.CreatedAt.After(picked.SnapshotFile.CreatedAt.Time) {
				picked = s
			}
		}
		return picked != nil, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for a back-populated local ETCDSnapshot from node %s in %s (found snapshots from %v): %v",
			nodeName, clusterName, seen, err)
	}
	t.Logf("using local snapshot %s/%s from node %s (file=%s)", picked.Namespace, picked.Name, nodeName, picked.SnapshotFile.Name)
	return picked
}

// assertETCDResetRanOnlyOn fails unless the restore's `--cluster-reset` ran on nodeName and on no
// other etcd node.
//
// The evidence is the reset's idempotency marker: the restore step runs through the idempotent
// action script, which records each attempt under <provisioning dir>/idempotence/<key>/, and the
// Shutdown step clears that tracking on every node at the start of the operation. So after a single
// restore, exactly the node that performed the reset has an entry under the restore key.
func assertETCDResetRanOnlyOn(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, nodeName string) {
	t.Helper()

	const markerDir = "/var/lib/rancher/capr/idempotence/etcd-restore/restore"
	configFile := fmt.Sprintf("/etc/rancher/%s/config.yaml.d/50-test.yaml", capr.GetRuntime(previousK8sVersion))

	ranOn := map[string]bool{}
	for _, pod := range fx.pods {
		out, err := cluster.ExecOnPod(cs, fx.ns.Name, pod.Name, "sh", "-c", fmt.Sprintf(
			"sed -n 's/^node-name: //p' %s; if [ -n \"$(ls -A %s 2>/dev/null)\" ]; then echo reset; fi",
			configFile, markerDir))
		require.NoError(t, err, "inspecting %s for the etcd reset marker: %s", pod.Name, out)

		lines := strings.Fields(out)
		require.NotEmpty(t, lines, "pod %s has no node-name in %s", pod.Name, configFile)
		ranOn[lines[0]] = slices.Contains(lines[1:], "reset")
	}

	require.Contains(t, ranOn, nodeName, "no node named %s in the fixture (nodes %v)", nodeName, ranOn)
	for node, reset := range ranOn {
		if node == nodeName {
			assert.True(t, reset, "etcd was not reset on %s, the node holding the snapshot", node)
		} else {
			assert.False(t, reset, "etcd was reset on %s, but the restore was supposed to run on %s", node, nodeName)
		}
	}
}

// requireHostIPFamilies skips unless the cluster the tests run against can give pods an address in
// every family in families, judged by its nodes' pod CIDRs. The node pods are ordinary pods there,
// so a family the host cluster does not hand out is one the downstream cluster cannot run on.
func requireHostIPFamilies(t *testing.T, cs *clients.Clients, families []corev1.IPFamily) {
	t.Helper()

	nodes, err := cs.Core.Node().List(metav1.ListOptions{})
	require.NoError(t, err, "listing host cluster nodes")

	var cidrs []string
	for _, node := range nodes.Items {
		cidrs = append(cidrs, node.Spec.PodCIDRs...)
	}
	for _, family := range families {
		found := slices.ContainsFunc(cidrs, func(cidr string) bool {
			ip, _, err := net.ParseCIDR(cidr)
			return err == nil && (ip.To4() != nil) == (family == corev1.IPv4Protocol)
		})
		if !found {
			t.Skipf("host cluster has no %s pod CIDR (pod CIDRs %v); create one with dev-scripts/k3d-dual-stack", family, cidrs)
		}
	}
}

// ipInFamily reports whether s is an IP address of family.
func ipInFamily(s string, family corev1.IPFamily) bool {
	ip := net.ParseIP(s)
	return ip != nil && (ip.To4() != nil) == (family == corev1.IPv4Protocol)
}

// assertImportedClusterOnFamily fails unless the downstream cluster runs on family: every node's
// first InternalIP is its own address in that family, and every etcd member advertises its peer URL
// on its node's address in that family.
//
// The first InternalIP is the one that counts because it is the node's primary address — the kubelet
// orders them by node-ip — and the etcd peer URLs are what members actually dial each other on, so
// together they say which network the cluster is really using rather than merely which addresses it
// has. Both are polled: after a restore the nodes re-register and etcd membership is rebuilt.
func assertImportedClusterOnFamily(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, family corev1.IPFamily) {
	t.Helper()

	logrus.Infof("checking that cluster %s runs on %s", fx.mgmtCluster.Name, family)

	nodes := importedNodes(t, cs, fx)
	wantIP := map[string]string{}
	etcdNodes := 0
	for _, node := range nodes {
		ip := node.ip(family)
		require.NotEmpty(t, ip, "node %s (pod %s) has no %s address: %v", node.name, node.pod.Name, family, node.pod.Status.PodIPs)
		wantIP[node.name] = ip
		if node.etcd {
			etcdNodes++
		}
	}

	dataDir := fmt.Sprintf("/var/lib/rancher/%s", capr.GetRuntime(previousK8sVersion))
	supervisorPort := capr.GetRuntimeSupervisorPort(previousK8sVersion)

	var reason string
	err := utilwait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
		out, err := fx.execKubectl(t, "kubectl get nodes -o jsonpath='{range .items[*]}"+
			"{.metadata.name}={range .status.addresses[?(@.type==\"InternalIP\")]}{.address},{end}{\"\\n\"}{end}'")
		if err != nil {
			reason = fmt.Sprintf("listing nodes: %v: %s", err, out)
			return false, nil
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if strings.TrimSpace(out) == "" || len(lines) != len(wantIP) {
			reason = fmt.Sprintf("downstream lists %d nodes, want %d: %q", len(lines), len(wantIP), out)
			return false, nil
		}
		for _, line := range lines {
			name, addrs, _ := strings.Cut(strings.TrimSpace(line), "=")
			primary, _, _ := strings.Cut(addrs, ",")
			if _, known := wantIP[name]; !known || primary != wantIP[name] {
				reason = fmt.Sprintf("node %s has primary InternalIP %q, want its %s address %s (InternalIPs %s)",
					name, primary, family, wantIP[name], strings.TrimSuffix(addrs, ","))
				return false, nil
			}
		}

		members, err := readETCDMembers(t, cs, fx, fx.pods[0], dataDir, supervisorPort)
		if err != nil {
			reason = err.Error()
			return false, nil
		}
		if len(members) != etcdNodes {
			reason = fmt.Sprintf("etcd lists %d members, want %d: %+v", len(members), etcdNodes, members)
			return false, nil
		}
		for _, m := range members {
			nodeName := etcdMemberNodeName(m.Name, wantIP)
			if nodeName == "" || len(m.PeerURLs) == 0 {
				reason = fmt.Sprintf("etcd member %s matches no node, or advertises no peer URLs: %+v", m.Name, m)
				return false, nil
			}
			for _, peer := range m.PeerURLs {
				u, err := url.Parse(peer)
				if err != nil || u.Hostname() != wantIP[nodeName] {
					reason = fmt.Sprintf("etcd member %s advertises peer URL %q, want its node's %s address %s",
						m.Name, peer, family, wantIP[nodeName])
					return false, nil
				}
			}
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("cluster %s is not running on %s: %s", fx.mgmtCluster.Name, family, reason)
	}
}

// etcdMemberNodeName maps an etcd member name, which the distro forms as "<node name>-<suffix>", back
// to the node in nodes it belongs to, or "" if none matches.
func etcdMemberNodeName(member string, nodes map[string]string) string {
	for name := range nodes {
		if strings.HasPrefix(member, name+"-") {
			return name
		}
	}
	return ""
}

// watchRestoreServerArgs starts recording what the restore tells each node to join through, and
// returns a function that stops recording and returns, per downstream node name, every distinct
// `server:` value written to the node's zz_etcd-snapshot-restore.yaml drop-in, in the order seen.
//
// The drop-in only exists while the restore runs — the final restart removes it — so it cannot be read
// off the nodes afterward. Instead this watches the machine-plan secrets the restore writes, which
// carry every file the plan lays down. The restore leader's own drop-in is `server: ""`, blanking the
// server its config would otherwise join; every other node's points at the leader.
func watchRestoreServerArgs(t *testing.T, cs *clients.Clients, clusterNamespace string) func() map[string][]string {
	t.Helper()

	ctx, cancel := context.WithCancel(cs.Ctx)
	var (
		mu   sync.Mutex
		seen = map[string][]string{}
		done = make(chan struct{})
	)

	record := func(secret *corev1.Secret) {
		var nodePlan plan.Plan
		if err := json.Unmarshal(secret.Data["plan"], &nodePlan); err != nil {
			return
		}
		for _, f := range nodePlan.Files {
			if path.Base(f.Path) != "zz_etcd-snapshot-restore.yaml" {
				continue
			}
			content, err := base64.StdEncoding.DecodeString(f.Content)
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(content), "\n") {
				value, ok := strings.CutPrefix(strings.TrimSpace(line), "server:")
				if !ok {
					continue
				}
				value = strings.Trim(strings.TrimSpace(value), `"`)
				node := secret.Labels[capr.NodeNameLabel]
				mu.Lock()
				if !slices.Contains(seen[node], value) {
					seen[node] = append(seen[node], value)
				}
				mu.Unlock()
			}
		}
	}

	opts := metav1.ListOptions{FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan)}

	// 1. Synchronously list existing plans before returning, capturing any updates that occurred prior to watch setup.
	if list, err := cs.Core.Secret().List(clusterNamespace, opts); err == nil {
		for i := range list.Items {
			record(&list.Items[i])
		}
		// Set ResourceVersion so the subsequent watch resumes from this snapshot point without missing events.
		opts.ResourceVersion = list.ResourceVersion
	}

	go func() {
		defer close(done)
		// Re-establish the watch until stopped: a watch can be closed by the server at any time, and
		// missing the window in which the drop-ins are assigned would make the assertion vacuous.
		for ctx.Err() == nil {
			w, err := cs.Core.Secret().Watch(clusterNamespace, opts)
			if err != nil {
				select {
				case <-ctx.Done():
				case <-time.After(time.Second):
				}
				continue
			}
			func() {
				defer w.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case event, ok := <-w.ResultChan():
						if !ok {
							return
						}
						if secret, ok := event.Object.(*corev1.Secret); ok {
							record(secret)
							// Track the latest ResourceVersion so re-established watches resume where left off.
							opts.ResourceVersion = secret.ResourceVersion
						}
					}
				}
			}()
		}
	}()

	return func() map[string][]string {
		cancel()
		<-done
		mu.Lock()
		defer mu.Unlock()
		return maps.Clone(seen)
	}
}

// assertRestoreServerArgsOnFamily fails unless the restore pointed every rejoining node at the
// restore leader's family address and supervisor port, and at nothing else.
//
// The leader is the node whose drop-in blanked its server. When restoreNode is set it must be that
// node, which also cross-checks the snapshot-to-leader correlation.
func assertRestoreServerArgsOnFamily(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, seen map[string][]string, family corev1.IPFamily, restoreNode string) {
	t.Helper()

	t.Logf("restore server drop-ins by node: %v", seen)

	var leaders []string
	for node, values := range seen {
		if slices.Contains(values, "") {
			leaders = append(leaders, node)
		}
	}
	require.Len(t, leaders, 1, "expected exactly one node to have its server blanked by the restore, got %v (drop-ins %v)", leaders, seen)
	leader := leaders[0]
	if restoreNode != "" {
		require.Equal(t, restoreNode, leader, "the restore reset etcd on %s, want %s", leader, restoreNode)
	}

	nodes := importedNodes(t, cs, fx)
	var leaderIP string
	for _, node := range nodes {
		if node.name == leader {
			leaderIP = node.ip(family)
		}
	}
	require.NotEmpty(t, leaderIP, "restore leader %s has no %s address", leader, family)
	want := "https://" + net.JoinHostPort(leaderIP, strconv.Itoa(capr.GetRuntimeSupervisorPort(previousK8sVersion)))

	for _, node := range nodes {
		if node.name == leader {
			continue
		}
		values := seen[node.name]
		if !assert.NotEmpty(t, values, "the restore never pointed %s at the leader", node.name) {
			continue
		}
		for _, value := range values {
			u, err := url.Parse(value)
			if !assert.NoError(t, err, "the restore pointed %s at %q, which is not a valid URL (an IPv6 address must be bracketed)", node.name, value) {
				continue
			}
			assert.True(t, ipInFamily(u.Hostname(), family), "the restore pointed %s at %q, which is not a %s address", node.name, value, family)
			assert.Equal(t, want, value, "the restore pointed %s at %q, want the leader %s at %q", node.name, value, leader, want)
		}
	}
}

// distroCACandidates are the paths, relative to the distro data directory, where a node keeps the
// cluster's server CA: servers under server/tls, agents under agent.
var distroCACandidates = []string{"server/tls/server-ca.crt", "agent/server-ca.crt"}

// probeServerTLS connects from pod to serverURL's /cacerts endpoint verifying the server's
// certificate against the cluster CA found on pod, and returns an error carrying curl's output if
// the connection is not trusted.
//
// This is the check RKE2/K3s itself makes before joining: the first thing a node does with its
// `server:` URL is fetch /cacerts and validate it, with ordinary TLS verification including the
// certificate's subject alternative names. An address the server's certificate does not list fails
// here exactly as it fails the join, but immediately and with the reason spelled out.
func probeServerTLS(cs *clients.Clients, namespace, pod, dataDir, serverURL string) error {
	var cas []string
	for _, rel := range distroCACandidates {
		cas = append(cas, path.Join(dataDir, rel))
	}
	out, err := cluster.ExecOnPod(cs, namespace, pod, "sh", "-c", fmt.Sprintf(
		`ca=""; for f in %s; do if [ -f "$f" ]; then ca=$f; break; fi; done
if [ -z "$ca" ]; then echo "no server CA found" >&2; exit 2; fi
curl -sS --globoff --cacert "$ca" -o /dev/null %q`,
		strings.Join(cas, " "), strings.TrimSuffix(serverURL, "/")+"/cacerts"))
	if err != nil {
		return fmt.Errorf("TLS probe of %s from %s failed: %w: %s", serverURL, pod, err, out)
	}
	return nil
}

// assertRestoreServerArgsTrusted fails unless every server URL the restore handed a node is trusted
// from that node: a verified TLS connection to it succeeds (see probeServerTLS). A URL that points at
// an address outside the leader's certificate leaves that node unable to rejoin, which otherwise
// surfaces only as a restart that times out.
//
// It is probed after the operation, from the node the URL was given to. The leader keeps its
// certificate across the restore, so what is trusted now is what was trusted when the node rejoined.
// The probe is retried for a while, since the leader's supervisor may still be restarting.
func assertRestoreServerArgsTrusted(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, seen map[string][]string) {
	t.Helper()

	nodes := importedNodes(t, cs, fx)
	dataDir := fmt.Sprintf("/var/lib/rancher/%s", capr.GetRuntime(previousK8sVersion))

	probed := 0
	for _, node := range nodes {
		for _, serverURL := range seen[node.name] {
			if serverURL == "" {
				// The leader's own drop-in blanks its server rather than pointing anywhere.
				continue
			}
			probed++

			var lastErr error
			err := utilwait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 3*time.Minute, true, func(context.Context) (bool, error) {
				lastErr = probeServerTLS(cs, fx.ns.Name, node.pod.Name, dataDir, serverURL)
				return lastErr == nil, nil
			})
			assert.NoError(t, err, "the restore told %s to rejoin through %s, which it cannot connect to with certificate verification: %v",
				node.name, serverURL, lastErr)
		}
	}
	if len(nodes) > 1 {
		require.NotZero(t, probed, "the restore handed no node a server URL to probe (drop-ins %v)", seen)
	}
}
