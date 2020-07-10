package rbac

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	rbacv1 "k8s.io/api/rbac/v1"
)

func Test_BuildSubjectFromRTB(t *testing.T) {
	type testCase struct {
		from  interface{}
		to    rbacv1.Subject
		iserr bool
	}

	userSubject := rbacv1.Subject{
		Kind: "User",
		Name: "tmp-user",
	}

	groupSubject := rbacv1.Subject{
		Kind: "Group",
		Name: "tmp-group",
	}

	saSubject := rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      "tmp-sa",
		Namespace: "tmp-namespace",
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
