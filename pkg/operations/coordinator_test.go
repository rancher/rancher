package operations

import (
	"encoding/json"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var testClusterRef = &corev1.ObjectReference{
	APIVersion: "management.cattle.io/v3",
	Kind:       "Cluster",
	Name:       "c-m-abcde",
}

// fakeCRDCache serves a fixed set of CustomResourceDefinitions by name.
type fakeCRDCache struct {
	crds map[string]*apiextv1.CustomResourceDefinition
}

func (f *fakeCRDCache) Get(name string) (*apiextv1.CustomResourceDefinition, error) {
	if crd, ok := f.crds[name]; ok {
		return crd, nil
	}

	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "customresourcedefinitions"}, name)
}

func (f *fakeCRDCache) List(_ labels.Selector) ([]*apiextv1.CustomResourceDefinition, error) {
	crds := make([]*apiextv1.CustomResourceDefinition, 0, len(f.crds))
	for _, crd := range f.crds {
		crds = append(crds, crd)
	}

	return crds, nil
}

func (f *fakeCRDCache) AddIndexer(string, generic.Indexer[*apiextv1.CustomResourceDefinition]) {}

func (f *fakeCRDCache) GetByIndex(string, string) ([]*apiextv1.CustomResourceDefinition, error) {
	return nil, nil
}

// fakeClusterClient records the cluster objects it is asked to update.
type fakeClusterClient struct {
	cluster *unstructured.Unstructured
	updated []*unstructured.Unstructured
}

func (f *fakeClusterClient) Get(_ schema.GroupVersionKind, _, _ string) (runtime.Object, error) {
	return f.cluster, nil
}

func (f *fakeClusterClient) Update(obj runtime.Object) (runtime.Object, error) {
	f.updated = append(f.updated, obj.(*unstructured.Unstructured))
	return obj, nil
}

type patchCall struct {
	kind      string
	namespace string
	name      string
	patch     map[string]any
}

// testCoordinator builds a Coordinator over the given in-flight operations, with the priorities and
// annotations rancher ships today: only a restore may preempt, and only a restore clears the
// restore-required mark.
func testCoordinator(cluster *unstructured.Unstructured, operations ...*opv1alpha1.Operation) (*Coordinator, *[]patchCall, *fakeClusterClient) {
	patches := &[]patchCall{}
	clusters := &fakeClusterClient{cluster: cluster}

	kindOf := func(kind, resource string) operationKind {
		return operationKind{
			kind:     kind,
			resource: resource,
			list: func() ([]*opv1alpha1.Operation, error) {
				var result []*opv1alpha1.Operation
				for _, operation := range operations {
					if operation.Kind == kind {
						result = append(result, operation)
					}
				}

				return result, nil
			},
			patch: func(namespace, name string, data []byte) error {
				decoded := map[string]any{}
				if err := json.Unmarshal(data, &decoded); err != nil {
					return err
				}

				*patches = append(*patches, patchCall{kind: kind, namespace: namespace, name: name, patch: decoded})
				return nil
			},
		}
	}

	crd := func(resource string, annotations map[string]string) *apiextv1.CustomResourceDefinition {
		return &apiextv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:        resource + "." + opv1alpha1.SchemeGroupVersion.Group,
				Annotations: annotations,
			},
		}
	}

	return &Coordinator{
		crds: &fakeCRDCache{crds: map[string]*apiextv1.CustomResourceDefinition{
			"etcdsnapshotsaves.operation.cattle.io": crd("etcdsnapshotsaves", map[string]string{
				opv1alpha1.PreemptionPriorityAnnotation: "0",
			}),
			"etcdsnapshotrestores.operation.cattle.io": crd("etcdsnapshotrestores", map[string]string{
				opv1alpha1.PreemptionPriorityAnnotation:            "100",
				opv1alpha1.RestoreRequiredOnCancellationAnnotation: "true",
				opv1alpha1.ClearsRestoreRequiredAnnotation:         "true",
			}),
			"encryptionkeyrotations.operation.cattle.io": crd("encryptionkeyrotations", map[string]string{
				opv1alpha1.PreemptionPriorityAnnotation:            "0",
				opv1alpha1.RestoreRequiredOnCancellationAnnotation: "true",
			}),
		}},
		dynamic: clusters,
		kinds: []operationKind{
			kindOf("ETCDSnapshotSave", "etcdsnapshotsaves"),
			kindOf("ETCDSnapshotRestore", "etcdsnapshotrestores"),
			kindOf("EncryptionKeyRotation", "encryptionkeyrotations"),
		},
	}, patches, clusters
}

func testOperation(kind, name string, phase opv1alpha1.OperationPhase) *opv1alpha1.Operation {
	return &opv1alpha1.Operation{
		TypeMeta: metav1.TypeMeta{Kind: kind},
		Metadata: metav1.ObjectMeta{Namespace: "fleet-default", Name: name},
		Spec:     opv1alpha1.OperationSpec{ClusterRef: testClusterRef},
		Status:   opv1alpha1.OperationStatus{Phase: phase},
	}
}

func testCluster(annotations map[string]string) *unstructured.Unstructured {
	cluster := &unstructured.Unstructured{}
	cluster.SetAPIVersion(testClusterRef.APIVersion)
	cluster.SetKind(testClusterRef.Kind)
	cluster.SetName(testClusterRef.Name)
	if annotations != nil {
		cluster.SetAnnotations(annotations)
	}

	return cluster
}

func TestPreempt(t *testing.T) {
	t.Parallel()

	t.Run("a restore cancels the operation holding the beacon", func(t *testing.T) {
		t.Parallel()

		rotation := testOperation("EncryptionKeyRotation", "ekr", opv1alpha1.OperationPhaseInProgress)
		coordinator, patches, _ := testCoordinator(testCluster(nil), rotation)

		restore := testOperation("ETCDSnapshotRestore", "restore", opv1alpha1.OperationPhasePending)
		blocking, err := coordinator.Preempt(restore)
		require.NoError(t, err)
		assert.Empty(t, blocking)

		require.Len(t, *patches, 1)
		patch := (*patches)[0]
		assert.Equal(t, "EncryptionKeyRotation", patch.kind)
		assert.Equal(t, "ekr", patch.name)
		assert.Equal(t, map[string]any{"cancel": true}, patch.patch["spec"])
		assert.Equal(t,
			map[string]any{"annotations": map[string]any{opv1alpha1.CanceledByAnnotation: "ETCDSnapshotRestore/fleet-default/restore"}},
			patch.patch["metadata"])
	})

	t.Run("a save cancels nothing and reports what blocks it", func(t *testing.T) {
		t.Parallel()

		rotation := testOperation("EncryptionKeyRotation", "ekr", opv1alpha1.OperationPhaseInProgress)
		coordinator, patches, _ := testCoordinator(testCluster(nil), rotation)

		blocking, err := coordinator.Preempt(testOperation("ETCDSnapshotSave", "save", opv1alpha1.OperationPhasePending))
		require.NoError(t, err)
		assert.Equal(t, []string{"EncryptionKeyRotation/fleet-default/ekr"}, blocking)
		assert.Empty(t, *patches)
	})

	t.Run("a restore may preempt another restore", func(t *testing.T) {
		t.Parallel()

		other := testOperation("ETCDSnapshotRestore", "other", opv1alpha1.OperationPhaseInProgress)
		coordinator, patches, _ := testCoordinator(testCluster(nil), other)

		blocking, err := coordinator.Preempt(testOperation("ETCDSnapshotRestore", "restore", opv1alpha1.OperationPhasePending))
		require.NoError(t, err)
		assert.Empty(t, blocking)
		require.Len(t, *patches, 1)
		assert.Equal(t, "other", (*patches)[0].name)
	})

	t.Run("terminal, unrelated and already cancelling operations are left alone", func(t *testing.T) {
		t.Parallel()

		succeeded := testOperation("EncryptionKeyRotation", "succeeded", opv1alpha1.OperationPhaseSucceeded)

		otherCluster := testOperation("EncryptionKeyRotation", "other-cluster", opv1alpha1.OperationPhaseInProgress)
		otherCluster.Spec.ClusterRef = &corev1.ObjectReference{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Cluster",
			Name:       "c-m-other",
		}

		cancelling := testOperation("ETCDSnapshotSave", "cancelling", opv1alpha1.OperationPhaseInProgress)
		cancelling.Spec.Cancel = true

		coordinator, patches, _ := testCoordinator(testCluster(nil), succeeded, otherCluster, cancelling)

		blocking, err := coordinator.Preempt(testOperation("ETCDSnapshotRestore", "restore", opv1alpha1.OperationPhasePending))
		require.NoError(t, err)
		assert.Empty(t, blocking)
		assert.Empty(t, *patches, "nothing left to cancel")
	})

	t.Run("an operation does not preempt itself", func(t *testing.T) {
		t.Parallel()

		restore := testOperation("ETCDSnapshotRestore", "restore", opv1alpha1.OperationPhasePending)
		coordinator, patches, _ := testCoordinator(testCluster(nil), restore)

		blocking, err := coordinator.Preempt(restore)
		require.NoError(t, err)
		assert.Empty(t, blocking)
		assert.Empty(t, *patches)
	})

	t.Run("a nil coordinator is inert", func(t *testing.T) {
		t.Parallel()

		var coordinator *Coordinator

		blocking, err := coordinator.Preempt(testOperation("ETCDSnapshotSave", "save", opv1alpha1.OperationPhasePending))
		require.NoError(t, err)
		assert.Empty(t, blocking)

		marked, err := coordinator.OnCanceled(testOperation("EncryptionKeyRotation", "ekr", opv1alpha1.OperationPhaseCanceled))
		require.NoError(t, err)
		assert.False(t, marked)

		require.NoError(t, coordinator.OnSucceeded(testOperation("ETCDSnapshotRestore", "restore", opv1alpha1.OperationPhaseSucceeded)))
	})
}

func TestOnCanceled(t *testing.T) {
	t.Parallel()

	t.Run("a rotation marks its cluster as requiring a restore", func(t *testing.T) {
		t.Parallel()

		coordinator, _, clusters := testCoordinator(testCluster(nil))

		marked, err := coordinator.OnCanceled(testOperation("EncryptionKeyRotation", "ekr", opv1alpha1.OperationPhaseCanceled))
		require.NoError(t, err)
		assert.True(t, marked)

		require.Len(t, clusters.updated, 1)
		assert.Equal(t, "EncryptionKeyRotation/fleet-default/ekr",
			clusters.updated[0].GetAnnotations()[opv1alpha1.RestoreRequiredAnnotation])
	})

	t.Run("a save leaves its cluster alone", func(t *testing.T) {
		t.Parallel()

		coordinator, _, clusters := testCoordinator(testCluster(nil))

		marked, err := coordinator.OnCanceled(testOperation("ETCDSnapshotSave", "save", opv1alpha1.OperationPhaseCanceled))
		require.NoError(t, err)
		assert.False(t, marked)
		assert.Empty(t, clusters.updated)
	})

	t.Run("a cluster already marked is not updated again", func(t *testing.T) {
		t.Parallel()

		coordinator, _, clusters := testCoordinator(testCluster(map[string]string{
			opv1alpha1.RestoreRequiredAnnotation: "EncryptionKeyRotation/fleet-default/ekr",
		}))

		marked, err := coordinator.OnCanceled(testOperation("EncryptionKeyRotation", "ekr", opv1alpha1.OperationPhaseCanceled))
		require.NoError(t, err)
		assert.True(t, marked)
		assert.Empty(t, clusters.updated)
	})
}

func TestOnSucceeded(t *testing.T) {
	t.Parallel()

	t.Run("a successful restore clears the mark", func(t *testing.T) {
		t.Parallel()

		coordinator, _, clusters := testCoordinator(testCluster(map[string]string{
			opv1alpha1.RestoreRequiredAnnotation: "EncryptionKeyRotation/fleet-default/ekr",
			"other":                              "keep-me",
		}))

		require.NoError(t, coordinator.OnSucceeded(testOperation("ETCDSnapshotRestore", "restore", opv1alpha1.OperationPhaseSucceeded)))

		require.Len(t, clusters.updated, 1)
		annotations := clusters.updated[0].GetAnnotations()
		assert.NotContains(t, annotations, opv1alpha1.RestoreRequiredAnnotation)
		assert.Equal(t, "keep-me", annotations["other"])
	})

	t.Run("a successful rotation does not clear the mark", func(t *testing.T) {
		t.Parallel()

		coordinator, _, clusters := testCoordinator(testCluster(map[string]string{
			opv1alpha1.RestoreRequiredAnnotation: "EncryptionKeyRotation/fleet-default/ekr",
		}))

		require.NoError(t, coordinator.OnSucceeded(testOperation("EncryptionKeyRotation", "ekr", opv1alpha1.OperationPhaseSucceeded)))
		assert.Empty(t, clusters.updated)
	})

	t.Run("an unmarked cluster is not updated", func(t *testing.T) {
		t.Parallel()

		coordinator, _, clusters := testCoordinator(testCluster(nil))

		require.NoError(t, coordinator.OnSucceeded(testOperation("ETCDSnapshotRestore", "restore", opv1alpha1.OperationPhaseSucceeded)))
		assert.Empty(t, clusters.updated)
	})
}

func TestMessages(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Waiting to acquire the beacon", WaitingForBeaconMessage(nil))
	assert.Equal(t, "Waiting for a, b to finish", WaitingForBeaconMessage([]string{"a", "b"}))

	byUser := &metav1.ObjectMeta{}
	assert.Equal(t, opv1alpha1.CanceledByUserReason, CanceledReason(byUser))
	assert.Equal(t, "Canceled by user request", CanceledMessage(byUser, testClusterRef, false))

	preempted := &metav1.ObjectMeta{Annotations: map[string]string{
		opv1alpha1.CanceledByAnnotation: "ETCDSnapshotRestore/fleet-default/restore",
	}}
	assert.Equal(t, opv1alpha1.PreemptedReason, CanceledReason(preempted))
	assert.Equal(t, "Preempted by ETCDSnapshotRestore/fleet-default/restore", CanceledMessage(preempted, testClusterRef, false))
	assert.Contains(t, CanceledMessage(preempted, testClusterRef, true), "requires a restore")
}
