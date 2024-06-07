package rbac

import (
	"errors"
	"fmt"
	"io"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	typesrbacv1fakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type prtbTestState struct {
	managerMock *MockmanagerInterface
}

func TestReconcileProjectAccessToGlobalResources(t *testing.T) {
	t.Parallel()

	defaultPRTB := v3.ProjectRoleTemplateBinding{
		ProjectName: "default",
	}

	tests := []struct {
		name       string
		stateSetup func(prtbTestState)
		prtb       *v3.ProjectRoleTemplateBinding
		rts        map[string]*v3.RoleTemplate
		wantError  bool
	}{
		{
			name: "error ensuring global resource roles",
			stateSetup: func(pts prtbTestState) {
				pts.managerMock.EXPECT().ensureGlobalResourcesRolesForPRTB(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
			prtb:      defaultPRTB.DeepCopy(),
			rts:       nil,
			wantError: true,
		},
		{
			name: "error reconciling access to global resources",
			stateSetup: func(pts prtbTestState) {
				pts.managerMock.EXPECT().ensureGlobalResourcesRolesForPRTB(gomock.Any(), gomock.Any()).Return(nil, nil)
				pts.managerMock.EXPECT().reconcileProjectAccessToGlobalResources(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
			prtb:      defaultPRTB.DeepCopy(),
			rts:       nil,
			wantError: true,
		},
		{
			name: "success",
			stateSetup: func(pts prtbTestState) {
				pts.managerMock.EXPECT().ensureGlobalResourcesRolesForPRTB(gomock.Any(), gomock.Any()).Return(nil, nil)
				pts.managerMock.EXPECT().reconcileProjectAccessToGlobalResources(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			prtb: defaultPRTB.DeepCopy(),
			rts:  nil,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			prtbLifecycle := prtbLifecycle{}
			state := setupPRTBTest(t)
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			prtbLifecycle.m = state.managerMock

			err := prtbLifecycle.reconcileProjectAccessToGlobalResources(test.prtb, test.rts)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func setupPRTBTest(t *testing.T) prtbTestState {
	ctrl := gomock.NewController(t)
	managerMock := NewMockmanagerInterface(ctrl)
	state := prtbTestState{
		managerMock: managerMock,
	}
	return state
}

func Test_manager_checkForGlobalResourceRules(t *testing.T) {
	type tests struct {
		name     string
		role     *v3.RoleTemplate
		resource string
		baseRule rbacv1.PolicyRule
		want     sets.Set[string]
	}

	testCases := []tests{
		{
			name: "valid_api_group_persistentvolumes",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{""},
						Resources: []string{"persistentvolumes"},
					},
				},
			},
			resource: "persistentvolumes",
			baseRule: rbacv1.PolicyRule{},
			want:     sets.New("put"),
		},
		{
			name: "invalid_api_group_persistentvolumes",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{"foo"},
						Resources: []string{"persistentvolumes"},
					},
				},
			},
			resource: "persistentvolumes",
			baseRule: rbacv1.PolicyRule{},
			want:     sets.New[string](),
		},
		{
			name: "valid_api_group_storageclasses",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{"storage.k8s.io"},
						Resources: []string{"storageclasses"},
					},
				},
			},
			resource: "storageclasses",
			baseRule: rbacv1.PolicyRule{},
			want:     sets.New("put"),
		},
		{
			name: "invalid_api_group_storageclasses",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{"foo"},
						Resources: []string{"storageclasses"},
					},
				},
			},
			resource: "storageclasses",
			baseRule: rbacv1.PolicyRule{},
			want:     sets.New[string](),
		},
		{
			name: "valid_api_group_start",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{""},
						Resources: []string{"*"},
					},
				},
			},
			resource: "persistentvolumes",
			baseRule: rbacv1.PolicyRule{},
			want:     sets.New("put"),
		},
		{
			name: "invalid_api_group_star",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"put"},
						APIGroups: []string{"foo"},
						Resources: []string{"*"},
					},
				},
			},
			resource: "persistentvolumes",
			baseRule: rbacv1.PolicyRule{},
			want:     sets.New[string](),
		},
		{
			name: "cluster_rule_match",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{"management.cattle.io"},
						Resources: []string{"clusters"},
					},
				},
			},
			resource: "clusters",
			baseRule: rbacv1.PolicyRule{},
			want:     sets.New("get"),
		},
		{
			name: "cluster_rule_resource_names_match",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:         []string{"get"},
						APIGroups:     []string{"management.cattle.io"},
						Resources:     []string{"clusters"},
						ResourceNames: []string{"local"},
					},
				},
			},
			resource: "clusters",
			baseRule: rbacv1.PolicyRule{
				ResourceNames: []string{"local"},
			},
			want: sets.New("get"),
		},
		{
			name: "cluster_rule_baserule_resource_names_no_match",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{"management.cattle.io"},
						Resources: []string{"clusters"},
					},
				},
			},
			resource: "clusters",
			baseRule: rbacv1.PolicyRule{
				ResourceNames: []string{"local"},
			},
			want: sets.New[string](),
		},
		{
			name: "cluster_rule_roletemplate_resource_names_no_match",
			role: &v3.RoleTemplate{
				Rules: []v1.PolicyRule{
					{
						Verbs:         []string{"get"},
						APIGroups:     []string{"management.cattle.io"},
						Resources:     []string{"clusters"},
						ResourceNames: []string{"local"},
					},
				},
			},
			resource: "clusters",
			baseRule: rbacv1.PolicyRule{},
			want:     sets.New[string](),
		},
	}

	m := &manager{}

	for _, test := range testCases {
		got, err := m.checkForGlobalResourceRules(test.role, test.resource, test.baseRule)
		assert.Nil(t, err)
		assert.Equal(t, test.want, got, fmt.Sprintf("test %v failed", test.name))
	}
}

func Test_manager_reconcileRoleForProjectAccessToGlobalResource(t *testing.T) {
	// discard logs to avoid cluttering
	logrus.SetOutput(io.Discard)

	type args struct {
		resource string
		rtName   string
		newVerbs sets.Set[string]
		baseRule rbacv1.PolicyRule
	}

	tests := []struct {
		name                         string
		crListerMockGetResult        *v1.ClusterRole
		crListerMockGetErr           error
		clusterRolesMockCreateResult *v1.ClusterRole
		clusterRolesMockCreateErr    error
		clusterRolesMockUpdateResult *v1.ClusterRole
		clusterRolesMockUpdateErr    error
		args                         args
		want                         string
		wantErr                      bool
	}{
		{
			name: "missing role name will fail",
			args: args{
				rtName: "",
			},
			wantErr: true,
		},
		{
			name: "failing create will return an error",
			args: args{
				rtName:   "myrole",
				newVerbs: sets.New("get", "list"),
			},
			crListerMockGetErr:        errNotFound,
			clusterRolesMockCreateErr: errors.New("something bad happened"),
			wantErr:                   true,
		},
		{
			name: "reconciling non existing ClusterRole with no verbs is no-op",
			args: args{
				rtName:   "myrole",
				newVerbs: sets.New[string](),
			},
			crListerMockGetErr: errNotFound,
			want:               "",
		},
		{
			name: "existing ClusterRole will not update if no need to reconcile",
			args: args{
				rtName:   "myrole",
				resource: "myresource",
				newVerbs: sets.New("list", "delete"),
				baseRule: rbacv1.PolicyRule{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
			crListerMockGetResult: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrole-promoted",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						ResourceNames: []string{"local"},
						Resources:     []string{"myresource"},
						Verbs:         []string{"list", "delete"},
					},
				},
			},
			want: "myrole-promoted",
		},
		{
			name: "non existing ClusterRole will create a new promoted ClusterRole",
			args: args{
				rtName:   "myrole",
				resource: "myresource",
				newVerbs: sets.New("get", "list"),
				baseRule: rbacv1.PolicyRule{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
			crListerMockGetErr: errNotFound,
			clusterRolesMockCreateResult: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrole-promoted",
				},
				Rules: []rbacv1.PolicyRule{{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
					Resources:     []string{"myresource"},
					Verbs:         []string{"get", "list"},
				}},
			},
			want: "myrole-promoted",
		},
		{
			name: "existing ClusterRole will update it adding missing rules",
			args: args{
				rtName:   "myrole",
				resource: "myresource",
				newVerbs: sets.New("list", "delete"),
				baseRule: rbacv1.PolicyRule{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
			crListerMockGetResult: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrole-promoted",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						ResourceNames: []string{"local"},
						Resources:     []string{"another"},
						Verbs:         []string{"get", "list"},
					},
				},
			},
			clusterRolesMockUpdateResult: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrole-promoted",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						ResourceNames: []string{"local"},
						Resources:     []string{"another"},
						Verbs:         []string{"get", "list"},
					},
					{
						APIGroups:     []string{"management.cattle.io"},
						ResourceNames: []string{"local"},
						Resources:     []string{"myresource"},
						Verbs:         []string{"delete", "list"},
					},
				},
			},
			want: "myrole-promoted",
		},
		{
			name: "existing ClusterRole will update it",
			args: args{
				rtName:   "myrole",
				resource: "myresource",
				newVerbs: sets.New("list", "delete"),
				baseRule: rbacv1.PolicyRule{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
			crListerMockGetResult: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrole-promoted",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"another.cattle.io"},
						ResourceNames: []string{"foobar"},
						Resources:     []string{"nodes"},
						Verbs:         []string{"create"},
					},
					{
						APIGroups:     []string{"management.cattle.io"},
						ResourceNames: []string{"local"},
						Resources:     []string{"myresource"},
						Verbs:         []string{"get", "list"},
					},
				},
			},
			clusterRolesMockUpdateResult: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrole-promoted",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"another.cattle.io"},
						ResourceNames: []string{"foobar"},
						Resources:     []string{"nodes"},
						Verbs:         []string{"create"},
					},
					{
						APIGroups:     []string{"management.cattle.io"},
						ResourceNames: []string{"local"},
						Resources:     []string{"myresource"},
						Verbs:         []string{"delete", "list"},
					},
				},
			},
			want: "myrole-promoted",
		},
		{
			name: "removing verbs from policy will remove the rule",
			args: args{
				rtName:   "myrole",
				resource: "myresource",
				newVerbs: sets.New[string](),
				baseRule: rbacv1.PolicyRule{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
			crListerMockGetResult: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrole-promoted",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"another.cattle.io"},
						ResourceNames: []string{"foobar"},
						Resources:     []string{"nodes"},
						Verbs:         []string{"create"},
					},
					{
						APIGroups:     []string{"management.cattle.io"},
						ResourceNames: []string{"local"},
						Resources:     []string{"myresource"},
						Verbs:         []string{"get", "list"},
					},
				},
			},
			clusterRolesMockUpdateResult: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrole-promoted",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"another.cattle.io"},
						ResourceNames: []string{"foobar"},
						Resources:     []string{"nodes"},
						Verbs:         []string{"create"},
					},
				},
			},
			want: "myrole-promoted",
		},
		{
			name: "get fail will return an error",
			args: args{
				rtName:   "myrole",
				resource: "myresource",
				newVerbs: sets.New("get", "list"),
				baseRule: rbacv1.PolicyRule{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
			crListerMockGetErr: errors.New("get failed"),
			wantErr:            true,
		},
		{
			name: "update fail will return an error",
			args: args{
				rtName:   "myrole",
				resource: "myresource",
				newVerbs: sets.New("list", "delete"),
				baseRule: rbacv1.PolicyRule{
					APIGroups:     []string{"management.cattle.io"},
					ResourceNames: []string{"local"},
				},
			},
			crListerMockGetResult: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrole-promoted",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						ResourceNames: []string{"local"},
						Resources:     []string{"myresource"},
						Verbs:         []string{"get", "list"},
					},
				},
			},
			clusterRolesMockUpdateErr: errors.New("something bad happened"),
			wantErr:                   true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// setup ClusterRoleLister mock
			crListerMock := &typesrbacv1fakes.ClusterRoleListerMock{
				GetFunc: func(namespace, name string) (*v1.ClusterRole, error) {
					return tc.crListerMockGetResult, tc.crListerMockGetErr
				},
			}

			// setup ClusterRole mock: it will just return the passed ClusterRole or the error
			clusterRolesMock := &typesrbacv1fakes.ClusterRoleInterfaceMock{
				CreateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					return in1, tc.clusterRolesMockCreateErr
				},
				UpdateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					return in1, tc.clusterRolesMockUpdateErr
				},
			}

			manager := manager{
				crLister:     crListerMock,
				clusterRoles: clusterRolesMock,
			}

			got, err := manager.reconcileRoleForProjectAccessToGlobalResource(tc.args.resource, tc.args.rtName, tc.args.newVerbs, tc.args.baseRule)
			assert.Equal(t, tc.want, got)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// if result and err are nil the method should have not been called
			if tc.crListerMockGetResult == nil && tc.crListerMockGetErr == nil {
				assert.Empty(t, crListerMock.GetCalls())
			} else {
				// otherwise only one call to Get is expected
				assert.Len(t, crListerMock.GetCalls(), 1)
			}

			// if result and err are nil the method should have not been called
			if tc.clusterRolesMockCreateResult == nil && tc.clusterRolesMockCreateErr == nil {
				assert.Empty(t, clusterRolesMock.CreateCalls())
			} else {
				// otherwise only one call to Get is expected, and the values should match
				results := clusterRolesMock.CreateCalls()
				assert.Len(t, results, 1)

				if tc.clusterRolesMockCreateErr == nil {
					assert.Equal(t, tc.clusterRolesMockCreateResult, results[0].In1)
				}
			}

			// if result and err are nil the method should have not been called
			if tc.clusterRolesMockUpdateResult == nil && tc.clusterRolesMockUpdateErr == nil {
				assert.Empty(t, clusterRolesMock.UpdateCalls())
			} else {
				// otherwise only one call to Update is expected, and the values should match
				results := clusterRolesMock.UpdateCalls()
				assert.Len(t, results, 1)

				if tc.clusterRolesMockUpdateErr == nil {
					assert.Equal(t, tc.clusterRolesMockUpdateResult, results[0].In1)
				}
			}
		})
	}
}
