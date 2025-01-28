package project_cluster

import (
	"testing"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
