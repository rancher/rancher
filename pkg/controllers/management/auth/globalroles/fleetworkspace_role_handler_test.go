package globalroles

import (
	"github.com/rancher/rancher/pkg/controllers"
	"testing"

	"github.com/golang/mock/gomock"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rbacv1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v2/pkg/generic/fake"
	wrangler "github.com/rancher/wrangler/v2/pkg/name"
	"github.com/stretchr/testify/assert"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	resourceRules = []rbac.PolicyRule{
		{
			Verbs:     []string{"get", "list"},
			APIGroups: []string{"fleet.cattle.io"},
			Resources: []string{"gitrepos", "bundles"},
		},
	}
	workspaceVerbs = []string{"get", "list"}
)

func TestReconcileFleetPermissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := map[string]struct {
		crClient func() rbacv1.ClusterRoleController
		crCache  func() rbacv1.ClusterRoleCache
		gr       *v3.GlobalRole
	}{
		"backing ClusterRoles are created for a new GlobalRole": {
			crCache: func() rbacv1.ClusterRoleCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl)
				mock.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				mock.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			crClient: createClusterRolesMock(ctrl),
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: grName,
					UID:  grUID,
				},
				InheritedFleetWorkspacePermissions: v3.FleetWorkspacePermission{
					ResourceRules:  resourceRules,
					WorkspaceVerbs: workspaceVerbs,
				},
			},
		},
		"no update if ClusterRoles are prensent, and haven't changed": {
			crCache: clusterRoleMock(ctrl),
			crClient: func() rbacv1.ClusterRoleController {
				return fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl)
			},
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: grName,
					UID:  grUID,
				},
				InheritedFleetWorkspacePermissions: v3.FleetWorkspacePermission{
					ResourceRules:  resourceRules,
					WorkspaceVerbs: workspaceVerbs,
				},
			},
		},
		"backing Roles and ClusterRoles are updated with new content": {
			crCache: clusterRoleMock(ctrl),
			crClient: func() rbacv1.ClusterRoleController {
				mock := fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl)
				// expect updates with new rules
				mock.EXPECT().Update(&rbac.ClusterRole{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "GlobalRole",
								Name:       grName,
								UID:        grUID,
							},
						},
					},
					Rules: []rbac.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"fleet.cattle.io"},
							Resources: []string{"gitrepos"},
						},
					},
				})
				mock.EXPECT().Update(&rbac.ClusterRole{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "GlobalRole",
								Name:       grName,
								UID:        grUID,
							},
						},
					},
					Rules: []rbac.PolicyRule{
						{
							Verbs:         []string{"*"},
							APIGroups:     []string{"management.cattle.io"},
							Resources:     []string{"fleetworkspaces"},
							ResourceNames: []string{"fleet-default"},
						},
					},
				})
				return mock
			},
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: grName,
					UID:  grUID,
				},
				InheritedFleetWorkspacePermissions: v3.FleetWorkspacePermission{
					ResourceRules: []rbac.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"fleet.cattle.io"},
							Resources: []string{"gitrepos"},
						},
					},
					WorkspaceVerbs: []string{"*"},
				},
			},
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := fleetWorkspaceRoleHandler{
				crClient: test.crClient(),
				crCache:  test.crCache(),
				fwCache:  fleetDefaultAndLocalWorkspaceCacheMock(ctrl),
			}

			err := h.reconcileFleetWorkspacePermissions(test.gr)

			assert.Equal(t, err, nil)
		})
	}
}

func TestReconcileFleetPermissions_errors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := map[string]struct {
		crClient       func() rbacv1.ClusterRoleController
		crCache        func() rbacv1.ClusterRoleCache
		fwCache        func() mgmtcontroller.FleetWorkspaceCache
		wantErrMessage string
	}{
		"Error retrieving ClusterRole": {
			fwCache: func() mgmtcontroller.FleetWorkspaceCache {
				return fake.NewMockNonNamespacedCacheInterface[*v3.FleetWorkspace](ctrl)
			},
			crCache: func() rbacv1.ClusterRoleCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl)
				mock.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, errors.NewServiceUnavailable("unexpected error"))
				return mock
			},
			crClient: func() rbacv1.ClusterRoleController {
				return fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl)
			},
			wantErrMessage: "error reconciling fleet permissions cluster role: unexpected error",
		},
		"Error creating ClusterRole": {
			fwCache: func() mgmtcontroller.FleetWorkspaceCache {
				return fake.NewMockNonNamespacedCacheInterface[*v3.FleetWorkspace](ctrl)
			},
			crCache: func() rbacv1.ClusterRoleCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl)
				mock.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			crClient: func() rbacv1.ClusterRoleController {
				mock := fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl)
				mock.EXPECT().Create(&rbac.ClusterRole{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "GlobalRole",
								Name:       grName,
								UID:        grUID,
							},
						},
						Labels: map[string]string{
							grOwnerLabel:                wrangler.SafeConcatName(grName),
							controllers.K8sManagedByKey: controllers.ManagerValue,
						},
					},
					Rules: resourceRules,
				}).Return(nil, errors.NewServiceUnavailable("unexpected error"))
				return mock
			},
			wantErrMessage: "error reconciling fleet permissions cluster role: unexpected error",
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := fleetWorkspaceRoleHandler{
				crClient: test.crClient(),
				crCache:  test.crCache(),
				fwCache:  test.fwCache(),
			}

			err := h.reconcileFleetWorkspacePermissions(&v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: grName,
					UID:  grUID,
				},
				InheritedFleetWorkspacePermissions: v3.FleetWorkspacePermission{
					ResourceRules:  resourceRules,
					WorkspaceVerbs: workspaceVerbs,
				},
			})

			assert.EqualError(t, err, test.wantErrMessage)
		})
	}
}

func createClusterRolesMock(ctrl *gomock.Controller) func() rbacv1.ClusterRoleController {
	return func() rbacv1.ClusterRoleController {
		mock := fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl)
		mock.EXPECT().Create(&rbac.ClusterRole{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRole",
						Name:       grName,
						UID:        grUID,
					},
				},
				Labels: map[string]string{
					grOwnerLabel:                wrangler.SafeConcatName(grName),
					controllers.K8sManagedByKey: controllers.ManagerValue,
				},
			},
			Rules: resourceRules,
		})
		mock.EXPECT().Create(&rbac.ClusterRole{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRole",
						Name:       grName,
						UID:        grUID,
					},
				},
				Labels: map[string]string{
					grOwnerLabel:                wrangler.SafeConcatName(grName),
					controllers.K8sManagedByKey: controllers.ManagerValue,
				},
			},
			Rules: []rbac.PolicyRule{
				{
					Verbs:         workspaceVerbs,
					APIGroups:     []string{"management.cattle.io"},
					Resources:     []string{"fleetworkspaces"},
					ResourceNames: []string{"fleet-default"},
				},
			},
		})
		return mock
	}
}

func clusterRoleMock(ctrl *gomock.Controller) func() rbacv1.ClusterRoleCache {
	return func() rbacv1.ClusterRoleCache {
		mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl)
		mock.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRole",
						Name:       grName,
						UID:        grUID,
					},
				},
			},
			Rules: resourceRules,
		}, nil)
		mock.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRole",
						Name:       grName,
						UID:        grUID,
					},
				},
			},
			Rules: []rbac.PolicyRule{
				{
					Verbs:         workspaceVerbs,
					APIGroups:     []string{"management.cattle.io"},
					Resources:     []string{"fleetworkspaces"},
					ResourceNames: []string{"fleet-default"},
				},
			},
		}, nil)
		return mock
	}
}
