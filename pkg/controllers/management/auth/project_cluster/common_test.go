package project_cluster

import (
	"fmt"
	"testing"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic"
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
	"k8s.io/client-go/rest"
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
		nsGetFunc    func(string, metav1.GetOptions) (*v1.Namespace, error)
		nsDeleteFunc func(string, metav1.DeleteOptions) error
		wantErr      bool
	}{
		{
			name: "error getting namespace",
			nsGetFunc: func(s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return nil, errDefault
			},
			wantErr: true,
		},
		{
			name: "namespace not found",
			nsGetFunc: func(s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return nil, errNsNotFound
			},
		},
		{
			name: "namespace is terminating",
			nsGetFunc: func(s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return terminatingNamespace.DeepCopy(), nil
			},
		},
		{
			name: "successfully delete namespace",
			nsGetFunc: func(s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return defaultNamespace.DeepCopy(), nil
			},
			nsDeleteFunc: func(s string, do metav1.DeleteOptions) error {
				return nil
			},
		},
		{
			name: "deleting namespace not found",
			nsGetFunc: func(s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return defaultNamespace.DeepCopy(), nil
			},
			nsDeleteFunc: func(s string, do metav1.DeleteOptions) error {
				return errNsNotFound
			},
		},
		{
			name: "error deleting namespace",
			nsGetFunc: func(s string, g metav1.GetOptions) (*v1.Namespace, error) {
				return defaultNamespace.DeepCopy(), nil
			},
			nsDeleteFunc: func(s string, do metav1.DeleteOptions) error {
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
	getter  func(name string, opts metav1.GetOptions) (*v1.Namespace, error)
	deleter func(name string, opts metav1.DeleteOptions) error
}

func (m mockNamespaces) Get(name string, opts metav1.GetOptions) (*v1.Namespace, error) {
	return m.getter(name, opts)
}

func (m mockNamespaces) Create(namespace *v1.Namespace) (*v1.Namespace, error) {
	panic("implement me")
}

func (m mockNamespaces) Update(namespace *v1.Namespace) (*v1.Namespace, error) {
	panic("implement me")
}

func (m mockNamespaces) Delete(name string, opts *metav1.DeleteOptions) error {
	return m.deleter(name, *opts)
}

func (m mockNamespaces) List(opts metav1.ListOptions) (*v1.NamespaceList, error) {
	panic("implement me")
}

func (m mockNamespaces) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (m mockNamespaces) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Namespace, err error) {
	panic("implement me")
}

func (m mockNamespaces) UpdateStatus(ns *v1.Namespace) (*v1.Namespace, error) {
	panic("implement me")
}

func (m mockNamespaces) WithImpersonation(impersonat rest.ImpersonationConfig) (generic.NonNamespacedClientInterface[*v1.Namespace, *v1.NamespaceList], error) {
	panic("implement me")
}
