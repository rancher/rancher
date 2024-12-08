package roletemplates

import (
	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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

// createOrUpdateMembershipBinding ensures that the user specified by a CRTB or PRTB has membership to the cluster or project specified by the CRTB or PRTB.
func createOrUpdateMembershipBinding(rtb metav1.Object, rt *v3.RoleTemplate, crbController crbacv1.ClusterRoleBindingController) error {
	roleName := getMembershipRoleName(rt)
	roleRef := v1.RoleRef{
		Kind: "ClusterRole",
		Name: roleName,
	}

	wantedCRB, err := buildMembershipBinding(roleRef, rtb)
	if err != nil {
		return err
	}
	// Create if not found
	existingCRB, err := crbController.Get(wantedCRB.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			_, err = crbController.Create(wantedCRB)
			return err
		}
		return err
	}

	// If the role referenced or subjects are wrong, delete and re-create the CRB
	if !pkgrbac.AreClusterRoleBindingContentsSame(wantedCRB, existingCRB) {
		if err := crbController.Delete(wantedCRB.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
		_, err = crbController.Create(wantedCRB)
		return err
	}
	// Update Label
	rtbLabel := getRTBLabel(rtb)
	if v, ok := existingCRB.Labels[rtbLabel]; !ok || v != "true" {
		existingCRB.Labels[rtbLabel] = "true"
		_, err = crbController.Update(existingCRB)
		return err
	}
	return nil
}

// deleteMembershipBinding checks if the user is still a member of the Project or Cluster specified by PRTB/CRTB. If they are no longer a member, delete the bindings.
func deleteMembershipBinding(rtb metav1.Object, crbController crbacv1.ClusterRoleBindingController) error {
	label := getRTBLabel(rtb)
	listOption := metav1.ListOptions{LabelSelector: label}
	crbs, err := crbController.List(listOption)
	if err != nil {
		return err
	}

	for _, c := range crbs.Items {
		delete(c.Labels, label)
		// If there are no items in Labels, the user is no longer a member/owner
		if len(c.Labels) == 0 {
			return crbController.Delete(c.Name, &metav1.DeleteOptions{
				Preconditions: &metav1.Preconditions{UID: &c.UID, ResourceVersion: &c.ResourceVersion},
			})
		}
	}
	return nil
}

// buildMembershipBinding returns the ClusterRoleBinding needed to give membership to the Cluster or Project.
func buildMembershipBinding(roleRef v1.RoleRef, rtb metav1.Object) (*v1.ClusterRoleBinding, error) {
	subject, err := pkgrbac.BuildSubjectFromRTB(rtb)
	if err != nil {
		return nil, err
	}

	crbName := pkgrbac.NameForClusterRoleBinding(roleRef, subject)
	rtbLabel := getRTBLabel(rtb)

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

// getMembershipRoleName returns the name of the membership role based on the RoleTemplate.
func getMembershipRoleName(rt *v3.RoleTemplate) string {
	var resourceName string
	if rt.Context == clusterContext {
		resourceName = apisv3.ClusterResourceName
	} else if rt.Context == projectContext {
		resourceName = apisv3.ProjectResourceName
	}
	if isOwnerRole(rt) {
		return name.SafeConcatName(resourceName, "owner")
	} else {
		return name.SafeConcatName(resourceName, "member")
	}
}

// isOwnerRole returns if the RoleTemplate is an Owner role. If not it is considered a Member role.
// The only valid OwnerRoles are the builtin "cluster-owner" and "project-owner" roles.
func isOwnerRole(rt *v3.RoleTemplate) bool {
	return rt.Builtin && (rt.Context == clusterContext && rt.Name == clusterOwner || rt.Context == clusterContext && rt.Name == projectOwner)
}

// getRTBLabel returns the label to be used to indicate what PRTB/CRTB make use of a membership role.
func getRTBLabel(obj metav1.Object) string {
	return name.SafeConcatName(obj.GetNamespace() + "_" + obj.GetName())
}
