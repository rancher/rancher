package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestToOperation(t *testing.T) {
	clusterRef := &corev1.ObjectReference{APIVersion: "management.cattle.io/v3", Kind: "Cluster", Name: "c-abc"}
	spec := OperationSpec{ClusterRef: clusterRef, Cancel: true, TTL: 60}
	status := OperationStatus{Phase: OperationPhaseCanceled, TerminatedAt: metav1.Now()}
	meta := metav1.ObjectMeta{Name: "op-1", Namespace: "c-abc", UID: "uid-1"}

	// Every kind is decoded through the fields it inlines, whatever else it carries. Objects served
	// from an informer have no TypeMeta, so the kind is recovered from the Go type.
	for _, tc := range []struct {
		kind string
		obj  any
	}{
		{"ETCDSnapshotSave", &ETCDSnapshotSave{ObjectMeta: meta,
			Spec:   ETCDSnapshotSaveSpec{OperationSpec: spec, Args: ETCDSnapshotSaveArgs{Name: "snap"}},
			Status: ETCDSnapshotSaveStatus{OperationStatus: status, Step: ETCDSnapshotSaveStepRestart}}},
		{"ETCDSnapshotRestore", &ETCDSnapshotRestore{ObjectMeta: meta,
			Spec:   ETCDSnapshotRestoreSpec{OperationSpec: spec},
			Status: ETCDSnapshotRestoreStatus{OperationStatus: status}}},
		{"EncryptionKeyRotation", &EncryptionKeyRotation{ObjectMeta: meta,
			Spec:   EncryptionKeyRotationSpec{OperationSpec: spec},
			Status: EncryptionKeyRotationStatus{OperationStatus: status}}},
		{"CertificateRotation", CertificateRotation{ObjectMeta: meta,
			Spec:   CertificateRotationSpec{OperationSpec: spec},
			Status: CertificateRotationStatus{OperationStatus: status}}},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			got, err := ToOperation(tc.obj)
			require.NoError(t, err)
			assert.Equal(t, tc.kind, got.Kind)
			assert.Equal(t, "op-1", got.Metadata.Name)
			assert.Equal(t, "c-abc", got.Metadata.Namespace)
			assert.Equal(t, clusterRef, got.Spec.ClusterRef)
			assert.True(t, got.Spec.Cancel)
			assert.Equal(t, int64(60), got.Spec.TTL)
			assert.Equal(t, OperationPhaseCanceled, got.Status.Phase)
			assert.True(t, got.Status.IsTerminated())
		})
	}

	t.Run("keeps a kind the object already carries", func(t *testing.T) {
		got, err := ToOperation(&ETCDSnapshotSave{TypeMeta: metav1.TypeMeta{Kind: "Explicit"}})
		require.NoError(t, err)
		assert.Equal(t, "Explicit", got.Kind)
	})

	t.Run("rejects what does not encode", func(t *testing.T) {
		_, err := ToOperation(func() {})
		assert.Error(t, err)
	})

	t.Run("rejects what does not decode as an operation", func(t *testing.T) {
		_, err := ToOperation(map[string]any{"spec": "not an object"})
		assert.Error(t, err)
	})
}

func TestOperationSameAs(t *testing.T) {
	op := func(kind, namespace, name string) *Operation {
		return &Operation{TypeMeta: metav1.TypeMeta{Kind: kind}, Metadata: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	}

	assert.True(t, op("ETCDSnapshotSave", "ns", "a").SameAs(op("ETCDSnapshotSave", "ns", "a")))
	assert.False(t, op("ETCDSnapshotSave", "ns", "a").SameAs(op("ETCDSnapshotRestore", "ns", "a")), "same name, different kind")
	assert.False(t, op("ETCDSnapshotSave", "ns", "a").SameAs(op("ETCDSnapshotSave", "other", "a")))
	assert.False(t, op("ETCDSnapshotSave", "ns", "a").SameAs(op("ETCDSnapshotSave", "ns", "b")))
	assert.False(t, op("ETCDSnapshotSave", "ns", "a").SameAs(nil))
	assert.True(t, (*Operation)(nil).SameAs(nil))
}

func TestSameCluster(t *testing.T) {
	ref := func(apiVersion, kind, namespace, name string) *corev1.ObjectReference {
		return &corev1.ObjectReference{APIVersion: apiVersion, Kind: kind, Namespace: namespace, Name: name}
	}

	tests := []struct {
		name        string
		left, right *corev1.ObjectReference
		want        bool
	}{
		{"identical", ref("cluster.x-k8s.io/v1beta2", "Cluster", "fleet-default", "c"), ref("cluster.x-k8s.io/v1beta2", "Cluster", "fleet-default", "c"), true},
		{"another version of the same API", ref("cluster.x-k8s.io/v1beta1", "Cluster", "fleet-default", "c"), ref("cluster.x-k8s.io/v1beta2", "Cluster", "fleet-default", "c"), true},
		{"a missing namespace matches any", ref("management.cattle.io/v3", "Cluster", "", "c"), ref("management.cattle.io/v3", "Cluster", "c", "c"), true},
		{"different namespaces", ref("cluster.x-k8s.io/v1beta2", "Cluster", "a", "c"), ref("cluster.x-k8s.io/v1beta2", "Cluster", "b", "c"), false},
		{"different groups", ref("cluster.x-k8s.io/v1beta2", "Cluster", "", "c"), ref("management.cattle.io/v3", "Cluster", "", "c"), false},
		{"different kinds", ref("provisioning.cattle.io/v1", "Cluster", "", "c"), ref("provisioning.cattle.io/v1", "Machine", "", "c"), false},
		{"different names", ref("management.cattle.io/v3", "Cluster", "", "a"), ref("management.cattle.io/v3", "Cluster", "", "b"), false},
		{"an unparsable apiVersion", ref("a/b/c", "Cluster", "", "c"), ref("a/b/c", "Cluster", "", "c"), false},
		{"one missing", ref("management.cattle.io/v3", "Cluster", "", "c"), nil, false},
		{"both missing", nil, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SameCluster(tt.left, tt.right))
			assert.Equal(t, tt.want, SameCluster(tt.right, tt.left), "symmetric")
		})
	}
}

func TestOperationPhaseIsTerminal(t *testing.T) {
	for phase, terminal := range map[OperationPhase]bool{
		OperationPhasePending:    false,
		OperationPhaseInProgress: false,
		OperationPhaseSucceeded:  true,
		OperationPhaseFailed:     true,
		OperationPhaseRejected:   true,
		OperationPhaseCanceled:   true,
		"":                       false,
	} {
		assert.Equal(t, terminal, phase.IsTerminal(), "%q", phase)
	}
}

func TestOperationStatusIsTerminated(t *testing.T) {
	status := &OperationStatus{}
	status.MarkCanceled(CancelRequestedReason, "cancellation requested")
	assert.False(t, status.IsTerminated(), "a terminal phase is not yet terminated")

	status.SetTerminated()
	assert.True(t, status.IsTerminated())
}
