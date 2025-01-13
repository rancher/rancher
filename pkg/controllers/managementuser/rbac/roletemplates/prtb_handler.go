package roletemplates

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	prtbOwnerLabel      = "authz.cluster.cattle.io/prtb-owner"
	projectIDAnnotation = "field.cattle.io/projectId"
)

type prtbHandler struct {
	impersonationHandler impersonationHandler
	crClient             wrbacv1.ClusterRoleController
	rtClient             mgmtv3.RoleTemplateController
	nsClient             wcorev1.NamespaceController
	rbClient             wrbacv1.RoleBindingClient
}

func newPRTBHandler(uc *config.UserContext) *prtbHandler {
	return &prtbHandler{
		impersonationHandler: impersonationHandler{
			userContext: uc,
			crClient:    uc.RBACw.ClusterRole(),
			crtbClient:  uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
			crtbCache:   uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
			prtbClient:  uc.Management.Wrangler.Mgmt.ProjectRoleTemplateBinding(),
			prtbCache:   uc.Management.Wrangler.Mgmt.ProjectRoleTemplateBinding().Cache(),
		},
		crClient: uc.RBACw.ClusterRole(),
		rtClient: uc.Management.Wrangler.Mgmt.RoleTemplate(),
		nsClient: uc.Corew.Namespace(),
		rbClient: uc.RBACw.RoleBinding(),
	}
}

// OnChange ensures a Role Binding exists in every project namespace to the RoleTemplate ClusterRole.
// If there are promoted rules, it creates a second Role Binding in each namaspace to the promoted ClusterRole
func (p *prtbHandler) OnChange(_ string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	if prtb == nil || prtb.DeletionTimestamp != nil {
		return nil, nil
	}

	if err := p.reconcileBindings(prtb); err != nil {
		return nil, err
	}

	// Ensure a service account impersonator exists on the cluster
	if prtb.UserName != "" {
		if err := p.impersonationHandler.ensureServiceAccountImpersonator(prtb.UserName); err != nil {
			return nil, fmt.Errorf("error deleting service account impersonator: %w", err)
		}
	}

	return prtb, nil
}

// reconcileBindings lists all existing RoleBindings in each project namespace and ensures they are correct.
// If not it deletes them and creates the correct RoleBindings.
func (p *prtbHandler) reconcileBindings(prtb *v3.ProjectRoleTemplateBinding) error {
	hasPromotedRule, err := p.doesRoleTemplateHavePromotedRules(prtb)
	if err != nil {
		return err
	}

	subject, err := rbac.BuildSubjectFromRTB(prtb)
	if err != nil {
		return err
	}

	// The desired rolebindings
	var rb, promotedRB *rbacv1.RoleBinding

	// Build RoleBinding and Promoted RoleBindings needed in each namespace
	roleName := rbac.AggregatedClusterRoleNameFor(prtb.RoleTemplateName)
	rb = buildRoleBinding(prtb, roleName, subject)

	// If there is no promoted rule, leave promotedRB nil
	if hasPromotedRule {
		promotedRoleName := rbac.AggregatedClusterRoleNameFor(rbac.PromotedClusterRoleNameFor(prtb.RoleTemplateName))
		promotedRB = buildRoleBinding(prtb, promotedRoleName, subject)
	}

	namespaces, err := p.getNamespacesFromProject(prtb)
	if err != nil {
		return err
	}

	for _, n := range namespaces.Items {
		if !n.DeletionTimestamp.IsZero() {
			continue
		}

		// Set the namespace of the RoleBindings
		rb.Namespace = n.Name
		if hasPromotedRule {
			promotedRB.Namespace = n.Name
		}

		if err := p.ensureOnlyDesiredRoleBindingsExist(rb, promotedRB, n.Name, rbac.GetPRTBOwnerLabel(prtb.Name)); err != nil {
			return err
		}
	}

	return nil
}

// OnRemove removes all Role Bindings in each project namespace made by the PRTB
func (p *prtbHandler) OnRemove(key string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	// Select all namespaces in project
	_, projectId, _ := strings.Cut(prtb.ProjectName, ":")
	namespaces, err := p.nsClient.List(metav1.ListOptions{
		LabelSelector: projectIDAnnotation + "=" + projectId,
	})
	if err != nil {
		return nil, err
	}

	lo := metav1.ListOptions{LabelSelector: rbac.GetPRTBOwnerLabel(prtb.Name)}

	var returnError error
	for _, n := range namespaces.Items {
		rbs, err := p.rbClient.List(n.Name, lo)
		if err != nil {
			return nil, err
		}
		for _, crb := range rbs.Items {
			err = p.rbClient.Delete(n.Name, crb.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				returnError = errors.Join(returnError, err)
			}
		}
	}

	if prtb.UserName != "" {
		if err = p.impersonationHandler.deleteServiceAccountImpersonator(prtb.UserName); err != nil {
			return nil, err
		}
	}

	return nil, returnError
}

// doesRoleTemplateHavePromotedRules checks if the PRTB's RoleTemplate has a ClusterRole for promoted rules.
func (p *prtbHandler) doesRoleTemplateHavePromotedRules(prtb *v3.ProjectRoleTemplateBinding) (bool, error) {
	rt, err := p.rtClient.Get(prtb.RoleTemplateName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	_, err = p.crClient.Get(rbac.PromotedClusterRoleNameFor(rt.Name), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	return !apierrors.IsNotFound(err), nil
}

// getNamespacesFromProject Lists all namespaces within a project
func (p *prtbHandler) getNamespacesFromProject(prtb *v3.ProjectRoleTemplateBinding) (*corev1.NamespaceList, error) {
	_, projectId, _ := strings.Cut(prtb.ProjectName, ":")
	return p.nsClient.List(metav1.ListOptions{
		LabelSelector: projectIDAnnotation + "=" + projectId,
	})
}

// buildRoleBinding creates a role binding owned by the prtb.
func buildRoleBinding(prtb *v3.ProjectRoleTemplateBinding, roleRefName string, subject rbacv1.Subject) *rbacv1.RoleBinding {
	roleRef := rbacv1.RoleRef{
		Kind: "Role",
		Name: roleRefName,
	}
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "rb-",
			Labels:       map[string]string{rbac.PrtbOwnerLabel: prtb.Name},
		},
		RoleRef:  roleRef,
		Subjects: []rbacv1.Subject{subject},
	}
}

// ensureOnlyDesiredRoleBindingsExist finds any RoleBindings owned by the PRTB, and removes them if they don't match either of our desired RoleBindings.
// If our desired RoleBindings aren't found, it creates them.
// If desiredPromotedRB is left nil, only check for desiredRB.
func (p *prtbHandler) ensureOnlyDesiredRoleBindingsExist(desiredRB, desiredPromotedRB *rbacv1.RoleBinding, namespace, prtbOwnerLabel string) error {
	// Check if any Role Bindings exist already
	currentRBs, err := p.rbClient.List(namespace, metav1.ListOptions{LabelSelector: prtbOwnerLabel})
	if err != nil || currentRBs == nil {
		return err
	}

	var matchingRB, matchingPromotedRB *rbacv1.RoleBinding
	var rbsToDelete []*rbacv1.RoleBinding
	// Search for RoleBindings that are needed, all others should be removed
	for _, currentRB := range currentRBs.Items {
		if areRoleBindingsSame(&currentRB, desiredRB) && matchingRB == nil {
			matchingRB = &currentRB
		} else if desiredPromotedRB != nil && areRoleBindingsSame(&currentRB, desiredPromotedRB) && matchingPromotedRB == nil {
			matchingPromotedRB = &currentRB
		} else {
			rbsToDelete = append(rbsToDelete, &currentRB)
		}
	}

	// Remove excess RBs
	for _, rbToDelete := range rbsToDelete {
		if err = p.rbClient.Delete(namespace, rbToDelete.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	if matchingRB == nil {
		if _, err := p.rbClient.Create(desiredRB); err != nil {
			return err
		}
	}
	if desiredPromotedRB != nil && matchingPromotedRB == nil {
		if _, err := p.rbClient.Create(desiredPromotedRB); err != nil {
			return err
		}
	}
	return nil
}

// areRoleBindingsSame compares the Subjects and RoleRef fields of two Role Bindings.
func areRoleBindingsSame(rb1, rb2 *rbacv1.RoleBinding) bool {
	return reflect.DeepEqual(rb1.Subjects, rb2.Subjects) &&
		reflect.DeepEqual(rb1.RoleRef, rb2.RoleRef)
}
