package project_cluster

import (
	"context"
	"fmt"
	"testing"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
)

var errNotFound = apierrors.NewNotFound(schema.GroupResource{}, "")

func TestCreateProjectMembershipRoles(t *testing.T) {
	project := &apisv3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-project",
			Namespace: "test-cluster",
			UID:       "1234abcd",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Project",
		},
		Spec: apisv3.ProjectSpec{
			ClusterName: "test-cluster",
		},
	}
	ownerRef := []metav1.OwnerReference{
		{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Project",
			Name:       "test-project",
			UID:        "1234abcd",
		},
	}
	memberRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-project-projectmember",
			Namespace:       "test-cluster",
			OwnerReferences: ownerRef,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"management.cattle.io"},
				Resources:     []string{"projects"},
				ResourceNames: []string{"test-project"},
				Verbs:         []string{"get"},
			},
		},
	}
	ownerRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-project-projectowner",
			Namespace:       "test-cluster",
			OwnerReferences: ownerRef,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"management.cattle.io"},
				Resources:     []string{"projects"},
				ResourceNames: []string{"test-project"},
				Verbs:         []string{"*"},
			},
		},
	}

	ctrl := gomock.NewController(t)
	roleClient := fake.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl)
	roleClient.EXPECT().Get(memberRole.Namespace, memberRole.Name, metav1.GetOptions{}).Return(nil, errNotFound)
	roleClient.EXPECT().Get(ownerRole.Namespace, ownerRole.Name, metav1.GetOptions{}).Return(nil, errNotFound)
	roleClient.EXPECT().Create(memberRole).Return(nil, nil)
	roleClient.EXPECT().Create(ownerRole).Return(nil, nil)
	assert.NoError(t, createProjectMembershipRoles(project, roleClient))
}

func TestCreateClusterMembershipRoles(t *testing.T) {
	cluster := &apisv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
			UID:  "1234abcd",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Cluster",
		},
	}
	ownerRef := []metav1.OwnerReference{
		{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Cluster",
			Name:       "test-cluster",
			UID:        "1234abcd",
		},
	}
	memberRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-cluster-clustermember",
			OwnerReferences: ownerRef,
			Annotations: map[string]string{
				"cluster.cattle.io/name": "test-cluster",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"management.cattle.io"},
				Resources:     []string{"clusters"},
				ResourceNames: []string{"test-cluster"},
				Verbs:         []string{"get"},
			},
		},
	}
	ownerRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-cluster-clusterowner",
			OwnerReferences: ownerRef,
			Annotations: map[string]string{
				"cluster.cattle.io/name": "test-cluster",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"management.cattle.io"},
				Resources:     []string{"clusters"},
				ResourceNames: []string{"test-cluster"},
				Verbs:         []string{"*"},
			},
		},
	}

	ctrl := gomock.NewController(t)
	crClient := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
	crClient.EXPECT().Get(memberRole.Name, metav1.GetOptions{}).Return(nil, errNotFound)
	crClient.EXPECT().Get(ownerRole.Name, metav1.GetOptions{}).Return(nil, errNotFound)
	crClient.EXPECT().Create(memberRole).Return(nil, nil)
	crClient.EXPECT().Create(ownerRole).Return(nil, nil)
	assert.NoError(t, createClusterMembershipRoles(cluster, crClient))
}

var (
	errDefault = fmt.Errorf("error")

	errNsNotFound = apierrors.NewNotFound(v1.Resource("namespace"), "")

	defaultNamespace = v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-namespace",
		},
	}
	terminatingNamespace = v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "terminating-namespace",
		},
		Status: v1.NamespaceStatus{
			Phase: v1.NamespaceTerminating,
		},
	}
)

func Test_deleteNamespace(t *testing.T) {
	tests := []struct {
		name         string
		nsGetFunc    func(context.Context, string, metav1.GetOptions) (*v1.Namespace, error)
		nsDeleteFunc func(context.Context, string, metav1.DeleteOptions) error
		wantErr      bool
	}{
		{
			name: "error getting namespace",
			nsGetFunc: func(ctx context.Context, s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return nil, errDefault
			},
			wantErr: true,
		},
		{
			name: "namespace not found",
			nsGetFunc: func(ctx context.Context, s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return nil, errNsNotFound
			},
		},
		{
			name: "namespace is terminating",
			nsGetFunc: func(ctx context.Context, s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return terminatingNamespace.DeepCopy(), nil
			},
		},
		{
			name: "successfully delete namespace",
			nsGetFunc: func(ctx context.Context, s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return defaultNamespace.DeepCopy(), nil
			},
			nsDeleteFunc: func(ctx context.Context, s string, do metav1.DeleteOptions) error {
				return nil
			},
		},
		{
			name: "deleting namespace not found",
			nsGetFunc: func(ctx context.Context, s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return defaultNamespace.DeepCopy(), nil
			},
			nsDeleteFunc: func(ctx context.Context, s string, do metav1.DeleteOptions) error {
				return errNsNotFound
			},
		},
		{
			name: "error deleting namespace",
			nsGetFunc: func(ctx context.Context, s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return defaultNamespace.DeepCopy(), nil
			},
			nsDeleteFunc: func(ctx context.Context, s string, do metav1.DeleteOptions) error {
				return errDefault
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nsClientFake := mockNamespaces{
				getter:  tt.nsGetFunc,
				deleter: tt.nsDeleteFunc,
			}
			if err := deleteNamespace("", "", nsClientFake); (err != nil) != tt.wantErr {
				t.Errorf("deleteNamespace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type mockNamespaces struct {
	getter  func(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Namespace, error)
	deleter func(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

func (m mockNamespaces) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Namespace, error) {
	return m.getter(ctx, name, opts)
}

func (m mockNamespaces) Create(ctx context.Context, namespace *v1.Namespace, opts metav1.CreateOptions) (*v1.Namespace, error) {
	panic("implement me")
}

func (m mockNamespaces) Update(ctx context.Context, namespace *v1.Namespace, opts metav1.UpdateOptions) (*v1.Namespace, error) {
	panic("implement me")
}

func (m mockNamespaces) UpdateStatus(ctx context.Context, namespace *v1.Namespace, opts metav1.UpdateOptions) (*v1.Namespace, error) {
	panic("implement me")
}

func (m mockNamespaces) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return m.deleter(ctx, name, opts)
}

func (m mockNamespaces) List(ctx context.Context, opts metav1.ListOptions) (*v1.NamespaceList, error) {
	panic("implement me")
}

func (m mockNamespaces) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (m mockNamespaces) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.Namespace, err error) {
	panic("implement me")
}

func (m mockNamespaces) Apply(ctx context.Context, namespace *applycorev1.NamespaceApplyConfiguration, opts metav1.ApplyOptions) (result *v1.Namespace, err error) {
	panic("implement me")
}

func (m mockNamespaces) ApplyStatus(ctx context.Context, namespace *applycorev1.NamespaceApplyConfiguration, opts metav1.ApplyOptions) (result *v1.Namespace, err error) {
	panic("implement me")
}

func (m mockNamespaces) Finalize(ctx context.Context, item *v1.Namespace, opts metav1.UpdateOptions) (*v1.Namespace, error) {
	panic("implement me")
}
