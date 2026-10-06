package certificaterotation

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// preflightScope is a rotation in the Preflight step on a cluster with one control-plane node, holding
// the beacon.
func preflightScope(adapter *stubAdapter) (*scope, *fakePlanSecrets) {
	cluster := &unstructured.Unstructured{}
	cluster.SetName("test")

	op := newOp()
	secrets := &fakePlanSecrets{items: []corev1.Secret{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cp-1", Namespace: "fleet-default", UID: "cp-1-uid",
			Labels: lifecycleLabels(op, map[string]string{capr.ClusterNameLabel: "test", capr.ControlPlaneRoleLabel: "true"}),
		},
		Type: plan.SecretTypeMachinePlan,
	}}}
	return &scope{
		ownerKey:   testOwnerKey,
		op:         op,
		beacon:     newBeacon(testOwnerKey, true),
		namespace:  "fleet-default",
		clusterObj: cluster,
		adapter:    adapter,
	}, secrets
}

// lifecycleLabels returns labels with the lifecycle labels a machine-plan secret of op's cluster
// carries added to them.
func lifecycleLabels(op *opv1alpha1.CertificateRotation, labels map[string]string) map[string]string {
	labels[planv1alpha1.ClusterLifecycleGroupLabel] = testClusterGVK.Group
	labels[planv1alpha1.ClusterLifecycleKindLabel] = op.Spec.ClusterRef.Kind
	labels[planv1alpha1.ClusterLifecycleNameLabel] = op.Spec.ClusterRef.Name
	labels[planv1alpha1.MachineLifecycleGroupLabel] = "cluster.x-k8s.io"
	labels[planv1alpha1.MachineLifecycleKindLabel] = "Machine"
	labels[planv1alpha1.MachineLifecycleNameLabel] = "machine-1"
	return labels
}

func preflightStatus() opv1alpha1.CertificateRotationStatus {
	status := opv1alpha1.CertificateRotationStatus{}
	status.SetPhase(opv1alpha1.OperationPhaseInProgress)
	status.SetStep(opv1alpha1.CertificateRotationStepPreflight)
	return status
}

// A rotation starts in Preflight, so that nothing on the cluster is changed until its checks pass.
func TestHandlePending_StartsInPreflight(t *testing.T) {
	t.Parallel()

	op := newOp()
	beacon := newBeacon("", false)
	h := &handler{beacons: &fakeBeaconClient{beacon: beacon}}
	s := &scope{ownerKey: testOwnerKey, op: op, beacon: beacon, adapter: &stubAdapter{runtime: capr.RuntimeRKE2}}

	got, err := h.handlePending(s, opv1alpha1.CertificateRotationStatus{})
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, got.Phase)
	assert.Equal(t, opv1alpha1.CertificateRotationStepPreflight, got.Step)
}

// Passing Preflight pauses the cluster, which is the rotation's point of no return, and moves it on to
// Rotate in the same reconcile. Nothing is dispatched yet.
func TestReconcilePreflight_PausesAndMovesToRotate(t *testing.T) {
	t.Parallel()

	adapter := &stubAdapter{runtime: capr.RuntimeRKE2}
	s, secrets := preflightScope(adapter)
	h := &handler{secrets: secrets}

	got, err := h.reconcilePreflight(s, preflightStatus())
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, got.Phase)
	assert.Equal(t, opv1alpha1.CertificateRotationStepRotate, got.Step)
	assert.Equal(t, []bool{true}, adapter.pauseCalls, "the cluster is paused on leaving Preflight")
	assert.Empty(t, secrets.updates, "no plan is assigned in Preflight")
}

// A machine-plan secret whose lifecycle labels don't tie it to the operation's cluster can't be fenced
// by the webhook, so Preflight turns the request away before anything is changed.
func TestReconcilePreflight_RejectsMislabeledSecrets(t *testing.T) {
	t.Parallel()

	for name, mislabel := range map[string]func(map[string]string){
		"another cluster":    func(l map[string]string) { l[planv1alpha1.ClusterLifecycleNameLabel] = "other" },
		"no cluster kind":    func(l map[string]string) { delete(l, planv1alpha1.ClusterLifecycleKindLabel) },
		"empty machine name": func(l map[string]string) { l[planv1alpha1.MachineLifecycleNameLabel] = "" },
		"no machine group":   func(l map[string]string) { delete(l, planv1alpha1.MachineLifecycleGroupLabel) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			adapter := &stubAdapter{runtime: capr.RuntimeRKE2}
			s, secrets := preflightScope(adapter)
			mislabel(secrets.items[0].Labels)
			h := &handler{secrets: secrets}

			got, err := h.reconcilePreflight(s, preflightStatus())
			require.NoError(t, err)
			assert.Equal(t, opv1alpha1.OperationPhaseRejected, got.Phase)
			assert.Equal(t, opv1alpha1.PreflightCheckFailedReason, opv1alpha1.RejectedCondition.GetReason(&got))
			assert.Contains(t, opv1alpha1.RejectedCondition.GetMessage(&got), "fleet-default/cp-1")
			assert.Empty(t, adapter.pauseCalls, "a rejected rotation leaves the cluster unpaused")
		})
	}
}

// A delegate on the Preflight step hook sees the cluster as it was before the rotation: the hook comes
// before the checks and the pause.
func TestReconcilePreflight_HookComesBeforeThePause(t *testing.T) {
	t.Parallel()

	adapter := &stubAdapter{runtime: capr.RuntimeRKE2}
	s, secrets := preflightScope(adapter)
	s.op.Labels = map[string]string{PreflightStepHookLabelPrefix + "inspect": "delegate-a"}
	h := &handler{secrets: secrets, beacons: &fakeBeaconClient{beacon: s.beacon}}

	got, err := h.reconcilePreflight(s, preflightStatus())
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.CertificateRotationStepPreflight, got.Step, "the step waits on the delegate")
	assert.Equal(t, opv1alpha1.WaitingForDelegateReason, opv1alpha1.InProgressCondition.GetReason(&got))
	assert.Empty(t, adapter.pauseCalls, "the delegate sees the cluster before it is paused")
	assert.True(t, plan.IsInDelegateChain(s.beacon, "delegate-a"))
}

// Every step has a hook prefix, which handleInProgress uses to tell a delegation from a lost beacon.
func TestStepHookPrefixFor(t *testing.T) {
	assert.Equal(t, PreflightStepHookLabelPrefix, stepHookPrefixFor(opv1alpha1.CertificateRotationStepPreflight))
	assert.Equal(t, RotateStepHookLabelPrefix, stepHookPrefixFor(opv1alpha1.CertificateRotationStepRotate))
	assert.Empty(t, stepHookPrefixFor(""))
	assert.True(t, ops.HasStepHookLabel(&opv1alpha1.CertificateRotation{ObjectMeta: metav1.ObjectMeta{
		Labels: map[string]string{PreflightStepHookLabelPrefix + "x": "delegate"},
	}}, stepHookPrefixFor(opv1alpha1.CertificateRotationStepPreflight)))
}
