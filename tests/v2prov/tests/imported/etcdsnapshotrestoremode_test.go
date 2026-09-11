package imported

import (
	"os"
	"testing"
	"time"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/restoremode"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
// version again.
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

	fx := setUpImportedClusterAtVersion(t, clients, "restore-mode-kv", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	}, previousK8sVersion)

	// The cluster comes up on the older version, so that is what the snapshot captures.
	waitForReportedVersion(t, clients, fx.mgmtCluster, previousK8sVersion)

	proof := newConfigMapProof()
	proof.create(t, fx)

	before := time.Now()
	RunETCDSnapshotSaveOperationTest(t, clients, fx.mgmtCluster.Name, fx.clusterRef)
	snapshot := waitForBackpopulatedSnapshot(t, clients, fx.mgmtCluster.Name, fx.mgmtCluster.Name, "imported-init-0", before)
	assertSnapshotOffersMode(t, snapshot, rkev1.RestoreRKEConfigKubernetesVersion)

	// Upgrade. This is what the restore has to undo.
	logrus.Infof("upgrading cluster %s from %s to %s", fx.mgmtCluster.Name, previousK8sVersion, defaults.SomeK8sVersion)
	setDesiredVersion(t, clients, fx.mgmtCluster.Name, defaults.SomeK8sVersion)
	waitForReportedVersion(t, clients, fx.mgmtCluster, defaults.SomeK8sVersion)

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
}

// Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeAll verifies that `all` restores the rest of
// the captured configuration too, not just the Kubernetes version. It changes the cluster agent's
// deployment customization alongside the version, then asserts the restore reverts both — and, for
// contrast, that a kubernetesVersion restore would not have touched the customization.
func Test_Imported_Operation_SetD_ETCDSnapshotRestoreModeAll(t *testing.T) {
	requireTwoVersions(t)

	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	fx := setUpImportedClusterAtVersion(t, clients, "restore-mode-all", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	}, previousK8sVersion)

	waitForReportedVersion(t, clients, fx.mgmtCluster, previousK8sVersion)

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
	snapshot := waitForBackpopulatedSnapshot(t, clients, fx.mgmtCluster.Name, fx.mgmtCluster.Name, "imported-init-0", before)
	assertSnapshotOffersMode(t, snapshot, rkev1.RestoreRKEConfigAll)

	// Change both things the restore should revert.
	logrus.Infof("upgrading cluster %s and changing its agent customization", fx.mgmtCluster.Name)
	setDesiredVersion(t, clients, fx.mgmtCluster.Name, defaults.SomeK8sVersion)
	waitForReportedVersion(t, clients, fx.mgmtCluster, defaults.SomeK8sVersion)

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
