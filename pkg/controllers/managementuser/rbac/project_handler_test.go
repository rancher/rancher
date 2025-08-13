package rbac

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wfakes "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCreate(t *testing.T) {
	const projectName = "p-123xyz"
	ctrl := gomock.NewController(t)
	tests := []struct {
		name                     string
		existingClusterRoleNames []string
		getErr                   error
		createErr                error

		wantErr   bool
		wantRoles []rbacv1.ClusterRole
	}{
		{
			name: "basic create",
			wantRoles: []rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "p-123xyz-namespaces-readonly",
						Annotations: map[string]string{
							projectNSAnn: "p-123xyz-namespaces-readonly",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "p-123xyz-namespaces-edit",
						Annotations: map[string]string{
							projectNSAnn: "p-123xyz-namespaces-edit",
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups:     []string{"management.cattle.io"},
							Verbs:         []string{"manage-namespaces"},
							Resources:     []string{"projects"},
							ResourceNames: []string{"p-123xyz"},
						},
					},
				},
			},
		},
		{
			name:    "get error",
			getErr:  fmt.Errorf("unexpected error"),
			wantErr: false,
		},
		{
			name:      "create error",
			createErr: fmt.Errorf("unexpected error"),
			wantErr:   true,
		},
		{
			name:                     "roles already exist",
			existingClusterRoleNames: []string{"p-123xyz-namespaces-readonly", "p-123xyz-namespaces-edit"},
			wantErr:                  false,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			project := &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
			}
			var newCRs []*rbacv1.ClusterRole
			crLister := wfakes.NewMockNonNamespacedCacheInterface[*rbacv1.ClusterRole](ctrl)
			crLister.EXPECT().Get(gomock.Any()).DoAndReturn(
				func(name string) (*rbacv1.ClusterRole, error) {
					if test.getErr != nil {
						return nil, test.getErr
					}
					for _, clusterRoleName := range test.existingClusterRoleNames {
						if clusterRoleName == name {
							return &rbacv1.ClusterRole{
								ObjectMeta: metav1.ObjectMeta{
									Name: clusterRoleName,
								},
							}, nil
						}
					}
					return nil, apierror.NewNotFound(schema.GroupResource{
						Group:    "rbac.authorization.k8s.io",
						Resource: "ClusterRoles",
					}, name)
				},
			).AnyTimes()
			clusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			clusterRoles.EXPECT().Create(gomock.Any()).DoAndReturn(
				func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
					newCRs = append(newCRs, in)
					if test.createErr != nil {
						return nil, test.createErr
					}
					return in, nil
				},
			).AnyTimes()
			lifecycle := pLifecycle{
				m: &manager{
					crLister:     crLister,
					clusterRoles: clusterRoles,
				},
			}
			_, err := lifecycle.Create(project)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(test.wantRoles), len(newCRs))
				for _, desiredRole := range test.wantRoles {
					assert.Contains(t, newCRs, &desiredRole)
				}
			}
		})
	}
}

func TestUpdated(t *testing.T) {
	const projectName = "p-123xyz"
	ctrl := gomock.NewController(t)
	tests := []struct {
		name               string
		currentClusterRole *rbacv1.ClusterRole
		getError           error
		updError           error
		createError        error

		wantError       bool
		wantClusterRole *rbacv1.ClusterRole
	}{
		{
			name: "missing cluster role annotation",
			currentClusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{""},
						Verbs:         []string{"*"},
						Resources:     []string{"namespaces"},
						ResourceNames: []string{"test-ns"},
					},
				},
			},
			wantError: false,
			wantClusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{""},
						Verbs:         []string{"*"},
						Resources:     []string{"namespaces"},
						ResourceNames: []string{"test-ns"},
					},
					{
						APIGroups:     []string{"management.cattle.io"},
						Verbs:         []string{"manage-namespaces"},
						Resources:     []string{"projects"},
						ResourceNames: []string{"p-123xyz"},
					},
				},
			},
		},
		{
			name: "annotation present",
			currentClusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{""},
						Verbs:         []string{"*"},
						Resources:     []string{"namespaces"},
						ResourceNames: []string{"test-ns"},
					},
					{
						APIGroups:     []string{"management.cattle.io"},
						Verbs:         []string{"manage-namespaces"},
						Resources:     []string{"projects"},
						ResourceNames: []string{"p-123xyz"},
					},
				},
			},
			wantError: false,
		},
		{
			name: "missing cluster role",
			wantClusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						Verbs:         []string{"manage-namespaces"},
						Resources:     []string{"projects"},
						ResourceNames: []string{"p-123xyz"},
					},
				},
			},
			wantError: false,
		},
		{
			name:      "get error",
			getError:  fmt.Errorf("unexpected error"),
			wantError: true,
		},
		{
			name:        "create error",
			createError: fmt.Errorf("unexpected error"),
			wantError:   true,
		},
		{
			name: "update error",
			currentClusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{""},
						Verbs:         []string{"*"},
						Resources:     []string{"namespaces"},
						ResourceNames: []string{"test-ns"},
					},
				},
			},
			updError:  fmt.Errorf("unexpected error"),
			wantError: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			project := &v3.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
			}
			var newCRs []*rbacv1.ClusterRole
			crLister := wfakes.NewMockNonNamespacedCacheInterface[*rbacv1.ClusterRole](ctrl)
			crLister.EXPECT().Get(gomock.Any()).DoAndReturn(
				func(name string) (*rbacv1.ClusterRole, error) {
					if test.getError != nil {
						return nil, test.getError
					}
					if test.currentClusterRole != nil && name == test.currentClusterRole.Name {
						return test.currentClusterRole, nil
					}
					return nil, apierror.NewNotFound(schema.GroupResource{
						Group:    "rbac.authorization.k8s.io",
						Resource: "ClusterRoles",
					}, name)
				},
			).AnyTimes()
			clusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			clusterRoles.EXPECT().Create(gomock.Any()).DoAndReturn(
				func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
					newCRs = append(newCRs, in)
					if test.createError != nil {
						return nil, test.createError
					}
					return in, nil
				},
			).AnyTimes()
			clusterRoles.EXPECT().Update(gomock.Any()).DoAndReturn(
				func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
					newCRs = append(newCRs, in)
					if test.updError != nil {
						return nil, test.updError
					}
					return in, nil
				},
			).AnyTimes()
			lifecycle := pLifecycle{
				m: &manager{
					crLister:     crLister,
					clusterRoles: clusterRoles,
				},
			}
			_, err := lifecycle.Updated(project)
			if test.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if test.wantClusterRole != nil {
					assert.Len(t, newCRs, 1)
					assert.Equal(t, test.wantClusterRole, newCRs[0])
				}
			}
		})
	}
}
