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
	"k8s.io/apimachinery/pkg/types"
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
	t.Parallel()
	tests := []struct {
		name               string
		rtb                metav1.Object
		rt                 *v3.RoleTemplate
		setupCRBController func(*fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList])
		wantErr            bool
	}{
		{
			name: "create new binding from prtb",
			rtb:  defaultPRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().Get(defaultProjectCRB.Name, metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(defaultProjectCRB.DeepCopy()).Return(defaultProjectCRB.DeepCopy(), nil)
			},
		},
		{
			name: "binding from prtb has wrong subject",
			rtb:  defaultPRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crb := defaultProjectCRB.DeepCopy()
				crb.Subjects = nil
				m.EXPECT().Get(defaultProjectCRB.Name, metav1.GetOptions{}).Return(crb, nil)
				m.EXPECT().Create(defaultProjectCRB.DeepCopy()).Return(defaultProjectCRB.DeepCopy(), nil)
				m.EXPECT().Delete(defaultProjectCRB.Name, &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name: "binding from prtb has wrong RoleRef",
			rtb:  defaultPRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crb := defaultProjectCRB.DeepCopy()
				crb.RoleRef = rbacv1.RoleRef{}
				m.EXPECT().Get(defaultProjectCRB.Name, metav1.GetOptions{}).Return(crb, nil)
				m.EXPECT().Create(defaultProjectCRB.DeepCopy()).Return(defaultProjectCRB.DeepCopy(), nil)
				m.EXPECT().Delete(defaultProjectCRB.Name, &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name: "binding from prtb missing label",
			rtb:  defaultPRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crb := defaultProjectCRB.DeepCopy()
				crb.Labels = map[string]string{}
				m.EXPECT().Get(defaultProjectCRB.Name, metav1.GetOptions{}).Return(crb, nil)
				m.EXPECT().Update(crb).Return(defaultProjectCRB.DeepCopy(), nil)
			},
		},
		{
			name: "binding from prtb correct",
			rtb:  defaultPRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().Get(defaultProjectCRB.Name, metav1.GetOptions{}).Return(defaultProjectCRB.DeepCopy(), nil)
			},
		},
		{
			name: "create new binding from crtb",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().Get(defaultClusterCRB.Name, metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(defaultClusterCRB.DeepCopy()).Return(defaultClusterCRB.DeepCopy(), nil)
			},
		},
		{
			name: "binding from crtb has wrong subject",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crb := defaultClusterCRB.DeepCopy()
				crb.Subjects = nil
				m.EXPECT().Get(defaultClusterCRB.Name, metav1.GetOptions{}).Return(crb, nil)
				m.EXPECT().Create(defaultClusterCRB.DeepCopy()).Return(defaultClusterCRB.DeepCopy(), nil)
				m.EXPECT().Delete(defaultClusterCRB.Name, &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name: "binding from crtb has wrong RoleRef",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crb := defaultClusterCRB.DeepCopy()
				crb.RoleRef = rbacv1.RoleRef{}
				m.EXPECT().Get(defaultClusterCRB.Name, metav1.GetOptions{}).Return(crb, nil)
				m.EXPECT().Create(defaultClusterCRB.DeepCopy()).Return(defaultClusterCRB.DeepCopy(), nil)
				m.EXPECT().Delete(defaultClusterCRB.Name, &metav1.DeleteOptions{}).Return(nil)
			},
		},
		{
			name: "binding from crtb missing label",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crb := defaultClusterCRB.DeepCopy()
				crb.Labels = map[string]string{}
				m.EXPECT().Get(defaultClusterCRB.Name, metav1.GetOptions{}).Return(crb, nil)
				m.EXPECT().Update(crb).Return(defaultClusterCRB.DeepCopy(), nil)
			},
		},
		{
			name: "binding from crtb correct",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().Get(defaultClusterCRB.Name, metav1.GetOptions{}).Return(defaultClusterCRB.DeepCopy(), nil)
			},
		},
		{
			name: "error getting CRB",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().Get(defaultClusterCRB.Name, metav1.GetOptions{}).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error creating CRB",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				m.EXPECT().Get(defaultClusterCRB.Name, metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(defaultClusterCRB.DeepCopy()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error deleting CRB",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crb := defaultClusterCRB.DeepCopy()
				crb.Subjects = nil
				m.EXPECT().Get(defaultClusterCRB.Name, metav1.GetOptions{}).Return(crb, nil)
				m.EXPECT().Delete(defaultClusterCRB.Name, &metav1.DeleteOptions{}).Return(errDefault)
			},
			wantErr: true,
		},
		{
			name: "error creating CRB after delete",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crb := defaultClusterCRB.DeepCopy()
				crb.Subjects = nil
				m.EXPECT().Get(defaultClusterCRB.Name, metav1.GetOptions{}).Return(crb, nil)
				m.EXPECT().Delete(defaultClusterCRB.Name, &metav1.DeleteOptions{}).Return(nil)
				m.EXPECT().Create(defaultClusterCRB.DeepCopy()).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error updating CRB",
			rtb:  defaultCRTB.DeepCopy(),
			rt: &v3.RoleTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-roletemplate"},
			},
			setupCRBController: func(m *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]) {
				crb := defaultClusterCRB.DeepCopy()
				crb.Labels = map[string]string{}
				m.EXPECT().Get(defaultClusterCRB.Name, metav1.GetOptions{}).Return(crb, nil)
				m.EXPECT().Update(crb).Return(nil, errDefault)
			},
			wantErr: true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			crbController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl)
			if tt.setupCRBController != nil {
				tt.setupCRBController(crbController)
			}

			err := createOrUpdateMembershipBinding(tt.rtb, tt.rt, crbController)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

var (
	listOption      = metav1.ListOptions{LabelSelector: "test-namespace_test-prtb"}
	uid             = types.UID("abc123")
	resourceVersion = "1"
	deleteOption    = metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{
			UID:             &uid,
			ResourceVersion: &resourceVersion,
		},
	}
)

func TestDeleteMembershipBinding(t *testing.T) {
	t.Parallel()
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
				crb := rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"test-namespace_test-prtb":  "true",
							"test-namespace_test-prtb2": "true",
						},
					},
				}
				crbList := &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{crb},
				}
				mock.EXPECT().List(listOption).Return(crbList, nil)

				crb.Labels = map[string]string{"test-namespace_test-prtb2": "true"}
				mock.EXPECT().Update(&crb).Return(nil, nil)
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
				crb := rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-crb",
						Labels: map[string]string{
							"test-namespace_test-prtb": "true",
						},
						UID:             uid,
						ResourceVersion: resourceVersion,
					},
				}
				crbList := &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{crb},
				}
				mock.EXPECT().List(listOption).Return(crbList, nil)

				mock.EXPECT().Delete("test-crb", &deleteOption).Return(nil)
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
				crb1 := rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-crb",
						Labels: map[string]string{
							"test-namespace_test-prtb": "true",
						},
						UID:             uid,
						ResourceVersion: resourceVersion,
					},
				}
				crb2 := rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-crb2",
						Labels: map[string]string{
							"test-namespace_test-prtb":  "true",
							"test-namespace_test-prtb2": "true",
						},
					},
				}
				crbList := &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{crb1, crb2},
				}
				mock.EXPECT().List(listOption).Return(crbList, nil)
				crb2.Labels = map[string]string{"test-namespace_test-prtb2": "true"}
				mock.EXPECT().Update(gomock.Any()).Return(nil, nil)
				mock.EXPECT().Delete("test-crb", &deleteOption).Return(nil)
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
				mock.EXPECT().List(listOption).Return(nil, fmt.Errorf("error"))
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
				crb1 := rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-crb",
						Labels: map[string]string{
							"test-namespace_test-prtb": "true",
						},
						UID:             uid,
						ResourceVersion: resourceVersion,
					},
				}
				crb2 := rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-crb2",
						Labels: map[string]string{
							"test-namespace_test-prtb":  "true",
							"test-namespace_test-prtb2": "true",
						},
					},
				}
				crbList := &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{crb1, crb2},
				}
				mock.EXPECT().List(listOption).Return(crbList, nil)
				crb2.Labels = map[string]string{"test-namespace_test-prtb2": "true"}
				mock.EXPECT().Update(gomock.Any()).Return(nil, nil)
				mock.EXPECT().Delete("test-crb", &deleteOption).Return(errDefault)
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
				crb1 := rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-crb",
						Labels: map[string]string{
							"test-namespace_test-prtb": "true",
						},
						UID:             uid,
						ResourceVersion: resourceVersion,
					},
				}
				crb2 := rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-crb2",
						Labels: map[string]string{
							"test-namespace_test-prtb":  "true",
							"test-namespace_test-prtb2": "true",
						},
					},
				}
				crbList := &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{crb1, crb2},
				}
				mock.EXPECT().List(listOption).Return(crbList, nil)
				crb2.Labels = map[string]string{"test-namespace_test-prtb2": "true"}
				mock.EXPECT().Update(gomock.Any()).Return(nil, errDefault)
				mock.EXPECT().Delete("test-crb", &deleteOption).Return(nil)
			},
			wantErr: true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			crbController := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl)

			tt.crbMock(crbController)

			if err := deleteMembershipBinding(tt.rtb, crbController); (err != nil) != tt.wantErr {
				t.Errorf("deleteMembershipBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
