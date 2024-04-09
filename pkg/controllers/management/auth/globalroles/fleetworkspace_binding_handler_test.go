package globalroles

import (
	"testing"

	wrangler "github.com/rancher/wrangler/v2/pkg/name"

	"github.com/golang/mock/gomock"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rbacv1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v2/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	grName  = "gr"
	grUID   = "9cf141b8-54ab-4711-8e43-eb1fc0a189a8"
	grbName = "grb"
	grbUID  = "3267582b-96eb-4752-81de-cb33e7d8f3e7"
	user    = "user"
)

func TestReconcileFleetWorkspacePermissionsBindings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := map[string]struct {
		crbClient func() rbacv1.ClusterRoleBindingController
		crbCache  func() rbacv1.ClusterRoleBindingCache
		grCache   func() mgmtcontroller.GlobalRoleCache
		rbClient  func() rbacv1.RoleBindingController
		rbCache   func() rbacv1.RoleBindingCache
		fwCache   func() mgmtcontroller.FleetWorkspaceCache
		grb       *v3.GlobalRoleBinding
	}{
		"backing RoleBindings and ClusterRoleBindings are created for a new GlobalRoleBinding": {
			grCache: globalRoleMock(ctrl),
			rbCache: func() rbacv1.RoleBindingCache {
				mock := fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
				mock.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			rbClient:  createRoleBindingsMock(ctrl),
			crbClient: createClusterRoleBindingsMock(ctrl),
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
				mock.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			fwCache: fleetDefaultAndLocalWorkspaceCacheMock(ctrl),
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: grbName,
					UID:  grbUID,
				},
				UserName:       user,
				GlobalRoleName: grName,
			},
		},
		"backing RoleBindings and ClusterRoleBindings are updated with new content": {
			grCache: globalRoleMock(ctrl),
			rbCache: func() rbacv1.RoleBindingCache {
				mock := fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
				mock.EXPECT().Get("fleet-default", grbName).Return(&rbac.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      grbName,
						Namespace: "fleet-default",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "GlobalRoleBinding",
								Name:       grbName,
								UID:        grbUID,
							},
						},
					},
					RoleRef: rbac.RoleRef{
						APIGroup: rbac.GroupName,
						Kind:     "ClusterRole",
						Name:     wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName),
					},
					Subjects: []rbac.Subject{
						{
							Kind:     "User",
							Name:     user,
							APIGroup: rbac.GroupName,
						},
					},
				}, nil)
				return mock
			},
			rbClient: func() rbacv1.RoleBindingController {
				mock := fake.NewMockControllerInterface[*rbac.RoleBinding, *rbac.RoleBindingList](ctrl)
				mock.EXPECT().Update(&rbac.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      grbName,
						Namespace: "fleet-default",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "GlobalRoleBinding",
								Name:       grbName,
								UID:        grbUID,
							},
						},
					},
					RoleRef: rbac.RoleRef{
						APIGroup: rbac.GroupName,
						Kind:     "ClusterRole",
						Name:     wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName),
					},
					Subjects: []rbac.Subject{
						{
							Kind:     "User",
							Name:     "newUser", // verify update with new user
							APIGroup: rbac.GroupName,
						},
					},
				})
				return mock
			},
			crbClient: func() rbacv1.ClusterRoleBindingController {
				mock := fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl)
				mock.EXPECT().Update(&rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "GlobalRoleBinding",
								Name:       grbName,
								UID:        grbUID,
							},
						},
					},
					RoleRef: rbac.RoleRef{
						APIGroup: rbac.GroupName,
						Kind:     "ClusterRole",
						Name:     wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName),
					},
					Subjects: []rbac.Subject{
						{
							Kind:     "User",
							Name:     "newUser", // verify update with new user
							APIGroup: rbac.GroupName,
						},
					}})
				return mock
			},
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
				mock.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "GlobalRoleBinding",
								Name:       grbName,
								UID:        grbUID,
							},
						},
					},
					RoleRef: rbac.RoleRef{
						APIGroup: rbac.GroupName,
						Kind:     "ClusterRole",
						Name:     wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName),
					},
					Subjects: []rbac.Subject{
						{
							Kind:     "User",
							Name:     user,
							APIGroup: rbac.GroupName,
						},
					}}, nil)
				return mock
			},
			fwCache: fleetDefaultAndLocalWorkspaceCacheMock(ctrl),
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: grbName,
					UID:  grbUID,
				},
				UserName:       "newUser",
				GlobalRoleName: grName,
			},
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := fleetWorkspaceBindingHandler{
				crbClient: test.crbClient(),
				crbCache:  test.crbCache(),
				grCache:   test.grCache(),
				rbClient:  test.rbClient(),
				rbCache:   test.rbCache(),
				fwCache:   test.fwCache(),
			}

			err := h.reconcileFleetWorkspacePermissionsBindings(test.grb)

			assert.Equal(t, err, nil)
		})
	}
}

func TestReconcileFleetWorkspacePermissionsBindings_errors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := map[string]struct {
		crClient       func() rbacv1.ClusterRoleController
		crCache        func() rbacv1.ClusterRoleCache
		crbClient      func() rbacv1.ClusterRoleBindingController
		crbCache       func() rbacv1.ClusterRoleBindingCache
		grCache        func() mgmtcontroller.GlobalRoleCache
		rbClient       func() rbacv1.RoleBindingController
		rbCache        func() rbacv1.RoleBindingCache
		fwCache        func() mgmtcontroller.FleetWorkspaceCache
		wantErrMessage string
	}{
		"GlobalRole not found": {
			grCache: func() mgmtcontroller.GlobalRoleCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
				mock.EXPECT().Get(grName).Return(nil, errors.NewNotFound(schema.GroupResource{
					Group:    "management.cattle.io",
					Resource: "GlobalRole",
				}, grName))
				return mock
			},
			rbCache: func() rbacv1.RoleBindingCache {
				return fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
			},
			rbClient: func() rbacv1.RoleBindingController {
				return fake.NewMockControllerInterface[*rbac.RoleBinding, *rbac.RoleBindingList](ctrl)
			},
			crbClient: func() rbacv1.ClusterRoleBindingController {
				return fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl)
			},
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				return fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
			},
			fwCache: func() mgmtcontroller.FleetWorkspaceCache {
				return fake.NewMockNonNamespacedCacheInterface[*v3.FleetWorkspace](ctrl)
			},
			wantErrMessage: "unable to get globalRole: GlobalRole.management.cattle.io \"gr\" not found",
		},
		"Error retrieving fleetworkspaces": {
			grCache: globalRoleMock(ctrl),
			fwCache: func() mgmtcontroller.FleetWorkspaceCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*v3.FleetWorkspace](ctrl)
				mock.EXPECT().List(labels.Everything()).Return(nil, errors.NewServiceUnavailable("unexpected error"))
				return mock
			},
			rbCache: func() rbacv1.RoleBindingCache {
				return fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
			},
			rbClient: func() rbacv1.RoleBindingController {
				return fake.NewMockControllerInterface[*rbac.RoleBinding, *rbac.RoleBindingList](ctrl)
			},
			crbClient: func() rbacv1.ClusterRoleBindingController {
				return fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl)
			},
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				return fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
			},
			wantErrMessage: "unable to list fleetWorkspaces when reconciling globalRoleBinding grb: unexpected error",
		},
		"Error creating backing RoleBindings for permission rules": {
			grCache: globalRoleMock(ctrl),
			fwCache: fleetDefaultAndLocalWorkspaceCacheMock(ctrl),
			rbCache: func() rbacv1.RoleBindingCache {
				mock := fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
				mock.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			rbClient: func() rbacv1.RoleBindingController {
				mock := fake.NewMockControllerInterface[*rbac.RoleBinding, *rbac.RoleBindingList](ctrl)
				mock.EXPECT().Create(&rbac.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      grbName,
						Namespace: "fleet-default",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "GlobalRoleBinding",
								Name:       grbName,
								UID:        grbUID,
							},
						},
						Labels: map[string]string{
							grbOwnerLabel:                 grbName,
							fleetWorkspacePermissionLabel: "true",
							controllers.K8sManagedByKey:   controllers.ManagerValue,
						},
					},
					RoleRef: rbac.RoleRef{
						APIGroup: rbac.GroupName,
						Kind:     "ClusterRole",
						Name:     wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName),
					},
					Subjects: []rbac.Subject{
						{
							Kind:     "User",
							Name:     user,
							APIGroup: rbac.GroupName,
						},
					},
				}).Return(nil, errors.NewServiceUnavailable("unexpected error"))
				return mock
			},
			crbClient: func() rbacv1.ClusterRoleBindingController {
				return fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl)
			},
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				return fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
			},
			wantErrMessage: "error reconciling fleet permissions rules: 1 error occurred:\n\t* unexpected error\n\n",
		},
		"Error updating backing RoleBindings for permission rules": {
			grCache: globalRoleMock(ctrl),
			fwCache: fleetDefaultAndLocalWorkspaceCacheMock(ctrl),
			rbCache: func() rbacv1.RoleBindingCache {
				mock := fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
				mock.EXPECT().Get("fleet-default", grbName).Return(&rbac.RoleBinding{}, nil)
				return mock
			},
			rbClient: func() rbacv1.RoleBindingController {
				mock := fake.NewMockControllerInterface[*rbac.RoleBinding, *rbac.RoleBindingList](ctrl)
				mock.EXPECT().Update(&rbac.RoleBinding{
					RoleRef: rbac.RoleRef{
						APIGroup: rbac.GroupName,
						Kind:     "ClusterRole",
						Name:     wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName),
					},
					Subjects: []rbac.Subject{
						{
							Kind:     "User",
							Name:     user,
							APIGroup: rbac.GroupName,
						},
					},
				}).Return(nil, errors.NewServiceUnavailable("unexpected error"))
				return mock
			},
			crbClient: func() rbacv1.ClusterRoleBindingController {
				return fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl)
			},
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				return fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
			},
			wantErrMessage: "error reconciling fleet permissions rules",
		},
		"Error creating backing RoleBindings for workspace verbs": {
			grCache: globalRoleMock(ctrl),
			fwCache: fleetDefaultAndLocalWorkspaceCacheMock(ctrl),
			rbCache: func() rbacv1.RoleBindingCache {
				mock := fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
				mock.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			rbClient: createRoleBindingsMock(ctrl),
			crbClient: func() rbacv1.ClusterRoleBindingController {
				mock := fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl)
				mock.EXPECT().Create(&rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "GlobalRoleBinding",
								Name:       grbName,
								UID:        grbUID,
							},
						},
						Labels: map[string]string{
							grbOwnerLabel:               grbName,
							controllers.K8sManagedByKey: controllers.ManagerValue,
						},
					},
					RoleRef: rbac.RoleRef{
						APIGroup: rbac.GroupName,
						Kind:     "ClusterRole",
						Name:     wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName),
					},
					Subjects: []rbac.Subject{
						{
							Kind:     "User",
							Name:     user,
							APIGroup: rbac.GroupName,
						},
					}}).Return(nil, errors.NewServiceUnavailable("unexpected error"))

				return mock
			},
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
				mock.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			wantErrMessage: "error reconciling fleet workspace verbs: unexpected error",
		},
		"Error updating backing RoleBindings for workspace verbs": {
			grCache: globalRoleMock(ctrl),
			fwCache: fleetDefaultAndLocalWorkspaceCacheMock(ctrl),
			rbCache: func() rbacv1.RoleBindingCache {
				mock := fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
				mock.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			rbClient: createRoleBindingsMock(ctrl),
			crbClient: func() rbacv1.ClusterRoleBindingController {
				mock := fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl)
				mock.EXPECT().Update(&rbac.ClusterRoleBinding{
					RoleRef: rbac.RoleRef{
						APIGroup: rbac.GroupName,
						Kind:     "ClusterRole",
						Name:     wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName),
					},
					Subjects: []rbac.Subject{
						{
							Kind:     "User",
							Name:     user,
							APIGroup: rbac.GroupName,
						},
					}}).Return(nil, errors.NewServiceUnavailable("unexpected error"))

				return mock
			},
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
				mock.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRoleBinding{}, nil)
				return mock
			},
			wantErrMessage: "error reconciling fleet workspace verbs",
		},
		"Error getting RoleBinding": {
			grCache: globalRoleMock(ctrl),
			fwCache: fleetDefaultAndLocalWorkspaceCacheMock(ctrl),
			rbCache: func() rbacv1.RoleBindingCache {
				mock := fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
				mock.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewServiceUnavailable("unexpected error"))
				return mock
			},
			rbClient: func() rbacv1.RoleBindingController {
				return fake.NewMockControllerInterface[*rbac.RoleBinding, *rbac.RoleBindingList](ctrl)
			},
			crbClient: func() rbacv1.ClusterRoleBindingController {
				return fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl)
			},
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				return fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
			},
			wantErrMessage: "error reconciling fleet permissions rules",
		},
		"Error getting ClusterRoleBinding": {
			grCache: globalRoleMock(ctrl),
			fwCache: fleetDefaultAndLocalWorkspaceCacheMock(ctrl),
			rbCache: func() rbacv1.RoleBindingCache {
				mock := fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
				mock.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			rbClient: createRoleBindingsMock(ctrl),
			crbClient: func() rbacv1.ClusterRoleBindingController {
				return fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl)
			},
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
				mock.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewServiceUnavailable("unexpected error"))
				return mock
			},
			wantErrMessage: "error reconciling fleet workspace verbs",
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := fleetWorkspaceBindingHandler{
				crbClient: test.crbClient(),
				crbCache:  test.crbCache(),
				grCache:   test.grCache(),
				rbClient:  test.rbClient(),
				rbCache:   test.rbCache(),
				fwCache:   test.fwCache(),
			}

			err := h.reconcileFleetWorkspacePermissionsBindings(&v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: grbName,
					UID:  grbUID,
				},
				UserName:       user,
				GlobalRoleName: grName,
			})

			assert.ErrorContains(t, err, test.wantErrMessage)
		})
	}
}

func createClusterRoleBindingsMock(ctrl *gomock.Controller) func() rbacv1.ClusterRoleBindingController {
	return func() rbacv1.ClusterRoleBindingController {
		mock := fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl)
		mock.EXPECT().Create(&rbac.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRoleBinding",
						Name:       grbName,
						UID:        grbUID,
					},
				},
				Labels: map[string]string{
					grbOwnerLabel:               grbName,
					controllers.K8sManagedByKey: controllers.ManagerValue,
				},
			},
			RoleRef: rbac.RoleRef{
				APIGroup: rbac.GroupName,
				Kind:     "ClusterRole",
				Name:     wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName),
			},
			Subjects: []rbac.Subject{
				{
					Kind:     "User",
					Name:     user,
					APIGroup: rbac.GroupName,
				},
			}})
		return mock
	}
}

func createRoleBindingsMock(ctrl *gomock.Controller) func() rbacv1.RoleBindingController {
	return func() rbacv1.RoleBindingController {
		mock := fake.NewMockControllerInterface[*rbac.RoleBinding, *rbac.RoleBindingList](ctrl)
		mock.EXPECT().Create(&rbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grbName,
				Namespace: "fleet-default",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRoleBinding",
						Name:       grbName,
						UID:        grbUID,
					},
				},
				Labels: map[string]string{
					grbOwnerLabel:                 grbName,
					fleetWorkspacePermissionLabel: "true",
					controllers.K8sManagedByKey:   controllers.ManagerValue,
				},
			},
			RoleRef: rbac.RoleRef{
				APIGroup: rbac.GroupName,
				Kind:     "ClusterRole",
				Name:     wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName),
			},
			Subjects: []rbac.Subject{
				{
					Kind:     "User",
					Name:     user,
					APIGroup: rbac.GroupName,
				},
			},
		})
		return mock
	}
}

func fleetDefaultAndLocalWorkspaceCacheMock(ctrl *gomock.Controller) func() mgmtcontroller.FleetWorkspaceCache {
	return func() mgmtcontroller.FleetWorkspaceCache {
		mock := fake.NewMockNonNamespacedCacheInterface[*v3.FleetWorkspace](ctrl)
		mock.EXPECT().List(labels.Everything()).Return([]*v3.FleetWorkspace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fleet-local",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fleet-default",
				},
			},
		}, nil)
		return mock
	}
}

func globalRoleMock(ctrl *gomock.Controller) func() mgmtcontroller.GlobalRoleCache {
	return func() mgmtcontroller.GlobalRoleCache {
		mock := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
		mock.EXPECT().Get(grName).Return(&v3.GlobalRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: grName,
				UID:  grUID,
			},
			InheritedFleetWorkspacePermissions: v3.FleetWorkspacePermission{
				ResourceRules: []rbac.PolicyRule{
					{
						Verbs:     []string{"get", "list"},
						APIGroups: []string{"fleet.cattle.io"},
						Resources: []string{"gitrepos", "bundles"},
					},
				},
				WorkspaceVerbs: []string{"get", "list"},
			},
		}, nil)
		return mock
	}
}
