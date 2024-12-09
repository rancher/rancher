package roletemplates

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	errNotFound    = apierrors.NewNotFound(schema.GroupResource{}, "error")
	defaultSubject = rbacv1.Subject{
		Kind:     "User",
		Name:     "test-user",
		APIGroup: "rbac.authorization.k8s.io",
	}

	defaultPRTB = v3.ProjectRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-prtb",
		},
		UserName:    "test-user",
		ProjectName: "test-project",
	}
	defaultProjectRoleRef = rbacv1.RoleRef{
		Name: "test-project-member",
		Kind: "ClusterRole",
	}
	defaultProjectCRB = rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "crb-lpmekfv6tk",
			Labels: map[string]string{"test-namespace_test-prtb": "true"},
		},
		Subjects: []rbacv1.Subject{defaultSubject},
		RoleRef:  defaultProjectRoleRef,
	}

	defaultCRTB = v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-crtb",
		},
		UserName:    "test-user",
		ClusterName: "test-cluster",
	}
	defaultClusterRoleRef = rbacv1.RoleRef{
		Name: "test-cluster-member",
		Kind: "ClusterRole",
	}
	defaultClusterCRB = rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "crb-intj3mhm4v",
			Labels: map[string]string{"test-namespace_test-crtb": "true"},
		},
		Subjects: []rbacv1.Subject{defaultSubject},
		RoleRef:  defaultClusterRoleRef,
	}
)

func TestCreateOrUpdateMembershipBinding(t *testing.T) {
	tests := []struct {
		name       string
		rtb        metav1.Object
		rt         *v3.RoleTemplate
		getFunc    func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error)
		createFunc func(*rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error)
		deleteFunc func(string, *metav1.DeleteOptions) error
		updateFunc func(*rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error)
		wantedCRB  *rbacv1.ClusterRoleBinding
		wantErr    bool
	}{
		{
			name: "create new binding from prtb",
			rtb:  defaultPRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				return nil, errNotFound
			},
			createFunc: func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				return crb, nil
			},
			wantedCRB: defaultProjectCRB.DeepCopy(),
		},
		{
			name: "binding from prtb has wrong subject",
			rtb:  defaultPRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				crb := defaultProjectCRB.DeepCopy()
				crb.Subjects = nil
				return crb, nil
			},
			createFunc: func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				return crb, nil
			},
			deleteFunc: func(s string, _ *metav1.DeleteOptions) error {
				assert.Equal(t, defaultProjectCRB.Name, s)
				return nil
			},
			wantedCRB: defaultProjectCRB.DeepCopy(),
		},
		{
			name: "binding from prtb has wrong RoleRef",
			rtb:  defaultPRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				crb := defaultProjectCRB.DeepCopy()
				crb.RoleRef = rbacv1.RoleRef{}
				return crb, nil
			},
			createFunc: func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				return crb, nil
			},
			deleteFunc: func(s string, _ *metav1.DeleteOptions) error {
				assert.Equal(t, defaultProjectCRB.Name, s)
				return nil
			},
			wantedCRB: defaultProjectCRB.DeepCopy(),
		},
		{
			name: "binding from prtb missing label",
			rtb:  defaultPRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				crb := defaultProjectCRB.DeepCopy()
				crb.Labels = map[string]string{}
				return crb, nil
			},
			updateFunc: func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				return crb, nil
			},
			wantedCRB: defaultProjectCRB.DeepCopy(),
		},
		{
			name: "binding from prtb correct",
			rtb:  defaultPRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				return defaultProjectCRB.DeepCopy(), nil
			},
			wantedCRB: defaultProjectCRB.DeepCopy(),
		},
		{
			name: "create new binding from crtb",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				return nil, errNotFound
			},
			createFunc: func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				return crb, nil
			},
			wantedCRB: defaultClusterCRB.DeepCopy(),
		},
		{
			name: "binding from crtb has wrong subject",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				crb := defaultClusterCRB.DeepCopy()
				crb.Subjects = nil
				return crb, nil
			},
			createFunc: func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				return crb, nil
			},
			deleteFunc: func(s string, _ *metav1.DeleteOptions) error {
				assert.Equal(t, defaultClusterCRB.Name, s)
				return nil
			},
			wantedCRB: defaultClusterCRB.DeepCopy(),
		},
		{
			name: "binding from crtb has wrong RoleRef",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				crb := defaultClusterCRB.DeepCopy()
				crb.RoleRef = rbacv1.RoleRef{}
				return crb, nil
			},
			createFunc: func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				return crb, nil
			},
			deleteFunc: func(s string, _ *metav1.DeleteOptions) error {
				assert.Equal(t, defaultClusterCRB.Name, s)
				return nil
			},
			wantedCRB: defaultClusterCRB.DeepCopy(),
		},
		{
			name: "binding from crtb missing label",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				crb := defaultClusterCRB.DeepCopy()
				crb.Labels = map[string]string{}
				return crb, nil
			},
			updateFunc: func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				return crb, nil
			},
			wantedCRB: defaultClusterCRB.DeepCopy(),
		},
		{
			name: "binding from crtb correct",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				return defaultClusterCRB.DeepCopy(), nil
			},
			wantedCRB: defaultClusterCRB.DeepCopy(),
		},
		{
			name: "error getting CRB",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				return nil, fmt.Errorf("error")
			},
			wantErr: true,
		},
		{
			name: "error creating CRB",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				return nil, errNotFound
			},
			createFunc: func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				return nil, fmt.Errorf("error")
			},
			wantErr: true,
		},
		{
			name: "error deleting CRB",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				crb := defaultClusterCRB.DeepCopy()
				crb.RoleRef = rbacv1.RoleRef{}
				return crb, nil
			},
			deleteFunc: func(string, *metav1.DeleteOptions) error {
				return fmt.Errorf("error")
			},
			wantErr: true,
		},
		{
			name: "error creating CRB after delete",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				crb := defaultClusterCRB.DeepCopy()
				crb.RoleRef = rbacv1.RoleRef{}
				return crb, nil
			},
			createFunc: func(*rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				return nil, fmt.Errorf("error")
			},
			deleteFunc: func(s string, _ *metav1.DeleteOptions) error {
				assert.Equal(t, defaultClusterCRB.Name, s)
				return nil
			},
			wantErr: true,
		},
		{
			name: "error updating CRB",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			getFunc: func(string, metav1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
				crb := defaultClusterCRB.DeepCopy()
				crb.Labels = map[string]string{}
				return crb, nil
			},
			updateFunc: func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
				return nil, fmt.Errorf("error")
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			crbController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl)
			if tt.getFunc != nil {
				crbController.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(tt.getFunc)
			}
			if tt.createFunc != nil {
				crbController.EXPECT().Create(gomock.Any()).DoAndReturn(tt.createFunc)
			}
			if tt.deleteFunc != nil {
				crbController.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(tt.deleteFunc)
			}
			if tt.updateFunc != nil {
				crbController.EXPECT().Update(gomock.Any()).DoAndReturn(tt.updateFunc)
			}

			crb, err := createOrUpdateMembershipBinding(tt.rtb, tt.rt, crbController)

			if tt.wantErr {
				assert.NotNil(t, err)
			}
			if tt.wantedCRB != nil {
				assert.Equal(t, tt.wantedCRB, crb)
			}
		})
	}
}

func TestDeleteMembershipBinding(t *testing.T) {
	tests := []struct {
		name    string
		crbMock func(*fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList])
		rtb     metav1.Object
		wantErr bool
	}{
		{
			name: "remove label and others exist",
			rtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-prtb",
					Namespace: "test-namespace",
				},
			},
			crbMock: func(mock *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crbList := &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test-namespace_test-prtb":  "true",
									"test-namespace_test-prtb2": "true",
								},
							},
						},
					},
				}
				mock.EXPECT().List(gomock.Any()).Return(crbList, nil)

				mock.EXPECT().Update(gomock.Any()).DoAndReturn(func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					assert.Equal(t, map[string]string{"test-namespace_test-prtb2": "true"}, crb.Labels)
					return nil, nil
				})
			},
		},
		{
			name: "remove label and no others exist",
			rtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-prtb",
					Namespace: "test-namespace",
				},
			},
			crbMock: func(mock *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crbList := &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-crb",
								Labels: map[string]string{
									"test-namespace_test-prtb": "true",
								},
							},
						},
					},
				}
				mock.EXPECT().List(gomock.Any()).Return(crbList, nil)

				mock.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(s string, _ *metav1.DeleteOptions) error {
					assert.Equal(t, "test-crb", s)
					return nil
				})
			},
		},
		{
			name: "multiple crbs",
			rtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-prtb",
					Namespace: "test-namespace",
				},
			},
			crbMock: func(mock *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crbList := &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-crb",
								Labels: map[string]string{
									"test-namespace_test-prtb": "true",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-crb2",
								Labels: map[string]string{
									"test-namespace_test-prtb":  "true",
									"test-namespace_test-prtb2": "true",
								},
							},
						},
					},
				}
				mock.EXPECT().List(gomock.Any()).Return(crbList, nil)
				mock.EXPECT().Update(gomock.Any()).DoAndReturn(func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					assert.Equal(t, map[string]string{"test-namespace_test-prtb2": "true"}, crb.Labels)
					return nil, nil
				})
				mock.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(s string, _ *metav1.DeleteOptions) error {
					assert.Equal(t, "test-crb", s)
					return nil
				})
			},
		},
		{
			name: "error listing crbs",
			rtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-prtb",
					Namespace: "test-namespace",
				},
			},
			crbMock: func(mock *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				mock.EXPECT().List(gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
			wantErr: true,
		},
		{
			name: "error deleting does not block",
			rtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-prtb",
					Namespace: "test-namespace",
				},
			},
			crbMock: func(mock *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crbList := &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-crb2",
								Labels: map[string]string{
									"test-namespace_test-prtb":  "true",
									"test-namespace_test-prtb2": "true",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-crb",
								Labels: map[string]string{
									"test-namespace_test-prtb": "true",
								},
							},
						},
					},
				}
				mock.EXPECT().List(gomock.Any()).Return(crbList, nil)
				mock.EXPECT().Update(gomock.Any()).DoAndReturn(func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					assert.Equal(t, map[string]string{"test-namespace_test-prtb2": "true"}, crb.Labels)
					return nil, nil
				})
				mock.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))
			},
			wantErr: true,
		},
		{
			name: "error updating does not block",
			rtb: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-prtb",
					Namespace: "test-namespace",
				},
			},
			crbMock: func(mock *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crbList := &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-crb",
								Labels: map[string]string{
									"test-namespace_test-prtb": "true",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-crb2",
								Labels: map[string]string{
									"test-namespace_test-prtb":  "true",
									"test-namespace_test-prtb2": "true",
								},
							},
						},
					},
				}
				mock.EXPECT().List(gomock.Any()).Return(crbList, nil)
				mock.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("error"))
				mock.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(s string, _ *metav1.DeleteOptions) error {
					assert.Equal(t, "test-crb", s)
					return nil
				})
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			crbController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl)

			tt.crbMock(crbController)

			if err := deleteMembershipBinding(tt.rtb, crbController); (err != nil) != tt.wantErr {
				t.Errorf("deleteMembershipBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
