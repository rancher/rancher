package imported

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/operations/etcdsnapshotrestore"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shortServerTokenHash mirrors k3s's util.ShortHash(password, 12): the first 12 characters of the
// hex-encoded sha256 of the token *password*, which is the value the distro stamps on every
// snapshot. Reimplemented here (rather than imported) so the test asserts against the documented
// distro behaviour without pulling the k3s module into the test binary.
func shortServerTokenHash(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])[:12]
}

// rawServerTokenFile returns the verbatim contents of the token file the restore preflight check
// reads. k3s writes it as `K10<sha256-of-server-CA>::server:<password>` (see
// clientaccess.FormatTokenBytes), so it is the only place a test can see how much of the file is
// prefix and how much is the password the distro actually hashes.
func (s *serverTokenFixture) rawServerTokenFile(t *testing.T) string {
	t.Helper()

	path := fmt.Sprintf("%s/server/token", s.dataDir)
	out, err := s.exec(t, fmt.Sprintf("cat %s", path))
	if err != nil {
		t.Fatalf("reading %s failed: %v\noutput: %s", path, err, out)
	}
	return strings.TrimSpace(out)
}

// Test_Imported_Operation_SetE_ImportedETCDSnapshotRestoreColonToken verifies that a snapshot taken
// under a server token whose password contains a colon can be restored, i.e. that the restore
// Preflight step derives the same token hash the distro stamped on the snapshot.
//
// The token file on disk is written by k3s as:
//
//	K10<sha256-of-server-CA>::server:<password>
//
// and the hash stamped on a snapshot is sha256(<password>)[:12] — where <password> is everything
// after the *first* colon of the credential portion, because clientaccess.parseToken splits
// username from password with strings.SplitN(token, ":", 2). A password containing a colon
// therefore keeps its colons in the hashed value.
//
// The controller's Preflight check has to reproduce that in shell without the k3s/kubeadm parsers
// (see etcdsnapshotrestore.TokenHashCommandFormat), which is exactly where the two can diverge:
// stripping the prefix greedily (up to the last colon) yields only the trailing segment of such a
// password.
//
// Unlike Test_Imported_Operation_SetE_ImportedETCDSnapshotRestoreTokenRotation, the token is *not*
// changed between the save and the restore — it is rotated once to a colon-bearing value before
// anything is snapshotted and then left alone. So the snapshot is always legitimately restorable,
// and any Preflight rejection here is a defect in the hash derivation rather than a genuine token
// mismatch.
func Test_Imported_Operation_SetE_ImportedETCDSnapshotRestoreColonToken(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-restore-colon-token", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	token := newServerTokenFixture(cs, fx)

	// The cluster is brought up with a colon-free hex token (see cluster.generateImportedToken), so
	// rotate to a colon-bearing one first. Two random halves joined by a colon: neither half can be
	// mistaken for a kubeadm bootstrap token (those require a `.`), so the distro parses this as
	// username-less basic auth and the whole string is the password.
	left, err := randomtoken.Generate()
	require.NoError(t, err)
	right, err := randomtoken.Generate()
	require.NoError(t, err)
	colonToken := left + ":" + right

	token.rotate(t, token.configuredToken(t), colonToken)
	t.Logf("rotated the server token to a colon-bearing value; token file on disk is %q", token.rawServerTokenFile(t))

	// Payload whose presence after the restore proves etcd was actually rolled back.
	cm := newConfigMapProof()
	cm.create(t, fx)

	snapshotsValidAfter := time.Now().Add(-30 * time.Second)
	saveOp := RunETCDSnapshotSaveOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	t.Logf("snapshot save operation %s/%s completed", saveOp.Namespace, saveOp.Name)
	waitForSnapshots(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, snapshotsValidAfter, 1)
	snapshot := waitForBackpopulatedSnapshot(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, "imported-init-0", snapshotsValidAfter)

	// Without a stamped hash Preflight has nothing to compare against and skips validation, which
	// would make the rest of this test silently vacuous.
	stampedHash := snapshot.Annotations[capr.SnapshotTokenHashAnnotation]
	if stampedHash == "" {
		t.Fatalf("snapshot %s carries no %s annotation, so the restore preflight check cannot validate the token: annotations=%v",
			snapshot.Name, capr.SnapshotTokenHashAnnotation, snapshot.Annotations)
	}

	// Pin down what the distro hashed: the full password, colon included. If this fails the premise
	// of the test (and of the Preflight check) is wrong, not the shell command.
	assert.Equal(t, shortServerTokenHash(colonToken), stampedHash,
		"the distro should stamp the snapshot with sha256 of the whole password, colons included")

	// The check the controller actually runs on the node. Reported before the restore is attempted
	// so a mismatch is visible as a hash comparison rather than only as a Preflight failure ~minutes
	// later.
	nodeHash := token.tokenHash(t)
	t.Logf("token hashes: snapshot=%s, sha256(password)[:12]=%s, %q=%s",
		stampedHash, shortServerTokenHash(colonToken),
		fmt.Sprintf(etcdsnapshotrestore.TokenHashCommandFormat, token.dataDir), nodeHash)
	assert.Equal(t, stampedHash, nodeHash,
		"the preflight token hash command must derive the same hash the distro stamped for a password containing a colon")

	// Delete the ConfigMap so the post-restore assertion is meaningful rather than trivially true.
	cm.delete(t, fx)

	// The token has not changed since the snapshot was taken, so this restore must succeed.
	restoreOp := RunETCDSnapshotRestoreOperationTest(t, cs, fx.ns.Name, snapshot.Name, fx.clusterRef)
	t.Logf("snapshot restore operation %s/%s completed", restoreOp.Namespace, restoreOp.Name)

	cm.assertRestored(t, fx)
}
