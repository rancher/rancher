package roletemplates

import (
	"reflect"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterContext = "cluster"
	clusterOwner   = "cluster-owner"
	projectContext = "project"
	projectOwner   = "project-owner"
)

func createOrUpdateMembershipBinding(crtb *v3.ClusterRoleTemplateBinding, rtController mgmtv3.RoleTemplateController, crbController crbacv1.ClusterRoleBindingController) error {
	rt, err := rtController.Get(crtb.RoleTemplateName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	roleName := getMembershipRoleName(rt)
	roleRef := v1.RoleRef{
		Kind: "ClusterRole",
		Name: roleName,
	}

	subject, err := pkgrbac.BuildSubjectFromRTB(crtb)
	if err != nil {
		return err
	}

	crbName := pkgrbac.NameForClusterRoleBinding(roleRef, subject)
	wantedCRB := buildMembershipBinding(roleRef, crbName, subject)
	existingCRB, err := crbController.Get(crbName, metav1.GetOptions{})
	if err == nil {
		// TODO update if there are labels or annotations we want

		// If the role referenced or subjects are wrong, delete and re-create the CRB
		if !reflect.DeepEqual(wantedCRB.RoleRef, existingCRB.RoleRef) ||
			!reflect.DeepEqual(wantedCRB.Subjects, existingCRB.Subjects) {
			if err := crbController.Delete(crbName, &metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	_, err = crbController.Create(&wantedCRB)
	return err
}

func buildMembershipBinding(roleRef v1.RoleRef, crbName string, subject v1.Subject) v1.ClusterRoleBinding {
	// TODO what labels and annotations do we need?
	return v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        crbName,
			Annotations: map[string]string{},
			Labels:      map[string]string{},
		},
		Subjects: []v1.Subject{subject},
		RoleRef:  roleRef,
	}
}

func getMembershipRoleName(rt *v3.RoleTemplate) string {
	var resourceName string
	if rt.Context == clusterContext {
		resourceName = apisv3.ClusterResourceName
	} else if rt.Context == projectContext {
		resourceName = apisv3.ProjectResourceName
	}
	if isOwnerRole(rt) {
		return name.SafeConcatName(resourceName + "owner")
	} else {
		return name.SafeConcatName(resourceName + "member")
	}
}

func isOwnerRole(rt *v3.RoleTemplate) bool {
	return rt.Builtin && (rt.Context == clusterContext && rt.Name == clusterOwner || rt.Context == clusterContext && rt.Name == projectOwner)
}
