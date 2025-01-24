package roletemplates

import (
	"errors"

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

const (
	// Statuses
	clusterRoleTemplateBindingDelete = "ClusterRoleTemplateBindingDelete"
	labelsReconciled                 = "LabelsReconciled"
	removeClusterRoleBindings        = "RemoveClusterRoleBindings"
	reconcileSubject                 = "ReconcileSubject"
	reconcileMembershipBindings      = "ReconcileMembershipBindings"
	reconcileBindings                = "ReconcileBindings"
	// Reasons
	clusterRoleBindingDeleted               = "ClusterRoleBindingDeleted"
	bindingsExists                          = "BindingsExists"
	membershipBindingExists                 = "MembershipBindingExists"
	subjectExists                           = "SubjectExists"
	crtbHasNoSubject                        = "CRTBHasNoSubject"
	clusterMembershipBindingDeleted         = "ClusterMembershipBindingDeleted"
	failedToCreateClusterRoleBinding        = "FailedToCreateClusterRoleBinding"
	failedToCreateOrUpdateMembershipBinding = "FailedToCreateOrUpdateMembershipBinding"
	failedToCreateUser                      = "FailedToCreateUser"
	failedToDeleteClusterRoleBinding        = "FailedToDeleteClusterRoleBinding"
	failedToGetDesiredClusterRoleBindings   = "FailedToGetDesiredClusterRoleBindings"
	failedToGetRoleTemplate                 = "FailedToGetRoleTemplate"
	failedToGetUser                         = "FailedToGetUser"
	failedToListExistingClusterRoleBindings = "FailedToGetExistingClusterRoleBindings"
	failedToDeleteClusterMembershipBinding  = "FailedToDeleteClusterMembershipBindings"
)

// createOrUpdateMembershipBinding ensures that the user specified by a CRTB or PRTB has membership to the cluster or project specified by the CRTB or PRTB.
func createOrUpdateMembershipBinding(rtb metav1.Object, rt *v3.RoleTemplate, crbController crbacv1.ClusterRoleBindingController) (*v1.ClusterRoleBinding, error) {
	roleName := getMembershipRoleName(rt, rtb)
	roleRef := v1.RoleRef{
		Kind: "ClusterRole",
		Name: roleName,
	}

	wantedCRB, err := buildMembershipBinding(roleRef, rtb)
	if err != nil {
		return nil, err
	}
	// Create if not found
	existingCRB, err := crbController.Get(wantedCRB.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return crbController.Create(wantedCRB)
		}
		return nil, err
	}

	// If the role referenced or subjects are wrong, delete and re-create the CRB
	if !pkgrbac.AreClusterRoleBindingContentsSame(wantedCRB, existingCRB) {
		if err := crbController.Delete(wantedCRB.Name, &metav1.DeleteOptions{}); err != nil {
			return nil, err
		}
		return crbController.Create(wantedCRB)
	}
	// Update Label
	rtbLabel := getRTBLabel(rtb)
	if v, ok := existingCRB.Labels[rtbLabel]; !ok || v != "true" {
		existingCRB.Labels[rtbLabel] = "true"
		return crbController.Update(existingCRB)
	}
	return existingCRB, nil
}

// deleteMembershipBinding checks if the user is still a member of the Project or Cluster specified by PRTB/CRTB. If they are no longer a member, delete the bindings.
func deleteMembershipBinding(rtb metav1.Object, crbController crbacv1.ClusterRoleBindingController) error {
	label := getRTBLabel(rtb)
	listOption := metav1.ListOptions{LabelSelector: label}
	crbs, err := crbController.List(listOption)
	if err != nil {
		return err
	}

	var returnedErr error
	for _, c := range crbs.Items {
		if _, ok := c.Labels[label]; ok {
			delete(c.Labels, label)
			// If there are no items in Labels, the user is no longer a member/owner
			if len(c.Labels) == 0 {
				err = crbController.Delete(c.Name, &metav1.DeleteOptions{
					Preconditions: &metav1.Preconditions{UID: &c.UID, ResourceVersion: &c.ResourceVersion},
				})
				returnedErr = errors.Join(returnedErr, err)
				continue
			}
			_, err := crbController.Update(&c)
			returnedErr = errors.Join(returnedErr, err)
		}
	}
	return returnedErr
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
			Name:   crbName,
			Labels: map[string]string{rtbLabel: "true"},
		},
		Subjects: []v1.Subject{subject},
		RoleRef:  roleRef,
	}, nil
}

// getMembershipRoleName returns the name of the membership role based on the RoleTemplate.
func getMembershipRoleName(rt *v3.RoleTemplate, rtb metav1.Object) string {
	var resourceName string
	switch obj := rtb.(type) {
	case *v3.ProjectRoleTemplateBinding:
		resourceName = obj.ProjectName
	case *v3.ClusterRoleTemplateBinding:
		resourceName = obj.ClusterName
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
