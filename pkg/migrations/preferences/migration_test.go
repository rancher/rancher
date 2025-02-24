package preferences

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
	"github.com/rancher/rancher/pkg/migrations/test"
)

func TestMigrationChanges(t *testing.T) {
	m := preferencesMigration{}
	preference1 := newTestUserPreference("user-d7tv1", "locale", "en-gb")
	preference2 := newTestUserPreference("user-xn8mf", "locale", "en-us")
	preference3 := newTestUserPreference("user-xn8mf", "seen-whatsnew", "dev")

	client := newFakeClient(t,
		preference1, preference2, preference3,
	)

	result, err := m.Changes(context.TODO(), changes.ClientFrom(client), migrations.MigrationOptions{})
	require.NoError(t, err)

	want := &migrations.MigrationChanges{
		Changes: []migrations.ChangeSet{
			{
				{
					Operation: changes.OperationCreate,
					Create: &changes.CreateChange{
						Resource: test.ToUnstructured(t,
							&corev1.ConfigMap{
								TypeMeta: metav1.TypeMeta{
									Kind:       "ConfigMap",
									APIVersion: "v1",
								},
								ObjectMeta: metav1.ObjectMeta{
									Name:      "user-d7tv1-preferences",
									Namespace: "user-d7tv1",
								},
								Data: map[string]string{
									"locale": "en-gb",
								},
							}),
					},
				},
				{
					Operation: changes.OperationDelete,
					Delete: &changes.DeleteChange{
						ResourceRef: changes.ResourceReference{
							ObjectRef: types.NamespacedName{
								Name:      preference1.Name,
								Namespace: preference1.Namespace,
							},
							Group:    "management.cattle.io",
							Resource: "preferences",
							Version:  "v3",
						},
					},
				},
				{
					Operation: changes.OperationCreate,
					Create: &changes.CreateChange{
						Resource: test.ToUnstructured(t,
							&corev1.ConfigMap{
								TypeMeta: metav1.TypeMeta{
									Kind:       "ConfigMap",
									APIVersion: "v1",
								},
								ObjectMeta: metav1.ObjectMeta{
									Name:      "user-xn8mf-preferences",
									Namespace: "user-xn8mf",
								},
								Data: map[string]string{
									"locale":        "en-us",
									"seen-whatsnew": "dev",
								},
							}),
					},
				},
				{
					Operation: changes.OperationDelete,
					Delete: &changes.DeleteChange{
						ResourceRef: changes.ResourceReference{
							ObjectRef: types.NamespacedName{
								Name:      preference2.Name,
								Namespace: preference2.Namespace,
							},
							Group:    "management.cattle.io",
							Resource: "preferences",
							Version:  "v3",
						},
					},
				},
				{
					Operation: changes.OperationDelete,
					Delete: &changes.DeleteChange{
						ResourceRef: changes.ResourceReference{
							ObjectRef: types.NamespacedName{
								Name:      preference3.Name,
								Namespace: preference3.Namespace,
							},
							Group:    "management.cattle.io",
							Resource: "preferences",
							Version:  "v3",
						},
					},
				},
			},
		},
	}

	assert.Equal(t, want, result)
}

func newFakeClient(t *testing.T, objs ...runtime.Object) *fake.FakeDynamicClient {
	testScheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(testScheme))
	return fake.NewSimpleDynamicClient(testScheme, objs...)
}

func newTestUserPreference(username, name, value string) *v3.Preference {
	return &v3.Preference{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Preference",
			APIVersion: "management.cattle.io/v3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: username,
		},
		Value: value,
	}
}
