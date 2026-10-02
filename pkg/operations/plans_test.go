package operations

import (
	"errors"
	"testing"

	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// fakeSecrets serves a fixed set of machine-plan secrets to the collector and records every write,
// which is how these tests see which plans were canceled. Every other method of the generated
// client panics, so an unexpected call is loud.
type fakeSecrets struct {
	corecontrollers.SecretClient

	items     []corev1.Secret
	listErr   error
	updates   []*corev1.Secret
	updateErr error
}

func (f *fakeSecrets) List(_ string, _ metav1.ListOptions) (*corev1.SecretList, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return &corev1.SecretList{Items: f.items}, nil
}

func (f *fakeSecrets) Update(secret *corev1.Secret) (*corev1.Secret, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.updates = append(f.updates, secret.DeepCopy())
	return secret, nil
}

func testOp(uid string) metav1.Object {
	return &metav1.ObjectMeta{Namespace: "fleet-default", Name: "op-1", UID: types.UID(uid)}
}

func testOwnerKey(uid string) string {
	return BeaconOwnerKey("CertificateRotation", testOp(uid))
}

func testCluster() *unstructured.Unstructured {
	cluster := &unstructured.Unstructured{}
	cluster.SetName("test-cluster")
	cluster.SetNamespace("fleet-default")
	return cluster
}

func heldBeacon(owner string, delegates ...string) *planv1alpha1.Beacon {
	return &planv1alpha1.Beacon{Status: planv1alpha1.BeaconStatus{Owner: owner, Active: true, Delegates: delegates}}
}

// planSecret builds a machine-plan secret holding a plan the given writer assigned, in the given
// plan-state. An empty writer leaves the plan unclaimed, standing in for one CAPR assigned.
func planSecret(name, writer string, state planapi.PlanState) corev1.Secret {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "fleet-default", UID: types.UID(name), Annotations: map[string]string{}},
		Data: map[string][]byte{
			planapi.PlanDataKey:  []byte(`{"instructions":[{"name":"rotate","command":"rke2"}]}`),
			planapi.PlanStateKey: []byte(state),
		},
	}
	if writer != "" {
		secret.Annotations[planapi.PlanWriterAnnotation] = writer
	}
	return secret
}

func TestPlanDispatchedBy(t *testing.T) {
	ownerKey := testOwnerKey("op-uid")

	t.Run("claims a plan this operation wrote", func(t *testing.T) {
		secret := planSecret("node-a", ownerKey, planapi.PlanStatePending)
		assert.True(t, PlanDispatchedBy(&secret, ownerKey))
	})

	// The whole point of matching on the writer: a secret another operation has taken over must be
	// left alone.
	t.Run("does not claim a plan another operation wrote", func(t *testing.T) {
		secret := planSecret("node-a", testOwnerKey("other-uid"), planapi.PlanStatePending)
		assert.False(t, PlanDispatchedBy(&secret, ownerKey))
	})

	// The UID is the last segment of the key, so a UID which is a prefix of another cannot match.
	t.Run("does not claim a plan whose writer's UID merely starts with this one", func(t *testing.T) {
		secret := planSecret("node-a", testOwnerKey("op-uid-2"), planapi.PlanStatePending)
		assert.False(t, PlanDispatchedBy(&secret, ownerKey))
	})

	t.Run("does not claim a plan with no writer", func(t *testing.T) {
		secret := planSecret("node-a", "", planapi.PlanStatePending)
		assert.False(t, PlanDispatchedBy(&secret, ownerKey))
	})

	t.Run("claims nothing when there is nothing to read", func(t *testing.T) {
		assert.False(t, PlanDispatchedBy(nil, ownerKey))
		assert.False(t, PlanDispatchedBy(&corev1.Secret{}, ownerKey))
		noPlan := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{planapi.PlanWriterAnnotation: ownerKey}}}
		assert.False(t, PlanDispatchedBy(noPlan, ownerKey), "a writer with no plan has nothing to claim")
		owned := planSecret("node-a", ownerKey, planapi.PlanStatePending)
		assert.False(t, PlanDispatchedBy(&owned, ""), "no operation, nothing to claim it")
	})
}

func TestCancelDispatchedPlans(t *testing.T) {
	op := testOp("op-uid")
	ownerKey := testOwnerKey("op-uid")

	cancel := func(secrets *fakeSecrets, beacon *planv1alpha1.Beacon) (int, error) {
		return CancelDispatchedPlans(planapi.NewStore(secrets), secrets, testCluster(), "fleet-default", op, ownerKey, beacon)
	}
	canceledNames := func(secrets *fakeSecrets) []string {
		var names []string
		for _, updated := range secrets.updates {
			assert.Equal(t, "true", updated.Annotations[planapi.PlanCanceledAnnotation])
			assert.Equal(t, ownerKey, updated.Annotations[planapi.PlanWriterAnnotation], "the cancellation is written on the operation's behalf")
			names = append(names, updated.Name)
		}
		return names
	}

	t.Run("cancels only the plans this operation dispatched", func(t *testing.T) {
		secrets := &fakeSecrets{items: []corev1.Secret{
			planSecret("node-a", ownerKey, planapi.PlanStateInProgress),
			planSecret("node-b", testOwnerKey("other-uid"), planapi.PlanStateInProgress),
			planSecret("node-c", ownerKey, planapi.PlanStatePending),
			planSecret("node-d", "", planapi.PlanStateInProgress),
		}}

		canceled, err := cancel(secrets, heldBeacon(ownerKey))
		require.NoError(t, err)
		assert.Equal(t, 2, canceled)
		assert.ElementsMatch(t, []string{"node-a", "node-c"}, canceledNames(secrets))
	})

	// A plan that has finished has nothing left to stop; canceling a succeeded one would only stop
	// its periodic instructions and misreport work that completed.
	t.Run("cancels only the plans the agent may still act on", func(t *testing.T) {
		secrets := &fakeSecrets{items: []corev1.Secret{
			planSecret("pending", ownerKey, planapi.PlanStatePending),
			planSecret("in-progress", ownerKey, planapi.PlanStateInProgress),
			planSecret("paused", ownerKey, planapi.PlanStatePaused),
			planSecret("succeeded", ownerKey, planapi.PlanStateSucceeded),
			planSecret("failed", ownerKey, planapi.PlanStateFailed),
			planSecret("canceled", ownerKey, planapi.PlanStateCanceled),
		}}

		canceled, err := cancel(secrets, heldBeacon(ownerKey))
		require.NoError(t, err)
		assert.Equal(t, 3, canceled)
		assert.ElementsMatch(t, []string{"pending", "in-progress", "paused"}, canceledNames(secrets))
	})

	// Asking the agent to stop is safe whatever state the plan is in, so bookkeeping that cannot be
	// read is not taken as a sign that there is nothing to stop.
	t.Run("cancels a plan whose status cannot be read", func(t *testing.T) {
		secret := planSecret("node-a", ownerKey, planapi.PlanStateSucceeded)
		secret.Annotations[planapi.PlanProbesPassedAnnotation] = "yes"
		secret.Data["probe-statuses"] = []byte("{")
		secrets := &fakeSecrets{items: []corev1.Secret{secret}}

		canceled, err := cancel(secrets, heldBeacon(ownerKey))
		require.NoError(t, err)
		assert.Equal(t, 1, canceled)
	})

	// The terminal phase is reconciled repeatedly until the operation is collected, so a second
	// pass over already-canceled plans must not keep writing.
	t.Run("a second pass writes nothing", func(t *testing.T) {
		secret := planSecret("node-a", ownerKey, planapi.PlanStateInProgress)
		secret.Annotations[planapi.PlanCanceledAnnotation] = "true"
		secrets := &fakeSecrets{items: []corev1.Secret{secret}}

		canceled, err := cancel(secrets, heldBeacon(ownerKey))
		require.NoError(t, err)
		assert.Zero(t, canceled)
		assert.Empty(t, secrets.updates)
	})

	// The owner and the delegate it last handed the beacon to are both authorized to write.
	for name, beacon := range map[string]*planv1alpha1.Beacon{
		"as the owner with a delegate":   heldBeacon(ownerKey, "delegate"),
		"as the delegate last handed it": heldBeacon("someone-else", ownerKey),
	} {
		t.Run("cancels "+name, func(t *testing.T) {
			secrets := &fakeSecrets{items: []corev1.Secret{planSecret("node-a", ownerKey, planapi.PlanStateInProgress)}}

			canceled, err := cancel(secrets, beacon)
			require.NoError(t, err)
			assert.Equal(t, 1, canceled)
		})
	}

	// Known not to hold the beacon: writing would reach into the work of whoever does now, so
	// nothing is written. There is nothing to retry either, so the operation is free to finish.
	for name, beacon := range map[string]*planv1alpha1.Beacon{
		"held by another operation": heldBeacon(testOwnerKey("other-uid")),
		"released":                  heldBeacon(""),
		"gone":                      nil,
	} {
		t.Run("writes nothing when the beacon is "+name, func(t *testing.T) {
			secrets := &fakeSecrets{items: []corev1.Secret{planSecret("node-a", ownerKey, planapi.PlanStateInProgress)}}

			canceled, err := cancel(secrets, beacon)
			require.NoError(t, err)
			assert.Zero(t, canceled)
			assert.Empty(t, secrets.updates)
		})
	}

	// Still on the beacon, but part-way down the delegate chain, having handed it on: the
	// cancellation is owed, but not the operation's to write yet, so the caller has to come back.
	t.Run("retries while it has handed the beacon on", func(t *testing.T) {
		secrets := &fakeSecrets{items: []corev1.Secret{planSecret("node-a", ownerKey, planapi.PlanStateInProgress)}}

		_, err := cancel(secrets, heldBeacon("someone-else", ownerKey, "delegate"))
		require.Error(t, err)
		assert.Empty(t, secrets.updates)
	})

	// A failure to reach the secrets must not let the operation release the beacon: the caller
	// returns the error and the next reconcile tries again.
	t.Run("propagates a listing failure", func(t *testing.T) {
		_, err := cancel(&fakeSecrets{listErr: errors.New("boom")}, heldBeacon(ownerKey))
		require.Error(t, err)
	})

	t.Run("propagates a write failure", func(t *testing.T) {
		secrets := &fakeSecrets{
			items:     []corev1.Secret{planSecret("node-a", ownerKey, planapi.PlanStateInProgress)},
			updateErr: errors.New("boom"),
		}

		_, err := cancel(secrets, heldBeacon(ownerKey))
		require.Error(t, err)
	})

	// An operation whose cluster is gone has no agent left running anything either, so there is
	// nothing to cancel and nothing to fail over.
	t.Run("does nothing without a cluster", func(t *testing.T) {
		secrets := &fakeSecrets{items: []corev1.Secret{planSecret("node-a", ownerKey, planapi.PlanStateInProgress)}}

		canceled, err := CancelDispatchedPlans(planapi.NewStore(secrets), secrets, nil, "fleet-default", op, ownerKey, heldBeacon(ownerKey))
		require.NoError(t, err)
		assert.Zero(t, canceled)
		assert.Empty(t, secrets.updates)
	})
}

func TestPlanFailureMessage(t *testing.T) {
	const message = "restart failed for ns/node"

	assert.Equal(t, message, PlanFailureMessage(nil, message))
	assert.Equal(t, message, PlanFailureMessage(&planapi.PlanStatus{State: planapi.PlanStateFailed}, message))
	assert.Equal(t, message+": the in-progress plan was canceled externally",
		PlanFailureMessage(&planapi.PlanStatus{State: planapi.PlanStateCanceled}, message))
	assert.Equal(t, message+`: the agent reported plan-state "from-the-future", which is not recognized`,
		PlanFailureMessage(&planapi.PlanStatus{State: "from-the-future"}, message))
}

// Helper function to build a mock secret pointer
func mockSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func TestPlansMessage(t *testing.T) {
	var (
		pending    = planapi.PlanStatus{State: planapi.PlanStatePending}
		inProgress = planapi.PlanStatus{State: planapi.PlanStateInProgress}
		paused     = planapi.PlanStatus{State: planapi.PlanStatePaused}
		probes     = planapi.PlanStatus{State: planapi.PlanStateSucceeded}
		retrying   = planapi.PlanStatus{State: planapi.PlanStateInProgress, Retrying: true}
		cooldown   = planapi.PlanStatus{State: planapi.PlanStateFailed, Retrying: true}
		succeeded  = planapi.PlanStatus{State: planapi.PlanStateSucceeded, ProbesPassed: true}
		failed     = planapi.PlanStatus{State: planapi.PlanStateFailed}
		canceled   = planapi.PlanStatus{State: planapi.PlanStateCanceled}
	)
	on := func(status planapi.PlanStatus, name string) planapi.PlanStatus {
		status.Secret = mockSecret(name)
		return status
	}

	tests := []struct {
		name     string
		results  []planapi.PlanStatus
		expected string
	}{
		{name: "Empty results slice", results: []planapi.PlanStatus{}, expected: ""},
		{name: "Nil secret items are skipped safely", results: []planapi.PlanStatus{pending}, expected: ""},
		{
			name:     "Single node waiting for plan applied",
			results:  []planapi.PlanStatus{on(inProgress, "node-alpha")},
			expected: "waiting for plan applied for node-alpha",
		},
		{
			name: "Two nodes waiting for plan picked up (Verifies exact suffix '1 other node')",
			// Out of order to test lexicographical sorting picks node-alpha as primary
			results:  []planapi.PlanStatus{on(pending, "node-beta"), on(pending, "node-alpha")},
			expected: "waiting for plan to be picked up for node-alpha & 1 other node",
		},
		{
			name:     "Four nodes waiting for probes (Verifies plural scaling suffix)",
			results:  []planapi.PlanStatus{on(probes, "node-d"), on(probes, "node-b"), on(probes, "node-a"), on(probes, "node-c")},
			expected: "waiting for probes for node-a & 3 other nodes",
		},
		{
			name:     "Plans that are done with are excluded",
			results:  []planapi.PlanStatus{on(succeeded, "node-good"), on(failed, "node-dead"), on(canceled, "node-stopped")},
			expected: "",
		},
		{
			name: "Mixed messages with priority ordering",
			results: []planapi.PlanStatus{
				on(probes, "node-probes"),
				on(paused, "node-paused"),
				on(retrying, "node-failing"),
				on(inProgress, "node-progress"),
				on(pending, "node-pending"),
			},
			expected: "failing plan for node-failing, waiting for plan to be picked up for node-pending, waiting for plan applied for node-progress, plan paused for node-paused, waiting for probes for node-probes",
		},
		{
			name:     "A failed plan with a retry to come is failing",
			results:  []planapi.PlanStatus{on(cooldown, "node-a"), on(retrying, "node-b")},
			expected: "failing plan for node-a & 1 other node",
		},
		{
			name: "Mixed messages with duplicate nodes per tier",
			results: []planapi.PlanStatus{
				on(pending, "node-p1"), on(pending, "node-p2"),
				on(retrying, "node-f1"), on(retrying, "node-f2"), on(retrying, "node-f3"),
			},
			expected: "failing plan for node-f1 & 2 other nodes, waiting for plan to be picked up for node-p1 & 1 other node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, PlansMessage(tt.results))
		})
	}

	t.Run("uses the machine name", func(t *testing.T) {
		s := mockSecret("secret-name")
		s.Labels = map[string]string{planv1alpha1.MachineLifecycleNameLabel: "machine-name"}
		inProgress.Secret = s
		assert.Equal(t, "waiting for plan applied for machine-name", PlansMessage([]planapi.PlanStatus{inProgress}))
	})
}
