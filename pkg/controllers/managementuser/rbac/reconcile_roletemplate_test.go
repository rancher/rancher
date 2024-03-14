package rbac

import (
	"io"
	"testing"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEnsureGlobalResourcesRolesForPRTB(t *testing.T) {
	t.Parallel()
	logrus.SetOutput(io.Discard)

	defaultManager := setupManager(
		map[string]*v3.RoleTemplate{"create-ns": createNSRoleTemplate},
		map[string]*v1.ClusterRole{},
		map[string]*v1.Role{},
		map[string]*v3.Project{},
		clientErrs{}, clientErrs{}, clientErrs{},
	)

	type testCase struct {
		description   string
		manager       *manager
		projectName   string
		roleTemplates map[string]*v3.RoleTemplate
		expectedRoles []string
		isErrExpected bool
	}
	testCases := []testCase{
		{
			description:   "global resource rule should grant namespace read",
			manager:       defaultManager,
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
			description:   "namespace create rule should grant create-ns and a namespaces-edit role",
			manager:       defaultManager,
			projectName:   "testproject",
			expectedRoles: []string{"create-ns", "testproject-namespaces-edit"},
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
			description:   "namespace create rule for other API group should grant namespaces-read role only",
			manager:       defaultManager,
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
			manager:       defaultManager,
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
			manager:       defaultManager,
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
			manager:       defaultManager,
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
			manager:       defaultManager,
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
			manager:       defaultManager,
			projectName:   "testproject",
			expectedRoles: []string{"create-ns", "testproject-namespaces-edit", "testrt8-promoted"},
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
			manager:       defaultManager,
			projectName:   "testproject",
			expectedRoles: []string{"create-ns", "testproject-namespaces-edit", "testrt9-promoted"},
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
			manager: setupManager(
				map[string]*v3.RoleTemplate{"create-ns": createNSRoleTemplate},
				make(map[string]*v1.ClusterRole),
				make(map[string]*v1.Role),
				make(map[string]*v3.Project),
				clientErrs{},
				clientErrs{getError: errNotFound},
				clientErrs{},
			),
			isErrExpected: true,
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
			manager: setupManager(
				map[string]*v3.RoleTemplate{"create-ns": createNSRoleTemplate},
				make(map[string]*v1.ClusterRole),
				make(map[string]*v1.Role),
				make(map[string]*v3.Project),
				clientErrs{getError: apierrors.NewInternalError(errors.New("error"))},
				clientErrs{},
				clientErrs{},
			),
			isErrExpected: true,
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

			roles, err := test.manager.ensureGlobalResourcesRolesForPRTB(test.projectName, test.roleTemplates)

			if test.isErrExpected {
				assert.Error(t, err)
				assert.Nil(t, roles)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedRoles, roles, test.description)
			}
		})
	}
}
