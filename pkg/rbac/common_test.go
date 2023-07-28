package rbac

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/rancher/norman/types"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_BuildSubjectFromRTB(t *testing.T) {
	type testCase struct {
		from  metav1.Object
		to    rbacv1.Subject
		iserr bool
	}

	userSubject := rbacv1.Subject{
		Kind:     "User",
		Name:     "tmp-user",
		APIGroup: "rbac.authorization.k8s.io",
	}

	groupSubject := rbacv1.Subject{
		Kind:     "Group",
		Name:     "tmp-group",
		APIGroup: "rbac.authorization.k8s.io",
	}

	saSubject := rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      "tmp-sa",
		Namespace: "tmp-namespace",
		APIGroup:  "",
	}

	testCases := []testCase{
		testCase{
			from:  nil,
			iserr: true,
		},
		testCase{
			from: &v3.ProjectRoleTemplateBinding{
				UserName: userSubject.Name,
			},
			to: userSubject,
		},
		testCase{
			from: &v3.ProjectRoleTemplateBinding{
				GroupName: groupSubject.Name,
			},
			to: groupSubject,
		},
		testCase{
			from: &v3.ProjectRoleTemplateBinding{
				ServiceAccount: fmt.Sprintf("%s:%s", saSubject.Namespace, saSubject.Name),
			},
			to: saSubject,
		},
		testCase{
			from: &v3.ClusterRoleTemplateBinding{
				UserName: userSubject.Name,
			},
			to: userSubject,
		},
		testCase{
			from: &v3.ClusterRoleTemplateBinding{
				GroupName: groupSubject.Name,
			},
			to: groupSubject,
		},
		testCase{
			from: &v3.ProjectRoleTemplateBinding{
				ServiceAccount: "wrong-format",
			},
			iserr: true,
		},
	}

	for _, tcase := range testCases {
		output, err := BuildSubjectFromRTB(tcase.from)
		if tcase.iserr && err == nil {
			t.Errorf("roletemplatebinding %v should return error", tcase.from)
		} else if !tcase.iserr && !reflect.DeepEqual(tcase.to, output) {
			t.Errorf("the subject %v from roletemplatebinding %v is mismatched, expect %v", output, tcase.from, tcase.to)
		}
	}
}

func Test_TypeFromContext(t *testing.T) {
	type testCase struct {
		apiContext   *types.APIContext
		resource     *types.RawResource
		expectedType string
	}

	testCases := []testCase{
		{
			apiContext: &types.APIContext{
				Type: "catalog",
			},
			resource:     nil,
			expectedType: "catalog",
		},
		{
			apiContext: &types.APIContext{
				Type: "subscribe",
			},
			resource: &types.RawResource{
				Type: "catalog",
			},
			expectedType: "catalog",
		},
	}

	for _, tcase := range testCases {
		outputType := TypeFromContext(tcase.apiContext, tcase.resource)
		if tcase.expectedType != outputType {
			t.Errorf("resource type %s is mismatched, expect %s", outputType, tcase.expectedType)
		}
	}
}

func Test_RuleGivesResourceAccess(t *testing.T) {
	type testCase struct {
		rule         rbacv1.PolicyRule
		resourceName string
		expected     bool
	}
	createTestCase := func(apiGroup string, ruleResource string, requestResource string, outcome bool) testCase {
		return testCase{
			rule: rbacv1.PolicyRule{
				APIGroups: []string{
					apiGroup,
				},
				Verbs: []string{
					"*",
				},
				Resources: []string{
					ruleResource,
				},
			},
			resourceName: requestResource,
			expected:     outcome,
		}
	}

	createMultiGroupResourceTestCase := func(apiGroups []string, resources []string, requestResource string, outcome bool) testCase {
		return testCase{
			rule: rbacv1.PolicyRule{
				APIGroups: apiGroups,
				Verbs: []string{
					"*",
				},
				Resources: resources,
			},
			resourceName: requestResource,
			expected:     outcome,
		}
	}

	testCases := []testCase{
		createTestCase("*", "test", "test", true),
		createTestCase("*", "test", "nottest", false),
		createTestCase("*", "*", "test", true),
		createTestCase(mgmt.GroupName, "test", "test", true),
		createTestCase(mgmt.GroupName, "test", "nottest", false),
		createTestCase(mgmt.GroupName, "*", "test", true),
		createTestCase("fake.company.io", "test", "test", false),
		createTestCase("fake.company.io", "test", "nottest", false),
		createTestCase("fake.company.io", "*", "nottest", false),
		createMultiGroupResourceTestCase([]string{"fake.company.io", mgmt.GroupName}, []string{"test"}, "test", true),
		createMultiGroupResourceTestCase([]string{"fake.company.io", mgmt.GroupName}, []string{"test"}, "nottest", false),
		createMultiGroupResourceTestCase([]string{"fake.company.io", mgmt.GroupName}, []string{"*"}, "test", true),
		createMultiGroupResourceTestCase([]string{"fake.company.io", mgmt.GroupName}, []string{"nottest", "test"}, "test", true),
		createMultiGroupResourceTestCase([]string{"fake.company.io", "*"}, []string{"nottest", "test"}, "test", true),
		createMultiGroupResourceTestCase([]string{"fake.company.io", "*"}, []string{"nottest", "test"}, "supertest", false),
		createMultiGroupResourceTestCase([]string{"fake.company.io", "faker.company.io"}, []string{"nottest", "test"}, "test", false),
	}

	for _, tcase := range testCases {
		givesAccess := RuleGivesResourceAccess(tcase.rule, tcase.resourceName)
		if tcase.expected != givesAccess {
			t.Errorf("got %t, expected %t, for rule %v resource %v", givesAccess, tcase.expected, tcase.rule, tcase.resourceName)
		}
	}
}

func TestGetRTBLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		metadata metav1.ObjectMeta
		output   string
	}{
		{
			name: "empty strings for name and namespace",
			metadata: metav1.ObjectMeta{
				Name:      "",
				Namespace: "",
			},
			output: "_",
		},
		{
			name: "short name and namespace",
			metadata: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			output: "default_foo",
		},
		{
			name: "longer name with whole string being shorter than 63 characters",
			metadata: metav1.ObjectMeta{
				Name:      strings.Repeat("rottweiler", 5),
				Namespace: "default",
			},
			output: "default_rottweilerrottweilerrottweilerrottweilerrottweiler",
		},
		{
			name: "longer name with whole string being longer than 63 characters",
			metadata: metav1.ObjectMeta{
				Name:      strings.Repeat("rottweiler", 10),
				Namespace: "default",
			},
			output: "default_rottweilerrottweilerrottweilerrottweilerrottweile-c4636",
		},
		{
			name: "longer namespace with whole string being shorter than 63 characters",
			metadata: metav1.ObjectMeta{
				Name:      "rottweiler",
				Namespace: strings.Repeat("default", 5),
			},
			output: "defaultdefaultdefaultdefaultdefault_rottweiler",
		},
		{
			name: "longer namespace with whole string being longer than 63 characters",
			metadata: metav1.ObjectMeta{
				Name:      "rottweiler",
				Namespace: strings.Repeat("default", 8),
			},
			output: "defaultdefaultdefaultdefaultdefaultdefaultdefaultdefault-829c4a",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := GetRTBLabel(test.metadata); got != test.output {
				t.Errorf("expected %s, but got %s", test.output, got)
			}
		})
	}
}
