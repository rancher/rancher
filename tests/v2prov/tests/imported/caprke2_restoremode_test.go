package imported

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/restoremode"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/objectstore"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// The two Test_Imported_Operation_SetE_CAPRKE2DockerRestoreMode* tests below are the CAPRKE2
// analogues of the imported restore-mode tests in etcdsnapshotrestoremode_test.go. Each one takes a
// snapshot on one Kubernetes version, upgrades the cluster, then restores selecting a mode and
// asserts the configuration the snapshot captured comes back — without the restore itself disturbing
// the cluster.
//
// What differs from the imported cluster case:
//
//   - The restore target is the RKE2ControlPlane, not the mgmt v3 Cluster. CAPRKE2Adapter.RestoreTarget
//     serves rke2controlplane.controlplane.cluster.x-k8s.io and nothing else, so the version lives at
//     spec.version rather than spec.rke2Config.kubernetesVersion.
//   - The upgrade is carried out by CAPRKE2's control-plane controller *replacing* machines, not by
//     the downstream system-upgrade-controller upgrading them in place. That replacement is asserted:
//     it is what makes the restore interesting, since the machine that took the snapshot is gone by
//     the time the snapshot is restored.
//   - restoremode.WritablePaths permits exactly one path for an RKE2ControlPlane, {spec, version}.
//     So `all` currently resolves to the same single field as `kubernetesVersion`; the `all` test
//     below asserts that, and additionally that a field outside the allowlist is NOT reverted.
//
// Three things these tests do differently from caprke2_test.go, all load-bearing:
//
//   - Snapshots go to S3 (Minio), not to machine-local disk. This is not about convenience: RKE2
//     stamps ETCDSnapshotFile.Spec.Metadata only on the resource belonging to the node that took the
//     snapshot, and does not persist that metadata next to the file, so a node that re-discovers a
//     local file registers it with no metadata and the snapshot collapses to the "none" restore mode.
//     For S3 the metadata is uploaded with the snapshot and read back by whichever node lists the
//     bucket. Since the upgrade replaces the machine that took the snapshot, S3 is the only way the
//     captured configuration is still there to restore.
//   - Operations are addressed at the mgmt v3 Cluster (fx.MgmtClusterRef) and created in its
//     namespace, not the CAPI Cluster's. Restore modes are the first thing that genuinely needs the
//     rkev1.ETCDSnapshot resource, and only the mgmt-cluster path resolves the namespace it lives
//     in — see the comment on MgmtClusterRef. caprke2_test.go gets away with a CAPI ClusterRef
//     because it restores by file name and a missing CR is tolerated there.
//   - The machines are pinned with a CAPI pre-terminate hook for the duration of the restore, so an
//     unexpected rollout shows up as a specific assertion failure instead of destroying the machine
//     holding the restored etcd data.
//
// Like the other CAPRKE2 tests these are LOCAL-DEV ONLY and gated by V2PROV_TEST_CAPRKE2=true; CI
// has no CAPRKE2 matrix entry. They additionally need SOME_K8S_VERSION_PREV, since a real upgrade
// needs two installable versions. Local recipe:
//
//	make dev-env
//	# run Rancher locally
//	make install-caprke2-providers
//	V2PROV_TEST_CAPRKE2=true \
//	  SOME_K8S_VERSION=v1.33.0+rke2r1 SOME_K8S_VERSION_PREV=v1.32.5+rke2r1 \
//	  go test -v -run '^Test_Imported_Operation_SetE_CAPRKE2DockerRestoreMode' \
//	  ./tests/v2prov/tests/imported/...

// Test_Imported_Operation_SetE_CAPRKE2DockerRestoreModeKubernetesVersion verifies the downgrade
// path on a CAPRKE2 cluster: snapshot on the older version, upgrade (which replaces the machines),
// then restore selecting kubernetesVersion, after which the control plane is configured for and
// running the snapshot's version again — on the same machines, with the cluster healthy.
func Test_Imported_Operation_SetE_CAPRKE2DockerRestoreModeKubernetesVersion(t *testing.T) {
	requireCAPRKE2(t)
	requireTwoVersions(t)

	cs, fx, downstream := setUpCAPRKE2RestoreModeCluster(t, "v2prov-caprke2-rm-kv")

	proof := newCAPRKE2ConfigMapProof(t, downstream)

	snapshot := saveCAPRKE2Snapshot(t, cs, fx)
	assertSnapshotOffersMode(t, snapshot, rkev1.RestoreRKEConfigKubernetesVersion)

	machinesBefore, err := fx.Machines(cs)
	require.NoError(t, err)

	// Upgrade. This is what the restore has to undo. CAPRKE2 converges on a new version by replacing
	// machines, so assert that actually happened: the snapshot is now the only remaining record of
	// the pre-upgrade cluster, which is the situation a restore mode exists for.
	t.Logf("upgrading RKE2ControlPlane %s/%s from %s to %s", fx.Namespace, fx.ClusterName, previousK8sVersion, defaults.SomeK8sVersion)
	require.NoError(t, fx.SetRKE2ControlPlaneVersion(cs, defaults.SomeK8sVersion))
	waitForCAPRKE2Upgrade(t, cs, fx, defaults.SomeK8sVersion)
	fx.WaitForMachineReplacement(t, cs, machinesBefore)
	fx.WaitForCAPIClusterHealthy(t, cs, 30*time.Minute)

	// The machine that took the snapshot is gone, so its ETCDSnapshot CR has been replaced by one
	// back-populated from the S3 listing of whichever machine now owns the cluster. Re-resolve, then
	// confirm the re-created CR still describes the pre-upgrade configuration.
	restoreName := reresolveCAPRKE2Snapshot(t, cs, fx, snapshot.SnapshotFile.Name)
	assertSnapshotCapturedVersion(t, cs, fx, restoreName, rkev1.RestoreRKEConfigKubernetesVersion)

	proof.delete(t)

	op := RunETCDSnapshotRestoreOperationTest(t, cs, fx.MgmtClusterName, restoreName, fx.MgmtClusterRef(),
		WithRestoreMode(rkev1.RestoreRKEConfigKubernetesVersion),
		WithRestoreTTL(-1))
	require.NotNil(t, op)

	proof.assertRestored(t)

	// The RestoreClusterConfig step writes spec.version back to the snapshot's value, and the
	// Restore step reinstalls that version before --cluster-reset.
	waitForCAPRKE2SpecVersion(t, cs, fx, previousK8sVersion)
	waitForDownstreamNodeVersion(t, downstream, previousK8sVersion)

	fx.WaitForCAPIClusterHealthy(t, cs, 30*time.Minute)
}

// Test_Imported_Operation_SetE_CAPRKE2DockerRestoreModeAll verifies `all` on a CAPRKE2 cluster.
//
// For an RKE2ControlPlane the allowlist permits only spec.version, so `all` reverts the version and
// nothing else. The test proves both halves of that: the version comes back, and a control label on
// spec.machineTemplate.metadata — captured in the snapshot but outside restoremode.WritablePaths —
// does not. That second assertion is the point of the test: it pins the upstream guard that stops a
// downstream payload from rewriting arbitrary fields of its own control plane.
func Test_Imported_Operation_SetE_CAPRKE2DockerRestoreModeAll(t *testing.T) {
	requireCAPRKE2(t)
	requireTwoVersions(t)

	cs, fx, downstream := setUpCAPRKE2RestoreModeCluster(t, "v2prov-caprke2-rm-all")

	// Captured in the snapshot alongside the version, but not restorable.
	const capturedMarker = "captured-before-snapshot"
	require.NoError(t, fx.SetNonRestorableMarker(cs, capturedMarker))

	proof := newCAPRKE2ConfigMapProof(t, downstream)

	snapshot := saveCAPRKE2Snapshot(t, cs, fx)
	assertSnapshotOffersMode(t, snapshot, rkev1.RestoreRKEConfigAll)

	machinesBefore, err := fx.Machines(cs)
	require.NoError(t, err)

	// Change both the version and the non-restorable field.
	t.Logf("upgrading RKE2ControlPlane %s/%s and changing the non-restorable marker", fx.Namespace, fx.ClusterName)
	require.NoError(t, fx.SetRKE2ControlPlaneVersion(cs, defaults.SomeK8sVersion))
	waitForCAPRKE2Upgrade(t, cs, fx, defaults.SomeK8sVersion)
	fx.WaitForMachineReplacement(t, cs, machinesBefore)
	fx.WaitForCAPIClusterHealthy(t, cs, 30*time.Minute)

	const changedMarker = "changed-after-snapshot"
	require.NoError(t, fx.SetNonRestorableMarker(cs, changedMarker))

	// See the kubernetesVersion test: the roll renames the CR, and the re-created one has to still
	// describe the pre-upgrade cluster for the restore to mean anything.
	restoreName := reresolveCAPRKE2Snapshot(t, cs, fx, snapshot.SnapshotFile.Name)
	assertSnapshotCapturedVersion(t, cs, fx, restoreName, rkev1.RestoreRKEConfigAll)

	proof.delete(t)

	op := RunETCDSnapshotRestoreOperationTest(t, cs, fx.MgmtClusterName, restoreName, fx.MgmtClusterRef(),
		WithRestoreMode(rkev1.RestoreRKEConfigAll),
		WithRestoreTTL(-1))
	require.NotNil(t, op)

	proof.assertRestored(t)

	waitForCAPRKE2SpecVersion(t, cs, fx, previousK8sVersion)
	waitForDownstreamNodeVersion(t, downstream, previousK8sVersion)

	// The control: the snapshot captured capturedMarker, but the field is outside
	// restoremode.WritablePaths, so the restore must have left the post-snapshot value in place.
	marker, err := fx.NonRestorableMarker(cs)
	require.NoError(t, err)
	assert.Equal(t, changedMarker, marker,
		"all must not restore a field outside restoremode.WritablePaths (captured %q)", capturedMarker)

	fx.WaitForCAPIClusterHealthy(t, cs, 30*time.Minute)
}

// requireCAPRKE2 skips unless the local CAPRKE2 provider set is installed and the caller opted in,
// matching the gate on the other CAPRKE2 tests.
func requireCAPRKE2(t *testing.T) {
	t.Helper()

	if os.Getenv("V2PROV_TEST_CAPRKE2") != "true" {
		t.Skip("V2PROV_TEST_CAPRKE2 not set; skipping CAPRKE2 + Docker restore-mode test (local-only)")
	}
}

// setUpCAPRKE2RestoreModeCluster brings up a single-server CAPRKE2 cluster pinned to the older
// Kubernetes version, with etcd snapshots going to a Minio object store, and returns the clients,
// fixture and a downstream client. Single server is deliberate: a restore replays one etcd member's
// snapshot, and the smaller topology keeps the upgrade's machine replacement to a single machine.
func setUpCAPRKE2RestoreModeCluster(t *testing.T, namePrefix string) (*clients.Clients, *cluster.CAPRKE2Fixture, kubernetes.Interface) {
	t.Helper()

	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cs.Close)

	// The object store lives in `default` — like the shared registry cache — rather than in the
	// cluster's namespace, which NewCAPRKE2Cluster creates itself (and labels for Turtles
	// auto-import). It has to be reachable from inside the CAPD machine containers, which are on the
	// same Docker bridge network as the local cluster's nodes but outside its pod network, so the
	// external (NodePort) flavour is required — a ClusterIP endpoint is simply unroutable there.
	osInfo, err := objectstore.GetExternalObjectStore(cs, "default", "extstore0", "s3snapshots")
	if err != nil {
		t.Fatalf("setting up object store: %v", err)
	}

	// One folder per run, so a rerun never has to reason about a previous run's snapshots in the
	// bucket (RKE2 lists the whole folder and back-populates everything it finds).
	folder := namePrefix + "-" + strings.ToLower(name.Hex(time.Now().String(), 8))
	t.Logf("etcd snapshots go to s3://%s/%s at %s", osInfo.Bucket, folder, osInfo.Endpoint)

	fx, err := cluster.NewCAPRKE2Cluster(cs, cluster.CAPRKE2Options{
		NamePrefix:  namePrefix,
		Replicas:    1,
		RKE2Version: previousK8sVersion,
		S3: &cluster.CAPRKE2S3{
			Endpoint:   osInfo.Endpoint,
			EndpointCA: osInfo.Cert,
			Bucket:     osInfo.Bucket,
			Folder:     folder,
			AccessKey:  osInfo.AccessKey,
			SecretKey:  osInfo.SecretKey,
		},
	})
	if err != nil {
		t.Fatalf("creating CAPRKE2 cluster: %v", err)
	}
	cluster.WaitForCAPRKE2Ready(t, cs, fx)

	t.Logf("CAPI cluster ready: namespace=%s name=%s mgmtV3Name=%s version=%s",
		fx.Namespace, fx.ClusterName, fx.MgmtClusterName, previousK8sVersion)

	// The cluster comes up on the older version, so that is what the snapshot captures.
	version, err := fx.RKE2ControlPlaneVersion(cs)
	require.NoError(t, err)
	require.Equal(t, previousK8sVersion, version, "cluster should have been provisioned on the older version")

	downstream, err := fx.DownstreamClient(cs)
	if err != nil {
		t.Fatalf("building downstream client: %v", err)
	}

	waitForSnapshotMetadataReady(t, downstream, previousK8sVersion)

	return cs, fx, downstream
}

// waitForSnapshotMetadataReady blocks until a snapshot taken now would actually capture the extra
// metadata that the restore modes are resolved from.
//
// RKE2 will happily take a snapshot and record no metadata at all, silently: a snapshot taken before
// snapshotextrametadata has written the ConfigMap comes out bare, which leaves the upstream
// ETCDSnapshot advertising only the "none" restore mode and cancels any mode-bearing restore during
// preflight. Waiting for the cluster to be Ready is not enough — the publisher runs per downstream
// cluster and is not ordered against anything the test does.
func waitForSnapshotMetadataReady(t *testing.T, downstream kubernetes.Interface, kubernetesVersion string) {
	t.Helper()

	runtime := capr.GetRuntime(kubernetesVersion)
	extraMetadataName := runtime + "-etcd-snapshot-extra-metadata"

	t.Logf("waiting for %s/%s before taking a snapshot", metav1.NamespaceSystem, extraMetadataName)

	cms := downstream.CoreV1().ConfigMaps(metav1.NamespaceSystem)

	var lastReason string
	err := utilwait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		extra, err := cms.Get(ctx, extraMetadataName, metav1.GetOptions{})
		if err != nil {
			lastReason = fmt.Sprintf("getting %s: %v", extraMetadataName, err)
			return false, nil
		}
		for _, key := range []string{rkev1.SnapshotMetadataResourcesKey, rkev1.SnapshotMetadataRestoreModesKey} {
			if extra.Data[key] == "" {
				lastReason = fmt.Sprintf("%s has no %q key yet (has %v)", extraMetadataName, key, mapKeys(extra.Data))
				return false, nil
			}
		}

		return true, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for the snapshot metadata to be publishable: %s", lastReason)
	}
}

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// saveCAPRKE2Snapshot takes a snapshot and returns the back-populated upstream ETCDSnapshot for the
// S3 copy of it.
func saveCAPRKE2Snapshot(t *testing.T, cs *clients.Clients, fx *cluster.CAPRKE2Fixture) *rkev1.ETCDSnapshot {
	t.Helper()

	validAfter := time.Now().Add(-30 * time.Second)
	saveOp := RunETCDSnapshotSaveOperationTest(t, cs, fx.MgmtClusterName, fx.MgmtClusterRef())
	t.Logf("snapshot save operation %s/%s completed", saveOp.Namespace, saveOp.Name)

	snapshot := waitForCAPRKE2S3Snapshot(t, cs, fx, validAfter)
	t.Logf("using snapshot %s (file=%s)", snapshot.Name, snapshot.SnapshotFile.Name)

	// Fail at the point of creation rather than letting a bare snapshot surface later as a restore
	// cancelled during preflight. waitForSnapshotMetadataReady should have made this impossible; if
	// it trips, RKE2 took the snapshot without reading the extra-metadata ConfigMap.
	metadata, err := snapshotutil.SnapshotMetadata(snapshot)
	require.NoError(t, err,
		"snapshot %s/%s captured no metadata, so no restore mode beyond 'none' will be offered",
		snapshot.Namespace, snapshot.Name)
	for _, key := range []string{rkev1.SnapshotMetadataResourcesKey, rkev1.SnapshotMetadataRestoreModesKey} {
		require.Contains(t, metadata, key,
			"snapshot %s/%s is missing the %q metadata key (captured %v)",
			snapshot.Namespace, snapshot.Name, key, mapKeys(metadata))
	}

	return snapshot
}

// waitForCAPRKE2Upgrade waits until the RKE2ControlPlane is configured for version and has finished
// rolling onto it. Both halves matter: spec.version flips immediately on write, while the machines
// take a full replacement to converge, and anything issued mid-roll would race the control plane.
//
// This is only usable for an upgrade driven through CAPRKE2. A restore reaches the same end state by
// a different route — it reinstalls the distro in place under a paused cluster — so status.version,
// which trails the machines the control plane created, is not the right thing to wait on there. See
// waitForCAPRKE2SpecVersion.
func waitForCAPRKE2Upgrade(t *testing.T, cs *clients.Clients, fx *cluster.CAPRKE2Fixture, version string) {
	t.Helper()

	t.Logf("waiting for RKE2ControlPlane %s/%s to settle on version %s", fx.Namespace, fx.ClusterName, version)

	err := utilwait.PollUntilContextTimeout(cs.Ctx, 15*time.Second, 45*time.Minute, true, func(context.Context) (bool, error) {
		obj, err := fx.RKE2ControlPlane(cs)
		if err != nil {
			return false, nil
		}

		spec, _, err := unstructured.NestedString(obj.Object, "spec", "version")
		if err != nil || spec != version {
			return false, nil
		}

		// status.version trails spec.version until the roll completes; replicas/readyReplicas
		// converging on each other is what says the roll is done.
		status, _, _ := unstructured.NestedString(obj.Object, "status", "version")
		replicas, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
		ready, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")

		return status == version && ready == replicas && ready > 0, nil
	})
	if err != nil {
		dumpCAPRKE2Version(t, cs, fx, version, err)
	}
}

// waitForCAPRKE2SpecVersion waits until the RKE2ControlPlane is *configured* for version, which is
// what a restore writes. That the cluster is actually running it is asserted separately, against the
// downstream nodes, by waitForDownstreamNodeVersion.
func waitForCAPRKE2SpecVersion(t *testing.T, cs *clients.Clients, fx *cluster.CAPRKE2Fixture, version string) {
	t.Helper()

	t.Logf("waiting for RKE2ControlPlane %s/%s spec.version to be %s", fx.Namespace, fx.ClusterName, version)

	err := utilwait.PollUntilContextTimeout(cs.Ctx, 10*time.Second, 20*time.Minute, true, func(context.Context) (bool, error) {
		obj, err := fx.RKE2ControlPlane(cs)
		if err != nil {
			return false, nil
		}
		spec, _, _ := unstructured.NestedString(obj.Object, "spec", "version")
		return spec == version, nil
	})
	if err != nil {
		dumpCAPRKE2Version(t, cs, fx, version, err)
	}
}

func dumpCAPRKE2Version(t *testing.T, cs *clients.Clients, fx *cluster.CAPRKE2Fixture, want string, cause error) {
	t.Helper()

	obj, err := fx.RKE2ControlPlane(cs)
	if err != nil {
		t.Fatalf("timed out waiting for version %s: %v", want, cause)
	}
	spec, _, _ := unstructured.NestedString(obj.Object, "spec", "version")
	status, _, _ := unstructured.NestedString(obj.Object, "status", "version")
	ready, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
	t.Fatalf("timed out waiting for version %s: spec.version=%s status.version=%s status.readyReplicas=%d: %v",
		want, spec, status, ready, cause)
}

// waitForDownstreamNodeVersion waits until every node in the downstream cluster reports version as
// its kubelet version, which is what says the distro was actually reinstalled rather than the control
// plane merely being configured for it.
//
// The poll tolerates a transient mix: a restore rolls etcd back to a point where the node objects of
// the machines that existed *then* are present again, and those stale objects hang around until the
// operation's PostRestoreNodeCleanup step removes them.
func waitForDownstreamNodeVersion(t *testing.T, downstream kubernetes.Interface, version string) {
	t.Helper()

	t.Logf("waiting for every downstream node to report kubelet version %s", version)

	var lastReason string
	err := utilwait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 30*time.Minute, true, func(ctx context.Context) (bool, error) {
		nodes, err := downstream.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			lastReason = fmt.Sprintf("listing nodes: %v", err)
			return false, nil
		}
		if len(nodes.Items) == 0 {
			lastReason = "no nodes registered"
			return false, nil
		}
		for _, node := range nodes.Items {
			if got := node.Status.NodeInfo.KubeletVersion; got != version {
				lastReason = fmt.Sprintf("node %s reports %s", node.Name, got)
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for downstream nodes to report version %s: %s", version, lastReason)
	}
}

// caprke2ConfigMapProof is the CAPRKE2 analogue of configMapProof: it writes a ConfigMap into the
// downstream cluster before the snapshot so its reappearance after the restore proves the etcd
// rollback happened, independently of any configuration change.
type caprke2ConfigMapProof struct {
	downstream kubernetes.Interface
	name       string
	value      string
}

func newCAPRKE2ConfigMapProof(t *testing.T, downstream kubernetes.Interface) *caprke2ConfigMapProof {
	t.Helper()

	p := &caprke2ConfigMapProof{
		downstream: downstream,
		name:       "caprke2-restore-cm-" + strings.ToLower(name.Hex(time.Now().String(), 10)),
		value:      "wow",
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: p.name, Namespace: "default"},
		Data:       map[string]string{"test": p.value},
	}
	if _, err := p.downstream.CoreV1().ConfigMaps("default").Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
		t.Fatalf("creating proof-of-restore configmap %s: %v", p.name, err)
	}

	return p
}

// delete removes the ConfigMap before the restore, so the post-restore assertion tests "restored"
// rather than "still there".
func (p *caprke2ConfigMapProof) delete(t *testing.T) {
	t.Helper()

	if err := p.downstream.CoreV1().ConfigMaps("default").Delete(context.TODO(), p.name, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("deleting proof-of-restore configmap %s: %v", p.name, err)
	}
}

// assertRestored polls the ConfigMap back into existence. The apiserver bounces during a restore, so
// the first several Gets are expected to fail before it settles.
func (p *caprke2ConfigMapProof) assertRestored(t *testing.T) {
	t.Helper()

	var (
		gotValue string
		getErr   error
	)
	for i := 0; i < 60; i++ {
		got, err := p.downstream.CoreV1().ConfigMaps("default").Get(context.TODO(), p.name, metav1.GetOptions{})
		if err == nil {
			gotValue = got.Data["test"]
			getErr = nil
			if gotValue == p.value {
				return
			}
		} else {
			getErr = err
		}
		time.Sleep(5 * time.Second)
	}
	if getErr != nil {
		t.Fatalf("get configmap %s failed after restore: %v", p.name, getErr)
	}
	t.Fatalf("expected configmap %s value restored to %q, got %q", p.name, p.value, gotValue)
}

// waitForCAPRKE2S3Snapshot polls until the S3 copy of a snapshot has been back-populated and returns
// the newest one.
//
// Both an S3 and a machine-local ETCDSnapshotFile exist for every snapshot taken by a cluster
// configured for S3 — RKE2 writes the file locally and then uploads it — and only the S3 one is
// usable here: the local file goes with the machine when the upgrade replaces it.
//
// The selector is the plan.cattle.io cluster lifecycle label, which is all an S3 snapshot carries.
// Unlike a local snapshot it has no machine identity stamped on it (snapshotbackpopulate owns it by
// the cluster, since any etcd node can pull it from the bucket), and the restore controller elects
// any etcd machine as the leader for the same reason.
//
// Snapshots live in the mgmt cluster's namespace, not the CAPI namespace.
func waitForCAPRKE2S3Snapshot(t *testing.T, cs *clients.Clients, fx *cluster.CAPRKE2Fixture, createdAfter time.Time) *rkev1.ETCDSnapshot {
	t.Helper()

	selector := labels.SelectorFromSet(labels.Set{
		planv1alpha1.ClusterLifecycleNameLabel: fx.ClusterName,
	}).String()

	var picked *rkev1.ETCDSnapshot
	err := utilwait.PollUntilContextTimeout(cs.Ctx, 5*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
		list, err := cs.RKE.ETCDSnapshot().List(fx.MgmtClusterName, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return false, err
		}
		for i := range list.Items {
			snapshot := &list.Items[i]
			if snapshot.SnapshotFile.S3 == nil || snapshot.SnapshotFile.Name == "" {
				continue
			}
			if snapshot.SnapshotFile.CreatedAt == nil || !snapshot.SnapshotFile.CreatedAt.Time.After(createdAfter) {
				continue
			}
			if picked == nil || snapshot.SnapshotFile.CreatedAt.After(picked.SnapshotFile.CreatedAt.Time) {
				picked = snapshot
			}
		}
		return picked != nil, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for a back-populated S3 ETCDSnapshot in %s with %s: %v", fx.MgmtClusterName, selector, err)
	}
	return picked
}

// reresolveCAPRKE2Snapshot returns the name of the ETCDSnapshot CR currently describing the S3 copy
// of the given snapshot file, and fails the test if there is none.
//
// A machine replacement invalidates the CR name the test captured before it: the CR name embeds a
// hash of the node name that registered the snapshot, so once the replacement machine lists the
// bucket, back-population produces a fresh CR under a different name. The file name is the only
// stable identifier across the roll.
func reresolveCAPRKE2Snapshot(t *testing.T, cs *clients.Clients, fx *cluster.CAPRKE2Fixture, snapshotFileName string) string {
	t.Helper()

	selector := labels.SelectorFromSet(labels.Set{
		planv1alpha1.ClusterLifecycleNameLabel: fx.ClusterName,
	}).String()

	var resolved string
	err := utilwait.PollUntilContextTimeout(cs.Ctx, 10*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
		list, err := cs.RKE.ETCDSnapshot().List(fx.MgmtClusterName, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return false, err
		}
		for i := range list.Items {
			if list.Items[i].SnapshotFile.S3 != nil && list.Items[i].SnapshotFile.Name == snapshotFileName {
				resolved = list.Items[i].Name
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("no S3 ETCDSnapshot in %s describes snapshot file %q after the machines were replaced "+
			"(the object store should have kept it, metadata included): %v",
			fx.MgmtClusterName, snapshotFileName, err)
	}

	t.Logf("re-resolved snapshot file %s to CR %s/%s", snapshotFileName, fx.MgmtClusterName, resolved)
	return resolved
}

// assertSnapshotCapturedVersion fails unless the snapshot's captured resources still describe the
// pre-upgrade Kubernetes version.
//
// This guards the assumption the whole test rests on. RKE2 stamps ETCDSnapshotFile.Spec.Metadata
// when it registers the resource; for S3 it reads that metadata back out of the bucket. If it were to
// re-stamp from the live cluster instead when a replacement machine re-registers the snapshot, the
// re-created CR would describe the *post-upgrade* cluster, kubernetesVersion would resolve to the
// version we just upgraded to, and the restore would quietly become a no-op that still passes every
// other assertion. Checking the captured value turns that into a specific failure.
func assertSnapshotCapturedVersion(t *testing.T, cs *clients.Clients, fx *cluster.CAPRKE2Fixture, snapshotName, mode string) {
	t.Helper()

	snapshot, err := cs.RKE.ETCDSnapshot().Get(fx.MgmtClusterName, snapshotName, metav1.GetOptions{})
	require.NoError(t, err)

	metadata, err := snapshotutil.SnapshotMetadata(snapshot)
	require.NoError(t, err, "snapshot %s/%s carries no decodable metadata", fx.MgmtClusterName, snapshotName)

	modes, err := restoremode.Modes(metadata)
	require.NoError(t, err)
	selector, ok := modes[mode]
	require.True(t, ok, "snapshot %s/%s does not declare restore mode %q (declares %v)",
		fx.MgmtClusterName, snapshotName, mode, modes)

	resources, err := restoremode.Resources(metadata)
	require.NoError(t, err)

	matches, err := restoremode.Resolve(selector, resources)
	require.NoError(t, err)
	require.NotEmpty(t, matches, "restore mode %q resolves to nothing in snapshot %s/%s",
		mode, fx.MgmtClusterName, snapshotName)

	for _, m := range matches {
		if len(m.Path) > 0 && m.Path[len(m.Path)-1] == "version" {
			assert.Equal(t, previousK8sVersion, m.Value,
				"snapshot %s/%s captured version %v, want the pre-upgrade %s — if this is the "+
					"post-upgrade version, the snapshot's metadata was re-stamped from the live "+
					"cluster when the replacement machine re-registered it, and the restore would "+
					"be a no-op",
				fx.MgmtClusterName, snapshotName, m.Value, previousK8sVersion)
			return
		}
	}
	t.Fatalf("restore mode %q in snapshot %s/%s selects no version field (selected %v)",
		mode, fx.MgmtClusterName, snapshotName, matches)
}
