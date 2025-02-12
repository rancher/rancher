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
		UserName:         "test-user",
		ProjectName:      "test-cluster:test-project",
		RoleTemplateName: "test-rt",
	}
	defaultProjectRoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Name:     "test-cluster-cluster-member",
		Kind:     "ClusterRole",
	}
	defaultProjectCRB = rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "crb-eawz62u5xd",
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
		UserName:         "test-user",
		ClusterName:      "test-cluster",
		RoleTemplateName: "test-rt",
	}
	defaultClusterRoleRef = rbacv1.RoleRef{
		Name:     "test-cluster-cluster-member",
		Kind:     "ClusterRole",
		APIGroup: "rbac.authorization.k8s.io",
	}
	defaultClusterCRB = rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "crb-eawz62u5xd",
			Labels: map[string]string{"test-namespace_test-crtb": "true"},
		},
		Subjects: []rbacv1.Subject{defaultSubject},
		RoleRef:  defaultClusterRoleRef,
	}
)

func TestCreateOrUpdateClusterMembershipBinding(t *testing.T) {
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

			err := createOrUpdateClusterMembershipBinding(tt.rtb, tt.rt, crbController)

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

func TestDeleteClusterMembershipBinding(t *testing.T) {
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

			if err := deleteClusterMembershipBinding(tt.rtb, crbController); (err != nil) != tt.wantErr {
				t.Errorf("deleteMembershipBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getClusterMembershipRoleName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rt   *v3.RoleTemplate
		rtb  metav1.Object
		want string
	}{
		{
			name: "get CRTB owner role",
			rt: &v3.RoleTemplate{
				Builtin: true,
				Context: "cluster",
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-owner",
				},
			},
			rtb: &v3.ClusterRoleTemplateBinding{
				ClusterName: "c-abc123",
			},
			want: "c-abc123-cluster-owner",
		},
		{
			name: "get CRTB member role",
			rt: &v3.RoleTemplate{
				Context: "cluster",
			},
			rtb: &v3.ClusterRoleTemplateBinding{
				ClusterName: "c-abc123",
			},
			want: "c-abc123-cluster-member",
		},
		{
			name: "get PRTB role",
			rt: &v3.RoleTemplate{
				Context: "project",
				Builtin: true,
				ObjectMeta: metav1.ObjectMeta{
					Name: "project-owner",
				},
			},
			rtb: &v3.ProjectRoleTemplateBinding{
				ProjectName: "c-abc123:p-xyz789",
			},
			want: "c-abc123-cluster-member",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := getClusterMembershipRoleName(tt.rt, tt.rtb); got != tt.want {
				t.Errorf("getClusterMembershipRoleName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getProjectMembershipRoleName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rt   *v3.RoleTemplate
		prtb *v3.ProjectRoleTemplateBinding
		want string
	}{
		{
			name: "get owner role",
			rt: &v3.RoleTemplate{
				Builtin: true,
				Context: "project",
				ObjectMeta: metav1.ObjectMeta{
					Name: "project-owner",
				},
			},
			prtb: &v3.ProjectRoleTemplateBinding{
				ProjectName: "c-abc123:p-xyz789",
			},
			want: "p-xyz789-project-owner",
		},
		{
			name: "get member role",
			rt: &v3.RoleTemplate{
				Context: "project",
			},
			prtb: &v3.ProjectRoleTemplateBinding{
				ProjectName: "c-abc123:p-xyz789",
			},
			want: "p-xyz789-project-member",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := getProjectMembershipRoleName(tt.rt, tt.prtb); got != tt.want {
				t.Errorf("getProjectMembershipRoleName() = %v, want %v", got, tt.want)
			}
		})
	}
}

var (
	prtbListOptions = metav1.ListOptions{LabelSelector: "authz.cluster.cattle.io/prtb-owner=test-prtb"}
	crtbListOptions = metav1.ListOptions{LabelSelector: "authz.cluster.cattle.io/crtb-owner=test-crtb"}
)

func Test_removeAuthV2Permissions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		setupController func(*fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList])
		obj             metav1.Object
		wantErr         bool
	}{
		{
			name: "error listing rolebindings",
			setupController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("fleet-default", prtbListOptions).Return(nil, errDefault)
			},
			obj:     defaultPRTB.DeepCopy(),
			wantErr: true,
		},
		{
			name: "error deleting rolebindings",
			setupController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("fleet-default", prtbListOptions).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "rb1",
								Namespace: "fleet-default",
							},
						},
					},
				}, nil)
				m.EXPECT().Delete("fleet-default", "rb1", &metav1.DeleteOptions{}).Return(errDefault)
			},
			obj:     defaultPRTB.DeepCopy(),
			wantErr: true,
		},
		{
			name: "delete rolebindings for PRTB",
			setupController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("fleet-default", prtbListOptions).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "rb1",
								Namespace: "fleet-default",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "rb2",
								Namespace: "fleet-default",
							},
						},
					},
				}, nil)
				m.EXPECT().Delete("fleet-default", "rb1", &metav1.DeleteOptions{}).Return(nil)
				m.EXPECT().Delete("fleet-default", "rb2", &metav1.DeleteOptions{}).Return(nil)
			},
			obj: defaultPRTB.DeepCopy(),
		},
		{
			name: "delete rolebindings for CRTB",
			setupController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("fleet-default", crtbListOptions).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "rb1",
								Namespace: "fleet-default",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "rb2",
								Namespace: "fleet-default",
							},
						},
					},
				}, nil)
				m.EXPECT().Delete("fleet-default", "rb1", &metav1.DeleteOptions{}).Return(nil)
				m.EXPECT().Delete("fleet-default", "rb2", &metav1.DeleteOptions{}).Return(nil)
			},
			obj: defaultCRTB.DeepCopy(),
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
			if tt.setupController != nil {
				tt.setupController(rbController)
			}
			if err := removeAuthV2Permissions(tt.obj, rbController); (err != nil) != tt.wantErr {
				t.Errorf("removeAuthV2Permissions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

var (
	defaultRoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Name:     "test-project-project-member",
		Kind:     "ClusterRole",
	}
	projectOwnerRoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Name:     "test-project-project-owner",
		Kind:     "ClusterRole",
	}
	defaultRoleBinding = rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rb-y5xedljk46",
			Namespace: "test-cluster",
			Labels:    map[string]string{"test-namespace_test-prtb": "true"},
		},
		Subjects: []rbacv1.Subject{defaultSubject},
		RoleRef:  defaultRoleRef,
	}
	projectOwnerRoleBinding = rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rb-3hgt2h3gvw",
			Namespace: "test-cluster",
			Labels:    map[string]string{"test-namespace_test-prtb": "true"},
		},
		Subjects: []rbacv1.Subject{defaultSubject},
		RoleRef:  projectOwnerRoleRef,
	}
	projectOwnerRT = v3.RoleTemplate{
		Builtin: true,
		Context: "project",
		ObjectMeta: metav1.ObjectMeta{
			Name: "project-owner",
		},
	}
	projectMemberRT = v3.RoleTemplate{
		Context: "project",
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-rt",
		},
	}
)

func Test_createOrUpdateProjectMembershipBinding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		setupRBController func(*fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList])
		prtb              *v3.ProjectRoleTemplateBinding
		rt                *v3.RoleTemplate
		wantErr           bool
	}{
		{
			name: "error getting rolebinding",
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().Get("test-cluster", defaultRoleBinding.Name, metav1.GetOptions{}).Return(nil, errDefault)
			},
			prtb:    defaultPRTB.DeepCopy(),
			rt:      projectMemberRT.DeepCopy(),
			wantErr: true,
		},
		{
			name: "rolebinding not found error creating",
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().Get("test-cluster", defaultRoleBinding.Name, metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(defaultRoleBinding.DeepCopy()).Return(nil, errDefault)
			},
			prtb:    defaultPRTB.DeepCopy(),
			rt:      projectMemberRT.DeepCopy(),
			wantErr: true,
		},
		{
			name: "rolebinding not found success creating",
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().Get("test-cluster", defaultRoleBinding.Name, metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(defaultRoleBinding.DeepCopy()).Return(nil, nil)
			},
			prtb: defaultPRTB.DeepCopy(),
			rt:   projectMemberRT.DeepCopy(),
		},
		{
			name: "rolebinding already exists",
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().Get("test-cluster", defaultRoleBinding.Name, metav1.GetOptions{}).Return(defaultRoleBinding.DeepCopy(), nil)
			},
			prtb: defaultPRTB.DeepCopy(),
			rt:   projectMemberRT.DeepCopy(),
		},
		{
			name: "existing rolebinding missing subject",
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				rb := defaultRoleBinding.DeepCopy()
				rb.Subjects = []rbacv1.Subject{}
				m.EXPECT().Get("test-cluster", defaultRoleBinding.Name, metav1.GetOptions{}).Return(rb, nil)
				m.EXPECT().Delete(rb.Namespace, rb.Name, &metav1.DeleteOptions{}).Return(nil)
				m.EXPECT().Create(defaultRoleBinding.DeepCopy()).Return(nil, nil)
			},
			prtb: defaultPRTB.DeepCopy(),
			rt:   projectMemberRT.DeepCopy(),
		},
		{
			name: "existing rolebinding missing roleref",
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				rb := defaultRoleBinding.DeepCopy()
				rb.RoleRef = rbacv1.RoleRef{}
				m.EXPECT().Get("test-cluster", defaultRoleBinding.Name, metav1.GetOptions{}).Return(rb, nil)
				m.EXPECT().Delete(rb.Namespace, rb.Name, &metav1.DeleteOptions{}).Return(nil)
				m.EXPECT().Create(defaultRoleBinding.DeepCopy()).Return(nil, nil)
			},
			prtb: defaultPRTB.DeepCopy(),
			rt:   projectMemberRT.DeepCopy(),
		},
		{
			name: "error deleting rolebinding",
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				rb := defaultRoleBinding.DeepCopy()
				rb.RoleRef = rbacv1.RoleRef{}
				m.EXPECT().Get("test-cluster", defaultRoleBinding.Name, metav1.GetOptions{}).Return(rb, nil)
				m.EXPECT().Delete(rb.Namespace, rb.Name, &metav1.DeleteOptions{}).Return(errDefault)
			},
			prtb:    defaultPRTB.DeepCopy(),
			rt:      projectMemberRT.DeepCopy(),
			wantErr: true,
		},
		{
			name: "error creating rolebinding with wrong contents",
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				rb := defaultRoleBinding.DeepCopy()
				rb.RoleRef = rbacv1.RoleRef{}
				m.EXPECT().Get("test-cluster", defaultRoleBinding.Name, metav1.GetOptions{}).Return(rb, nil)
				m.EXPECT().Delete(rb.Namespace, rb.Name, &metav1.DeleteOptions{}).Return(nil)
				m.EXPECT().Create(defaultRoleBinding.DeepCopy()).Return(nil, errDefault)
			},
			prtb:    defaultPRTB.DeepCopy(),
			rt:      projectMemberRT.DeepCopy(),
			wantErr: true,
		},
		{
			name: "rolebinding needs labels updated",
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				rb := defaultRoleBinding.DeepCopy()
				rb.Labels = map[string]string{}
				m.EXPECT().Get("test-cluster", defaultRoleBinding.Name, metav1.GetOptions{}).Return(rb, nil)
				m.EXPECT().Update(defaultRoleBinding.DeepCopy()).Return(nil, nil)
			},
			prtb: defaultPRTB.DeepCopy(),
			rt:   projectMemberRT.DeepCopy(),
		},
		{
			name: "error updating labels",
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				rb := defaultRoleBinding.DeepCopy()
				rb.Labels = map[string]string{}
				m.EXPECT().Get("test-cluster", defaultRoleBinding.Name, metav1.GetOptions{}).Return(rb, nil)
				m.EXPECT().Update(defaultRoleBinding.DeepCopy()).Return(nil, errDefault)
			},
			prtb:    defaultPRTB.DeepCopy(),
			rt:      projectMemberRT.DeepCopy(),
			wantErr: true,
		},
		{
			name: "create owner rolebinding",
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().Get("test-cluster", projectOwnerRoleBinding.Name, metav1.GetOptions{}).Return(nil, errNotFound)
				m.EXPECT().Create(projectOwnerRoleBinding.DeepCopy()).Return(nil, nil)
			},
			prtb: defaultPRTB.DeepCopy(),
			rt:   projectOwnerRT.DeepCopy(),
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
			if err := createOrUpdateProjectMembershipBinding(tt.prtb, tt.rt, rbController); (err != nil) != tt.wantErr {
				t.Errorf("createOrUpdateProjectMembershipBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_deleteProjectMembershipBinding(t *testing.T) {
	tests := []struct {
		name              string
		prtb              *v3.ProjectRoleTemplateBinding
		setupRBController func(*fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList])
		wantErr           bool
	}{
		{
			name: "error listing role bindings",
			prtb: defaultPRTB.DeepCopy(),
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				m.EXPECT().List("test-project", listOption).Return(nil, errDefault)
			},
			wantErr: true,
		},
		{
			name: "error deleting role bindings",
			prtb: defaultPRTB.DeepCopy(),
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				rb := rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rb1",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"test-namespace_test-prtb": "true",
						},
						UID:             uid,
						ResourceVersion: resourceVersion,
					},
				}

				m.EXPECT().List("test-project", listOption).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{rb},
				}, nil)
				m.EXPECT().Delete(rb.Namespace, rb.Name, &deleteOption).Return(errDefault)
			},
			wantErr: true,
		},
		{
			name: "success deleting role binding",
			prtb: defaultPRTB.DeepCopy(),
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				rb := rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rb1",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"test-namespace_test-prtb": "true",
						},
						UID:             uid,
						ResourceVersion: resourceVersion,
					},
				}

				m.EXPECT().List("test-project", listOption).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{rb},
				}, nil)
				m.EXPECT().Delete(rb.Namespace, rb.Name, &deleteOption).Return(nil)
			},
		},
		{
			name: "role binding has multiple labels, update with correct labels",
			prtb: defaultPRTB.DeepCopy(),
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				rb := rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rb1",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"test-namespace_test-prtb":  "true",
							"test-namespace_test-prtb2": "true",
						},
						UID:             uid,
						ResourceVersion: resourceVersion,
					},
				}

				m.EXPECT().List("test-project", listOption).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{rb},
				}, nil)
				rb.Labels = map[string]string{"test-namespace_test-prtb2": "true"}
				m.EXPECT().Update(&rb).Return(nil, nil)
			},
		},
		{
			name: "error updating role binding",
			prtb: defaultPRTB.DeepCopy(),
			setupRBController: func(m *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]) {
				rb := rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rb1",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"test-namespace_test-prtb":  "true",
							"test-namespace_test-prtb2": "true",
						},
						UID:             uid,
						ResourceVersion: resourceVersion,
					},
				}

				m.EXPECT().List("test-project", listOption).Return(&rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{rb},
				}, nil)
				rb.Labels = map[string]string{"test-namespace_test-prtb2": "true"}
				m.EXPECT().Update(&rb).Return(nil, errDefault)
			},
			wantErr: true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rbController := fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
			if tt.setupRBController != nil {
				tt.setupRBController(rbController)
			}
			if err := deleteProjectMembershipBinding(tt.prtb, rbController); (err != nil) != tt.wantErr {
				t.Errorf("deleteProjectMembershipBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
