package rbac

import (
	"testing"

	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEnsureGlobalResourcesRolesForPRTB(t *testing.T) {
	t.Parallel()
	m := setupManager(map[string]*v3.RoleTemplate{"create-ns": createNSRoleTemplate}, make(map[string]*v1.ClusterRole), make(map[string]*v1.Role), make(map[string]*v3.Project), clientErrs{}, clientErrs{}, clientErrs{})
	type testCase struct {
		description   string
		projectName   string
		roleTemplates map[string]*v3.RoleTemplate
		expectedRoles []string
		isErrExpected bool
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
			description:   "namespace create rule should grant create-ns and a namespaces-edit role",
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
			description: "* resources and * APIGroup should only result in namespace-readonly and promoted role",
			projectName: "testproject",
			// at the time of adding these tests ensureGlobalResourceRoleForPRTB returns duplicate promoted roles
			// names per applicable rule found in globalResourceRulesNeededInProjects. This is not incompatible with
			// current reconcile logic but should be fixed in the future.
			expectedRoles: []string{"create-ns", "testproject-namespaces-edit", "testrt8-promoted", "testrt8-promoted", "testrt8-promoted", "testrt8-promoted", "testrt8-promoted", "testrt8-promoted"},
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
			expectedRoles: []string{"create-ns", "testproject-namespaces-edit", "testrt9-promoted", "testrt9-promoted"},
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
	}
	for _, test := range testCases {
		test := test
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()
			roles, err := m.ensureGlobalResourcesRolesForPRTB(test.projectName, test.roleTemplates)
			assert.Nil(t, err)
			assert.Equal(t, test.expectedRoles, roles, test.description)
		})
	}

	test := testCase{
		projectName:   "testproject",
		expectedRoles: []string{"create-ns", "testproject-namespaces-edit"},
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
	}
	m = setupManager(map[string]*v3.RoleTemplate{"create-ns": createNSRoleTemplate}, make(map[string]*v1.ClusterRole), make(map[string]*v1.Role), make(map[string]*v3.Project), clientErrs{}, clientErrs{getError: errNotFound}, clientErrs{})
	test1 := test
	test1.description = "error return when RoleTemplate client returns error"
	t.Run(test.description, func(t *testing.T) {
		t.Parallel()
		_, err := m.ensureGlobalResourcesRolesForPRTB(test.projectName, test.roleTemplates)
		assert.NotNil(t, err)
	})
	m = setupManager(map[string]*v3.RoleTemplate{"create-ns": createNSRoleTemplate}, make(map[string]*v1.ClusterRole), make(map[string]*v1.Role), make(map[string]*v3.Project), clientErrs{}, clientErrs{}, clientErrs{createError: errAlreadyExist})
	test2 := test
	test2.description = "error return when Role client returns error"
	t.Run(test.description, func(t *testing.T) {
		t.Parallel()
		_, err := m.ensureGlobalResourcesRolesForPRTB(test.projectName, test.roleTemplates)
		assert.NotNil(t, err)
	})
	m = setupManager(map[string]*v3.RoleTemplate{"create-ns": createNSRoleTemplate}, make(map[string]*v1.ClusterRole), make(map[string]*v1.Role), make(map[string]*v3.Project), clientErrs{getError: apierrors.NewInternalError(errors.New("error"))}, clientErrs{}, clientErrs{})
	test3 := test
	test3.description = "error return when ClusterRole client returns error and RoleTemplate is external"
	test3.roleTemplates["testrt"].External = true
	t.Run(test.description, func(t *testing.T) {
		t.Parallel()
		_, err := m.ensureGlobalResourcesRolesForPRTB(test.projectName, test.roleTemplates)
		assert.NotNil(t, err)
	})
}
