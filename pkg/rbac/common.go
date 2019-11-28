package rbac

import (
	"crypto/sha256"
	"encoding/base32"
	"strings"

	"github.com/pkg/errors"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "k8s.io/api/rbac/v1"
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

func GrbCRBName(grb *v3.GlobalRoleBinding) string {
	return "globaladmin-" + GetGRBTargetKey(grb)
}

// GetGRBSubject creates and returns a subject that is
// determined by inspecting the the GRB's target fields
func GetGRBSubject(grb *v3.GlobalRoleBinding) rbacv1.Subject {
	kind := "User"
	name := grb.UserName
	if grb.ClusterName == "" && grb.GroupPrincipalName != "" {
		kind = "Group"
		name = grb.GroupPrincipalName
	}

	return rbacv1.Subject{
		Kind:     kind,
		Name:     name,
		APIGroup: rbacv1.GroupName,
	}
}

// getGRBTargetKey returns a key that uniquely identifies the given GRB's target.
// If a user is being targeted, then the user's name is returned.
// Otherwise, the group principal name is converted to a valid user string and
// is returned.
func GetGRBTargetKey(grb *v3.GlobalRoleBinding) string {
	name := grb.UserName

	if name == "" {
		hasher := sha256.New()
		hasher.Write([]byte(grb.GroupPrincipalName))
		sha := base32.StdEncoding.WithPadding(-1).EncodeToString(hasher.Sum(nil))[:10]
		name = "u-" + strings.ToLower(sha)
	}
	return name
}
