package operations

import (
	"encoding/json"
	"errors"
	"testing"

	planapi "github.com/rancher/rancher/pkg/plan"
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

func testCluster() *unstructured.Unstructured {
	cluster := &unstructured.Unstructured{}
	cluster.SetName("test-cluster")
	cluster.SetNamespace("fleet-default")
	return cluster
}

// planSecret builds a machine-plan secret carrying a plan whose instructions were stamped with the
// operation environment for opUID, the way AssignPlan does for every plan an operation dispatches.
// An empty opUID leaves the plan unstamped, standing in for a plan nobody claims.
func planSecret(t *testing.T, name, opUID string) corev1.Secret {
	t.Helper()

	p := &planapi.Plan{
		OneTimeInstructions: []planapi.OneTimeInstruction{{Name: "rotate", Command: "rke2"}},
	}
	if opUID != "" {
		WithOperationEnv(p, OperationEnv("certificate-rotation", testOp(opUID), "Rotate"))
	}

	data, err := json.Marshal(p)
	require.NoError(t, err)

	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "fleet-default", UID: types.UID(name)},
		Data:       map[string][]byte{planapi.PlanDataKey: data},
	}
}

func TestPlanDispatchedBy(t *testing.T) {
	op := testOp("op-uid")

	t.Run("claims a plan stamped with this operation", func(t *testing.T) {
		secret := planSecret(t, "node-a", "op-uid")
		assert.True(t, PlanDispatchedBy(&secret, op))
	})

	// The whole point of matching on the UID: a secret another operation has taken over must be
	// left alone, since that is what makes canceling safe without holding the beacon.
	t.Run("does not claim a plan another operation dispatched", func(t *testing.T) {
		secret := planSecret(t, "node-a", "other-uid")
		assert.False(t, PlanDispatchedBy(&secret, op))
	})

	t.Run("does not claim an unstamped plan", func(t *testing.T) {
		secret := planSecret(t, "node-a", "")
		assert.False(t, PlanDispatchedBy(&secret, op))
	})

	// A UID which is a prefix of another must not match: the env entry ends at the UID, so the
	// comparison is anchored there rather than being a substring search.
	t.Run("does not claim a plan whose UID merely starts with this one", func(t *testing.T) {
		secret := planSecret(t, "node-a", "op-uid-2")
		assert.False(t, PlanDispatchedBy(&secret, op))
	})

	t.Run("finds the stamp on a periodic instruction", func(t *testing.T) {
		p := &planapi.Plan{PeriodicInstructions: []planapi.PeriodicInstruction{{Name: "watch"}}}
		WithOperationEnv(p, OperationEnv("etcd-snapshot-save", op, "Save"))
		data, err := json.Marshal(p)
		require.NoError(t, err)

		secret := &corev1.Secret{Data: map[string][]byte{planapi.PlanDataKey: data}}
		assert.True(t, PlanDispatchedBy(secret, op))
	})

	t.Run("claims nothing when there is nothing to read", func(t *testing.T) {
		assert.False(t, PlanDispatchedBy(nil, op))
		assert.False(t, PlanDispatchedBy(&corev1.Secret{}, op))
		assert.False(t, PlanDispatchedBy(&corev1.Secret{Data: map[string][]byte{planapi.PlanDataKey: []byte("{{{")}}, op))
		owned := planSecret(t, "node-a", "op-uid")
		assert.False(t, PlanDispatchedBy(&owned, nil), "no operation, nothing to claim it")
	})
}

func TestCancelDispatchedPlans(t *testing.T) {
	op := testOp("op-uid")

	t.Run("cancels only the plans this operation dispatched", func(t *testing.T) {
		secrets := &fakeSecrets{items: []corev1.Secret{
			planSecret(t, "node-a", "op-uid"),
			planSecret(t, "node-b", "other-uid"),
			planSecret(t, "node-c", "op-uid"),
			planSecret(t, "node-d", ""),
		}}

		canceled, err := CancelDispatchedPlans(planapi.NewStore(secrets), secrets, testCluster(), "fleet-default", op)
		require.NoError(t, err)
		assert.Equal(t, 2, canceled)

		var names []string
		for _, updated := range secrets.updates {
			assert.Equal(t, "true", updated.Annotations[planapi.PlanCanceledAnnotation])
			names = append(names, updated.Name)
		}
		assert.ElementsMatch(t, []string{"node-a", "node-c"}, names)
	})

	// The Canceled phase is reconciled repeatedly until the operation is collected, so a second
	// pass over already-canceled plans must not keep writing.
	t.Run("a second pass writes nothing", func(t *testing.T) {
		secret := planSecret(t, "node-a", "op-uid")
		secret.Annotations = map[string]string{planapi.PlanCanceledAnnotation: "true"}
		secrets := &fakeSecrets{items: []corev1.Secret{secret}}

		canceled, err := CancelDispatchedPlans(planapi.NewStore(secrets), secrets, testCluster(), "fleet-default", op)
		require.NoError(t, err)
		assert.Zero(t, canceled)
		assert.Empty(t, secrets.updates)
	})

	// A failure to reach the secrets must not let the operation release the beacon: the caller
	// returns the error and the next reconcile tries again.
	t.Run("propagates a listing failure", func(t *testing.T) {
		secrets := &fakeSecrets{listErr: errors.New("boom")}

		_, err := CancelDispatchedPlans(planapi.NewStore(secrets), secrets, testCluster(), "fleet-default", op)
		require.Error(t, err)
	})

	t.Run("propagates a write failure", func(t *testing.T) {
		secrets := &fakeSecrets{
			items:     []corev1.Secret{planSecret(t, "node-a", "op-uid")},
			updateErr: errors.New("boom"),
		}

		_, err := CancelDispatchedPlans(planapi.NewStore(secrets), secrets, testCluster(), "fleet-default", op)
		require.Error(t, err)
	})

	// An operation whose cluster is gone has no agent left running anything either, so there is
	// nothing to cancel and nothing to fail over.
	t.Run("does nothing without a cluster", func(t *testing.T) {
		secrets := &fakeSecrets{items: []corev1.Secret{planSecret(t, "node-a", "op-uid")}}

		canceled, err := CancelDispatchedPlans(planapi.NewStore(secrets), secrets, nil, "fleet-default", op)
		require.NoError(t, err)
		assert.Zero(t, canceled)
		assert.Empty(t, secrets.updates)
	})
}
