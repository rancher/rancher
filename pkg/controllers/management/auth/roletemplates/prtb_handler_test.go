package roletemplates

import (
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/rbac"
	userMocks "github.com/rancher/rancher/pkg/user/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_reconcileSubject(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		setupUserManager    func(*userMocks.MockManager)
		setupUserController func(*fake.MockNonNamespacedControllerInterface[*v3.User, *v3.UserList])
		binding             *v3.ProjectRoleTemplateBinding
		want                *v3.ProjectRoleTemplateBinding
		wantErr             bool
	}{
		{
			name: "prtb with a UserPrincipalName and Username is no-op",
			binding: &v3.ProjectRoleTemplateBinding{
				UserName:          "test-user",
				UserPrincipalName: "test-principal",
			},
			want: &v3.ProjectRoleTemplateBinding{
				UserName:          "test-user",
				UserPrincipalName: "test-principal",
			},
		},
		{
			name: "prtb with a GroupName is no-op",
			binding: &v3.ProjectRoleTemplateBinding{
				GroupName: "test-group",
			},
			want: &v3.ProjectRoleTemplateBinding{
				GroupName: "test-group",
			},
		},
		{
			name: "prtb with GroupPrincipalName is no-op",
			binding: &v3.ProjectRoleTemplateBinding{
				GroupPrincipalName: "test-group-principal",
			},
			want: &v3.ProjectRoleTemplateBinding{
				GroupPrincipalName: "test-group-principal",
			},
		},
		{
			name: "prtb without a UserPrincipalName or Username produces error",
			binding: &v3.ProjectRoleTemplateBinding{
				UserName:          "",
				UserPrincipalName: "",
			},
			wantErr: true,
		},
		{
			name: "prtb with a UserPrincipalName and no Username creates user",
			binding: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"auth.cattle.io/principal-display-name": "display-name",
					},
				},
				UserName:          "",
				UserPrincipalName: "test-principal",
			},
			setupUserManager: func(m *userMocks.MockManager) {
				m.EXPECT().EnsureUser("test-principal", "display-name").Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				}, nil)
			},
			want: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"auth.cattle.io/principal-display-name": "display-name",
					},
				},
				UserName:          "test-user",
				UserPrincipalName: "test-principal",
			},
		},
		{
			name: "error in EnsureUser",
			binding: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"auth.cattle.io/principal-display-name": "display-name",
					},
				},
				UserName:          "",
				UserPrincipalName: "test-principal",
			},
			setupUserManager: func(m *userMocks.MockManager) {
				m.EXPECT().EnsureUser("test-principal", "display-name").Return(nil, errDefault)
			},
			want: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"auth.cattle.io/principal-display-name": "display-name",
					},
				},
				UserName:          "",
				UserPrincipalName: "test-principal",
			},
			wantErr: true,
		},
		{
			name: "prtb with a UserName and no UserPrincipalName sets UserPrincipalName",
			binding: &v3.ProjectRoleTemplateBinding{
				UserName:          "test-user",
				UserPrincipalName: "",
			},
			setupUserController: func(m *fake.MockNonNamespacedControllerInterface[*v3.User, *v3.UserList]) {
				m.EXPECT().Get("test-user", metav1.GetOptions{}).Return(&v3.User{
					PrincipalIDs: []string{"principal-test-user"},
				}, nil)
			},
			want: &v3.ProjectRoleTemplateBinding{
				UserName:          "test-user",
				UserPrincipalName: "principal-test-user",
			},
		},
		{
			name: "error getting users",
			binding: &v3.ProjectRoleTemplateBinding{
				UserName:          "test-user",
				UserPrincipalName: "",
			},
			setupUserController: func(m *fake.MockNonNamespacedControllerInterface[*v3.User, *v3.UserList]) {
				m.EXPECT().Get("test-user", metav1.GetOptions{}).Return(nil, errDefault)
			},
			want: &v3.ProjectRoleTemplateBinding{
				UserName:          "test-user",
				UserPrincipalName: "",
			},
			wantErr: true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockUserManager := userMocks.NewMockManager(ctrl)
			mockUserController := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
			if tt.setupUserManager != nil {
				tt.setupUserManager(mockUserManager)
			}
			if tt.setupUserController != nil {
				tt.setupUserController(mockUserController)
			}
			p := &prtbHandler{
				userMGR:        mockUserManager,
				userController: mockUserController,
			}

			got, err := p.reconcileSubject(tt.binding)

			if (err != nil) != tt.wantErr {
				t.Errorf("prtbHandler.reconcileSubject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("prtbHandler.reconcileSubject() = %v, want %v", got, tt.want)
			}
		})
	}
}

var (
	ownerLabel = "authz.cluster.cattle.io/prtb-owner-test-prtb"
	defaultRB  = rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rb-visjzlqzqw",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"authz.cluster.cattle.io/prtb-owner-test-prtb":  "true",
				"management.cattle.io/roletemplate-aggregation": "true",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "test-rt-project-mgmt-aggregator",
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbacv1.Subject{
			{
				Namespace: "",
				Kind:      "User",
				Name:      "test-user",
				APIGroup:  "rbac.authorization.k8s.io",
			},
		},
	}
	badRB = rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad-crb",
			Namespace: "test-namespace",
		},
	}
)

func Test_reconcileBindings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		setupCRController func(*fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList])
		setupRBController func(*fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList])
		prtb              *v3.ProjectRoleTemplateBinding
		wantErr           bool
	}{
		{
			name: "error getting cluster role",
			setupCRController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errDefault)
			},
			prtb: &v3.ProjectRoleTemplateBinding{
				RoleTemplateName: "test-rt",
			},
			wantErr: true,
		},
		{
			name: "no error when cluster role doesn't exist",
			setupCRController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
			},
			prtb: &v3.ProjectRoleTemplateBinding{
				RoleTemplateName: "test-rt",
			},
			wantErr: false,
		},
		{
			name: "error building rolebinding",
			setupCRController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, nil)
			},
			prtb: &v3.ProjectRoleTemplateBinding{
				RoleTemplateName: "test-rt",
			},
			wantErr: true,
		},
		{
			name: "error listing rolebindings",
			setupCRController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, nil)
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("test-namespace", metav1.ListOptions{LabelSelector: ownerLabel}).Return(nil, errDefault)
			},
			prtb:    defaultPRTB.DeepCopy(),
			wantErr: true,
		},
		{
			name: "error listing rolebindings",
			setupCRController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, nil)
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("test-namespace", metav1.ListOptions{LabelSelector: ownerLabel}).Return(nil, errDefault)
			},
			prtb:    defaultPRTB.DeepCopy(),
			wantErr: true,
		},
		{
			name: "error deleting unwanted rolebindings",
			setupCRController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, nil)
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("test-namespace", metav1.ListOptions{LabelSelector: ownerLabel}).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{badRB},
				}, nil)
				m.EXPECT().Delete("test-namespace", "bad-crb", &metav1.DeleteOptions{}).Return(errDefault)
			},
			prtb:    defaultPRTB.DeepCopy(),
			wantErr: true,
		},
		{
			name: "RB already exists",
			setupCRController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, nil)
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("test-namespace", metav1.ListOptions{LabelSelector: ownerLabel}).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{defaultRB},
				}, nil)
			},
			prtb: defaultPRTB.DeepCopy(),
		},
		{
			name: "RB already exists with extra bad RBs",
			setupCRController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, nil)
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("test-namespace", metav1.ListOptions{LabelSelector: ownerLabel}).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{defaultRB, badRB},
				}, nil)
				m.EXPECT().Delete("test-namespace", "bad-crb", &metav1.DeleteOptions{}).Return(nil)
			},
			prtb: defaultPRTB.DeepCopy(),
		},
		{
			name: "RB needs to be created",
			setupCRController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, nil)
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("test-namespace", metav1.ListOptions{LabelSelector: ownerLabel}).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{},
				}, nil)
				m.EXPECT().Create(defaultRB.DeepCopy()).Return(nil, nil)
			},
			prtb: defaultPRTB.DeepCopy(),
		},
		{
			name: "error creating RB",
			setupCRController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, nil)
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("test-namespace", metav1.ListOptions{LabelSelector: ownerLabel}).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{},
				}, nil)
				m.EXPECT().Create(defaultRB.DeepCopy()).Return(nil, errDefault)
			},
			prtb:    defaultPRTB.DeepCopy(),
			wantErr: true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
			if tt.setupRBController != nil {
				tt.setupRBController(rbController)
			}
			crController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			if tt.setupCRController != nil {
				tt.setupCRController(crController)
			}

			p := &prtbHandler{
				crController: crController,
				rbController: rbController,
			}
			if err := p.reconcileBindings(tt.prtb); (err != nil) != tt.wantErr {
				t.Errorf("prtbHandler.reconcileBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_prtbHandler_reconcileMembershipBindings(t *testing.T) {
	type controllers struct {
		rbController  *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]
		crbController *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]
		crController  *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]
	}
	tests := []struct {
		name             string
		prtb             *v3.ProjectRoleTemplateBinding
		setupControllers func(controllers)
		wantErr          bool
	}{
		{
			name: "error getting roletemplate",
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(nil, errDefault)
			},
			prtb:    defaultPRTB.DeepCopy(),
			wantErr: true,
		},
		// Cluster and Project Membership are more thoroughly tested in common_test.go.
		// This test is to ensure that a project gets both cluster and project membership.
		{
			name: "create cluster and project membership bindings",
			setupControllers: func(c controllers) {
				c.crController.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-rt"},
				}, nil)

				c.crbController.EXPECT().Get(defaultProjectCRB.Name, metav1.GetOptions{}).Return(nil, errNotFound)
				c.crbController.EXPECT().Create(defaultProjectCRB.DeepCopy()).Return(nil, nil)

				c.rbController.EXPECT().Get(defaultRoleBinding.Namespace, defaultRoleBinding.Name, metav1.GetOptions{}).Return(nil, errNotFound)
				c.rbController.EXPECT().Create(defaultRoleBinding.DeepCopy()).Return(nil, nil)
			},
			prtb: defaultPRTB.DeepCopy(),
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := controllers{
				rbController:  fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl),
				crbController: fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl),
				crController:  fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl),
			}
			if tt.setupControllers != nil {
				tt.setupControllers(c)
			}
			p := &prtbHandler{
				rbController:  c.rbController,
				crbController: c.crbController,
				crController:  c.crController,
			}
			if err := p.reconcileMembershipBindings(tt.prtb); (err != nil) != tt.wantErr {
				t.Errorf("prtbHandler.reconcileMembershipBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_prtbHandler_deleteDownstreamRoleBindings(t *testing.T) {
	t.Parallel()

	prtbName := "test-prtb"
	ownerLabelKey := "authz.cluster.cattle.io/prtb-owner-test-prtb"
	aggregationLabelKey := "management.cattle.io/roletemplate-aggregation"

	rb1 := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rb-downstream-1",
			Namespace: "namespace-1",
			Labels: map[string]string{
				ownerLabelKey:       "true",
				aggregationLabelKey: "true",
			},
		},
	}

	rb2 := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rb-downstream-2",
			Namespace: "namespace-2",
			Labels: map[string]string{
				ownerLabelKey:       "true",
				aggregationLabelKey: "true",
			},
		},
	}

	tests := []struct {
		name              string
		prtb              *v3.ProjectRoleTemplateBinding
		setupRBController func(*fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList])
		wantErr           bool
	}{
		{
			name: "successfully delete multiple role bindings",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List(metav1.NamespaceAll, metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{rb1, rb2},
				}, nil)
				m.EXPECT().Delete("namespace-1", "rb-downstream-1", &metav1.DeleteOptions{}).Return(nil)
				m.EXPECT().Delete("namespace-2", "rb-downstream-2", &metav1.DeleteOptions{}).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "no role bindings to delete",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List(metav1.NamespaceAll, metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{},
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "error listing role bindings",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List(metav1.NamespaceAll, metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error deleting one role binding",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List(metav1.NamespaceAll, metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{rb1, rb2},
				}, nil)
				m.EXPECT().Delete("namespace-1", "rb-downstream-1", &metav1.DeleteOptions{}).Return(errDefault)
				m.EXPECT().Delete("namespace-2", "rb-downstream-2", &metav1.DeleteOptions{}).Return(nil)
			},
			wantErr: true,
		},
		{
			name: "error deleting multiple role bindings",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List(metav1.NamespaceAll, metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{rb1, rb2},
				}, nil)
				m.EXPECT().Delete("namespace-1", "rb-downstream-1", &metav1.DeleteOptions{}).Return(errDefault)
				m.EXPECT().Delete("namespace-2", "rb-downstream-2", &metav1.DeleteOptions{}).Return(errDefault)
			},
			wantErr: true,
		},
		{
			name: "role binding not found during delete does not error",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List(metav1.NamespaceAll, metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{rb1},
				}, nil)
				m.EXPECT().Delete("namespace-1", "rb-downstream-1", &metav1.DeleteOptions{}).Return(errNotFound)
			},
			wantErr: false,
		},
	}

	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
			if tt.setupRBController != nil {
				tt.setupRBController(rbController)
			}

			p := &prtbHandler{}
			err := p.deleteDownstreamRoleBindings(tt.prtb, rbController)

			if (err != nil) != tt.wantErr {
				t.Errorf("prtbHandler.deleteDownstreamRoleBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_prtbHandler_deleteDownstreamClusterRoleBindings(t *testing.T) {
	t.Parallel()

	prtbName := "test-prtb"
	ownerLabelKey := "authz.cluster.cattle.io/prtb-owner-test-prtb"
	anotherOwnerLabelKey := "authz.cluster.cattle.io/prtb-owner-another-prtb"
	aggregationLabelKey := "management.cattle.io/roletemplate-aggregation"

	crb1 := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crb-downstream-1",
			Labels: map[string]string{
				ownerLabelKey:       "true",
				aggregationLabelKey: "true",
			},
		},
	}

	crb2 := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crb-downstream-2",
			Labels: map[string]string{
				ownerLabelKey:       "true",
				aggregationLabelKey: "true",
			},
		},
	}

	crbShared := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crb-shared",
			Labels: map[string]string{
				ownerLabelKey:        "true",
				anotherOwnerLabelKey: "true",
				aggregationLabelKey:  "true",
			},
		},
	}

	tests := []struct {
		name               string
		prtb               *v3.ProjectRoleTemplateBinding
		setupCRBController func(*fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList])
		wantErr            bool
	}{
		{
			name: "successfully delete multiple cluster role bindings",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{crb1, crb2},
				}, nil)
				m.EXPECT().Delete("crb-downstream-1", &metav1.DeleteOptions{}).Return(nil)
				m.EXPECT().Delete("crb-downstream-2", &metav1.DeleteOptions{}).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "no cluster role bindings to delete",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{},
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "error listing cluster role bindings",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "shared cluster role binding is updated not deleted",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{crbShared},
				}, nil)
				// Expect Update to be called with the CRB that has the owner label removed
				m.EXPECT().Update(gomock.Any()).DoAndReturn(func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					// Verify the owner label was removed but other labels remain
					if _, exists := crb.Labels[ownerLabelKey]; exists {
						t.Errorf("Expected owner label to be removed")
					}
					if _, exists := crb.Labels[anotherOwnerLabelKey]; !exists {
						t.Errorf("Expected another owner label to still exist")
					}
					return crb, nil
				})
			},
			wantErr: false,
		},
		{
			name: "error updating shared cluster role binding",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{crbShared},
				}, nil)
				m.EXPECT().Update(gomock.Any()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error deleting cluster role binding",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{crb1},
				}, nil)
				m.EXPECT().Delete("crb-downstream-1", &metav1.DeleteOptions{}).Return(errDefault)
			},
			wantErr: true,
		},
		{
			name: "mix of shared and non-shared cluster role bindings",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{crb1, crbShared, crb2},
				}, nil)
				m.EXPECT().Delete("crb-downstream-1", &metav1.DeleteOptions{}).Return(nil)
				m.EXPECT().Update(gomock.Any()).Return(&crbShared, nil)
				m.EXPECT().Delete("crb-downstream-2", &metav1.DeleteOptions{}).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "error deleting one but shared crb updates successfully",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{crb1, crbShared},
				}, nil)
				m.EXPECT().Delete("crb-downstream-1", &metav1.DeleteOptions{}).Return(errDefault)
				m.EXPECT().Update(gomock.Any()).Return(&crbShared, nil)
			},
			wantErr: true,
		},
		{
			name: "cluster role binding not found during delete does not error",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: prtbName,
				},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(metav1.ListOptions{
					LabelSelector: ownerLabelKey + "=true,management.cattle.io/roletemplate-aggregation=true",
				}).Return(&rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{crb1},
				}, nil)
				m.EXPECT().Delete("crb-downstream-1", &metav1.DeleteOptions{}).Return(errNotFound)
			},
			wantErr: false,
		},
	}

	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crbController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl)
			if tt.setupCRBController != nil {
				tt.setupCRBController(crbController)
			}

			p := &prtbHandler{}
			err := p.deleteDownstreamClusterRoleBindings(tt.prtb, crbController)

			if (err != nil) != tt.wantErr {
				t.Errorf("prtbHandler.deleteDownstreamClusterRoleBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_prtbHandler_handleMigration(t *testing.T) {
	ctrl := gomock.NewController(t)

	type controllers struct {
		prtbController *fake.MockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList]
		rbCache        *fake.MockCacheInterface[*rbacv1.RoleBinding]
	}

	tests := []struct {
		name               string
		prtb               *v3.ProjectRoleTemplateBinding
		featureFlagEnabled bool
		setupControllers   func(controllers)
		wantLabel          bool
		wantErr            bool
	}{
		{
			name: "feature flag disabled, label present - should remove label and call deleteRoleBindings",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-prtb",
					Namespace: "test-ns",
					Labels: map[string]string{
						rbac.AggregationFeatureLabel: "true",
					},
				},
			},
			featureFlagEnabled: false,
			setupControllers: func(c controllers) {
				// Expect Update to be called with label removed
				c.prtbController.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
					if _, exists := obj.Labels[rbac.AggregationFeatureLabel]; exists {
						t.Error("expected label to be removed from updated PRTB")
					}
					return obj, nil
				})
				// deleteRoleBindings will be called, so mock rbCache.List to return empty list
				c.rbCache.EXPECT().List("test-ns", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			wantLabel: false,
		},
		{
			name: "feature flag disabled, label absent - no-op",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-prtb",
					Namespace: "test-ns",
					Labels:    map[string]string{},
				},
			},
			featureFlagEnabled: false,
			setupControllers:   func(c controllers) {},
			wantLabel:          false,
		},
		{
			name: "feature flag enabled, label absent - should add label and call deleteLegacyBinding",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-prtb",
					Namespace: "test-ns",
					Labels:    map[string]string{},
				},
			},
			featureFlagEnabled: true,
			setupControllers: func(c controllers) {
				// deleteLegacyBinding will be called
				// Mock rbCache.GetByIndex to return empty list
				c.rbCache.EXPECT().GetByIndex(rbByPRTBOwnerReferenceIndex, "test-prtb").Return([]*rbacv1.RoleBinding{}, nil)
				// Expect Update to be called with label added
				c.prtbController.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
					if obj.Labels[rbac.AggregationFeatureLabel] != "true" {
						t.Error("expected label to be added to updated PRTB")
					}
					return obj, nil
				})
			},
			wantLabel: true,
		},
		{
			name: "feature flag enabled, label present - no-op",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-prtb",
					Namespace: "test-ns",
					Labels: map[string]string{
						rbac.AggregationFeatureLabel: "true",
					},
				},
			},
			featureFlagEnabled: true,
			setupControllers:   func(c controllers) {},
			wantLabel:          true,
		},
		{
			name: "feature flag enabled, nil labels - should add label",
			prtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-prtb",
					Namespace: "test-ns",
					Labels:    nil,
				},
			},
			featureFlagEnabled: true,
			setupControllers: func(c controllers) {
				// deleteLegacyBinding will be called
				c.rbCache.EXPECT().GetByIndex(rbByPRTBOwnerReferenceIndex, "test-prtb").Return([]*rbacv1.RoleBinding{}, nil)
				// Expect Update to be called with label added
				c.prtbController.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
					if obj.Labels == nil {
						t.Error("expected labels map to be initialized")
					}
					if obj.Labels[rbac.AggregationFeatureLabel] != "true" {
						t.Error("expected label to be added to updated PRTB")
					}
					return obj, nil
				})
			},
			wantLabel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := features.AggregatedRoleTemplates.Enabled()
			t.Cleanup(func() {
				features.AggregatedRoleTemplates.Set(prev)
			})
			features.AggregatedRoleTemplates.Set(tt.featureFlagEnabled)

			prtbController := fake.NewMockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
			rbCache := fake.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl)

			if tt.setupControllers != nil {
				tt.setupControllers(controllers{
					prtbController: prtbController,
					rbCache:        rbCache,
				})
			}

			h := &prtbHandler{
				prtbClient: prtbController,
				rbCache:    rbCache,
			}

			result, err := h.handleMigration(tt.prtb)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleMigration() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantLabel {
				if result.Labels[rbac.AggregationFeatureLabel] != "true" {
					t.Error("expected label to be present and set to 'true'")
				}
			} else {
				if result != nil && result.Labels[rbac.AggregationFeatureLabel] == "true" {
					t.Error("expected label to be absent or not set to 'true'")
				}
			}
		})
	}
}
