package etcdsnapshotrestore

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

// labeledEtcdSecret is an etcd node's machine-plan secret carrying the lifecycle labels of op's cluster.
func labeledEtcdSecret(op *opv1alpha1.ETCDSnapshotRestore) *corev1.Secret {
	secret := makePlanSecret("etcd-1", "node-etcd-1", map[string]string{capr.EtcdRoleLabel: "true"})
	secret.Labels[planv1alpha1.ClusterLifecycleGroupLabel] = testClusterGVK.Group
	secret.Labels[planv1alpha1.ClusterLifecycleKindLabel] = op.Spec.ClusterRef.Kind
	secret.Labels[planv1alpha1.ClusterLifecycleNameLabel] = op.Spec.ClusterRef.Name
	secret.Labels[planv1alpha1.MachineLifecycleGroupLabel] = "cluster.x-k8s.io"
	secret.Labels[planv1alpha1.MachineLifecycleKindLabel] = "Machine"
	secret.Labels[planv1alpha1.MachineLifecycleNameLabel] = "machine-1"
	return secret
}

func runPreflight(t *testing.T, secret *corev1.Secret) (opv1alpha1.ETCDSnapshotRestoreStatus, *stubAdapter, *fakePlanSecrets) {
	t.Helper()

	op := newOp()
	s := newScope(op, newBeacon(testOwnerKey, true))
	adapter := s.adapter.(*stubAdapter)
	secrets := &fakePlanSecrets{items: []*corev1.Secret{secret}}
	// No snapshot record, so there is no token hash to check and no token-hash plan to assign.
	h := &handler{secrets: secrets, store: planapi.NewStore(secrets), etcdsnapshots: &stubSnapshotClient{notFound: true}}

	status := opv1alpha1.ETCDSnapshotRestoreStatus{}
	status.SetPhase(opv1alpha1.OperationPhaseInProgress)
	status.SetStep(opv1alpha1.ETCDSnapshotRestoreStepPreflight)

	got, err := h.reconcilePreflight(s, status)
	require.NoError(t, err)
	return got, adapter, secrets
}

func TestReconcilePreflight_PassesLabeledSecrets(t *testing.T) {
	t.Parallel()

	got, adapter, _ := runPreflight(t, labeledEtcdSecret(newOp()))
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, got.Phase)
	assert.Equal(t, opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig, got.Step)
	assert.Equal(t, []bool{true}, adapter.pauseCalls, "the cluster is paused on leaving Preflight")
}

// A machine-plan secret whose lifecycle labels don't tie it to the operation's cluster can't be fenced
// by the webhook, so Preflight turns the request away before anything is changed: before the cluster
// is paused, and before the token-hash plans are assigned.
func TestReconcilePreflight_RejectsMislabeledSecrets(t *testing.T) {
	t.Parallel()

	secret := labeledEtcdSecret(newOp())
	secret.Labels[planv1alpha1.ClusterLifecycleKindLabel] = "Other"

	got, adapter, secrets := runPreflight(t, secret)
	assert.Equal(t, opv1alpha1.OperationPhaseRejected, got.Phase)
	assert.Equal(t, opv1alpha1.PreflightCheckFailedReason, opv1alpha1.RejectedCondition.GetReason(&got))
	assert.Contains(t, opv1alpha1.RejectedCondition.GetMessage(&got), "fleet-default/etcd-1")
	assert.Empty(t, adapter.pauseCalls)
	assert.Empty(t, secrets.updates, "nothing is assigned")
}
