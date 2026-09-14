package imported

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
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

// Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeKubernetesVersion verifies the downgrade path:
// a snapshot taken on one minor, the cluster upgraded to the next, then restored selecting
// kubernetesVersion — after which the cluster is configured for, and running, the snapshot's
// version again, on the same nodes, and healthy.
//
// This is the mode's whole purpose. The restore has to reinstall the distro at the snapshot's
// version before `--cluster-reset`, because a newer server cannot reset onto etcd data written by
// an older one.
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeKubernetesVersion(t *testing.T) {
	requireTwoVersions(t)

	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	fx := setUpImportedRestoreModeCluster(t, clients, "restore-mode-kv")

	// The cluster comes up on the older version, so that is what the snapshot captures.
	waitForReportedVersion(t, clients, fx.mgmtCluster, previousK8sVersion)
	waitForImportedSnapshotMetadataReady(t, fx)

	proof := newConfigMapProof()
	proof.create(t, fx)

	before := time.Now()
	RunETCDSnapshotSaveOperationTest(t, clients, fx.mgmtCluster.Name, fx.clusterRef)
	snapshot := waitForBackpopulatedS3Snapshot(t, clients, fx.mgmtCluster.Name, fx.mgmtCluster.Name, before)
	assertSnapshotOffersMode(t, snapshot, rkev1.RestoreRKEConfigKubernetesVersion)
	assertImportedSnapshotCapturedVersion(t, clients, fx.mgmtCluster.Name, snapshot.Name, rkev1.RestoreRKEConfigKubernetesVersion)

	nodesBefore := importedNodeIdentities(t, fx)

	// Upgrade. This is what the restore has to undo.
	logrus.Infof("upgrading cluster %s from %s to %s", fx.mgmtCluster.Name, previousK8sVersion, defaults.SomeK8sVersion)
	setDesiredVersion(t, clients, fx.mgmtCluster.Name, defaults.SomeK8sVersion)
	waitForReportedVersion(t, clients, fx.mgmtCluster, defaults.SomeK8sVersion)

	// An imported cluster upgrades in place — the system-upgrade-controller drains a node, swaps the
	// distro binary and brings it back — so unlike the CAPRKE2 case the nodes are expected to be the
	// same objects afterwards. Asserting it here is what makes the identical assertion after the
	// restore meaningful: both halves of the test measure the same thing.
	assertImportedNodesUnchanged(t, fx, nodesBefore, "the upgrade should have been carried out in place")

	// Delete the ConfigMap so the etcd rollback is observable independently of the version change.
	proof.delete(t, fx)

	op := RunETCDSnapshotRestoreOperationTest(t, clients, fx.mgmtCluster.Name, snapshot.Name, fx.clusterRef,
		WithRestoreMode(rkev1.RestoreRKEConfigKubernetesVersion),
		WithRestoreTTL(-1))
	require.NotNil(t, op)

	proof.assertRestored(t, fx)

	// The restore should have rewritten the desired version back to the snapshot's, and the nodes
	// should be running it — the reinstall in the Restore step is what puts the older binary back.
	waitForDesiredVersion(t, clients, fx.mgmtCluster, previousK8sVersion)
	waitForReportedVersion(t, clients, fx.mgmtCluster, previousK8sVersion)

	latest, err := clients.Mgmt.Cluster().Get(fx.mgmtCluster.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, previousK8sVersion, desiredVersion(latest),
		"kubernetesVersion restore should configure the cluster for the snapshot's version")
	require.NotNil(t, latest.Status.Version)
	assert.Equal(t, previousK8sVersion, latest.Status.Version.GitVersion,
		"cluster should be running the snapshot's version after the restore")

	assertImportedNodesUnchanged(t, fx, nodesBefore, "the restore must not replace any node")
	assertImportedClusterHealthy(t, clients, fx, previousK8sVersion)
}

// Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeAll verifies that `all` restores the rest of
// the captured configuration too, not just the Kubernetes version. It changes the cluster agent's
// deployment customization alongside the version, then asserts the restore reverts both.
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeAll(t *testing.T) {
	requireTwoVersions(t)

	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	fx := setUpImportedRestoreModeCluster(t, clients, "restore-mode-all")

	waitForReportedVersion(t, clients, fx.mgmtCluster, previousK8sVersion)
	waitForImportedSnapshotMetadataReady(t, fx)

	// The customization captured in the snapshot. Tolerations are a convenient probe: they are a
	// plain list on the mgmt cluster spec that Rancher applies to the cattle-cluster-agent
	// deployment, so a restore either brings the whole value back or it does not.
	capturedToleration := "captured-before-snapshot"
	setAgentToleration(t, clients, fx.mgmtCluster.Name, capturedToleration)
	waitForAgentToleration(t, clients, fx.mgmtCluster, capturedToleration)

	proof := newConfigMapProof()
	proof.create(t, fx)

	before := time.Now()
	RunETCDSnapshotSaveOperationTest(t, clients, fx.mgmtCluster.Name, fx.clusterRef)
	snapshot := waitForBackpopulatedS3Snapshot(t, clients, fx.mgmtCluster.Name, fx.mgmtCluster.Name, before)
	assertSnapshotOffersMode(t, snapshot, rkev1.RestoreRKEConfigAll)
	assertImportedSnapshotCapturedVersion(t, clients, fx.mgmtCluster.Name, snapshot.Name, rkev1.RestoreRKEConfigAll)

	nodesBefore := importedNodeIdentities(t, fx)

	// Change both things the restore should revert.
	logrus.Infof("upgrading cluster %s and changing its agent customization", fx.mgmtCluster.Name)
	setDesiredVersion(t, clients, fx.mgmtCluster.Name, defaults.SomeK8sVersion)
	waitForReportedVersion(t, clients, fx.mgmtCluster, defaults.SomeK8sVersion)
	assertImportedNodesUnchanged(t, fx, nodesBefore, "the upgrade should have been carried out in place")

	setAgentToleration(t, clients, fx.mgmtCluster.Name, "changed-after-snapshot")
	waitForAgentToleration(t, clients, fx.mgmtCluster, "changed-after-snapshot")

	proof.delete(t, fx)

	op := RunETCDSnapshotRestoreOperationTest(t, clients, fx.mgmtCluster.Name, snapshot.Name, fx.clusterRef,
		WithRestoreMode(rkev1.RestoreRKEConfigAll),
		WithRestoreTTL(-1))
	require.NotNil(t, op)

	proof.assertRestored(t, fx)

	// `all` covers every field the snapshot captured that Rancher permits restoring, so both the
	// version and the agent customization go back.
	waitForDesiredVersion(t, clients, fx.mgmtCluster, previousK8sVersion)
	waitForAgentToleration(t, clients, fx.mgmtCluster, capturedToleration)
	waitForReportedVersion(t, clients, fx.mgmtCluster, previousK8sVersion)

	latest, err := clients.Mgmt.Cluster().Get(fx.mgmtCluster.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, previousK8sVersion, desiredVersion(latest),
		"all should restore the snapshot's kubernetes version")
	assert.Equal(t, capturedToleration, agentToleration(latest),
		"all should restore the snapshot's cluster agent customization")

	assertImportedNodesUnchanged(t, fx, nodesBefore, "the restore must not replace any node")
	assertImportedClusterHealthy(t, clients, fx, previousK8sVersion)
}

// setUpImportedRestoreModeCluster brings up a single-node imported cluster on the older version with
// etcd snapshots going to a Minio object store.
//
// S3 rather than machine-local disk is the point here, and it is not about convenience. RKE2/K3s
// stamp ETCDSnapshotFile.Spec.Metadata only on the resource belonging to the node that took the
// snapshot, and do not persist that metadata next to the file; a node that merely re-discovers a local
// file registers it bare, which leaves the upstream ETCDSnapshot offering only the "none" restore
// mode. For S3 the metadata is uploaded with the snapshot (as <folder>/.metadata/<name>) and read back
// out of the bucket by whichever node lists it, so nothing about the test depends on which node
// happens to reconcile the snapshot.
func setUpImportedRestoreModeCluster(t *testing.T, cs *clients.Clients, displayName string) *importedClusterFixture {
	t.Helper()

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

	fx := setUpImportedClusterAtVersion(t, cs, displayName, []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	}, previousK8sVersion, importedS3Options(previousK8sVersion, osInfo, folder))

	return fx
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
		return mgmtv3.Ready.IsTrue(obj.(*mgmtv3.Cluster)), nil
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
