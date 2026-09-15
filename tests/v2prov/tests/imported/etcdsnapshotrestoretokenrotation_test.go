package imported

import (
	"fmt"
	"strings"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/operations/etcdsnapshotrestore"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serverTokenFixture drives the server token of an imported cluster's init node: reading the token
// the node was brought up with, hashing the token the node currently has on disk, and rotating to a
// different one. The distro-specific paths and unit name are resolved once from the k8s version
// under test so the individual operations read as the story rather than as string formatting.
type serverTokenFixture struct {
	cs *clients.Clients
	fx *importedClusterFixture

	// runtimeCommand is the distro CLI (`rke2` / `k3s`), which is also the subcommand host for
	// `token rotate`.
	runtimeCommand string
	// serverUnit is the systemd unit that must be restarted for a new token to take effect.
	serverUnit string
	// dataDir is where the distro persists the token it is running with. Imported clusters always
	// use the runtime default (see ImportedAdapter.DistroDataDirectory).
	dataDir string
	// configFile is the config drop-in that cluster.NewImportedClusterPods wrote the token into. It
	// is the node's source of truth on restart, so a rotation has to update it.
	configFile string
}

func newServerTokenFixture(cs *clients.Clients, fx *importedClusterFixture) *serverTokenFixture {
	distro := capr.GetRuntime(defaults.SomeK8sVersion)
	return &serverTokenFixture{
		cs:             cs,
		fx:             fx,
		runtimeCommand: capr.GetRuntimeCommand(defaults.SomeK8sVersion),
		serverUnit:     capr.GetRuntimeServerUnit(defaults.SomeK8sVersion),
		dataDir:        fmt.Sprintf("/var/lib/rancher/%s", distro),
		configFile:     fmt.Sprintf("/etc/rancher/%s/config.yaml.d/50-test.yaml", distro),
	}
}

// exec runs a shell command on the init node. The init node is the only etcd member in the
// single-node topology this test uses, so it is also the only node the restore preflight checks.
func (s *serverTokenFixture) exec(t *testing.T, cmd string) (string, error) {
	t.Helper()
	return cluster.ExecOnPod(s.cs, s.fx.ns.Name, s.fx.pods[0].Name, "sh", "-c", cmd)
}

// configuredToken reads the token the node was brought up with out of its config drop-in. The token
// is generated inside cluster.NewImportedClusterPods and is not surfaced on the fixture, and
// rotating requires knowing the current value.
func (s *serverTokenFixture) configuredToken(t *testing.T) string {
	t.Helper()

	out, err := s.exec(t, fmt.Sprintf("sed -n 's/^token: //p' %s", s.configFile))
	if err != nil {
		t.Fatalf("reading the configured server token from %s failed: %v\noutput: %s", s.configFile, err, out)
	}
	token := strings.TrimSpace(out)
	if token == "" {
		t.Fatalf("no token found in %s", s.configFile)
	}
	return token
}

// tokenHash computes the hash of the token the node currently has on disk, using the very command
// the restore preflight check runs, so the test's notion of the current hash cannot drift from the
// controller's.
func (s *serverTokenFixture) tokenHash(t *testing.T) string {
	t.Helper()

	out, err := s.exec(t, fmt.Sprintf(etcdsnapshotrestore.TokenHashCommandFormat, s.dataDir))
	if err != nil {
		// The preflight instruction would fail the same way, and because its plan is assigned with a
		// failure threshold of -1 the operation would stall instead of failing — so surface this as
		// the token file not being where the check looks for it, rather than letting the test hang.
		t.Fatalf("hashing the on-disk server token under %s failed, so the restore preflight check cannot read it either: %v\noutput: %s",
			s.dataDir, err, out)
	}
	return strings.TrimSpace(out)
}

// rotate rotates the server token from currentToken to newToken and returns once the cluster is
// serving again with the new token in place.
func (s *serverTokenFixture) rotate(t *testing.T, currentToken, newToken string) {
	t.Helper()

	// `token rotate` re-encrypts the datastore's bootstrap data under the new token. The node keeps
	// running with the old one until it restarts with the new value in its config, which is also
	// when the token it persists on disk — the value the restore preflight check hashes — changes.
	if out, err := s.exec(t, fmt.Sprintf("%s token rotate --token %s --new-token %s", s.runtimeCommand, currentToken, newToken)); err != nil {
		t.Fatalf("rotating the server token failed: %v\noutput: %s", err, out)
	}

	if out, err := s.exec(t, fmt.Sprintf("sed -i 's|^token: .*|token: %s|' %s", newToken, s.configFile)); err != nil {
		t.Fatalf("updating the token in %s failed: %v\noutput: %s", s.configFile, err, out)
	}

	if out, err := s.exec(t, fmt.Sprintf("systemctl restart %s", s.serverUnit)); err != nil {
		diag, _ := s.exec(t, fmt.Sprintf("journalctl -u %s --no-pager -n 40 2>/dev/null || true", s.serverUnit))
		t.Fatalf("restarting %s after the token rotation failed: %v\noutput: %s\njournal:\n%s", s.serverUnit, err, out, diag)
	}

	s.waitForAPI(t)
}

// waitForAPI polls the downstream API server until it serves again. A restart of the server unit
// takes the API down for a while, and every subsequent step (snapshot back-population, the restore
// itself) depends on it being back.
func (s *serverTokenFixture) waitForAPI(t *testing.T) {
	t.Helper()

	for i := 0; i < 60; i++ {
		out, err := s.fx.execKubectl(t, "kubectl get nodes")
		if err == nil {
			return
		}
		if i == 59 {
			t.Fatalf("timed out waiting for the %s API server to serve after restarting %s: %v\noutput: %s",
				s.runtimeCommand, s.serverUnit, err, out)
		}
		time.Sleep(5 * time.Second)
	}
}

// Test_Imported_Operation_SetE_ImportedETCDSnapshotRestoreTokenRotation verifies that an
// ETCDSnapshotRestore refuses a snapshot that was taken under a different server token, and that the
// same snapshot restores successfully once the original token is back in place.
//
// A snapshot's bootstrap data is encrypted with the server token that was current when it was taken,
// so restoring it into a cluster whose token has since been rotated leaves a cluster that cannot
// start. The restore's Preflight step guards against that: the distro stamps a hash of the token on
// each snapshot, snapshotbackpopulate mirrors it onto the upstream ETCDSnapshot as
// rke.cattle.io/snapshot-token-hash, and Preflight hashes the token each etcd node currently has on
// disk and compares the two.
//
// The test takes a snapshot, rotates the server token (re-encrypting the live cluster's bootstrap
// data and restarting the node with the new token), and asserts the restore fails at Preflight with
// the token-hash mismatch. It then rotates back to the original token — the documented way to make
// a pre-rotation snapshot restorable again — and asserts a fresh restore of that same snapshot
// succeeds and rolls etcd back, proving the guard rejected the restore on the token rather than on
// anything else about the snapshot.
//
// Note that the restore is created against the ETCDSnapshot CR name, not the snapshot file name: the
// controller resolves either, but only a CR lookup can read the stamped hash, so restoring by file
// name skips the check entirely.
func Test_Imported_Operation_SetE_ImportedETCDSnapshotRestoreTokenRotation(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-restore-token-rotation", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	token := newServerTokenFixture(cs, fx)
	originalToken := token.configuredToken(t)

	// Payload whose presence after the restore proves etcd was actually rolled back.
	cm := newConfigMapProof()
	cm.create(t, fx)

	// Take the snapshot that the rest of the test restores from. It is stamped with a hash of the
	// token the cluster currently runs with.
	snapshotsValidAfter := time.Now().Add(-30 * time.Second)
	saveOp := RunETCDSnapshotSaveOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	t.Logf("snapshot save operation %s/%s completed", saveOp.Namespace, saveOp.Name)
	waitForSnapshots(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, snapshotsValidAfter, 1)
	snapshot := waitForBackpopulatedSnapshot(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, "imported-init-0", snapshotsValidAfter)

	// Without a stamped hash the preflight check has nothing to compare against and skips validation,
	// which would make the rest of this test silently vacuous.
	stampedHash := snapshot.Annotations[capr.SnapshotTokenHashAnnotation]
	if stampedHash == "" {
		t.Fatalf("snapshot %s carries no %s annotation, so the restore preflight check cannot validate the token: annotations=%v",
			snapshot.Name, capr.SnapshotTokenHashAnnotation, snapshot.Annotations)
	}
	assert.Equal(t, token.tokenHash(t), stampedHash, "the snapshot should be stamped with the hash of the token the cluster was running with")

	// Delete the ConfigMap so the post-restore assertion is meaningful rather than trivially true.
	cm.delete(t, fx)

	rotatedToken, err := randomtoken.Generate()
	require.NoError(t, err)

	token.rotate(t, originalToken, rotatedToken)
	rotatedHash := token.tokenHash(t)
	if rotatedHash == stampedHash {
		t.Fatalf("the node still hashes to %s after rotating its token, so the preflight check cannot detect the rotation", rotatedHash)
	}
	t.Logf("rotated the server token: node hash %s, snapshot hash %s", rotatedHash, stampedHash)

	// The restore must be refused. A longer TTL than the default keeps the operation around while its
	// terminal state is asserted, instead of racing garbage collection.
	refused := CreateETCDSnapshotRestoreOp(t, cs, fx.ns.Name, snapshot.Name, fx.clusterRef, WithRestoreTTL(600))
	refused = WaitForSnapshotRestoreFailed(t, cs, refused)

	assert.Equal(t, opv1alpha1.OperationPhaseFailed, refused.Status.Phase)
	assert.Equal(t, opv1alpha1.ETCDSnapshotRestoreStepPreflight, refused.Status.Step,
		"the restore should be refused before it shuts anything down")
	assert.Equal(t, opv1alpha1.PreflightCheckFailedReason, opv1alpha1.FailedCondition.GetReason(refused))
	assert.Contains(t, opv1alpha1.FailedCondition.GetMessage(refused), "does not match snapshot token hash")

	// Rotating back restores the bootstrap data the snapshot's token can decrypt, which is what makes
	// a pre-rotation snapshot restorable again.
	token.rotate(t, rotatedToken, originalToken)
	assert.Equal(t, stampedHash, token.tokenHash(t), "rotating back should return the node to the token the snapshot was taken with")

	// The same snapshot, by the same CR name, now passes the check and restores. This leg also covers
	// plan reuse across operations: the refused restore left its preflight plan on the machine-plan
	// secret, so unless this operation's plan is distinct from it the node is never asked to hash its
	// token again and this restore is refused on the previous operation's output.
	restoreOp := RunETCDSnapshotRestoreOperationTest(t, cs, fx.ns.Name, snapshot.Name, fx.clusterRef)
	t.Logf("snapshot restore operation %s/%s completed", restoreOp.Namespace, restoreOp.Name)

	cm.assertRestored(t, fx)
}
