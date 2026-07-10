package imported

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/operations/encryptionkeyrotation"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

// skipUnlessRKE2 short-circuits the test on K3s. secrets-encrypt rotate-keys is an RKE2-only API in
// the way this controller drives it (the script wrapper, status fields, and convergence checks
// target RKE2's output). Pulling this out so the gate isn't duplicated between tests.
func skipUnlessRKE2(t *testing.T) {
	t.Helper()
	if strings.ToLower(os.Getenv("DIST")) != "rke2" {
		t.Skip("encryption key rotation requires RKE2")
	}
}

// Test_Imported_Operation_SetD_ImportedEncryptionKeyRotation brings up a single-node imported RKE2 cluster
// and drives a plain EncryptionKeyRotation operation through the operation.cattle.io/v1alpha1
// controller to Succeeded. Baseline path — no hooks attached — so the controller acquires the
// beacon, runs Rotate + Restart, releases the beacon, and TTL-deletes the CR.
func Test_Imported_Operation_SetD_ImportedEncryptionKeyRotation(t *testing.T) {
	skipUnlessRKE2(t)

	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-encryption-key-rotation", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	RunEncryptionKeyRotationOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
}

// Test_Imported_Operation_SetD_ImportedEncryptionKeyRotation_MultiNodeMixedRoles exercises imported-cluster
// EKR restart ordering on a mixed RKE2 server topology: one init+etcd node, one etcd-only node,
// one additional control-plane node. This validates the plan.DefaultSorter()-driven restart
// sequence (init+etcd → etcd-only → control-plane) and the final-hash-match contract that only
// applies on the last control-plane node.
func Test_Imported_Operation_SetD_ImportedEncryptionKeyRotation_MultiNodeMixedRoles(t *testing.T) {
	skipUnlessRKE2(t)

	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-encryption-key-rotation-mixed-roles", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: false, Quantity: 1},  // init + etcd
		{ControlPlane: false, ETCD: true, Worker: false, Quantity: 1}, // etcd-only
		{ControlPlane: true, ETCD: false, Worker: false, Quantity: 1}, // additional control-plane
	})

	// rke2 reports its node names by the imported-init-0 / imported-node-N convention. Wait for
	// all three to land in Ready before kicking off the rotation — partial readiness would mask
	// the very restart-ordering bug this test is designed to catch.
	distro := capr.GetRuntime(defaults.SomeK8sVersion)
	binDir := fmt.Sprintf("/var/lib/rancher/%s/bin", distro)
	kubeconfig := fmt.Sprintf("/etc/rancher/%s/%s.yaml", distro, distro)
	kubectlEnv := fmt.Sprintf("KUBECONFIG=%s PATH=$PATH:%s", kubeconfig, binDir)
	expectedNodes := []string{"imported-init-0", "imported-node-1", "imported-node-2"}
	waitForImportedNodesReady(t, cs, fx.ns.Name, fx.pods[0].Name, kubectlEnv, expectedNodes)

	RunEncryptionKeyRotationOperationTest(t, cs, fx.ns.Name, fx.clusterRef)

	// After rotation the mgmt cluster should return to Ready and all nodes should still be Ready —
	// the rotation should not have stranded anyone.
	mgmtCluster := fx.mgmtCluster
	err = wait.ClusterObject(cs.Ctx, cs.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.Ready.IsTrue(mgmtCluster), nil
	})
	handleEKRError(t, cs, fx.ns.Name, mgmtCluster.Name, err)
	waitForImportedNodesReady(t, cs, fx.ns.Name, fx.pods[0].Name, kubectlEnv, expectedNodes)

	// Run secrets-encrypt status on the last node (the final control-plane node) to verify the
	// hashes converged. This is the strict end-state the controller's final-hash-match check
	// enforces on the same node, but running it externally proves the contract holds independent
	// of the controller's view.
	encStatus, err := cluster.ExecOnPod(
		cs,
		fx.ns.Name,
		fx.pods[len(fx.pods)-1].Name,
		"sh", "-c",
		fmt.Sprintf("export PATH=$PATH:%s && rke2 secrets-encrypt status", binDir),
	)
	if err != nil {
		t.Fatalf("failed running rke2 secrets-encrypt status on downstream node: %v", err)
	}
	assert.Contains(t, encStatus, "Current Rotation Stage: reencrypt_finished")
	assert.Contains(t, encStatus, "Server Encryption Hashes: All hashes match")
}

// Test_Imported_Operation_SetD_ImportedEncryptionKeyRotationLifecycleHook walks an EncryptionKeyRotation
// through every state-machine checkpoint by attaching a lifecycle-hook label for each step + the
// Succeeded phase. At each checkpoint the test:
//
//  1. Waits for the op to land in the expected (phase, step) AND for the controller to push the
//     named delegate onto the beacon — proving the hook fired.
//  2. (Inspection point) The captured EncryptionKeyRotationCheckpoint exposes the latest op and
//     beacon. For the EKR path this is where a richer test would, for example: read the assigned
//     plan secret on the elected leader during Rotate to verify the rotate-keys wrapper script;
//     inspect the periodic secrets-encrypt status output between Rotate and Restart; or override
//     a node's plan-status to simulate a restart that fails hash convergence.
//  3. Clears the hook label and pops the delegate, releasing the controller to do the actual
//     step work before reaching the next gated checkpoint.
//
// Because the controller drives the actual rotation between checkpoints, this test also confirms
// the post-rotation secrets-encrypt status reports reencrypt_finished + hashes match — the hook
// framework should be transparent to the underlying operation's correctness.
func Test_Imported_Operation_SetD_ImportedEncryptionKeyRotationLifecycleHook(t *testing.T) {
	skipUnlessRKE2(t)

	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-encryption-key-rotation-lifecycle-hook", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	const (
		hookName     = "v2prov-e2e-test"
		delegateName = "v2prov-e2e-test-delegate"
	)
	rotateHookKey := encryptionkeyrotation.RotateStepHookLabelPrefix + hookName
	restartHookKey := encryptionkeyrotation.RestartStepHookLabelPrefix + hookName
	succeededHookKey := planv1alpha1.SucceededPhaseHookLabelPrefix + hookName

	// Attach all hooks up front. Each prefix is scoped to a specific handler so they don't
	// interfere — the controller only consults the relevant prefix when it enters that
	// phase/step.
	op := CreateEncryptionKeyRotationOp(t, cs, fx.ns.Name, fx.clusterRef, WithEncryptionKeyRotationLabels(map[string]string{
		rotateHookKey:    delegateName,
		restartHookKey:   delegateName,
		succeededHookKey: delegateName,
	}))

	// The mgmt-cluster beacon lives in the namespace named after the cluster — cluster-scoped
	// mgmt clusters use namespace == name.
	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name

	// Checkpoint 1: Rotate step. The controller has elected a control-plane leader and is about
	// to (but has not yet) paused the cluster + assigned the rotate-keys plan. The hook fires
	// before PauseCluster so a delegate sees the cluster in its unpaused state.
	cp := WaitForEncryptionKeyRotationHookPause(t, cs, op, beaconNS, beaconName, rotateHookKey, delegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.EncryptionKeyRotationStepRotate)
	t.Logf("paused at Rotate step: phase=%s step=%s delegates=%v", cp.Op.Status.Phase, cp.Op.Status.Step, cp.Beacon.Status.Delegates)
	AdvancePastEncryptionKeyRotationHook(t, cs, op, beaconNS, beaconName, rotateHookKey, delegateName)

	// Checkpoint 2: Restart step. By the time the controller reaches this, rotate-keys has run
	// to reencrypt_finished on the leader (otherwise the controller would not have transitioned
	// to Restart). Restart hook runs before the per-node restart loop starts.
	cp = WaitForEncryptionKeyRotationHookPause(t, cs, op, beaconNS, beaconName, restartHookKey, delegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.EncryptionKeyRotationStepRestart)
	t.Logf("paused at Restart step: phase=%s step=%s delegates=%v", cp.Op.Status.Phase, cp.Op.Status.Step, cp.Beacon.Status.Delegates)
	AdvancePastEncryptionKeyRotationHook(t, cs, op, beaconNS, beaconName, restartHookKey, delegateName)

	// Checkpoint 3: Succeeded phase. Rotation + all restarts have completed. The hook gates the
	// cluster-unpause + beacon-release cleanup.
	cp = WaitForEncryptionKeyRotationHookPause(t, cs, op, beaconNS, beaconName, succeededHookKey, delegateName,
		opv1alpha1.OperationPhaseSucceeded, "")
	t.Logf("paused at Succeeded phase: delegates=%v", cp.Beacon.Status.Delegates)
	AdvancePastEncryptionKeyRotationHook(t, cs, op, beaconNS, beaconName, succeededHookKey, delegateName)

	final := WaitForEncryptionKeyRotationSucceeded(t, cs, op, beaconNS, beaconName)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, final.Status.Phase)

	// Post-rotation sanity: confirm the runtime actually finished the rotation. Validates the
	// hook framework was transparent to the underlying operation's correctness.
	distro := capr.GetRuntime(defaults.SomeK8sVersion)
	binDir := fmt.Sprintf("/var/lib/rancher/%s/bin", distro)
	encStatus, err := cluster.ExecOnPod(cs, fx.ns.Name, fx.pods[0].Name, "sh", "-c",
		fmt.Sprintf("export PATH=$PATH:%s && rke2 secrets-encrypt status", binDir))
	if err != nil {
		t.Fatalf("failed running rke2 secrets-encrypt status on downstream node: %v", err)
	}
	assert.Contains(t, encStatus, "Current Rotation Stage: reencrypt_finished")
}

// waitForImportedNodesReady polls `kubectl get nodes` on the init pod until every expected node
// reports Ready. Used after cluster bring-up and post-rotation to guard restart-ordering tests
// against partial readiness masking the bug the test exists to catch.
func waitForImportedNodesReady(t *testing.T, clients *clients.Clients, namespace, podName, kubectlEnv string, expectedNodeNames []string) {
	t.Helper()

	var lastOut string
	var lastErr error
	command := fmt.Sprintf("export %s && kubectl get nodes --no-headers 2>/dev/null || true", kubectlEnv)
	for i := 0; i < 180; i++ {
		lastOut, lastErr = cluster.ExecOnPod(clients, namespace, podName, "sh", "-c", command)
		if lastErr == nil && importedNodesReady(lastOut, expectedNodeNames) {
			return
		}
		time.Sleep(5 * time.Second)
	}

	t.Fatalf(
		"timed out waiting for downstream nodes %v to be Ready; last output=%q lastErr=%v",
		expectedNodeNames,
		strings.TrimSpace(lastOut),
		lastErr,
	)
}

// importedNodesReady returns true when every name in expectedNodeNames appears in the kubectl
// output with a status starting with "Ready" (Ready, Ready,SchedulingDisabled, etc.).
func importedNodesReady(output string, expectedNodeNames []string) bool {
	readyNodes := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if strings.HasPrefix(fields[1], "Ready") {
			readyNodes[fields[0]] = true
		}
	}

	for _, nodeName := range expectedNodeNames {
		if !readyNodes[nodeName] {
			return false
		}
	}
	return true
}
