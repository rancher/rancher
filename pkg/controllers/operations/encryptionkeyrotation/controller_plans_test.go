package encryptionkeyrotation

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/rancher/rancher/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The Store only runs a plan afresh when its content or its writer changes, and every step of one
// operation is written by the same writer. A step whose plan on a node matched the plan the step
// before it left there would be reported on that plan's outcome instead of being run. The leader is
// the one node both steps assign a plan to, so this pins that its rotate-keys plan and its restart
// plan differ, using the plans the real step reconcilers assign.
func TestConsecutiveStepsAssignDistinctPlans(t *testing.T) {
	t.Parallel()

	leader := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cp-1",
			Namespace: "fleet-default",
			Labels: map[string]string{
				capr.ClusterNameLabel:      "test",
				capr.ControlPlaneRoleLabel: "true",
				capr.EtcdRoleLabel:         "true",
			},
		},
		Type: plan.SecretTypeMachinePlan,
	}
	op := newOp()
	s := newScope(op, nil, &stubAdapter{leader: leader})

	// Rotate assigns the rotate-keys plan to the leader.
	secrets := &fakePlanSecrets{items: []*corev1.Secret{leader}}
	h := &handler{secrets: secrets, store: plan.NewStore(secrets)}
	got, err := h.reconcileRotate(s, opv1alpha1.EncryptionKeyRotationStatus{Step: opv1alpha1.EncryptionKeyRotationStepRotate})
	require.NoError(t, err)
	assert.Empty(t, string(got.Phase))
	require.Len(t, secrets.updates, 1, "rotate must assign the leader a plan")
	rotate := secrets.updates[0]
	assert.Equal(t, ops.BeaconOwnerKey(OperationKind, op), rotate.Annotations[plan.PlanWriterAnnotation])

	// The agent completes it, and Restart assigns the leader its restart plan.
	rotate.Data[plan.PlanStateKey] = []byte(plan.PlanStateSucceeded)
	rotate.Data["probe-statuses"] = []byte(`{"x":{"healthy":true}}`)
	rotate.Annotations[plan.PlanProbesPassedAnnotation] = "applied"

	secrets = &fakePlanSecrets{items: []*corev1.Secret{rotate}}
	h = &handler{secrets: secrets, store: plan.NewStore(secrets)}
	_, done, err := h.reconcileRestartNode(s, opv1alpha1.EncryptionKeyRotationStatus{Step: opv1alpha1.EncryptionKeyRotationStepRestart},
		rotate, "rke2-server", "rke2", false)
	require.NoError(t, err)
	assert.False(t, done, "the restart plan has only just been assigned")
	require.Len(t, secrets.updates, 1, "restart must assign the leader a plan of its own")
	restart := secrets.updates[0]

	assert.NotEqual(t, rotate.Data[plan.PlanDataKey], restart.Data[plan.PlanDataKey],
		"the restart step assigns the leader the same plan as the rotate step")
	assert.Equal(t, string(plan.PlanStatePending), string(restart.Data[plan.PlanStateKey]))
}
