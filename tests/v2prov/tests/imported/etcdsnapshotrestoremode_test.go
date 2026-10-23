package imported

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/managementuser/snapshotbackpopulate"
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

// waitForReportedVersion waits until the cluster reports version, i.e. the nodes have actually
// converged on it rather than the desired field merely having been written.
func waitForReportedVersion(t *testing.T, clients *clients.Clients, c *mgmtv3.Cluster, version string) {
	t.Helper()

	logrus.Infof("waiting for cluster %s to report version %s", c.Name, version)

	err := wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, c, func(obj runtime.Object) (bool, error) {
		current := obj.(*mgmtv3.Cluster)
		if current.Status.Version == nil {
			return false, nil
		}
		// Upgraded has to be true as well: mid-rollout the reported version flips to the new value
		// as soon as the first node comes back, well before the rest have followed.
		return current.Status.Version.GitVersion == version, nil
	})
	handleError(t, clients, c.Name, err)
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
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeKubernetesVersionLocalHA(t *testing.T) {
	runImportedRestoreModeTestOn(t, "rm-kv-local-ha", rkev1.RestoreRKEConfigKubernetesVersion, snapshotbackpopulate.Local,
		[]cluster.ImportedNodePool{
			{ETCD: true, ControlPlane: true, Quantity: 3},
			{Worker: true, Quantity: 1},
		},
		cluster.ImportedClusterOptions{ExtractServerToken: true})
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

	runImportedRestoreModeTestOn(t, displayName, mode, storage, defaultRestoreModePools, cluster.ImportedClusterOptions{})
}

// runImportedRestoreModeTestOn is runImportedRestoreModeTest on an explicit topology. When the
// topology has more than one etcd node, it additionally asserts that those nodes agree with each
// other both before the snapshot and after the restore — see assertImportedServersConverged.
func runImportedRestoreModeTestOn(t *testing.T, displayName, mode string, storage snapshotbackpopulate.Storage, pools []cluster.ImportedNodePool, opts cluster.ImportedClusterOptions) {
	t.Helper()

	requireTwoVersions(t)

	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	fx := setUpImportedRestoreModeCluster(t, clients, displayName, storage, pools, opts)
	servers := importedETCDPods(fx, pools)

	// The cluster comes up on the older version, so that is what the snapshot captures.
	waitForReportedVersion(t, clients, fx.mgmtCluster, previousK8sVersion)
	if len(servers) > 1 {
		// Establish that the servers agree before anything happens, so that disagreement after the
		// restore is attributable to the restore.
		assertImportedServersConverged(t, clients, fx, servers, previousK8sVersion)
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
	snapshot := waitForBackpopulatedSnapshotForStorage(t, clients, fx.mgmtCluster.Name, fx.mgmtCluster.Name, storage, before)
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

	op := RunETCDSnapshotRestoreOperationTest(t, clients, fx.mgmtCluster.Name, snapshot.Name, fx.clusterRef,
		WithRestoreMode(mode),
		WithRestoreTTL(-1))
	require.NotNil(t, op)

	proof.assertRestored(t, fx)

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
// Cluster is Ready, and every downstream node is Ready and running the expected version. Ready on the
// mgmt cluster covers the parts of the downstream cluster Rancher itself watches — the agent is
// connected and the cluster's components are responding.
func assertImportedClusterHealthy(t *testing.T, clients *clients.Clients, fx *importedClusterFixture, version string) {
	t.Helper()

	logrus.Infof("waiting for cluster %s to be Ready after the restore", fx.mgmtCluster.Name)

	err := wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, fx.mgmtCluster, func(obj runtime.Object) (bool, error) {
		return mgmtv3.ClusterConditionReady.IsTrue(obj.(*mgmtv3.Cluster)), nil
	})
	handleError(t, clients, fx.mgmtCluster.Name, err)

	var lastOut string
	pollErr := utilwait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 15*time.Minute, true, func(context.Context) (bool, error) {
		out, err := fx.execKubectl(t, "kubectl get nodes -o jsonpath='{range .items[*]}"+
			"{.metadata.name}={.status.nodeInfo.kubeletVersion}={range .status.conditions[?(@.type==\"Ready\")]}{.status}{end}{\"\\n\"}{end}'")
		if err != nil {
			lastOut = fmt.Sprintf("%v: %s", err, out)
			return false, nil
		}
		lastOut = out

		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) == 0 || strings.TrimSpace(out) == "" {
			return false, nil
		}
		for _, line := range lines {
			fields := strings.Split(strings.TrimSpace(line), "=")
			if len(fields) != 3 {
				return false, nil
			}
			if fields[1] != version || fields[2] != "True" {
				return false, nil
			}
		}
		return true, nil
	})
	if pollErr != nil {
		t.Fatalf("downstream nodes are not all Ready on version %s (last read %q)", version, lastOut)
	}
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

// importedETCDPods returns the fixture's etcd pods. It relies on the
// ordering cluster.NewImportedClusterPods produces: the first etcd node is the init node at pods[0],
// and the rest follow in pool order with the init node left out.
func importedETCDPods(fx *importedClusterFixture, pools []cluster.ImportedNodePool) []*corev1.Pod {
	type role struct{ etcd bool }

	var (
		roles   []role
		initSet bool
	)
	for _, pool := range pools {
		for i := 0; i < pool.Quantity; i++ {
			r := role{etcd: pool.ETCD}
			if !initSet && pool.ETCD {
				roles = append([]role{r}, roles...)
				initSet = true
				continue
			}
			roles = append(roles, r)
		}
	}

	var servers []*corev1.Pod
	for i, r := range roles {
		if r.etcd && i < len(fx.pods) {
			servers = append(servers, fx.pods[i])
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
	Members []struct {
		ID        uint64 `json:"ID"`
		Name      string `json:"name"`
		IsLearner bool   `json:"isLearner"`
	} `json:"members"`
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

	// /db/info authenticates with the server token's password, i.e. everything after "::server:".
	serverDir := path.Join(dataDir, "server")
	out, err := cluster.ExecOnPod(cs, fx.ns.Name, pod.Name, "sh", "-c", fmt.Sprintf(
		`pw=$(cut -d: -f4- %[1]s/token) && curl -sSf --cacert %[1]s/tls/server-ca.crt -u "server:$pw" https://127.0.0.1:%[2]d/db/info`,
		serverDir, supervisorPort))
	if err != nil {
		return serverView{}, fmt.Errorf("listing etcd members: %w: %s", err, out)
	}

	var list etcdMemberList
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return serverView{}, fmt.Errorf("decoding etcd member list %q: %w", out, err)
	}

	var view serverView
	for _, m := range list.Members {
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
func assertImportedServersConverged(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, servers []*corev1.Pod, version string) {
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
		for _, pod := range servers {
			view, err := readServerView(t, cs, fx, pod, dataDir, supervisorPort)
			if err != nil {
				views[pod.Name] = err.Error()
				converged = false
				continue
			}
			views[pod.Name] = view.String()

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
		for _, pod := range servers {
			fmt.Fprintf(&report, "\n  %s: %s", pod.Name, views[pod.Name])
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
