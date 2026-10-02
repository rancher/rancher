package etcdsnapshotrestore

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	planapi "github.com/rancher/rancher/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

// When the etcd leader is not a control-plane node, pod cleanup runs a plan on each, and a failure of
// the control-plane node's plan must be reported against that node rather than the etcd leader.
func TestReconcilePostRestorePodCleanup_ControlPlaneFailureNamesControlPlaneNode(t *testing.T) {
	t.Parallel()

	etcd := makePlanSecret("etcd-1", "node-etcd-1", map[string]string{capr.EtcdRoleLabel: "true"})
	etcd.Annotations = map[string]string{}
	controlPlane := makePlanSecret("cp-1", "node-cp-1", map[string]string{capr.ControlPlaneRoleLabel: "true"})
	controlPlane.Annotations = map[string]string{}

	adapter := defaultAdapter()
	adapter.leader = etcd
	s := newTestScope(adapter, "restore-uid")
	secrets := &fakePlanSecrets{items: []*corev1.Secret{etcd, controlPlane}}
	h := &handler{secrets: secrets, store: planapi.NewStore(secrets)}

	reconcile := func() opv1alpha1.ETCDSnapshotRestoreStatus {
		t.Helper()
		got, err := h.reconcilePostRestorePodCleanup(s, opv1alpha1.ETCDSnapshotRestoreStatus{Step: opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup})
		require.NoError(t, err)
		return got
	}
	// latest returns the last version of the named secret written through the fake, and makes it the
	// one served from now on, standing in for the agent's view of the secret.
	latest := func(name string) *corev1.Secret {
		t.Helper()
		for i := len(secrets.updates) - 1; i >= 0; i-- {
			if secrets.updates[i].Name == name {
				for j, item := range secrets.items {
					if item.Name == name {
						secrets.items[j] = secrets.updates[i]
					}
				}
				if adapter.leader.Name == name {
					adapter.leader = secrets.updates[i]
				}
				return secrets.updates[i]
			}
		}
		t.Fatalf("no plan was assigned to %s", name)
		return nil
	}

	// The etcd leader's plan goes first; the agent completes it.
	got := reconcile()
	require.Empty(t, string(got.Phase))
	etcdAssigned := latest("etcd-1")
	etcdAssigned.Data[planapi.PlanStateKey] = []byte(planapi.PlanStateSucceeded)
	etcdAssigned.Data["probe-statuses"] = []byte(`{"x":{"healthy":true}}`)
	etcdAssigned.Annotations[planapi.PlanProbesPassedAnnotation] = "applied"

	// The control-plane node's plan goes next; the agent fails it.
	got = reconcile()
	require.Empty(t, string(got.Phase))
	cpAssigned := latest("cp-1")
	cpAssigned.Data[planapi.PlanStateKey] = []byte(planapi.PlanStateFailed)

	got = reconcile()
	assert.Equal(t, opv1alpha1.OperationPhaseFailed, got.Phase)
	assert.Equal(t, opv1alpha1.PlanFailedReason, opv1alpha1.FailedCondition.GetReason(&got))
	assert.Equal(t, "post-restore pod cleanup failed for fleet-default/cp-1", opv1alpha1.FailedCondition.GetMessage(&got))
}
