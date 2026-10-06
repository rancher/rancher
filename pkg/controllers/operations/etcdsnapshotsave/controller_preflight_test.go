package etcdsnapshotsave

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

// preflightOp is a save of the cluster newScope resolves.
func preflightOp() *opv1alpha1.ETCDSnapshotSave {
	op := newOp()
	op.Spec.ClusterRef = &corev1.ObjectReference{APIVersion: "provisioning.cattle.io/v1", Kind: "Cluster", Namespace: "fleet-default", Name: "test"}
	return op
}

// labeledSecret is an etcd node's machine-plan secret carrying the lifecycle labels of op's cluster.
func labeledSecret(op *opv1alpha1.ETCDSnapshotSave) *corev1.Secret {
	secret := newPlanSecret("etcd-1")
	secret.Labels[planv1alpha1.ClusterLifecycleGroupLabel] = "provisioning.cattle.io"
	secret.Labels[planv1alpha1.ClusterLifecycleKindLabel] = op.Spec.ClusterRef.Kind
	secret.Labels[planv1alpha1.ClusterLifecycleNameLabel] = op.Spec.ClusterRef.Name
	secret.Labels[planv1alpha1.MachineLifecycleGroupLabel] = "cluster.x-k8s.io"
	secret.Labels[planv1alpha1.MachineLifecycleKindLabel] = "Machine"
	secret.Labels[planv1alpha1.MachineLifecycleNameLabel] = "machine-1"
	return secret
}

func runPreflight(t *testing.T, op *opv1alpha1.ETCDSnapshotSave, secret *corev1.Secret) opv1alpha1.ETCDSnapshotSaveStatus {
	t.Helper()

	h := &handler{secrets: &fakePlanSecrets{items: []*corev1.Secret{secret}}}
	got, err := h.reconcilePreflight(newScope(op, nil, defaultAdapter()), opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepPreflight})
	require.NoError(t, err)
	return got
}

func TestReconcilePreflight_PassesLabeledSecrets(t *testing.T) {
	t.Parallel()

	op := preflightOp()
	got := runPreflight(t, op, labeledSecret(op))
	assert.Empty(t, string(got.Phase))
	assert.Equal(t, opv1alpha1.ETCDSnapshotSaveStepSave, got.Step)
}

// A machine-plan secret whose lifecycle labels don't tie it to the operation's cluster can't be fenced
// by the webhook, so Preflight turns the request away before anything is assigned.
func TestReconcilePreflight_RejectsMislabeledSecrets(t *testing.T) {
	t.Parallel()

	op := preflightOp()
	secret := labeledSecret(op)
	delete(secret.Labels, planv1alpha1.MachineLifecycleKindLabel)

	got := runPreflight(t, op, secret)
	assert.Equal(t, opv1alpha1.OperationPhaseRejected, got.Phase)
	assert.Equal(t, opv1alpha1.PreflightCheckFailedReason, opv1alpha1.RejectedCondition.GetReason(&got))
	assert.Contains(t, opv1alpha1.RejectedCondition.GetMessage(&got), "fleet-default/etcd-1")
	assert.Contains(t, opv1alpha1.RejectedCondition.GetMessage(&got), planv1alpha1.MachineLifecycleKindLabel)
}
