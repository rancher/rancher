package restrictedadmin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
	"github.com/rancher/rancher/pkg/migrations/test"
)

func TestMigrationChanges(t *testing.T) {
	m := restrictedAdminMigration{}
	userName := "u-5djc7"
	restrictedGRB := newTestGlobalRoleBinding(userName, "restricted-admin")

	client := newFakeClient(t,
		restrictedGRB,
		newTestGlobalRoleBinding(userName, "other-role"),
	)

	result, err := m.Changes(context.TODO(), changes.ClientFrom(client), migrations.MigrationOptions{})
	require.NoError(t, err)

	want := &migrations.MigrationChanges{
		Changes: []changes.ResourceChange{
			{
				Operation: changes.OperationCreate,
				Create: &changes.CreateChange{
					Resource: test.ToUnstructured(t,
						&v3.GlobalRoleBinding{
							TypeMeta: metav1.TypeMeta{
								Kind:       "GlobalRoleBinding",
								APIVersion: "management.cattle.io/v3",
							},
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: "grb-",
							},
							GlobalRoleName: "restricted-admin-replacement",
							UserName:       userName,
						}),
				},
			},
			{
				Operation: changes.OperationDelete,
				Delete: &changes.DeleteChange{
					ResourceRef: changes.ResourceReference{
						ObjectRef: types.NamespacedName{
							Name: restrictedGRB.Name,
						},
						Group:    "management.cattle.io",
						Resource: "globalrolebindings",
						Version:  "v3",
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

func newTestGlobalRoleBinding(username, rolename string) *v3.GlobalRoleBinding {
	grb := &v3.GlobalRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GlobalRoleBinding",
			APIVersion: "management.cattle.io/v3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "grb-" + rand.String(5),
			Annotations: map[string]string{
				"cleanup.cattle.io/grbUpgradeCluster":                 "true",
				"field.cattle.io/creatorId":                           "user-zjvch",
				"lifecycle.cattle.io/create.mgmt-auth-grb-controller": "true",
			},
			Labels: map[string]string{
				"cattle.io/creator": "norman",
			},
		},
		GlobalRoleName: rolename,
		UserName:       username,
	}

	return grb
}
