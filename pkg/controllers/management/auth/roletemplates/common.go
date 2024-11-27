package roletemplates

import (
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

	wantedCRB, err := buildMembershipBinding(roleRef, crtb)
	if err != nil {
		return err
	}
	existingCRB, err := crbController.Get(wantedCRB.Name, metav1.GetOptions{})
	if err == nil {
		// If the role referenced or subjects are wrong, delete and re-create the CRB
		if pkgrbac.AreClusterRoleBindingsSame(wantedCRB, existingCRB) {
			// Update Label
			rtbLabel := pkgrbac.GetRTBLabel(crtb.ObjectMeta)
			existingCRB.Labels[rtbLabel] = "true"
			_, err := crbController.Update(existingCRB)
			return err
		} else {
			if err := crbController.Delete(wantedCRB.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	_, err = crbController.Create(wantedCRB)
	return err
}

func deleteMembershipBinding(crtb *v3.ClusterRoleTemplateBinding, crbController crbacv1.ClusterRoleBindingController) error {
	label := pkgrbac.GetRTBLabel(crtb.ObjectMeta)
	listOption := metav1.ListOptions{LabelSelector: label}
	crbs, err := crbController.List(listOption)
	if err != nil {
		return err
	}

	for _, c := range crbs.Items {
		delete(c.Labels, label)
		// If there are no items in Labels, the user is no longer a member/owner
		if len(c.Labels) == 0 {
			return crbController.Delete(c.Name, &metav1.DeleteOptions{})
		}
	}
	return nil
}

func buildMembershipBinding(roleRef v1.RoleRef, crtb *v3.ClusterRoleTemplateBinding) (*v1.ClusterRoleBinding, error) {
	subject, err := pkgrbac.BuildSubjectFromRTB(crtb)
	if err != nil {
		return nil, err
	}

	crbName := pkgrbac.NameForClusterRoleBinding(roleRef, subject)
	rtbLabel := pkgrbac.GetRTBLabel(crtb.ObjectMeta)

	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        crbName,
			Annotations: map[string]string{},
			Labels:      map[string]string{rtbLabel: "true"},
		},
		Subjects: []v1.Subject{subject},
		RoleRef:  roleRef,
	}, nil
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
