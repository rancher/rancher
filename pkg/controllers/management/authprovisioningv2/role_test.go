package authprovisioningv2

import (
	"errors"
	"testing"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apisv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	clusterDeletingAnnotationLabel = "deleting"
	clusterDeletedAnnotationLabel  = "deleted"
	genericAnnotationLabel         = "clusternamespace"
	clusterErrorAnnotationLabel    = "other error"
	noAnnotation                   = map[string]string{}
)

func Test_handler_hasClusterAnnotationsAndDeletionTimestamp(t *testing.T) {
	funcName := "handler.hasClusterAnnotationsAndDeletionTimestamp()"

	tests := []struct {
		name        string
		annotations map[string]string
		wantErr     bool
		want        bool
	}{
		// Enqueued (returned generic.ErrSkip)
		{
			name:        "Cluster not deleted, should requeue",
			annotations: createNameAndNamespaceAnnotation(clusterDeletingAnnotationLabel),
			wantErr:     false,
			want:        true,
		},
		// Other cluster error (clusters.get return err other than IsNotFound)
		{
			name:        "Cluster cache returned unexpected error",
			annotations: createNameAndNamespaceAnnotation(clusterErrorAnnotationLabel),
			wantErr:     true,
			want:        false,
		},
		// cluster is not found (clusters.get returns IsNotFound err)
		{
			name:        "Cluster deleted, can delete",
			annotations: createNameAndNamespaceAnnotation(clusterDeletedAnnotationLabel),
			wantErr:     false,
			want:        false,
		},
		// non-cluster deletion (cluster has no DeletionTimestamp)
		{
			name:        "Other removal reason, can delete",
			annotations: createNameAndNamespaceAnnotation(genericAnnotationLabel),
			wantErr:     false,
			want:        false,
		},
		// no annotations
		{
			name:        "non-annotated, can delete",
			annotations: noAnnotation,
			wantErr:     false,
			want:        false,
		},
	}
	isClusterArg := true
	for i := 0; i < 2; i++ {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// first returned value is always the same role object as the role argument
				enqueue, err := simpleHandler(t).hasAnnotationsForDeletingCluster(tt.annotations, isClusterArg)
				if enqueue != tt.want {
					t.Errorf("%v returned %v, wanted %v", funcName, enqueue, tt.want)
				}
				if err == nil && tt.wantErr {
					t.Errorf("%v got nil error, wanted %v", funcName, tt.want)
					return
				}
				if err != nil && !tt.wantErr {
					t.Errorf("%v got error = %v, but wanted none", funcName, err)
					return
				}
				if err != nil && tt.wantErr {
					if tt.want && err != generic.ErrSkip {
						t.Errorf("%v wanted ErrSkip, got err = %v", funcName, err)
					}
				}
			})
		}
		isClusterArg = !isClusterArg
	}
}

func Test_OnRemoveRole(t *testing.T) {
	funcName := "OnRemoveRole()"
	tests := []struct {
		name        string
		role        *v1.Role
		wantEnqueue bool
		wantErr     bool
	}{
		{
			name: "Want enqueue",
			role: &v1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAndNamespaceAnnotation(clusterDeletingAnnotationLabel),
				},
			},
			wantEnqueue: true,
			wantErr:     false,
		},
		{
			name:        "Want no error",
			role:        &v1.Role{},
			wantEnqueue: false,
			wantErr:     false,
		},
		{
			name: "Want error",
			role: &v1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAndNamespaceAnnotation(clusterErrorAnnotationLabel),
				},
			},
			wantEnqueue: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := simpleHandler(t).OnRemoveRole("", tt.role)
			if tt.wantEnqueue && err != generic.ErrSkip {
				t.Errorf("%v wanted enqueue, got err = %v", funcName, err)
				return
			}
			if err != nil && !tt.wantEnqueue && !tt.wantErr {
				t.Errorf("%v wanted no error, got err = %v", funcName, err)
			}
		})
	}
}

func Test_OnRemoveRoleBinding(t *testing.T) {
	funcName := "OnRemoveRoleBinding()"
	tests := []struct {
		name        string
		role        *v1.RoleBinding
		wantEnqueue bool
		wantErr     bool
	}{
		{
			name: "Want enqueue",
			role: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAndNamespaceAnnotation(clusterDeletingAnnotationLabel),
				},
			},
			wantEnqueue: true,
			wantErr:     false,
		},
		{
			name:        "Want no error",
			role:        &v1.RoleBinding{},
			wantEnqueue: false,
			wantErr:     false,
		},
		{
			name: "Want error",
			role: &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAndNamespaceAnnotation(clusterErrorAnnotationLabel),
				},
			},
			wantEnqueue: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := simpleHandler(t).OnRemoveRoleBinding("", tt.role)
			if tt.wantEnqueue && err != generic.ErrSkip {
				t.Errorf("%v wanted enqueue, got err = %v", funcName, err)
				return
			}
			if err != nil && !tt.wantEnqueue && !tt.wantErr {
				t.Errorf("%v wanted no error, got err = %v", funcName, err)
			}
		})
	}
}

func Test_OnRemoveClusterRole(t *testing.T) {
	funcName := "OnRemoveClusterRole()"
	tests := []struct {
		name        string
		role        *v1.ClusterRole
		wantEnqueue bool
		wantErr     bool
	}{
		{
			name: "Want enqueue",
			role: &v1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAnnotation(clusterDeletingAnnotationLabel),
				},
			},
			wantEnqueue: true,
			wantErr:     false,
		},
		{
			name:        "Want no error",
			role:        &v1.ClusterRole{},
			wantEnqueue: false,
			wantErr:     false,
		},
		{
			name: "Want error",
			role: &v1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAnnotation(clusterErrorAnnotationLabel),
				},
			},
			wantEnqueue: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := simpleHandler(t).OnRemoveClusterRole("", tt.role)
			if tt.wantEnqueue && err != generic.ErrSkip {
				t.Errorf("%v wanted enqueue, got err = %v", funcName, err)
				return
			}
			if err != nil && !tt.wantEnqueue && !tt.wantErr {
				t.Errorf("%v wanted no error, got err = %v", funcName, err)
			}
		})
	}
}

func Test_OnRemoveClusterRoleBinding(t *testing.T) {
	funcName := "OnRemoveClusterRoleBinding()"
	tests := []struct {
		name        string
		role        *v1.ClusterRoleBinding
		wantEnqueue bool
		wantErr     bool
	}{
		{
			name: "Want enqueue",
			role: &v1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAnnotation(clusterDeletingAnnotationLabel),
				},
			},
			wantEnqueue: true,
			wantErr:     false,
		},
		{
			name:        "Want no error",
			role:        &v1.ClusterRoleBinding{},
			wantEnqueue: false,
			wantErr:     false,
		},
		{
			name: "Want error",
			role: &v1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: createNameAnnotation(clusterErrorAnnotationLabel),
				},
			},
			wantEnqueue: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := simpleHandler(t).OnRemoveClusterRoleBinding("", tt.role)
			if tt.wantEnqueue && err != generic.ErrSkip {
				t.Errorf("%v wanted enqueue, got err = %v", funcName, err)
				return
			}
			if err != nil && !tt.wantEnqueue && !tt.wantErr {
				t.Errorf("%v wanted no error, got err = %v", funcName, err)
			}
		})
	}
}

type mockIndexGetter []runtime.Object

func (m mockIndexGetter) GetByIndex(schema.GroupVersionKind, string, string) ([]runtime.Object, error) {
	return m, nil
}

func Test_getResourceNames_sorted(t *testing.T) {
	objs := []runtime.Object{
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "b3"}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "c5"}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "b4"}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "a2"}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "a1"}},
	}
	want := []string{"a1", "a2", "b3", "b4", "c5"}

	got, err := getResourceNames(mockIndexGetter(objs), resourceMatch{}, &provisioningv1.Cluster{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, want, got)
}

func createNameAndNamespaceAnnotation(n string) map[string]string {
	return map[string]string{clusterNameLabel: n, clusterNamespaceLabel: n}
}

func createNameAnnotation(n string) map[string]string {
	return map[string]string{clusterNameLabel: n}
}

func simpleHandler(t *testing.T) *handler {
	t.Helper()
	ctrl := gomock.NewController(t)

	mockRoleController := fake.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
	mockRoleController.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockRoleBindingController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
	mockRoleBindingController.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockCluster := fake.NewMockCacheInterface[*apisv1.Cluster](ctrl)
	mockCluster.EXPECT().Get("deleted", gomock.Any()).Return(nil, apierror.NewNotFound(schema.GroupResource{}, "deleted")).AnyTimes()
	mockCluster.EXPECT().Get("deleting", gomock.Any()).Return(newDeletingCuster(), nil).AnyTimes()
	mockCluster.EXPECT().Get("other error", gomock.Any()).Return(nil, errors.New("other error")).AnyTimes()
	mockCluster.EXPECT().Get(gomock.Any(), gomock.Any()).Return(&provisioningv1.Cluster{}, nil).AnyTimes()

	mockMgmtCluster := fake.NewMockNonNamespacedCacheInterface[*apisv3.Cluster](ctrl)
	mockMgmtCluster.EXPECT().Get("deleted").Return(nil, apierror.NewNotFound(schema.GroupResource{}, "deleted")).AnyTimes()
	mockMgmtCluster.EXPECT().Get("deleting").Return(newDeletingMgmtCuster(), nil).AnyTimes()
	mockMgmtCluster.EXPECT().Get("other error").Return(nil, errors.New("other error")).AnyTimes()
	mockMgmtCluster.EXPECT().Get(gomock.Any()).Return(&v3.Cluster{}, nil).AnyTimes()

	mockClusterRoleController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
	mockClusterRoleController.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any()).AnyTimes()

	mockClusterRoleBindingController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl)
	mockClusterRoleBindingController.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any()).AnyTimes()

	return &handler{
		roleController:               mockRoleController,
		roleBindingController:        mockRoleBindingController,
		clusters:                     mockCluster,
		mgmtClusters:                 mockMgmtCluster,
		clusterRoleController:        mockClusterRoleController,
		clusterRoleBindingController: mockClusterRoleBindingController,
	}

}
func newDeletingCuster() *provisioningv1.Cluster {
	c := &provisioningv1.Cluster{}
	c.DeletionTimestamp = &metav1.Time{}
	return c
}
func newDeletingMgmtCuster() *v3.Cluster {
	c := &v3.Cluster{}
	c.DeletionTimestamp = &metav1.Time{}
	return c
}

func Test_isProtectedRBACResource(t *testing.T) {
	tests := []struct {
		name string
		obj  runtime.Object
		want bool
	}{
		{
			name: "role without required annotations",
			obj: &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{
				Name: "myrole", Namespace: "test",
			}},
			want: false,
		},
		{
			name: "role with partial annotations",
			obj: &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{
				Name: "myrole", Namespace: "test",
				Annotations: map[string]string{
					"cluster.cattle.io/name": "test",
				},
			}},
			want: false,
		},
		{
			name: "role with expected annotations",
			obj: &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{
				Name: "myrole", Namespace: "test",
				Annotations: map[string]string{
					"cluster.cattle.io/name":      "test",
					"cluster.cattle.io/namespace": "test",
				},
			}},
			want: true,
		},

		{
			name: "rolebinding without required annotations",
			obj: &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{
				Name: "myrole", Namespace: "test",
			}},
			want: false,
		},
		{
			name: "rolebinding with expected annotations",
			obj: &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{
				Name: "myrole", Namespace: "test",
				Annotations: map[string]string{
					"cluster.cattle.io/name":      "test",
					"cluster.cattle.io/namespace": "test",
				},
			}},
			want: true,
		},

		{
			name: "clusterrole without annotations",
			obj: &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
				Name: "myrole",
			}},
			want: false,
		},
		{
			name: "clusterrole with expected annotations",
			obj: &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
				Name: "myrole",
				Annotations: map[string]string{
					"cluster.cattle.io/name": "test",
				},
			}},
			want: true,
		},

		{
			name: "clusterrolebinding without annotations",
			obj: &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
				Name: "myrole",
			}},
			want: false,
		},
		{
			name: "clusterrolebinding with expected annotations",
			obj: &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
				Name: "myrole",
				Annotations: map[string]string{
					"cluster.cattle.io/name": "test",
				},
			}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, isProtectedRBACResource(tt.obj), "isProtectedRBACResource(%v)", tt.obj)
		})
	}
}
