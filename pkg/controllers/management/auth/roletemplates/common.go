package roletemplates

import (
	"errors"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/rancher/pkg/rbac"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterContext = "cluster"
	projectContext = "project"
)

const (
	// Statuses
	clusterRoleTemplateBindingDelete = "ClusterRoleTemplateBindingDelete"
	removeRoleBindings               = "RemoveRoleBindings"
	reconcileSubject                 = "ReconcileSubject"
	reconcileMembershipBindings      = "ReconcileMembershipBindings"
	reconcileBindings                = "ReconcileBindings"
	// Reasons
	roleBindingDeleted                      = "roleBindingDeleted"
	bindingsExists                          = "BindingsExists"
	membershipBindingExists                 = "MembershipBindingExists"
	subjectExists                           = "SubjectExists"
	crtbHasNoSubject                        = "CRTBHasNoSubject"
	clusterMembershipBindingDeleted         = "ClusterMembershipBindingDeleted"
	authv2ProvisioningBindingDeleted        = "AuthV2ProvisioningBindingDeleted"
	failedToCreateRoleBinding               = "FailedToCreateRoleBinding"
	failedToCreateOrUpdateMembershipBinding = "FailedToCreateOrUpdateMembershipBinding"
	failedToCreateUser                      = "FailedToCreateUser"
	failedToDeleteRoleBinding               = "FailedToDeleteRoleBinding"
	failedToGetDesiredRoleBindings          = "FailedToGetDesiredRoleBindings"
	failedToGetUser                         = "FailedToGetUser"
	failedToGetClusterRole                  = "FailedToGetClusterRole"
	failedToListExistingRoleBindings        = "FailedToGetExistingRoleBindings"
)

// createOrUpdateClusterMembershipBinding ensures that the user specified by a CRTB or PRTB has membership to the cluster referenced by the CRTB or PRTB.
func createOrUpdateClusterMembershipBinding(rtb metav1.Object, crbController crbacv1.ClusterRoleBindingController, isClusterOwner bool) error {
	roleName := getClusterMembershipRoleName(rtb, isClusterOwner)
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
			logrus.Infof("Creating clusterRoleBinding %s for cluster membership role %s for subjects %v", wantedCRB.Name, wantedCRB.RoleRef.Name, wantedCRB.Subjects)
			if _, err := crbController.Create(wantedCRB); err != nil {
				return fmt.Errorf("failed to create cluster membership binding %s: %w", wantedCRB.Name, err)
			}
			return nil
		}
		return err
	}

	// If the role referenced or subjects are wrong, delete and re-create the CRB
	if !rbac.IsClusterRoleBindingContentSame(wantedCRB, existingCRB) {
		logrus.Infof("Re-creating clusterRoleBinding %s for cluster membership role %s for subjects %v", wantedCRB.Name, wantedCRB.RoleRef.Name, wantedCRB.Subjects)
		if err := crbController.Delete(wantedCRB.Name, &metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("failed to delete cluster membership binding %s: %w", wantedCRB.Name, err)
		}

		if _, err := crbController.Create(wantedCRB); err != nil {
			return fmt.Errorf("failed to create cluster membership binding %s: %w", wantedCRB.Name, err)
		}
		return nil
	}
	// Update Label
	rtbLabel := getRTBLabel(rtb)
	if v, ok := existingCRB.Labels[rtbLabel]; !ok || v != "true" {
		logrus.Infof("Updating clusterRoleBinding %s for cluster membership role %s for subjects %v", wantedCRB.Name, wantedCRB.RoleRef.Name, wantedCRB.Subjects)
		existingCRB.Labels[rtbLabel] = "true"

		if _, err := crbController.Update(existingCRB); err != nil {
			return fmt.Errorf("failed to update cluster membership binding %s: %w", wantedCRB.Name, err)
		}
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
				if err != nil {
					returnedErr = errors.Join(returnedErr, fmt.Errorf("failed to delete cluster membership binding %s: %w", c.Name, err))
				}
				continue
			}
			if _, err := crbController.Update(&c); err != nil {
				returnedErr = errors.Join(returnedErr, fmt.Errorf("failed to update cluster membership binding %s: %w", c.Name, err))
			}
		}
	}
	return returnedErr
}

// getMembershipRoleName returns the name of the membership role based on the RoleTemplate.
func getClusterMembershipRoleName(rtb metav1.Object, isClusterOwner bool) string {
	var clusterName string
	switch obj := rtb.(type) {
	case *v3.ProjectRoleTemplateBinding:
		clusterName, _ = rbac.GetClusterAndProjectNameFromPRTB(obj)
	case *v3.ClusterRoleTemplateBinding:
		clusterName = obj.ClusterName
	}
	if isClusterOwner {
		return name.SafeConcatName(clusterName, clusterContext+"owner")
	}
	return name.SafeConcatName(clusterName, clusterContext+"member")
}

// createOrUpdateProjectMembershipBinding ensures the RoleBinding required to give Project access to a user exists.
func createOrUpdateProjectMembershipBinding(prtb *v3.ProjectRoleTemplateBinding, rbController crbacv1.RoleBindingController, isProjectOwner bool) error {
	roleName := getProjectMembershipRoleName(prtb, isProjectOwner)
	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     roleName,
	}

	wantedRB, err := buildProjectMembershipBinding(roleRef, prtb)
	if err != nil {
		return err
	}

	existingRB, err := rbController.Get(wantedRB.Namespace, wantedRB.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("Creating roleBinding %s for project membership role %s for subjects %v", wantedRB.Name, wantedRB.RoleRef.Name, wantedRB.Subjects)
			if _, err := rbController.Create(wantedRB); err != nil {
				return fmt.Errorf("failed to create project membership binding %s: %w", wantedRB.Name, err)
			}
			return nil
		}
		return err
	}

	// RoleRef is immutable, so if it's incorrect it needs to be deleted and re-created
	if !rbac.IsRoleBindingContentSame(wantedRB, existingRB) {
		logrus.Infof("Re-creating roleBinding %s for project membership role %s for subjects %v", wantedRB.Name, wantedRB.RoleRef.Name, wantedRB.Subjects)
		if err := rbController.Delete(wantedRB.Namespace, wantedRB.Name, &metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("failed to delete project membership binding %s: %w", wantedRB.Name, err)
		}

		if _, err := rbController.Create(wantedRB); err != nil {
			return fmt.Errorf("failed to create project membership binding %s: %w", wantedRB.Name, err)
		}
		return nil
	}

	// Update label
	rtbLabel := getRTBLabel(prtb)
	if v, ok := existingRB.Labels[rtbLabel]; !ok || v != "true" {
		logrus.Infof("Updating roleBinding %s for project membership role %s for subjects %v", wantedRB.Name, wantedRB.RoleRef.Name, wantedRB.Subjects)
		existingRB.Labels[rtbLabel] = "true"

		if _, err := rbController.Update(existingRB); err != nil {
			return fmt.Errorf("failed to update project membership binding %s: %w", wantedRB.Name, err)
		}
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
	clusterName, _ := rbac.GetClusterAndProjectNameFromPRTB(prtb)

	label := getRTBLabel(prtb)
	listOption := metav1.ListOptions{LabelSelector: label}
	rbs, err := rbController.List(clusterName, listOption)
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
				if err != nil {
					returnedErr = errors.Join(returnedErr, fmt.Errorf("failed to delete project membership binding %s: %w", rb.Name, err))
				}
				continue
			}
			if _, err := rbController.Update(&rb); err != nil {
				returnedErr = errors.Join(returnedErr, fmt.Errorf("failed to update project membership binding %s: %w", rb.Name, err))
			}
		}
	}

	return returnedErr
}

// getProjectMembershipRoleName returns the name of the project member or owner binding for the PRTB.
func getProjectMembershipRoleName(prtb *v3.ProjectRoleTemplateBinding, isProjectOwner bool) string {
	_, projectName := rbac.GetClusterAndProjectNameFromPRTB(prtb)
	if isProjectOwner {
		return name.SafeConcatName(projectName, projectContext+"owner")
	}
	return name.SafeConcatName(projectName, projectContext+"member")
}

// getRTBLabel returns the label to be used to indicate what PRTB/CRTB make use of a membership role.
func getRTBLabel(obj metav1.Object) string {
	return name.SafeConcatName(obj.GetNamespace() + "_" + obj.GetName())
}

// removeAuthV2Permissions removes all Role Bindings with the RTB owner label.
// The only intentionally created RoleBindings are created by auth provisioning v2 to provide access to the fleet cluster.
// The creation of those is handled in pkg/controllers/management/authprovisioningv2.
func removeAuthV2Permissions(obj metav1.Object, rbController crbacv1.RoleBindingController) error {
	listOptions := metav1.ListOptions{LabelSelector: rbac.GetAuthV2OwnerLabel(obj)}

	roleBindings, err := rbController.List(fleet.ClustersDefaultNamespace, listOptions)
	if err != nil {
		return err
	}
	for _, roleBinding := range roleBindings.Items {
		if err := rbController.Delete(roleBinding.Namespace, roleBinding.Name, &metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("failed to delete AuthV2 role binding %s: %w", roleBinding.Name, err)
		}
	}
	return nil
}
