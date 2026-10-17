package rbac

import (
	"fmt"
	"io"
	"testing"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	wfakes "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func TestEnsureGlobalResourcesRolesForPRTB(t *testing.T) {
	t.Parallel()
	logrus.SetOutput(io.Discard)

	ctrl := gomock.NewController(t)
	defaultManager := newManager(
		withRoleTemplates(map[string]*v3.RoleTemplate{"create-ns": createNSRoleTemplate}, nil, ctrl),
		withClusterRoles(nil, nil, ctrl),
		func(m *manager) {
			clusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList](ctrl)
			clusterRoles.EXPECT().Create(gomock.Any()).DoAndReturn(
				func(In1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					return In1, nil
				},
			).AnyTimes()
			m.clusterRoles = clusterRoles
		},
	)

	type testCase struct {
		description   string
		manager       *manager
		projectName   string
		roleTemplates map[string]*v3.RoleTemplate
		expectedRoles []string
		expectedError string
	}
	testCases := []testCase{
		{
			description:   "global resource rule should grant namespace read",
			projectName:   "testproject",
			expectedRoles: []string{"testproject-namespaces-readonly"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt1",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{""},
							Resources: []string{"configmaps"},
						},
					},
				},
			},
		},
		{
			description:   "namespace create rule should grant create-ns, namespaces-readonly and namespaces-manage roles",
			projectName:   "testproject",
			expectedRoles: []string{"create-ns", "testproject-namespaces-manage", "testproject-namespaces-readonly"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt2",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"create"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
		},
		{
			description:   "namespace * rule should grant create-ns, namespaces-edit and namespaces-manage roles",
			projectName:   "testproject",
			expectedRoles: []string{"create-ns", "testproject-namespaces-edit", "testproject-namespaces-manage"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
		},
		{
			description:   "namespace update rule should grant namespaces-readonly and namespaces-manage roles",
			projectName:   "testproject",
			expectedRoles: []string{"testproject-namespaces-manage", "testproject-namespaces-readonly"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"update"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
		},
		{
			description:   "namespace patch rule should grant namespaces-readonly and namespaces-manage roles",
			projectName:   "testproject",
			expectedRoles: []string{"testproject-namespaces-manage", "testproject-namespaces-readonly"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"patch"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
		},
		{
			description:   "namespace delete rule should only grant namespaces-readonly role",
			projectName:   "testproject",
			expectedRoles: []string{"testproject-namespaces-readonly"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"delete"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
		},
		{
			description:   "namespace rule listing every verb without * should not grant namespaces-edit role",
			projectName:   "testproject",
			expectedRoles: []string{"create-ns", "testproject-namespaces-manage", "testproject-namespaces-readonly"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
		},
		{
			description:   "namespace rules are combined across inherited RoleTemplates",
			projectName:   "testproject",
			expectedRoles: []string{"create-ns", "testproject-namespaces-edit", "testproject-namespaces-manage"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt-parent": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt-parent",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"create"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
				"testrt-child": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt-child",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
		},
		{
			description: "external RoleTemplate uses the rules of its ClusterRole",
			projectName: "testproject",
			manager: newManager(
				withRoleTemplates(map[string]*v3.RoleTemplate{"create-ns": createNSRoleTemplate}, nil, ctrl),
				withClusterRoles(map[string]*v1.ClusterRole{
					"testrt-external": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "testrt-external",
						},
						Rules: []v1.PolicyRule{
							{
								Verbs:     []string{"create"},
								APIGroups: []string{""},
								Resources: []string{"namespaces"},
							},
						},
					},
				}, nil, ctrl),
				func(m *manager) {
					clusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList](ctrl)
					clusterRoles.EXPECT().Create(gomock.Any()).DoAndReturn(
						func(In1 *v1.ClusterRole) (*v1.ClusterRole, error) {
							return In1, nil
						},
					).AnyTimes()
					m.clusterRoles = clusterRoles
				},
			),
			expectedRoles: []string{"create-ns", "testproject-namespaces-manage", "testproject-namespaces-readonly"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt-external": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt-external",
					},
					External: true,
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
		},
		{
			description:   "namespace create rule for other API group should grant namespaces-read role only",
			projectName:   "testproject",
			expectedRoles: []string{"testproject-namespaces-readonly"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt3": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt3",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"create"},
							APIGroups: []string{"some.other.apigroup"},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
		},
		{
			description:   "namespace * rule for other API group should grant namespaces-read role only",
			projectName:   "testproject",
			expectedRoles: []string{"testproject-namespaces-readonly"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt4": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt4",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"some.other.apigroup"},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
		},
		{
			description:   "global resource rule result in promoted role returned",
			projectName:   "testproject",
			expectedRoles: []string{"testproject-namespaces-readonly", "testrt5-promoted"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt5": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt5",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"catalog.cattle.io"},
							Resources: []string{"clusterrepos"},
						},
					},
				},
			},
		},
		{
			description:   "empty project name will result in no roles returned",
			projectName:   "",
			expectedRoles: nil,
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt6": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt6",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"catalog.cattle.io"},
							Resources: []string{"clusterrepos"},
						},
					},
				},
			},
		},
		{
			description:   "* resources and non-core APIGroup should only result in namespace-readonly role",
			projectName:   "testproject",
			expectedRoles: []string{"testproject-namespaces-readonly"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt7": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt7",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"some.other.apigroup"},
							Resources: []string{"*"},
						},
					},
				},
			},
		},
		{
			description:   "* resources and * APIGroup should only result in namespace-readonly and promoted role",
			projectName:   "testproject",
			expectedRoles: []string{"create-ns", "testproject-namespaces-edit", "testproject-namespaces-manage", "testrt8-promoted"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt8": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt8",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"*"},
							Resources: []string{"*"},
						},
					},
				},
			},
		},
		{
			description:   "* resources and core (\"\") APIGroup should only result in namespace-readonly and promoted role",
			projectName:   "testproject",
			expectedRoles: []string{"create-ns", "testproject-namespaces-edit", "testproject-namespaces-manage", "testrt9-promoted"},
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt9": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt9",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{""},
							Resources: []string{"*"},
						},
					},
				},
			},
		},
		{
			projectName: "testproject",
			description: "error return when RoleTemplate client returns error",
			manager: newManager(
				withRoleTemplates(map[string]*v3.RoleTemplate{"create-ns": createNSRoleTemplate}, &clientErrs{getError: errNotFound}, ctrl),
				withClusterRoles(nil, nil, ctrl),
			),
			expectedError: "not found",
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"create"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
				},
			},
		},
		{
			projectName: "testproject",
			description: "error return when ClusterRole client returns error and RoleTemplate is external",
			manager: newManager(
				withRoleTemplates(map[string]*v3.RoleTemplate{"create-ns": createNSRoleTemplate}, nil, ctrl),
				withClusterRoles(nil, &clientErrs{getError: apierrors.NewInternalError(errors.New("internal error"))}, ctrl),
				func(m *manager) {
					clusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList](ctrl)
					clusterRoles.EXPECT().Create(gomock.Any()).DoAndReturn(
						func(In1 *v1.ClusterRole) (*v1.ClusterRole, error) {
							return nil, fmt.Errorf("internal error")
						},
					).AnyTimes()
					m.clusterRoles = clusterRoles
				},
			),
			expectedError: "internal error",
			roleTemplates: map[string]*v3.RoleTemplate{
				"testrt": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "testrt",
					},
					Rules: []v1.PolicyRule{
						{
							Verbs:     []string{"create"},
							APIGroups: []string{""},
							Resources: []string{"namespaces"},
						},
					},
					External: true,
				},
			},
		},
	}
	for _, test := range testCases {
		test := test
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			manager := test.manager
			if manager == nil {
				manager = defaultManager
			}

			roles, err := manager.ensureGlobalResourcesRolesForPRTB(test.projectName, test.roleTemplates)

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
				assert.Nil(t, roles)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedRoles, roles, test.description)
			}
		})
	}
}

func TestRemoveStaleProjectNamespaceCRBs(t *testing.T) {
	t.Parallel()

	binding := &v3.ProjectRoleTemplateBinding{
		ObjectMeta:  metav1.ObjectMeta{Name: "prtb-1", Namespace: "p-abc", UID: "uid-1"},
		ProjectName: "c-123:p-abc",
	}
	otherBinding := &v3.ProjectRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "prtb-2", Namespace: "p-abc", UID: "uid-2"},
	}
	bindingLabel := "p-abc_prtb-1"
	otherBindingLabel := "p-abc_prtb-2"
	newCRB := func(name, roleName string, crbLabels map[string]string) *v1.ClusterRoleBinding {
		return &v1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: crbLabels},
			RoleRef:    v1.RoleRef{Kind: "ClusterRole", Name: roleName},
		}
	}

	tests := []struct {
		name        string
		binding     *v3.ProjectRoleTemplateBinding
		crbs        []*v1.ClusterRoleBinding
		crbsToKeep  map[string]bool
		listErr     error
		deleteErr   error
		wantErr     bool
		wantDeleted []string
		wantUpdated []*v1.ClusterRoleBinding
	}{
		{
			name:    "stale namespace role bindings owned only by the binding are deleted",
			binding: binding,
			crbs: []*v1.ClusterRoleBinding{
				newCRB("edit", "p-abc-namespaces-edit", map[string]string{bindingLabel: owner}),
				newCRB("create-ns", "create-ns", map[string]string{bindingLabel: owner}),
				newCRB("manage", "p-abc-namespaces-manage", map[string]string{bindingLabel: owner}),
				newCRB("readonly", "p-abc-namespaces-readonly", map[string]string{bindingLabel: owner}),
			},
			wantDeleted: []string{"edit", "create-ns", "manage", "readonly"},
		},
		{
			name:    "stale binding with a pre 2.5 UID label is deleted",
			binding: binding,
			crbs: []*v1.ClusterRoleBinding{
				newCRB("edit", "p-abc-namespaces-edit", map[string]string{bindingLabel: owner, "uid-1": owner}),
			},
			wantDeleted: []string{"edit"},
		},
		{
			name:    "stale binding shared with another binding only loses the binding's labels",
			binding: binding,
			crbs: []*v1.ClusterRoleBinding{
				newCRB("edit", "p-abc-namespaces-edit", map[string]string{bindingLabel: owner, "uid-1": owner, otherBindingLabel: owner}),
			},
			wantUpdated: []*v1.ClusterRoleBinding{
				newCRB("edit", "p-abc-namespaces-edit", map[string]string{otherBindingLabel: owner}),
			},
		},
		{
			name:    "desired bindings are kept",
			binding: binding,
			crbs: []*v1.ClusterRoleBinding{
				newCRB("readonly", "p-abc-namespaces-readonly", map[string]string{bindingLabel: owner}),
				newCRB("create-ns", "create-ns", map[string]string{bindingLabel: owner}),
			},
			crbsToKeep: map[string]bool{"readonly": true, "create-ns": true},
		},
		{
			name:    "bindings to other roles are not changed",
			binding: binding,
			crbs: []*v1.ClusterRoleBinding{
				newCRB("promoted", "rt-promoted", map[string]string{bindingLabel: owner}),
				newCRB("other-project", "p-other-namespaces-edit", map[string]string{bindingLabel: owner}),
			},
		},
		{
			name:    "bindings without the binding's label are not changed",
			binding: binding,
			crbs: []*v1.ClusterRoleBinding{
				newCRB("edit", "p-abc-namespaces-edit", map[string]string{otherBindingLabel: owner}),
			},
		},
		{
			name: "binding without a project is skipped",
			binding: &v3.ProjectRoleTemplateBinding{
				ObjectMeta: binding.ObjectMeta,
			},
			crbs: []*v1.ClusterRoleBinding{
				newCRB("edit", "p-abc-namespaces-edit", map[string]string{bindingLabel: owner}),
			},
		},
		{
			name:    "already deleted binding is ignored",
			binding: binding,
			crbs: []*v1.ClusterRoleBinding{
				newCRB("edit", "p-abc-namespaces-edit", map[string]string{bindingLabel: owner}),
			},
			deleteErr:   apierrors.NewNotFound(v1.Resource("clusterrolebindings"), "edit"),
			wantDeleted: []string{"edit"},
		},
		{
			name:    "delete error",
			binding: binding,
			crbs: []*v1.ClusterRoleBinding{
				newCRB("edit", "p-abc-namespaces-edit", map[string]string{bindingLabel: owner}),
			},
			deleteErr:   fmt.Errorf("unexpected error"),
			wantErr:     true,
			wantDeleted: []string{"edit"},
		},
		{
			name:    "list error",
			binding: binding,
			listErr: fmt.Errorf("unexpected error"),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			prtbIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
				prtbByUIDIndex:       prtbByUID,
				prtbByNsAndNameIndex: prtbByNsName,
			})
			assert.NoError(t, prtbIndexer.Add(binding))
			assert.NoError(t, prtbIndexer.Add(otherBinding))

			crbLister := wfakes.NewMockNonNamespacedCacheInterface[*v1.ClusterRoleBinding](ctrl)
			crbLister.EXPECT().List(gomock.Any()).DoAndReturn(func(selector labels.Selector) ([]*v1.ClusterRoleBinding, error) {
				if test.listErr != nil {
					return nil, test.listErr
				}
				var matched []*v1.ClusterRoleBinding
				for _, crb := range test.crbs {
					if selector.Matches(labels.Set(crb.Labels)) {
						matched = append(matched, crb)
					}
				}
				return matched, nil
			}).AnyTimes()

			var deleted []string
			var updated []*v1.ClusterRoleBinding
			crbClient := wfakes.NewMockNonNamespacedControllerInterface[*v1.ClusterRoleBinding, *v1.ClusterRoleBindingList](ctrl)
			crbClient.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, _ *metav1.DeleteOptions) error {
				deleted = append(deleted, name)
				return test.deleteErr
			}).AnyTimes()
			crbClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(crb *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
				updated = append(updated, crb)
				return crb, nil
			}).AnyTimes()

			m := &manager{
				prtbIndexer:         prtbIndexer,
				crbLister:           crbLister,
				clusterRoleBindings: crbClient,
			}
			if test.crbsToKeep == nil {
				test.crbsToKeep = make(map[string]bool)
			}

			err := m.removeStaleProjectNamespaceCRBs(test.binding, test.crbsToKeep)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.ElementsMatch(t, test.wantDeleted, deleted)
			assert.ElementsMatch(t, test.wantUpdated, updated)
		})
	}
}
