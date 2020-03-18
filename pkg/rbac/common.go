package rbac

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "k8s.io/api/rbac/v1"
)

const (
	NamespaceID = "namespaceId"
	ProjectID   = "projectId"
	ClusterID   = "clusterId"
)

// BuildSubjectFromRTB This function will generate
// PRTB and CRTB to the subject with user, group
// or service account
func BuildSubjectFromRTB(object interface{}) (rbacv1.Subject, error) {
	var userName, groupPrincipalName, groupName, name, kind, sa, namespace string
	if rtb, ok := object.(*v3.ProjectRoleTemplateBinding); ok {
		userName = rtb.UserName
		groupPrincipalName = rtb.GroupPrincipalName
		groupName = rtb.GroupName
		sa = rtb.ServiceAccount
	} else if rtb, ok := object.(*v3.ClusterRoleTemplateBinding); ok {
		userName = rtb.UserName
		groupPrincipalName = rtb.GroupPrincipalName
		groupName = rtb.GroupName
	} else {
		return rbacv1.Subject{}, errors.Errorf("unrecognized roleTemplateBinding type: %v", object)
	}

	if userName != "" {
		name = userName
		kind = "User"
	}

	if groupPrincipalName != "" {
		if name != "" {
			return rbacv1.Subject{}, errors.Errorf("roletemplatebinding has more than one subject fields set: %v", object)
		}
		name = groupPrincipalName
		kind = "Group"
	}

	if groupName != "" {
		if name != "" {
			return rbacv1.Subject{}, errors.Errorf("roletemplatebinding has more than one subject fields set: %v", object)
		}
		name = groupName
		kind = "Group"
	}

	if sa != "" {
		parts := strings.SplitN(sa, ":", 2)
		if len(parts) < 2 {
			return rbacv1.Subject{}, errors.Errorf("service account %s of projectroletemplatebinding is invalid: %v", sa, object)
		}
		namespace = parts[0]
		name = parts[1]
		kind = "ServiceAccount"
	}

	if name == "" {
		return rbacv1.Subject{}, errors.Errorf("roletemplatebinding doesn't have any subject fields set: %v", object)
	}

	return rbacv1.Subject{
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
	}, nil
}

// Returns object with available information to check against users permissions, used in combination with CanDo
func ObjFromContext(apiContext *types.APIContext, resource *types.RawResource) map[string]interface{} {
	var obj map[string]interface{}
	if resource != nil && resource.Values["id"] != nil {
		obj = resource.Values
	}
	if obj == nil {
		obj = map[string]interface{}{
			"id": apiContext.ID,
		}
		// collection endpoint without id needs to know which cluster-namespace for rbac check
		if apiContext.Query.Get(ClusterID) != "" {
			obj[NamespaceID] = apiContext.Query.Get(ClusterID)
		}
		if apiContext.Query.Get(ProjectID) != "" {
			_, obj[NamespaceID] = ref.Parse(apiContext.Query.Get(ProjectID))
		}
	}
	return obj
}
