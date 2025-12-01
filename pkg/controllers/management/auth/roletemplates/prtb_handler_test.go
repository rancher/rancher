package roletemplates

import (
	"fmt"
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	userMocks "github.com/rancher/rancher/pkg/user/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	errDefault = fmt.Errorf("error")
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
				m.EXPECT().EnsureUser("test-principal", "display-name").Return(nil, fmt.Errorf("error"))
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
				m.EXPECT().Get("test-user", metav1.GetOptions{}).Return(nil, fmt.Errorf("error"))
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
			Labels:    map[string]string{"authz.cluster.cattle.io/prtb-owner-test-prtb": "true"},
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
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, fmt.Errorf("error"))
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
				m.EXPECT().Get("test-namespace", defaultRB.Name, metav1.GetOptions{}).Return(nil, errNotFound)
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
				m.EXPECT().Get("test-namespace", defaultRB.Name, metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(defaultRB.DeepCopy()).Return(nil, errDefault)
			},
			prtb:    defaultPRTB.DeepCopy(),
			wantErr: true,
		},
		{
			name: "error getting RB",
			setupCRController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-project-mgmt-aggregator", metav1.GetOptions{}).Return(nil, nil)
			},
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("test-namespace", metav1.ListOptions{LabelSelector: ownerLabel}).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{},
				}, nil)
				m.EXPECT().Get("test-namespace", defaultRB.Name, metav1.GetOptions{}).Return(nil, errDefault)
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
