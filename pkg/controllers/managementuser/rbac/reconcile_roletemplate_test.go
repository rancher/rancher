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
