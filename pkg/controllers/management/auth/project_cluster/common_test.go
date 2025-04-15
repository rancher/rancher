package project_cluster

import (
	"context"
	"fmt"
	"testing"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
)

var errNotFound = apierrors.NewNotFound(schema.GroupResource{}, "")

func TestCreateMembershipRoles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		obj       runtime.Object
		wantedCRs []*rbacv1.ClusterRole
		wantErr   bool
	}{
		{
			name: "roles for project",
			obj: &apisv3.Project{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v3",
					Kind:       "Project",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-project",
					UID:  "1234abcd",
				},
			},
			wantedCRs: []*rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-project-project-member",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v3",
								Kind:       "Project",
								Name:       "test-project",
								UID:        "1234abcd",
							},
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups:     []string{"management.cattle.io"},
							Resources:     []string{"projects"},
							ResourceNames: []string{"test-project"},
							Verbs:         []string{"get"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-project-project-owner",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v3",
								Kind:       "Project",
								Name:       "test-project",
								UID:        "1234abcd",
							},
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups:     []string{"management.cattle.io"},
							Resources:     []string{"projects"},
							ResourceNames: []string{"test-project"},
							Verbs:         []string{"*"},
						},
					},
				},
			},
		},
		{
			name: "roles for cluster",
			obj: &apisv3.Cluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v3",
					Kind:       "Cluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					UID:  "1234abcd",
				},
			},
			wantedCRs: []*rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster-cluster-member",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v3",
								Kind:       "Cluster",
								Name:       "test-cluster",
								UID:        "1234abcd",
							},
						},
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
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster-cluster-owner",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v3",
								Kind:       "Cluster",
								Name:       "test-cluster",
								UID:        "1234abcd",
							},
						},
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
				},
			},
		},
		{
			name:    "called with unsupported type",
			obj:     &apisv3.ProjectNetworkPolicyList{},
			wantErr: true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			crClient := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)

			if tt.wantedCRs != nil {
				crClient.EXPECT().Get(tt.wantedCRs[0].Name, metav1.GetOptions{}).Return(nil, errNotFound)
				crClient.EXPECT().Get(tt.wantedCRs[1].Name, metav1.GetOptions{}).Return(nil, errNotFound)
				crClient.EXPECT().Create(tt.wantedCRs[0]).Return(nil, nil)
				crClient.EXPECT().Create(tt.wantedCRs[1]).Return(nil, nil)
			}

			if err := createMembershipRoles(tt.obj, crClient); (err != nil) != tt.wantErr {
				t.Errorf("createMembershipRoles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

var (
	errDefault    = fmt.Errorf("error")
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
