package roletemplates

import (
	"errors"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/rancher/pkg/rbac"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	rbacv1 "k8s.io/api/rbac/v1"
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
	authv2ProvisioningBindingDeleted        = "AuthV2ProvisioningBindingDeleted"
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

// createOrUpdateClusterMembershipBinding ensures that the user specified by a CRTB or PRTB has membership to the cluster referenced by the CRTB or PRTB.
func createOrUpdateClusterMembershipBinding(rtb metav1.Object, rt *v3.RoleTemplate, crbController crbacv1.ClusterRoleBindingController) error {
	roleName := getClusterMembershipRoleName(rt, rtb)
	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     roleName,
	}

	wantedCRB, err := buildClusterMembershipBinding(roleRef, rtb)
	if err != nil {
		return err
	}
	// Create if not found
	existingCRB, err := crbController.Get(wantedCRB.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := crbController.Create(wantedCRB)
			return err
		}
		return err
	}

	// If the role referenced or subjects are wrong, delete and re-create the CRB
	if !rbac.AreClusterRoleBindingContentsSame(wantedCRB, existingCRB) {
		if err := crbController.Delete(wantedCRB.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
		_, err := crbController.Create(wantedCRB)
		return err
	}
	// Update Label
	rtbLabel := getRTBLabel(rtb)
	if v, ok := existingCRB.Labels[rtbLabel]; !ok || v != "true" {
		existingCRB.Labels[rtbLabel] = "true"
		_, err := crbController.Update(existingCRB)
		return err
	}
	return nil
}

// buildClusterMembershipBinding returns the ClusterRoleBinding needed to give membership to the Cluster.
func buildClusterMembershipBinding(roleRef rbacv1.RoleRef, rtb metav1.Object) (*rbacv1.ClusterRoleBinding, error) {
	subject, err := rbac.BuildSubjectFromRTB(rtb)
	if err != nil {
		return nil, err
	}

	crbName := rbac.NameForClusterRoleBinding(roleRef, subject)
	rtbLabel := getRTBLabel(rtb)

	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   crbName,
			Labels: map[string]string{rtbLabel: "true"},
		},
		Subjects: []rbacv1.Subject{subject},
		RoleRef:  roleRef,
	}, nil
}

// deleteClusterMembershipBinding checks if the user is still a member of the Cluster specified by PRTB/CRTB. If the user is no longer a member, delete the binding.
func deleteClusterMembershipBinding(rtb metav1.Object, crbController crbacv1.ClusterRoleBindingController) error {
	label := getRTBLabel(rtb)
	listOption := metav1.ListOptions{LabelSelector: label}
	crbs, err := crbController.List(listOption)
	if err != nil {
		return err
	}

	// There should only ever be 1 member ClusterRoleBinding per cluster.
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

// getMembershipRoleName returns the name of the membership role based on the RoleTemplate.
func getClusterMembershipRoleName(rt *v3.RoleTemplate, rtb metav1.Object) string {
	var clusterName string
	switch obj := rtb.(type) {
	case *v3.ProjectRoleTemplateBinding:
		clusterName, _ = rbac.GetClusterAndProjectNameFromPRTB(obj)
	case *v3.ClusterRoleTemplateBinding:
		clusterName = obj.ClusterName
	}
	if isClusterOwnerRole(rt) {
		return name.SafeConcatName(clusterName, clusterContext, "owner")
	} else {
		return name.SafeConcatName(clusterName, clusterContext, "member")
	}
}

// createOrUpdateProjectMembershipBinding ensures the RoleBinding required to give Project access to a user exists.
func createOrUpdateProjectMembershipBinding(prtb *v3.ProjectRoleTemplateBinding, rt *v3.RoleTemplate, rbController crbacv1.RoleBindingController) error {
	roleName := getProjectMembershipRoleName(rt, prtb)
	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     roleName,
	}

	wantedRB, err := buildProjectMembershipBinding(roleRef, prtb)
	if err != nil {
		return err
	}

	existingRB, err := rbController.Get(wantedRB.Namespace, wantedRB.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := rbController.Create(wantedRB)
			return err
		}
		return err
	}

	if !rbac.AreRoleBindingContentsSame(wantedRB, existingRB) {
		if err := rbController.Delete(wantedRB.Namespace, wantedRB.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
		_, err := rbController.Create(wantedRB)
		return err
	}

	// Update label
	rtbLabel := getRTBLabel(prtb)
	if v, ok := existingRB.Labels[rtbLabel]; !ok || v != "true" {
		existingRB.Labels[rtbLabel] = "true"
		_, err := rbController.Update(existingRB)
		return err
	}
	return nil
}

// buildProjectMembershipBinding returns the RoleBinding required to give access to the Project specified by the PRTB.
func buildProjectMembershipBinding(roleRef rbacv1.RoleRef, prtb *v3.ProjectRoleTemplateBinding) (*rbacv1.RoleBinding, error) {
	subject, err := rbac.BuildSubjectFromRTB(prtb)
	if err != nil {
		return nil, err
	}
	clusterName, projectName := rbac.GetClusterAndProjectNameFromPRTB(prtb)
	rbName := rbac.NameForRoleBinding(projectName, roleRef, subject)
	rtbLabel := getRTBLabel(prtb)

	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterName,
			Name:      rbName,
			Labels:    map[string]string{rtbLabel: "true"},
		},
		Subjects: []rbacv1.Subject{subject},
		RoleRef:  roleRef,
	}, nil
}

// deleteProjectMembershipBinding removes the Project membership RoleBinding if no other PRTBs are using it.
func deleteProjectMembershipBinding(prtb *v3.ProjectRoleTemplateBinding, rbController crbacv1.RoleBindingController) error {
	_, projectNamespace := rbac.GetClusterAndProjectNameFromPRTB(prtb)

	label := getRTBLabel(prtb)
	listOption := metav1.ListOptions{LabelSelector: label}
	rbs, err := rbController.List(projectNamespace, listOption)
	if err != nil {
		return err
	}

	// There should only ever be 1 member RoleBinding per project
	var returnedErr error
	for _, rb := range rbs.Items {
		if _, ok := rb.Labels[label]; ok {
			delete(rb.Labels, label)
			// If there are no items in Labels, the user is no longer a member/owner.
			if len(rb.Labels) == 0 {
				err = rbController.Delete(rb.Namespace, rb.Name, &metav1.DeleteOptions{
					Preconditions: &metav1.Preconditions{UID: &rb.UID, ResourceVersion: &rb.ResourceVersion},
				})
				returnedErr = errors.Join(returnedErr, err)
				continue
			}
			_, err := rbController.Update(&rb)
			returnedErr = errors.Join(returnedErr, err)
		}
	}

	return returnedErr
}

// getProjectMembershipRoleName returns the name of the project member or owner binding for the PRTB.
func getProjectMembershipRoleName(rt *v3.RoleTemplate, prtb *v3.ProjectRoleTemplateBinding) string {
	_, projectName := rbac.GetClusterAndProjectNameFromPRTB(prtb)
	if isProjectOwnerRole(rt) {
		return name.SafeConcatName(projectName, projectContext, "owner")
	} else {
		return name.SafeConcatName(projectName, projectContext, "member")
	}
}

// isClusterOwnerRole returns if the RoleTemplate is an Owner role. If not it is considered a Member role.
// The only valid OwnerRole is the builtin "cluster-owner" role.
func isClusterOwnerRole(rt *v3.RoleTemplate) bool {
	return rt.Builtin && rt.Context == clusterContext && rt.Name == clusterOwner
}

// isProjectOwnerRole returns if the RoleTemplate is an Owner role. If not it is considered a Member role.
// The only valid OwnerRole is the builtin "project-owner" role.
func isProjectOwnerRole(rt *v3.RoleTemplate) bool {
	return rt.Builtin && rt.Context == projectContext && rt.Name == projectOwner
}

// getRTBLabel returns the label to be used to indicate what PRTB/CRTB make use of a membership role.
func getRTBLabel(obj metav1.Object) string {
	return name.SafeConcatName(obj.GetNamespace() + "_" + obj.GetName())
}

// removeAuthV2Permissions removes all Role Bindings with the RTB owner label.
// The only intentionally created RoleBindings are created by auth provisioning v2 to provide access to the fleet cluster.
// The creation of those is handled in pkg/controllers/management/authprovisioningv2.
func removeAuthV2Permissions(obj metav1.Object, rbController crbacv1.RoleBindingController) error {
	listOptions := metav1.ListOptions{LabelSelector: rbac.GetRTBOwnerLabel(obj)}

	roleBindings, err := rbController.List(fleet.ClustersDefaultNamespace, listOptions)
	if err != nil {
		return err
	}
	for _, roleBinding := range roleBindings.Items {
		if err := rbController.Delete(roleBinding.Namespace, roleBinding.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}
