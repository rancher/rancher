package etcdsnapshotrestore

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

// The restore unpauses the cluster for its restart, since the cluster has to reconcile the restarted
// nodes, but keeps the whitelist until it has succeeded: a restore stopped part-way through its
// restart still leaves a cluster only another restore can repair. The whitelist is lifted only at the
// very end, by the restore succeeding.
func TestReconcileRestartCluster_KeepsTheWhitelistUntilSuccess(t *testing.T) {
	t.Parallel()

	initSecret := makePlanSecret("init", "node-init", map[string]string{
		capr.EtcdRoleLabel: "true", capr.ControlPlaneRoleLabel: "true", capr.InitNodeLabel: "true",
	})
	initSecret.Annotations = map[string]string{}
	adapter := defaultAdapter()
	adapter.leader = initSecret
	s := newTestScope(adapter, "restore-uid")
	s.ownerKey = "restore-owner"
	secrets := &fakePlanSecrets{items: []*corev1.Secret{initSecret}}

	// reconcile runs the pass once, then has the agent complete the plan it assigned and runs it again,
	// returning the status the second run left.
	reconcile := func(step, nextStep opv1alpha1.ETCDSnapshotRestoreStep) opv1alpha1.ETCDSnapshotRestoreStatus {
		t.Helper()

		h := &handler{secrets: secrets, store: planapi.NewStore(secrets)}
		status := opv1alpha1.ETCDSnapshotRestoreStatus{Step: step}
		got, err := h.reconcileRestartCluster(s, status, nextStep)
		require.NoError(t, err)
		require.NotEmpty(t, secrets.updates, "the pass assigns a restart plan")

		written := secrets.updates[len(secrets.updates)-1]
		written.Data[planapi.PlanStateKey] = []byte(planapi.PlanStateSucceeded)
		written.Data["probe-statuses"] = []byte(`{"x":{"healthy":true}}`)
		written.Annotations[planapi.PlanProbesPassedAnnotation] = "applied"
		secrets.items = []*corev1.Secret{written}
		adapter.leader = written

		got, err = h.reconcileRestartCluster(s, got, nextStep)
		require.NoError(t, err)
		return got
	}

	got := reconcile(opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster, opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup)
	assert.Equal(t, opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup, got.Step)
	assert.NotContains(t, adapter.pauseCalls, true)
	assert.NotContains(t, adapter.whitelistCalls, ops.WhitelistCleared, "the initial restart keeps the whitelist")

	got = reconcile(opv1alpha1.ETCDSnapshotRestoreStepRestartCluster, "")
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, got.Phase)
	require.NotEmpty(t, adapter.whitelistCalls)
	assert.Equal(t, ops.WhitelistCleared, adapter.whitelistCalls[len(adapter.whitelistCalls)-1],
		"the whitelist is lifted by the restore succeeding")
	for _, change := range adapter.whitelistCalls[:len(adapter.whitelistCalls)-1] {
		assert.Equal(t, ops.WhitelistUnchanged, change, "every earlier unpause keeps the whitelist")
	}
	assert.NotContains(t, adapter.pauseCalls, true)
}
