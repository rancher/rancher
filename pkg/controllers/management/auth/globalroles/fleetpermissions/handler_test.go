package fleetpermissions

import (
	"testing"

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
	grUID   = "abcd"
	grbName = "grb"
	grbUID  = "efdj"
	user    = "user"
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
		crClient  func() rbacv1.ClusterRoleController
		crCache   func() rbacv1.ClusterRoleCache
		crbClient func() rbacv1.ClusterRoleBindingController
		crbCache  func() rbacv1.ClusterRoleBindingCache
		grCache   func() mgmtcontroller.GlobalRoleCache
		rbClient  func() rbacv1.RoleBindingController
		rbCache   func() rbacv1.RoleBindingCache
	}{
		"backing Roles and ClusterRoles are created for a new GlobalRoleBinding": {
			grCache: globalRoleMock(ctrl),
			crCache: func() rbacv1.ClusterRoleCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl)
				mock.EXPECT().Get("fwcr-"+grName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				mock.EXPECT().Get("fwv-"+grName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			crClient: createClusterRolesMock(ctrl),
			rbCache: func() rbacv1.RoleBindingCache {
				mock := fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
				mock.EXPECT().Get("fleet-default", user+"-fwcr-"+grName+"-fleet-default").Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			rbClient:  createRoleBindingsMock(ctrl),
			crbClient: createClusterRoleBindingsMock(ctrl),
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
				mock.EXPECT().Get("fwv-gr-user").Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
		},
		/*
				There are 2 ClusterRoles created for each GlobalRole: one for resourcesRules and another for workspaceVerbs. These
				roles are created just once when the first GlobalRoleBinding is created.
			    This test case verifies it is not recreated if they are already present in the cluster.
		*/
		"backing Roles are created, however ClusterRoles are not created if already present": {
			grCache: globalRoleMock(ctrl),
			crCache: clusterRoleMock(ctrl),
			crClient: func() rbacv1.ClusterRoleController {
				return fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl)
			},
			rbCache: func() rbacv1.RoleBindingCache {
				mock := fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
				mock.EXPECT().Get("fleet-default", user+"-fwcr-"+grName+"-fleet-default").Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			rbClient:  createRoleBindingsMock(ctrl),
			crbClient: createClusterRoleBindingsMock(ctrl),
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
				mock.EXPECT().Get("fwv-gr-user").Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
		},
		"backing Roles and ClusterRoles are updated with new content": {
			grCache: func() mgmtcontroller.GlobalRoleCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
				mock.EXPECT().Get(grName).Return(&v3.GlobalRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: grName,
						UID:  grUID,
					},
					InheritedFleetWorkspacePermissions: v3.FleetWorkspacePermission{
						ResourceRules: []rbac.PolicyRule{ // simulates it has changed
							{
								Verbs:     []string{"*"},
								APIGroups: []string{"fleet.cattle.io"},
								Resources: []string{"gitrepos"},
							},
						},
						WorkspaceVerbs: []string{"*"}, // simulates it has changed
					},
				}, nil)
				return mock
			},
			crCache: clusterRoleMock(ctrl),
			crClient: func() rbacv1.ClusterRoleController {
				mock := fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl)
				// expect updates with new rules
				mock.EXPECT().Update(&rbac.ClusterRole{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: "fwcr-" + grName,
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
						Name: "fwv-" + grName,
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
			rbCache: func() rbacv1.RoleBindingCache {
				mock := fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
				mock.EXPECT().Get("fleet-default", user+"-fwcr-"+grName+"-fleet-default").Return(&rbac.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      user + "-fwcr-" + grName + "-fleet-default",
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
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     "fwcr-" + grName,
					},
					Subjects: []rbac.Subject{
						{
							Kind:     "User",
							Name:     "newUser", // simulates user has changed
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
						Name:      user + "-fwcr-" + grName + "-fleet-default",
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
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     "fwcr-" + grName,
					},
					Subjects: []rbac.Subject{
						{
							Kind:     "User",
							Name:     user, // verify update with original user
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
						Name: "fwv-gr-user",
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
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     "fwv-" + grName,
					},
					Subjects: []rbac.Subject{
						{
							Kind:     "User",
							Name:     user, // verify update with original user
							APIGroup: rbac.GroupName,
						},
					}})
				return mock
			},
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
				mock.EXPECT().Get("fwv-gr-user").Return(&rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fwv-gr-user",
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
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     "fwv-" + grName,
					},
					Subjects: []rbac.Subject{
						{
							Kind:     "User",
							Name:     "newUser", // simulate user has changed
							APIGroup: rbac.GroupName,
						},
					}}, nil)
				return mock
			},
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := Handler{
				crClient:  test.crClient(),
				crCache:   test.crCache(),
				crbClient: test.crbClient(),
				crbCache:  test.crbCache(),
				grCache:   test.grCache(),
				rbClient:  test.rbClient(),
				rbCache:   test.rbCache(),
				fwCache:   fleetDefaultAndLocalWorkspaceCacheMock(ctrl),
			}

			err := h.ReconcileFleetWorkspacePermissions(&v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: grbName,
					UID:  grbUID,
				},
				UserName:       user,
				GlobalRoleName: grName,
			})

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
			crCache: func() rbacv1.ClusterRoleCache {
				return fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl)
			},
			crClient: func() rbacv1.ClusterRoleController {
				return fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl)
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
			crCache: func() rbacv1.ClusterRoleCache {
				return fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl)
			},
			crClient: func() rbacv1.ClusterRoleController {
				return fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl)
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
			fwCache: func() mgmtcontroller.FleetWorkspaceCache {
				return fleetDefaultAndLocalWorkspaceCacheMock(ctrl)
			},
			crCache: func() rbacv1.ClusterRoleCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl)
				mock.EXPECT().Get("fwcr-"+grName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			crClient: func() rbacv1.ClusterRoleController {
				mock := fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl)
				mock.EXPECT().Create(&rbac.ClusterRole{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: "fwcr-" + grName,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "GlobalRole",
								Name:       grName,
								UID:        grUID,
							},
						},
						Labels: map[string]string{
							GRBFleetWorkspaceOwnerLabel: grbName,
							controllers.K8sManagedByKey: controllers.ManagerValue,
						},
					},
					Rules: resourceRules,
				}).Return(nil, errors.NewServiceUnavailable("unexpected error"))
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
			wantErrMessage: "error reconciling fleet permissions rules: unexpected error",
		},
		"Error creating backing RoleBindings for workspace verbs": {
			grCache: globalRoleMock(ctrl),
			fwCache: func() mgmtcontroller.FleetWorkspaceCache {
				return fleetDefaultAndLocalWorkspaceCacheMock(ctrl)
			},
			crCache: func() rbacv1.ClusterRoleCache {
				mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl)
				mock.EXPECT().Get("fwcr-"+grName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				mock.EXPECT().Get("fwv-"+grName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			crClient: func() rbacv1.ClusterRoleController {
				mock := fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl)
				mock.EXPECT().Create(&rbac.ClusterRole{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: "fwcr-" + grName,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "GlobalRole",
								Name:       grName,
								UID:        grUID,
							},
						},
						Labels: map[string]string{
							GRBFleetWorkspaceOwnerLabel: grbName,
							controllers.K8sManagedByKey: controllers.ManagerValue,
						},
					},
					Rules: resourceRules,
				})
				mock.EXPECT().Create(&rbac.ClusterRole{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: "fwv-" + grName,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "GlobalRole",
								Name:       grName,
								UID:        grUID,
							},
						},
						Labels: map[string]string{
							GRBFleetWorkspaceOwnerLabel: grbName,
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
				}).Return(nil, errors.NewServiceUnavailable("unexpected error"))
				return mock
			},
			rbCache: func() rbacv1.RoleBindingCache {
				mock := fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl)
				mock.EXPECT().Get("fleet-default", user+"-fwcr-"+grName+"-fleet-default").Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				return mock
			},
			rbClient: createRoleBindingsMock(ctrl),
			crbClient: func() rbacv1.ClusterRoleBindingController {
				return fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl)
			},
			crbCache: func() rbacv1.ClusterRoleBindingCache {
				return fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl)
			},
			wantErrMessage: "error reconciling fleet workspace verbs: unexpected error",
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := Handler{
				crClient:  test.crClient(),
				crCache:   test.crCache(),
				crbClient: test.crbClient(),
				crbCache:  test.crbCache(),
				grCache:   test.grCache(),
				rbClient:  test.rbClient(),
				rbCache:   test.rbCache(),
				fwCache:   test.fwCache(),
			}

			err := h.ReconcileFleetWorkspacePermissions(&v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: grbName,
					UID:  grbUID,
				},
				UserName:       user,
				GlobalRoleName: grName,
			})

			assert.EqualError(t, err, test.wantErrMessage)
		})
	}
}

func createClusterRoleBindingsMock(ctrl *gomock.Controller) func() rbacv1.ClusterRoleBindingController {
	return func() rbacv1.ClusterRoleBindingController {
		mock := fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl)
		mock.EXPECT().Create(&rbac.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fwv-gr-user",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRoleBinding",
						Name:       grbName,
						UID:        grbUID,
					},
				},
				Labels: map[string]string{
					GRBFleetWorkspaceOwnerLabel: grbName,
					controllers.K8sManagedByKey: controllers.ManagerValue,
				},
			},
			RoleRef: rbac.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "fwv-" + grName,
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
				Name:      user + "-fwcr-" + grName + "-fleet-default",
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
					GRBFleetWorkspaceOwnerLabel: grbName,
					controllers.K8sManagedByKey: controllers.ManagerValue,
				},
			},
			RoleRef: rbac.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "fwcr-" + grName,
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

func createClusterRolesMock(ctrl *gomock.Controller) func() rbacv1.ClusterRoleController {
	return func() rbacv1.ClusterRoleController {
		mock := fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl)
		mock.EXPECT().Create(&rbac.ClusterRole{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "fwcr-" + grName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRole",
						Name:       grName,
						UID:        grUID,
					},
				},
				Labels: map[string]string{
					GRBFleetWorkspaceOwnerLabel: grbName,
					controllers.K8sManagedByKey: controllers.ManagerValue,
				},
			},
			Rules: resourceRules,
		})
		mock.EXPECT().Create(&rbac.ClusterRole{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "fwv-" + grName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "GlobalRole",
						Name:       grName,
						UID:        grUID,
					},
				},
				Labels: map[string]string{
					GRBFleetWorkspaceOwnerLabel: grbName,
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

func fleetDefaultAndLocalWorkspaceCacheMock(ctrl *gomock.Controller) mgmtcontroller.FleetWorkspaceCache {
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

func globalRoleMock(ctrl *gomock.Controller) func() mgmtcontroller.GlobalRoleCache {
	return func() mgmtcontroller.GlobalRoleCache {
		mock := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
		mock.EXPECT().Get(grName).Return(&v3.GlobalRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: grName,
				UID:  grUID,
			},
			InheritedFleetWorkspacePermissions: v3.FleetWorkspacePermission{
				ResourceRules:  resourceRules,
				WorkspaceVerbs: workspaceVerbs,
			},
		}, nil)
		return mock
	}
}

func clusterRoleMock(ctrl *gomock.Controller) func() rbacv1.ClusterRoleCache {
	return func() rbacv1.ClusterRoleCache {
		mock := fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl)
		mock.EXPECT().Get("fwcr-"+grName).Return(&rbac.ClusterRole{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "fwcr-" + grName,
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
		mock.EXPECT().Get("fwv-"+grName).Return(&rbac.ClusterRole{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "fwv-" + grName,
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
