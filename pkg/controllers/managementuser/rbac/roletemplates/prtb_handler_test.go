package roletemplates

import (
	"fmt"
	"reflect"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	defaultPRTB = v3.ProjectRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-prtb",
		},
		UserName:         "test-user",
		RoleTemplateName: "test-rt",
		ProjectName:      "c-abc123:p-xyz789",
	}
	defaultRT = v3.RoleTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "test-rt"},
	}
	promotedCRName = "test-rt-promoted"
	errDefault     = fmt.Errorf("error")
	errNotFound    = apierrors.NewNotFound(schema.GroupResource{}, "error")
)

func Test_doesRoleTemplateHavePromotedRules(t *testing.T) {
	tests := []struct {
		name      string
		rt        *v3.RoleTemplate
		getCRFunc func() (*rbacv1.ClusterRole, error)
		want      bool
		wantErr   bool
	}{
		{
			name:      "error getting cluster role",
			rt:        defaultRT.DeepCopy(),
			getCRFunc: func() (*rbacv1.ClusterRole, error) { return nil, errDefault },
			want:      false,
			wantErr:   true,
		},
		{
			name:      "cluster role not found",
			rt:        defaultRT.DeepCopy(),
			getCRFunc: func() (*rbacv1.ClusterRole, error) { return nil, errNotFound },
			want:      false,
			wantErr:   false,
		},
		{
			name:      "cluster role found",
			rt:        defaultRT.DeepCopy(),
			getCRFunc: func() (*rbacv1.ClusterRole, error) { return &rbacv1.ClusterRole{}, nil },
			want:      true,
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			crController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			if tt.getCRFunc != nil {
				crController.EXPECT().Get(promotedCRName, metav1.GetOptions{}).Return(tt.getCRFunc())
			}

			p := &prtbHandler{
				crClient: crController,
			}

			got, err := p.doesRoleTemplateHavePromotedRules(tt.rt)

			if (err != nil) != tt.wantErr {
				t.Errorf("prtbHandler.doesRoleTemplateHavePromotedRules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("prtbHandler.doesRoleTemplateHavePromotedRules() = %v, want %v", got, tt.want)
			}
		})
	}
}

var (
	listOption = metav1.ListOptions{LabelSelector: "ownerlabel"}
	subject    = rbacv1.Subject{
		Name: "test-subject",
	}
	roleRef = rbacv1.RoleRef{
		Name: "test-roleref",
	}
	defaultRB = rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rb",
			Namespace: "test-ns",
		},
		Subjects: []rbacv1.Subject{subject},
		RoleRef:  roleRef,
	}
)

func Test_ensureOnlyDesiredRoleBindingsExist(t *testing.T) {
	tests := []struct {
		name         string
		desiredRB    *rbacv1.RoleBinding
		rbController func(*gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]
		wantErr      bool
	}{
		{
			name: "error listing existing rolebindings",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				rbController.EXPECT().List("test-ns", listOption).Return(nil, errDefault)
				return rbController
			},
			wantErr:   true,
			desiredRB: defaultRB.DeepCopy(),
		},
		{
			name: "list returns nil list is no op",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				rbController.EXPECT().List("test-ns", listOption).Return(nil, nil)
				return rbController
			},
			desiredRB: defaultRB.DeepCopy(),
		},
		{
			name: "no pre-existing rolebindings, no promoted rolebinding",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				rbController.EXPECT().List("test-ns", listOption).Return(&rbacv1.RoleBindingList{}, nil)
				rbController.EXPECT().Create(defaultRB.DeepCopy()).Return(nil, nil)
				return rbController
			},
			desiredRB: defaultRB.DeepCopy(),
		},
		{
			name: "error creating desired rolebinding",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				rbController.EXPECT().List("test-ns", listOption).Return(&rbacv1.RoleBindingList{}, nil)
				rbController.EXPECT().Create(defaultRB.DeepCopy()).Return(nil, errDefault)
				return rbController
			},
			desiredRB: defaultRB.DeepCopy(),
			wantErr:   true,
		},
		{
			name: "no pre-existing rolebindings, have promoted rolebinding",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				rbController.EXPECT().List("test-ns", listOption).Return(&rbacv1.RoleBindingList{}, nil)
				rbController.EXPECT().Create(defaultRB.DeepCopy()).Return(nil, nil)
				return rbController
			},
			desiredRB: defaultRB.DeepCopy(),
		},
		{
			name: "rolebindings already exist",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				list := &rbacv1.RoleBindingList{Items: []rbacv1.RoleBinding{
					*defaultRB.DeepCopy(),
				}}
				rbController.EXPECT().List("test-ns", listOption).Return(list, nil)
				return rbController
			},
			desiredRB: defaultRB.DeepCopy(),
		},
		{
			name: "unwanted rolebindings exist with no desired rolebindings",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				list := &rbacv1.RoleBindingList{Items: []rbacv1.RoleBinding{
					{ObjectMeta: metav1.ObjectMeta{Name: "bad-rt"}},
				}}
				rbController.EXPECT().List("test-ns", listOption).Return(list, nil)
				rbController.EXPECT().Create(defaultRB.DeepCopy()).Return(nil, nil)
				rbController.EXPECT().Delete("test-ns", "bad-rt", &metav1.DeleteOptions{}).Return(nil)
				return rbController
			},
			desiredRB: defaultRB.DeepCopy(),
		},
		{
			name: "unwanted rolebindings exist and desired rolebindings exist",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				list := &rbacv1.RoleBindingList{Items: []rbacv1.RoleBinding{
					{ObjectMeta: metav1.ObjectMeta{Name: "bad-rt"}},
					*defaultRB.DeepCopy(),
				}}
				rbController.EXPECT().List("test-ns", listOption).Return(list, nil)
				rbController.EXPECT().Delete("test-ns", "bad-rt", &metav1.DeleteOptions{}).Return(nil)
				return rbController
			},
			desiredRB: defaultRB.DeepCopy(),
		},
		{
			name: "error deleting unwanted rolebindings",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				list := &rbacv1.RoleBindingList{Items: []rbacv1.RoleBinding{
					{ObjectMeta: metav1.ObjectMeta{Name: "bad-rt"}},
					*defaultRB.DeepCopy(),
				}}
				rbController.EXPECT().List("test-ns", listOption).Return(list, nil)
				rbController.EXPECT().Delete("test-ns", "bad-rt", &metav1.DeleteOptions{}).Return(errDefault)
				return rbController
			},
			desiredRB: defaultRB.DeepCopy(),
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			p := &prtbHandler{
				rbClient: tt.rbController(ctrl),
			}
			if err := p.ensureOnlyDesiredRoleBindingExists(tt.desiredRB, "ownerlabel"); (err != nil) != tt.wantErr {
				t.Errorf("prtbHandler.ensureOnlyDesiredRoleBindingsExist() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

var (
	namespaceListOptions = metav1.ListOptions{LabelSelector: "field.cattle.io/projectId=p-xyz789"}
	rbListOptions        = metav1.ListOptions{LabelSelector: "authz.cluster.cattle.io/prtb-owner=test-prtb"}
	defaultRoleBinding   = rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "rb-d2l5e2jqi6",
			Labels: map[string]string{"authz.cluster.cattle.io/prtb-owner": "test-prtb"},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "test-rt-aggregator",
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbacv1.Subject{{
			Kind:     "User",
			APIGroup: "rbac.authorization.k8s.io",
			Name:     "test-user",
		}},
	}
)

func Test_prtbHandler_reconcileBindings(t *testing.T) {
	type controllers struct {
		rtController *fake.MockNonNamespacedControllerInterface[*v3.RoleTemplate, *v3.RoleTemplateList]
		nsController *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList]
		rbController *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]
	}
	tests := []struct {
		name             string
		prtb             *v3.ProjectRoleTemplateBinding
		setupControllers func(controllers)
		wantErr          bool
	}{
		{
			name:    "error building subject",
			prtb:    &v3.ProjectRoleTemplateBinding{},
			wantErr: true,
		},
		{
			name: "error reconciling promoted role",
			prtb: defaultPRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.rtController.EXPECT().Get(defaultPRTB.RoleTemplateName, metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error checking if role template is external",
			prtb: defaultPRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.rtController.EXPECT().Get(defaultPRTB.RoleTemplateName, metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error getting namespaces",
			prtb: defaultPRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.rtController.EXPECT().Get(defaultPRTB.RoleTemplateName, metav1.GetOptions{}).Return(nil, errNotFound)
				c.nsController.EXPECT().List(namespaceListOptions).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error creating role binding",
			prtb: defaultPRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.rtController.EXPECT().Get(defaultPRTB.RoleTemplateName, metav1.GetOptions{}).Return(nil, errNotFound)
				c.nsController.EXPECT().List(namespaceListOptions).Return(&corev1.NamespaceList{
					Items: []corev1.Namespace{
						{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
					},
				}, nil)
				c.rbController.EXPECT().List("ns1", rbListOptions).Return(&rbacv1.RoleBindingList{}, nil)
				rb := defaultRoleBinding.DeepCopy()
				rb.Namespace = "ns1"
				c.rbController.EXPECT().Create(rb).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "create role binding in multiple namespaces",
			prtb: defaultPRTB.DeepCopy(),
			setupControllers: func(c controllers) {
				c.rtController.EXPECT().Get(defaultPRTB.RoleTemplateName, metav1.GetOptions{}).Return(nil, errNotFound)
				c.nsController.EXPECT().List(namespaceListOptions).Return(&corev1.NamespaceList{
					Items: []corev1.Namespace{
						{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}},
					},
				}, nil)
				c.rbController.EXPECT().List("ns1", rbListOptions).Return(&rbacv1.RoleBindingList{}, nil)
				rb1 := defaultRoleBinding.DeepCopy()
				rb1.Namespace = "ns1"
				c.rbController.EXPECT().Create(rb1).Return(nil, nil)
				c.rbController.EXPECT().List("ns2", rbListOptions).Return(&rbacv1.RoleBindingList{}, nil)
				rb2 := defaultRoleBinding.DeepCopy()
				rb2.Namespace = "ns2"
				rb2.Name = "rb-x3nurktcw6"
				c.rbController.EXPECT().Create(rb2).Return(nil, nil)
			},
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := controllers{
				rtController: fake.NewMockNonNamespacedControllerInterface[*v3.RoleTemplate, *v3.RoleTemplateList](ctrl),
				nsController: fake.NewMockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl),
				rbController: fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl),
			}
			if tt.setupControllers != nil {
				tt.setupControllers(c)
			}
			p := &prtbHandler{
				rtClient: c.rtController,
				nsClient: c.nsController,
				rbClient: c.rbController,
			}
			if err := p.reconcileBindings(tt.prtb); (err != nil) != tt.wantErr {
				t.Errorf("prtbHandler.reconcileBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

var (
	crReadOnly = rbacv1.ClusterRole{
		Rules: []rbacv1.PolicyRule{
			{
				Resources: []string{"pods"},
				APIGroups: []string{""},
				Verbs:     []string{"get"},
			},
		},
	}
	crbReadOnly = rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "crb-cbjxdllmgx",
			Labels: map[string]string{"authz.cluster.cattle.io/prtb-owner": "test-prtb"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "p-xyz789-namespaces-readonly",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     "test-user",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
	crbCreateNS = rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "crb-h4flnes3qr",
			Labels: map[string]string{"authz.cluster.cattle.io/prtb-owner": "test-prtb"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "create-ns",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     "test-user",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
	crbEdit = rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "crb-lgj7lku5l7",
			Labels: map[string]string{"authz.cluster.cattle.io/prtb-owner": "test-prtb"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "p-xyz789-namespaces-edit",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     "test-user",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
)

func Test_prtbHandler_buildNamespaceBindings(t *testing.T) {
	tests := []struct {
		name          string
		prtb          *v3.ProjectRoleTemplateBinding
		setupCRClient func(*fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList])
		want          []*rbacv1.ClusterRoleBinding
		wantErr       bool
	}{
		{
			name: "no cluster role found",
			prtb: defaultPRTB.DeepCopy(),
			setupCRClient: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(nil, errNotFound)
			},
			want: nil,
		},
		{
			name: "cluster role is nil",
			prtb: defaultPRTB.DeepCopy(),
			setupCRClient: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(nil, nil)
			},
			want: nil,
		},
		{
			name: "error getting cluster role",
			prtb: defaultPRTB.DeepCopy(),
			setupCRClient: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{}, errDefault)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "namespaces read only cluster role",
			prtb: defaultPRTB.DeepCopy(),
			setupCRClient: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(crReadOnly.DeepCopy(), nil)
			},
			want: []*rbacv1.ClusterRoleBinding{crbReadOnly.DeepCopy()},
		},
		{
			name: "namespaces create cluster roles",
			prtb: defaultPRTB.DeepCopy(),
			setupCRClient: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{
					Rules: []rbacv1.PolicyRule{
						{
							Resources: []string{"namespaces"},
							APIGroups: []string{""},
							Verbs:     []string{"create"},
						},
					},
				}, nil)
			},
			want: []*rbacv1.ClusterRoleBinding{crbCreateNS.DeepCopy(), crbEdit.DeepCopy()},
		},
		{
			name: "wildcards create edit cluster roles",
			prtb: defaultPRTB.DeepCopy(),
			setupCRClient: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]) {
				m.EXPECT().Get("test-rt-aggregator", metav1.GetOptions{}).Return(&rbacv1.ClusterRole{
					Rules: []rbacv1.PolicyRule{
						{
							Resources: []string{"*"},
							APIGroups: []string{"*"},
							Verbs:     []string{"*"},
						},
					},
				}, nil)
			},
			want: []*rbacv1.ClusterRoleBinding{crbCreateNS.DeepCopy(), crbEdit.DeepCopy()},
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			if tt.setupCRClient != nil {
				tt.setupCRClient(crController)
			}
			p := &prtbHandler{
				crClient: crController,
			}
			got, err := p.buildNamespaceBindings(tt.prtb)
			if (err != nil) != tt.wantErr {
				t.Errorf("prtbHandler.buildNamespaceBindings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("prtbHandler.buildNamespaceBindings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ensureOnlyDesiredClusterRoleBindingsExists(t *testing.T) {
	tests := []struct {
		name               string
		crbs               []*rbacv1.ClusterRoleBinding
		setupCRBController func(*fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList])
		wantErr            bool
	}{
		{
			name: "error listing crbs",
			crbs: []*rbacv1.ClusterRoleBinding{},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(listOption).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error listing crbs",
			crbs: []*rbacv1.ClusterRoleBinding{},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(listOption).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error creating CRB",
			crbs: []*rbacv1.ClusterRoleBinding{
				{ObjectMeta: metav1.ObjectMeta{Name: "crb1"}},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(listOption).Return(&rbacv1.ClusterRoleBindingList{}, nil)
				m.EXPECT().Create(&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "crb1"}}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "no pre-existing CRBs, create all",
			crbs: []*rbacv1.ClusterRoleBinding{
				{ObjectMeta: metav1.ObjectMeta{Name: "crb1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "crb2"}},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(listOption).Return(&rbacv1.ClusterRoleBindingList{}, nil)
				m.EXPECT().Create(&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "crb1"}}).Return(nil, nil)
				m.EXPECT().Create(&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "crb2"}}).Return(nil, nil)
			},
		},
		{
			name: "pre-existing CRBs, no creation necessary",
			crbs: []*rbacv1.ClusterRoleBinding{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "crb1"},
					RoleRef:    defaultCRB.RoleRef,
					Subjects:   defaultCRB.Subjects,
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "crb2"},
					RoleRef:    defaultCRB.RoleRef,
					Subjects:   defaultCRB.Subjects,
				},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(listOption).Return(&rbacv1.ClusterRoleBindingList{Items: []rbacv1.ClusterRoleBinding{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "crb1"},
						RoleRef:    defaultCRB.RoleRef,
						Subjects:   defaultCRB.Subjects,
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "crb2"},
						RoleRef:    defaultCRB.RoleRef,
						Subjects:   defaultCRB.Subjects,
					},
				}}, nil)
			},
		},
		{
			name: "pre-existing incorrect CRB, delete and create correct CRB",
			crbs: []*rbacv1.ClusterRoleBinding{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "crb1"},
					RoleRef:    defaultCRB.RoleRef,
					Subjects:   defaultCRB.Subjects,
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "crb2"},
					RoleRef:    defaultCRB.RoleRef,
					Subjects:   defaultCRB.Subjects,
				},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(listOption).Return(&rbacv1.ClusterRoleBindingList{Items: []rbacv1.ClusterRoleBinding{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "crb1"},
						RoleRef:    defaultCRB.RoleRef,
						Subjects:   defaultCRB.Subjects,
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "crb2"},
					},
				}}, nil)
				m.EXPECT().Delete("crb2", &metav1.DeleteOptions{}).Return(nil)
				m.EXPECT().Create(&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "crb2"},
					RoleRef:    defaultCRB.RoleRef,
					Subjects:   defaultCRB.Subjects,
				})
			},
		},
		{
			name: "error deleting CRB",
			crbs: []*rbacv1.ClusterRoleBinding{},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().List(listOption).Return(&rbacv1.ClusterRoleBindingList{Items: []rbacv1.ClusterRoleBinding{
					{ObjectMeta: metav1.ObjectMeta{Name: "crb1"}},
				}}, nil)
				m.EXPECT().Delete("crb1", &metav1.DeleteOptions{}).Return(errDefault)
			},
			wantErr: true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crbController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl)
			if tt.setupCRBController != nil {
				tt.setupCRBController(crbController)
			}
			p := &prtbHandler{
				crbClient: crbController,
			}
			if err := p.ensureOnlyDesiredClusterRoleBindingsExists(tt.crbs, "ownerlabel"); (err != nil) != tt.wantErr {
				t.Errorf("prtbHandler.reconcileClusterRoleBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
