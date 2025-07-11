package rbac

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/rancher/rancher/pkg/apis/management.cattle.io"
	apisV3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rancherv3fakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	typesrbacv1fakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

var (
	defaultRule = rbacv1.PolicyRule{
		Resources:     []string{"myresource"},
		Verbs:         []string{"list", "delete"},
		APIGroups:     []string{"management.cattle.io"},
		ResourceNames: []string{"local"},
	}
)

func Test_manager_reconcileRoleForProjectAccessToGlobalResource(t *testing.T) {
	// discard logs to avoid cluttering
	logrus.SetOutput(io.Discard)

	type args struct {
		rtName        string
		promotedRules []rbacv1.PolicyRule
	}

	tests := []struct {
		name                         string
		crListerMockGetResult        *v1.ClusterRole
		crListerMockGetErr           error
		clusterRolesMockCreateResult *v1.ClusterRole
		clusterRolesMockCreateErr    error
		clusterRolesMockUpdateResult *v1.ClusterRole
		clusterRolesMockUpdateErr    error
		clusterRolesMockDeleteErr    error
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
				rtName:        "myrole",
				promotedRules: []rbacv1.PolicyRule{defaultRule},
			},
			crListerMockGetErr:        errNotFound,
			clusterRolesMockCreateErr: errors.New("something bad happened"),
			wantErr:                   true,
		},
		{
			name: "reconciling non existing ClusterRole with no verbs is no-op",
			args: args{
				rtName:        "myrole",
				promotedRules: []rbacv1.PolicyRule{},
			},
			crListerMockGetErr: errNotFound,
			want:               "",
		},
		{
			name: "existing ClusterRole will not update if no need to reconcile",
			args: args{
				rtName:        "myrole",
				promotedRules: []rbacv1.PolicyRule{defaultRule},
			},
			crListerMockGetResult: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrole-promoted",
				},
				Rules: []rbacv1.PolicyRule{defaultRule},
			},
			want: "myrole-promoted",
		},
		{
			name: "non existing ClusterRole will create a new promoted ClusterRole",
			args: args{
				rtName:        "myrole",
				promotedRules: []rbacv1.PolicyRule{defaultRule},
			},
			crListerMockGetErr: errNotFound,
			clusterRolesMockCreateResult: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myrole-promoted",
				},
				Rules: []rbacv1.PolicyRule{defaultRule},
			},
			want: "myrole-promoted",
		},
		{
			name: "existing ClusterRole will update if adding missing rules",
			args: args{
				rtName:        "myrole",
				promotedRules: []rbacv1.PolicyRule{defaultRule},
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
				Rules: []rbacv1.PolicyRule{defaultRule},
			},
			want: "myrole-promoted",
		},
		{
			name: "clusterRole with extra rules will remove them",
			args: args{
				rtName:        "myrole",
				promotedRules: []rbacv1.PolicyRule{defaultRule},
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
				Rules: []rbacv1.PolicyRule{defaultRule},
			},
			want: "myrole-promoted",
		},
		{
			name: "removing verbs from policy will remove the rule",
			args: args{
				rtName:        "myrole",
				promotedRules: []rbacv1.PolicyRule{defaultRule},
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
				Rules: []rbacv1.PolicyRule{defaultRule},
			},
			want: "myrole-promoted",
		},
		{
			name: "get fail will return an error",
			args: args{
				rtName:        "myrole",
				promotedRules: []rbacv1.PolicyRule{defaultRule},
			},
			crListerMockGetErr: errors.New("get failed"),
			wantErr:            true,
		},
		{
			name: "update fail will return an error",
			args: args{
				rtName:        "myrole",
				promotedRules: []rbacv1.PolicyRule{defaultRule},
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
		{
			name: "promoted clusterrole exists, but no promotedrules. delete clusterrole",
			args: args{
				rtName:        "myrole",
				promotedRules: []rbacv1.PolicyRule{},
			},
			crListerMockGetErr:        errNotFound,
			clusterRolesMockDeleteErr: nil,
			wantErr:                   false,
			want:                      "",
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
				DeleteFunc: func(name string, options *metav1.DeleteOptions) error {
					return tc.clusterRolesMockDeleteErr
				},
			}

			manager := manager{
				crLister:     crListerMock,
				clusterRoles: clusterRolesMock,
			}

			got, err := manager.reconcileRoleForProjectAccessToGlobalResource(tc.args.rtName, tc.args.promotedRules)
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

			// if nil the method should have not been called
			if tc.clusterRolesMockDeleteErr == nil {
				assert.Empty(t, clusterRolesMock.DeleteCalls())
			} else {
				// otherwise only one call to Delete is expected
				results := clusterRolesMock.DeleteCalls()
				assert.Len(t, results, 1)
			}
		})
	}
}

func Test_ensurePSAPermissions(t *testing.T) {
	type args struct {
		binding *v3.ProjectRoleTemplateBinding
		roles   map[string]*v3.RoleTemplate
	}

	p := &prtbLifecycle{}

	tests := []struct {
		name      string
		mockSetup func()
		args      args
		wantErr   bool
	}{
		{
			name: "create psa rbac resources without errors",
			mockSetup: func() {
				p.crLister = &typesrbacv1fakes.ClusterRoleListerMock{
					GetFunc: func(namespace, name string) (*v1.ClusterRole, error) {
						return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
					},
				}
				p.crClient = &typesrbacv1fakes.ClusterRoleInterfaceMock{
					CreateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
						return &v1.ClusterRole{
							Rules: []rbacv1.PolicyRule{
								{
									APIGroups:     []string{management.GroupName},
									Verbs:         []string{"updatepsa"},
									Resources:     []string{apisV3.ProjectResourceName},
									ResourceNames: []string{"p-example"},
								},
							},
						}, nil
					},
				}
				p.crbClient = &typesrbacv1fakes.ClusterRoleBindingInterfaceMock{
					CreateFunc: func(in1 *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
						return &v1.ClusterRoleBinding{}, nil
					},
				}
			},
			args: args{
				binding: &apisV3.ProjectRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "prtb-example",
					},
					UserName:    "u-test1",
					ProjectName: "c-abc:p-example",
				},
				roles: map[string]*v3.RoleTemplate{
					"role_template_0": &apisV3.RoleTemplate{
						Rules: []v1.PolicyRule{
							{
								APIGroups: []string{management.GroupName},
								Verbs:     []string{"updatepsa"},
								Resources: []string{apisV3.ProjectResourceName},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "unable to create CRB",
			mockSetup: func() {
				p.crLister = &typesrbacv1fakes.ClusterRoleListerMock{
					GetFunc: func(namespace, name string) (*v1.ClusterRole, error) {
						return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
					},
				}
				p.crClient = &typesrbacv1fakes.ClusterRoleInterfaceMock{
					CreateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
						return &v1.ClusterRole{
							Rules: []rbacv1.PolicyRule{
								{
									APIGroups:     []string{management.GroupName},
									Verbs:         []string{"updatepsa"},
									Resources:     []string{apisV3.ProjectResourceName},
									ResourceNames: []string{"p-example"},
								},
							},
						}, nil
					},
				}
				p.crbClient = &typesrbacv1fakes.ClusterRoleBindingInterfaceMock{
					CreateFunc: func(in1 *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
						return nil, fmt.Errorf("error")
					},
				}
			},
			args: args{
				binding: &apisV3.ProjectRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "prtb-example",
					},
					UserName:    "u-test1",
					ProjectName: "c-abc:p-example",
				},
				roles: map[string]*v3.RoleTemplate{
					"role_template_0": &apisV3.RoleTemplate{
						Rules: []v1.PolicyRule{
							{
								APIGroups: []string{management.GroupName},
								Verbs:     []string{"updatepsa"},
								Resources: []string{apisV3.ProjectResourceName},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "unable to create CR",
			mockSetup: func() {
				p.crLister = &typesrbacv1fakes.ClusterRoleListerMock{
					GetFunc: func(namespace, name string) (*v1.ClusterRole, error) {
						return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
					},
				}
				p.crClient = &typesrbacv1fakes.ClusterRoleInterfaceMock{
					CreateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
						return nil, fmt.Errorf("error")
					},
				}
			},
			args: args{
				binding: &apisV3.ProjectRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "prtb-example",
					},
					UserName:    "u-test1",
					ProjectName: "c-abc:p-example",
				},
				roles: map[string]*v3.RoleTemplate{
					"role_template_0": &apisV3.RoleTemplate{
						Rules: []v1.PolicyRule{
							{
								APIGroups: []string{management.GroupName},
								Verbs:     []string{"updatepsa"},
								Resources: []string{apisV3.ProjectResourceName},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "unable to update CR",
			mockSetup: func() {
				p.crLister = &typesrbacv1fakes.ClusterRoleListerMock{
					GetFunc: func(namespace, name string) (*v1.ClusterRole, error) {
						return &v1.ClusterRole{
							Rules: []rbacv1.PolicyRule{
								{
									APIGroups:     []string{management.GroupName},
									Verbs:         []string{"updatepsa"},
									Resources:     []string{apisV3.ProjectResourceName},
									ResourceNames: []string{"p-example"},
								},
								{
									APIGroups:     []string{management.GroupName},
									Verbs:         []string{"get"},
									Resources:     []string{apisV3.ProjectResourceName},
									ResourceNames: []string{"p-example"},
								},
							},
						}, nil
					},
				}
				p.crClient = &typesrbacv1fakes.ClusterRoleInterfaceMock{
					UpdateFunc: func(in1 *v1.ClusterRole) (*v1.ClusterRole, error) {
						return &v1.ClusterRole{}, fmt.Errorf("error")
					},
				}
			},
			args: args{
				binding: &apisV3.ProjectRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "prtb-example",
					},
					UserName:    "u-test1",
					ProjectName: "c-abc:p-example",
				},
				roles: map[string]*v3.RoleTemplate{
					"role_template_0": &apisV3.RoleTemplate{
						Rules: []v1.PolicyRule{
							{
								APIGroups: []string{management.GroupName},
								Verbs:     []string{"updatepsa"},
								Resources: []string{apisV3.ProjectResourceName},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			if err := p.ensurePSAPermissions(tt.args.binding, tt.args.roles); (err != nil) != tt.wantErr {
				t.Errorf("prtbLifecycle.ensurePSAPermissions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_ensurePSAPermissionsDelete(t *testing.T) {
	type args struct {
		binding *v3.ProjectRoleTemplateBinding
	}

	p := &prtbLifecycle{}

	tests := []struct {
		name      string
		mockSetup func()
		args      args
		wantErr   bool
	}{
		{
			name: "clean up psa rbac resources",
			mockSetup: func() {
				p.rtLister = &rancherv3fakes.RoleTemplateListerMock{
					GetFunc: func(namespace, name string) (*apisV3.RoleTemplate, error) {
						return &apisV3.RoleTemplate{
							Rules: []v1.PolicyRule{
								{
									APIGroups: []string{management.GroupName},
									Verbs:     []string{"updatepsa"},
									Resources: []string{apisV3.ProjectResourceName},
								},
							},
						}, nil
					},
				}
				p.crClient = &typesrbacv1fakes.ClusterRoleInterfaceMock{
					DeleteFunc: func(name string, options *metav1.DeleteOptions) error {
						return nil
					},
				}
				p.crbClient = &typesrbacv1fakes.ClusterRoleBindingInterfaceMock{
					ListFunc: func(opts metav1.ListOptions) (*v1.ClusterRoleBindingList, error) {
						return &v1.ClusterRoleBindingList{}, nil
					},
					DeleteFunc: func(name string, options *metav1.DeleteOptions) error {
						return nil
					},
				}
			},
			args: args{
				binding: &apisV3.ProjectRoleTemplateBinding{
					ProjectName:      "c-abc:p-example",
					RoleTemplateName: "role_template_0",
					UserName:         "u-zxv",
				},
			},
			wantErr: false,
		},
		{
			name: "unable to delete CRB",
			mockSetup: func() {
				p.rtLister = &rancherv3fakes.RoleTemplateListerMock{
					GetFunc: func(namespace, name string) (*apisV3.RoleTemplate, error) {
						return &apisV3.RoleTemplate{
							Rules: []v1.PolicyRule{
								{
									APIGroups: []string{management.GroupName},
									Verbs:     []string{"updatepsa"},
									Resources: []string{apisV3.ProjectResourceName},
								},
							},
						}, nil
					},
				}
				p.crClient = &typesrbacv1fakes.ClusterRoleInterfaceMock{
					DeleteFunc: func(name string, options *metav1.DeleteOptions) error {
						return nil
					},
				}
				p.crbClient = &typesrbacv1fakes.ClusterRoleBindingInterfaceMock{
					ListFunc: func(opts metav1.ListOptions) (*v1.ClusterRoleBindingList, error) {
						return &v1.ClusterRoleBindingList{}, nil
					},
					DeleteFunc: func(name string, options *metav1.DeleteOptions) error {
						return fmt.Errorf("error")
					},
				}
			},
			args: args{
				binding: &apisV3.ProjectRoleTemplateBinding{
					ProjectName:      "c-abc:p-example",
					RoleTemplateName: "role_template_0",
				},
			},
			wantErr: true,
		},
		{
			name: "unable to delete CR",
			mockSetup: func() {
				p.rtLister = &rancherv3fakes.RoleTemplateListerMock{
					GetFunc: func(namespace, name string) (*apisV3.RoleTemplate, error) {
						return &apisV3.RoleTemplate{
							Rules: []v1.PolicyRule{
								{
									APIGroups: []string{management.GroupName},
									Verbs:     []string{"updatepsa"},
									Resources: []string{apisV3.ProjectResourceName},
								},
							},
						}, nil
					},
				}
				p.crClient = &typesrbacv1fakes.ClusterRoleInterfaceMock{
					DeleteFunc: func(name string, options *metav1.DeleteOptions) error {
						return fmt.Errorf("error")
					},
				}
			},
			args: args{
				binding: &apisV3.ProjectRoleTemplateBinding{
					ProjectName:      "c-abc:p-example",
					RoleTemplateName: "role_template_0",
				},
			},
			wantErr: true,
		},
		{
			name: "delete only CRB (CR locked by other CRBs)",
			mockSetup: func() {
				p.rtLister = &rancherv3fakes.RoleTemplateListerMock{
					GetFunc: func(namespace, name string) (*apisV3.RoleTemplate, error) {
						return &apisV3.RoleTemplate{
							Rules: []v1.PolicyRule{
								{
									APIGroups: []string{management.GroupName},
									Verbs:     []string{"updatepsa"},
									Resources: []string{apisV3.ProjectResourceName},
								},
							},
						}, nil
					},
				}
				p.crbClient = &typesrbacv1fakes.ClusterRoleBindingInterfaceMock{
					ListFunc: func(opts metav1.ListOptions) (*v1.ClusterRoleBindingList, error) {
						return &v1.ClusterRoleBindingList{
							Items: []v1.ClusterRoleBinding{
								v1.ClusterRoleBinding{
									RoleRef: v1.RoleRef{
										Kind: "ClusterRole",
										Name: "p-example-namespaces-psa",
									},
								},
							},
						}, nil
					},
					DeleteFunc: func(name string, options *metav1.DeleteOptions) error {
						return nil
					},
				}
			},
			args: args{
				binding: &apisV3.ProjectRoleTemplateBinding{
					ProjectName:      "c-abc:p-example",
					RoleTemplateName: "role_template_0",
					UserName:         "u-zxv",
				},
			},
			wantErr: false,
		},
		{
			name: "unable to get RT",
			mockSetup: func() {
				p.rtLister = &rancherv3fakes.RoleTemplateListerMock{
					GetFunc: func(namespace, name string) (*apisV3.RoleTemplate, error) {
						return nil, fmt.Errorf("error")
					},
				}
				p.crClient = &typesrbacv1fakes.ClusterRoleInterfaceMock{
					DeleteFunc: func(name string, options *metav1.DeleteOptions) error {
						return fmt.Errorf("error")
					},
				}
			},
			args: args{
				binding: &apisV3.ProjectRoleTemplateBinding{
					ProjectName:      "c-abc:p-example",
					RoleTemplateName: "role_template_0",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			if err := p.ensurePSAPermissionsDelete(tt.args.binding); (err != nil) != tt.wantErr {
				t.Errorf("prtbLifecycle.ensurePRTBDelete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
