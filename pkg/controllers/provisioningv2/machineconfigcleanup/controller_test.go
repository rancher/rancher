package machineconfigcleanup

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func legacyMeta(name, namespace string, annotations map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: namespace, Annotations: annotations}
}

func legacyObjects(namespace string, annotations map[string]string) []runtime.Object {
	return []runtime.Object{
		&batchv1.CronJob{ObjectMeta: legacyMeta(prefix+"-cronjob", namespace, annotations)},
		&corev1.ConfigMap{ObjectMeta: legacyMeta(prefix+"-script", namespace, annotations)},
		&corev1.ServiceAccount{ObjectMeta: legacyMeta(prefix+"-sa", namespace, annotations)},
	}
}

func deleteActions(client *fake.Clientset) []string {
	var deleted []string
	for _, action := range client.Actions() {
		if deleteAction, ok := action.(k8stesting.DeleteAction); ok {
			deleted = append(deleted, deleteAction.GetResource().Resource+"/"+deleteAction.GetNamespace()+"/"+deleteAction.GetName())
		}
	}
	return deleted
}

func TestRemoveLegacyObjects(t *testing.T) {
	owned := map[string]string{apply.LabelID: setID}
	allLegacy := []string{
		"cronjobs/fleet-default/" + prefix + "-cronjob",
		"configmaps/fleet-default/" + prefix + "-script",
		"serviceaccounts/fleet-default/" + prefix + "-sa",
	}

	tests := []struct {
		name        string
		objects     []runtime.Object
		wantDeleted []string
	}{
		{
			name: "no legacy objects",
		},
		{
			name:        "all legacy objects owned by the object set are deleted, cronjob first",
			objects:     legacyObjects(legacyNamespace, owned),
			wantDeleted: allLegacy,
		},
		{
			name:    "objects without the object set annotation are kept",
			objects: legacyObjects(legacyNamespace, nil),
		},
		{
			name:    "objects from a different object set are kept",
			objects: legacyObjects(legacyNamespace, map[string]string{apply.LabelID: "something-else"}),
		},
		{
			name:    "current fleet-local objects are kept",
			objects: legacyObjects("fleet-local", owned),
		},
		{
			name:        "partially removed legacy objects",
			objects:     legacyObjects(legacyNamespace, owned)[2:],
			wantDeleted: allLegacy[2:],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset(tt.objects...)
			h := &handler{k8s: client}

			require.NoError(t, h.removeLegacyObjects(context.Background()))
			assert.Equal(t, tt.wantDeleted, deleteActions(client))

			// Running again is a no-op.
			client.ClearActions()
			require.NoError(t, h.removeLegacyObjects(context.Background()))
			assert.Empty(t, deleteActions(client))
		})
	}
}

func TestRemoveLegacyObjectsDeleteErrors(t *testing.T) {
	owned := map[string]string{apply.LabelID: setID}

	t.Run("not found on delete is ignored", func(t *testing.T) {
		client := fake.NewClientset(legacyObjects(legacyNamespace, owned)...)
		client.PrependReactor("delete", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: action.GetResource().Resource}, "")
		})

		require.NoError(t, (&handler{k8s: client}).removeLegacyObjects(context.Background()))
	})

	t.Run("other delete errors are returned and stop the cleanup", func(t *testing.T) {
		client := fake.NewClientset(legacyObjects(legacyNamespace, owned)...)
		client.PrependReactor("delete", "cronjobs", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("boom")
		})

		err := (&handler{k8s: client}).removeLegacyObjects(context.Background())
		require.ErrorContains(t, err, "boom")
		assert.Equal(t, []string{"cronjobs/fleet-default/" + prefix + "-cronjob"}, deleteActions(client))
	})

	t.Run("get errors are returned", func(t *testing.T) {
		client := fake.NewClientset()
		client.PrependReactor("get", "cronjobs", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("boom")
		})

		require.ErrorContains(t, (&handler{k8s: client}).removeLegacyObjects(context.Background()), "boom")
	})
}
