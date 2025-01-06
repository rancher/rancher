package roletemplates

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	defaultPRTB = v3.ProjectRoleTemplateBinding{
		RoleTemplateName: "test-rt",
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
		prtb      *v3.ProjectRoleTemplateBinding
		getRTFunc func() (*v3.RoleTemplate, error)
		getCRFunc func() (*rbacv1.ClusterRole, error)
		want      bool
		wantErr   bool
	}{
		{
			name:      "error getting role template",
			prtb:      defaultPRTB.DeepCopy(),
			getRTFunc: func() (*v3.RoleTemplate, error) { return nil, errDefault },
			want:      false,
			wantErr:   true,
		},
		{
			name:      "role template not found",
			prtb:      defaultPRTB.DeepCopy(),
			getRTFunc: func() (*v3.RoleTemplate, error) { return nil, errNotFound },
			want:      false,
			wantErr:   false,
		},
		{
			name:      "error getting cluster role",
			prtb:      defaultPRTB.DeepCopy(),
			getRTFunc: func() (*v3.RoleTemplate, error) { return defaultRT.DeepCopy(), nil },
			getCRFunc: func() (*rbacv1.ClusterRole, error) { return nil, errDefault },
			want:      false,
			wantErr:   true,
		},
		{
			name:      "cluster role not found",
			prtb:      defaultPRTB.DeepCopy(),
			getRTFunc: func() (*v3.RoleTemplate, error) { return defaultRT.DeepCopy(), nil },
			getCRFunc: func() (*rbacv1.ClusterRole, error) { return nil, errNotFound },
			want:      false,
			wantErr:   false,
		},
		{
			name:      "cluster role found",
			prtb:      defaultPRTB.DeepCopy(),
			getRTFunc: func() (*v3.RoleTemplate, error) { return defaultRT.DeepCopy(), nil },
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
				crController.EXPECT().Get(promotedCRName, gomock.Any()).Return(tt.getCRFunc())
			}
			rtController := fake.NewMockNonNamespacedControllerInterface[*v3.RoleTemplate, *v3.RoleTemplateList](ctrl)
			if tt.getRTFunc != nil {
				rtController.EXPECT().Get(tt.prtb.RoleTemplateName, gomock.Any()).Return(tt.getRTFunc())
			}

			p := &prtbHandler{
				crClient: crController,
				rtClient: rtController,
			}

			got, err := p.doesRoleTemplateHavePromotedRules(tt.prtb)

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
	subject = rbacv1.Subject{
		Name: "test-subject",
	}
	roleRef = rbacv1.RoleRef{
		Name: "test-roleref",
	}
	defaultRB = rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "test-rb"},
		Subjects:   []rbacv1.Subject{subject},
		RoleRef:    roleRef,
	}
	defaultPromotedRB = rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "test-rb-promoted"},
		Subjects:   []rbacv1.Subject{subject},
		RoleRef:    roleRef,
	}
)

func Test_prtbHandler_ensureOnlyDesiredRoleBindingsExist(t *testing.T) {
	tests := []struct {
		name              string
		desiredRB         *rbacv1.RoleBinding
		desiredPromotedRB *rbacv1.RoleBinding
		rbController      func(*gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]
		wantErr           bool
	}{
		{
			name: "error listing existing rolebindings",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				rbController.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errDefault)
				return rbController
			},
			wantErr: true,
		},
		{
			name: "list returns nil list is no op",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				rbController.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				return rbController
			},
		},
		{
			name: "no pre-existing rolebindings, no promoted rolebinding",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				rbController.EXPECT().List(gomock.Any(), gomock.Any()).Return(&rbacv1.RoleBindingList{}, nil)
				rbController.EXPECT().Create(defaultRB.DeepCopy()).Return(nil, nil)
				return rbController
			},
			desiredRB: defaultRB.DeepCopy(),
		},
		{
			name: "error creating desired rolebinding",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				rbController.EXPECT().List(gomock.Any(), gomock.Any()).Return(&rbacv1.RoleBindingList{}, nil)
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
				rbController.EXPECT().List(gomock.Any(), gomock.Any()).Return(&rbacv1.RoleBindingList{}, nil)
				rbController.EXPECT().Create(defaultRB.DeepCopy()).Return(nil, nil)
				rbController.EXPECT().Create(defaultPromotedRB.DeepCopy()).Return(nil, nil)
				return rbController
			},
			desiredRB:         defaultRB.DeepCopy(),
			desiredPromotedRB: defaultPromotedRB.DeepCopy(),
		},
		{
			name: "error creating promoted rolebinding",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				rbController.EXPECT().List(gomock.Any(), gomock.Any()).Return(&rbacv1.RoleBindingList{}, nil)
				rbController.EXPECT().Create(defaultRB.DeepCopy()).Return(nil, nil)
				rbController.EXPECT().Create(defaultPromotedRB.DeepCopy()).Return(nil, errDefault)
				return rbController
			},
			desiredRB:         defaultRB.DeepCopy(),
			desiredPromotedRB: defaultPromotedRB.DeepCopy(),
			wantErr:           true,
		},
		{
			name: "rolebindings already exist",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				list := &rbacv1.RoleBindingList{Items: []rbacv1.RoleBinding{
					*defaultRB.DeepCopy(),
					*defaultPromotedRB.DeepCopy(),
				}}
				rbController.EXPECT().List(gomock.Any(), gomock.Any()).Return(list, nil)
				return rbController
			},
			desiredRB:         defaultRB.DeepCopy(),
			desiredPromotedRB: defaultPromotedRB.DeepCopy(),
		},
		{
			name: "unwanted rolebindings exist with no desired rolebindings",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				list := &rbacv1.RoleBindingList{Items: []rbacv1.RoleBinding{
					{ObjectMeta: metav1.ObjectMeta{Name: "bad-rt"}},
				}}
				rbController.EXPECT().List(gomock.Any(), gomock.Any()).Return(list, nil)
				rbController.EXPECT().Create(defaultRB.DeepCopy()).Return(nil, nil)
				rbController.EXPECT().Create(defaultPromotedRB.DeepCopy()).Return(nil, nil)
				rbController.EXPECT().Delete("namespace", "bad-rt", gomock.Any()).Return(nil)
				return rbController
			},
			desiredRB:         defaultRB.DeepCopy(),
			desiredPromotedRB: defaultPromotedRB.DeepCopy(),
		},
		{
			name: "unwanted rolebindings exist and desired rolebindings exist",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				list := &rbacv1.RoleBindingList{Items: []rbacv1.RoleBinding{
					{ObjectMeta: metav1.ObjectMeta{Name: "bad-rt"}},
					*defaultRB.DeepCopy(),
					*defaultPromotedRB.DeepCopy(),
				}}
				rbController.EXPECT().List(gomock.Any(), gomock.Any()).Return(list, nil)
				rbController.EXPECT().Delete("namespace", "bad-rt", gomock.Any()).Return(nil)
				return rbController
			},
			desiredRB:         defaultRB.DeepCopy(),
			desiredPromotedRB: defaultPromotedRB.DeepCopy(),
		},
		{
			name: "error deleting unwanted rolebindings",
			rbController: func(c *gomock.Controller) *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList] {
				rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](c)
				list := &rbacv1.RoleBindingList{Items: []rbacv1.RoleBinding{
					{ObjectMeta: metav1.ObjectMeta{Name: "bad-rt"}},
					*defaultRB.DeepCopy(),
					*defaultPromotedRB.DeepCopy(),
				}}
				rbController.EXPECT().List(gomock.Any(), gomock.Any()).Return(list, nil)
				rbController.EXPECT().Delete("namespace", "bad-rt", gomock.Any()).Return(errDefault)
				return rbController
			},
			desiredRB:         defaultRB.DeepCopy(),
			desiredPromotedRB: defaultPromotedRB.DeepCopy(),
			wantErr:           true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			p := &prtbHandler{
				rbClient: tt.rbController(ctrl),
			}
			if err := p.ensureOnlyDesiredRoleBindingsExist(tt.desiredRB, tt.desiredPromotedRB, "namespace", "ownerlabel"); (err != nil) != tt.wantErr {
				t.Errorf("prtbHandler.ensureOnlyDesiredRoleBindingsExist() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
