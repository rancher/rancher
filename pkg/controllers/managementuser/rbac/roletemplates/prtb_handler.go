package roletemplates

import (
	"errors"
	"fmt"
	"reflect"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/slice"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	prtbOwnerLabel      = "authz.cluster.cattle.io/prtb-owner"
	projectIDAnnotation = "field.cattle.io/projectId"
	namespaceReadOnly   = "namespaces-readonly"
	namespaceEdit       = "namespaces-edit"
	namespacesCreate    = "create-ns"
)

type prtbHandler struct {
	impersonationHandler impersonationHandler
	crClient             wrbacv1.ClusterRoleController
	crbClient            wrbacv1.ClusterRoleBindingController
	rtClient             mgmtv3.RoleTemplateController
	nsClient             wcorev1.NamespaceController
	rbClient             wrbacv1.RoleBindingClient
	clusterName          string
}

func newPRTBHandler(uc *config.UserContext) *prtbHandler {
	return &prtbHandler{
		impersonationHandler: impersonationHandler{
			userContext: uc,
			crClient:    uc.RBACw.ClusterRole(),
			crtbCache:   uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
			prtbCache:   uc.Management.Wrangler.Mgmt.ProjectRoleTemplateBinding().Cache(),
		},
		crClient:    uc.RBACw.ClusterRole(),
		crbClient:   uc.RBACw.ClusterRoleBinding(),
		rtClient:    uc.Management.Wrangler.Mgmt.RoleTemplate(),
		nsClient:    uc.Corew.Namespace(),
		rbClient:    uc.RBACw.RoleBinding(),
		clusterName: uc.ClusterName,
	}
}

// OnChange ensures a Role Binding exists in every project namespace to the RoleTemplate ClusterRole.
// If there are promoted rules, it creates a second Role Binding in each namaspace to the promoted ClusterRole
func (p *prtbHandler) OnChange(_ string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	if prtb == nil || prtb.DeletionTimestamp != nil {
		return nil, nil
	}

	// Only run this controller if the PRTB is for this cluster
	clusterName, _ := rbac.GetClusterAndProjectNameFromPRTB(prtb)
	if clusterName != p.clusterName {
		return nil, nil
	}

	// Handle cluster role bindings for special permissions.
	if err := p.reconcileClusterRoleBindings(prtb); err != nil {
		return nil, err
	}

	if err := p.reconcileBindings(prtb); err != nil {
		return nil, err
	}

	// Ensure a service account impersonator exists on the cluster.
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
	subject, err := rbac.BuildSubjectFromRTB(prtb)
	if err != nil {
		return err
	}

	isExternal, err := isRoleTemplateExternal(prtb.RoleTemplateName, p.rtClient)
	if err != nil {
		return err
	}

	// External Role Templates don't support aggregation, bind to the external cluster role directly.
	var roleName string
	if isExternal {
		roleName = prtb.RoleTemplateName
	} else {
		roleName = rbac.AggregatedClusterRoleNameFor(prtb.RoleTemplateName)
	}

	roleRef := rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     roleName,
		APIGroup: rbacv1.GroupName,
	}

	namespaces, err := p.getNamespacesFromProject(prtb)
	if err != nil {
		return err
	}

	for _, namespace := range namespaces.Items {
		if !namespace.DeletionTimestamp.IsZero() {
			continue
		}

		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rbac.NameForRoleBinding(namespace.Name, roleRef, subject),
				Labels:    map[string]string{rbac.PrtbOwnerLabel: prtb.Name},
				Namespace: namespace.Name,
			},
			RoleRef:  roleRef,
			Subjects: []rbacv1.Subject{subject},
		}

		if err := p.ensureOnlyDesiredRoleBindingExists(rb, rbac.GetPRTBOwnerLabel(prtb.Name)); err != nil {
			return err
		}
	}

	return nil
}

// OnRemove removes all Role Bindings in each project namespace made by the PRTB.
func (p *prtbHandler) OnRemove(_ string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	// Select all namespaces in project.
	_, projectName := rbac.GetClusterAndProjectNameFromPRTB(prtb)
	namespaces, err := p.nsClient.List(metav1.ListOptions{
		LabelSelector: projectIDAnnotation + "=" + projectName,
	})
	if err != nil {
		return nil, err
	}

	lo := metav1.ListOptions{LabelSelector: rbac.GetPRTBOwnerLabel(prtb.Name)}

	var returnError error
	// Remove all role bindings.
	for _, n := range namespaces.Items {
		rbs, err := p.rbClient.List(n.Name, lo)
		if err != nil {
			return nil, err
		}
		for _, rb := range rbs.Items {
			err = p.rbClient.Delete(n.Name, rb.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				returnError = errors.Join(returnError, err)
			}
		}
	}

	// Remove all cluster role bindings.
	crbs, err := p.crbClient.List(lo)
	if err != nil {
		return nil, err
	}
	for _, crb := range crbs.Items {
		err = p.crbClient.Delete(crb.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			returnError = errors.Join(returnError, err)
		}
	}

	if prtb.UserName != "" {
		if err = p.impersonationHandler.deleteServiceAccountImpersonator(prtb.UserName); err != nil {
			return nil, err
		}
	}

	return prtb, returnError
}

// reconcileClusterRoleBindings handles the promoted and namespace Cluster Role Bindings for a PRTB.
// Promoted CRBs are for any rules that are non-namespace scoped that are given by the PRTB.
// Namespace CRBs are to give the user either edit or read-only access to the namespaces within the project. Primarily used by the UI.
func (p *prtbHandler) reconcileClusterRoleBindings(prtb *v3.ProjectRoleTemplateBinding) error {
	crbs := []*rbacv1.ClusterRoleBinding{}

	// If the RoleTemplate doesn't exist yet, there's no way to tell if Promoted or Namespace Rules exist
	rt, err := p.rtClient.Get(prtb.RoleTemplateName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	// Check for promoted rules.
	hasPromotedRule, err := p.doesRoleTemplateHavePromotedRules(rt)
	if err != nil {
		return err
	}
	if hasPromotedRule {
		promotedRuleName := rbac.PromotedClusterRoleNameFor(prtb.RoleTemplateName)
		crb, err := rbac.BuildAggregatingClusterRoleBindingFromRTB(prtb, promotedRuleName)
		if err != nil {
			return err
		}
		crbs = append(crbs, crb)
	}

	// Namespace rules always need to be created.
	namespaceCRBs, err := p.buildNamespaceBindings(prtb)
	if err != nil {
		return err
	}

	crbs = append(crbs, namespaceCRBs...)

	return p.ensureOnlyDesiredClusterRoleBindingsExists(crbs, rbac.GetPRTBOwnerLabel(prtb.Name))
}

// buildNamespaceBindings builds the Cluster Role Bindings used to provide access to the project's namespaces.
func (p *prtbHandler) buildNamespaceBindings(prtb *v3.ProjectRoleTemplateBinding) ([]*rbacv1.ClusterRoleBinding, error) {
	cr, err := p.crClient.Get(rbac.AggregatedClusterRoleNameFor(prtb.RoleTemplateName), metav1.GetOptions{})
	// With no CR the namespace bindings can't be created
	if apierrors.IsNotFound(err) || cr == nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	_, projectName := rbac.GetClusterAndProjectNameFromPRTB(prtb)
	for _, rule := range cr.Rules {
		hasNamespaceResources := slice.ContainsString(rule.Resources, "namespaces") || slice.ContainsString(rule.Resources, rbacv1.ResourceAll)
		hasNamespaceGroup := slice.ContainsString(rule.APIGroups, "") || slice.ContainsString(rule.APIGroups, rbacv1.APIGroupAll)
		if hasNamespaceGroup && hasNamespaceResources && len(rule.ResourceNames) == 0 {
			if slice.ContainsString(rule.Verbs, rbacv1.VerbAll) || slice.ContainsString(rule.Verbs, "create") {
				// need binding for create-ns and namespaces-edit
				namespaceCreateCR, err := rbac.BuildClusterRoleBindingFromRTB(prtb, namespacesCreate)
				if err != nil {
					return nil, err
				}
				namespaceEditCR, err := rbac.BuildClusterRoleBindingFromRTB(prtb, projectName+"-"+namespaceEdit)
				if err != nil {
					return nil, err
				}
				return []*rbacv1.ClusterRoleBinding{namespaceCreateCR, namespaceEditCR}, nil
			}
		}
	}
	// Didn't have edit access to namespaces, needs read access binding
	namespaceCR, err := rbac.BuildClusterRoleBindingFromRTB(prtb, projectName+"-"+namespaceReadOnly)
	if err != nil {
		return nil, err
	}
	return []*rbacv1.ClusterRoleBinding{namespaceCR}, nil
}

// ensureOnlyDesiredClusterRoleBindingsExists takes a list of ClusterRoleBindings and ensures they are the only CRBs that exist for this PRTB.
// Deletes any CRBs with the prtbOwnerLabel that aren't in the given list.
func (p *prtbHandler) ensureOnlyDesiredClusterRoleBindingsExists(crbs []*rbacv1.ClusterRoleBinding, prtbOwnerLabel string) error {
	// Turn the slice into a map for easier operations.
	desiredCRBs := map[string]*rbacv1.ClusterRoleBinding{}
	for _, crb := range crbs {
		desiredCRBs[crb.Name] = crb
	}

	// Check if any Cluster Role Bindings exist already.
	currentCRBs, err := p.crbClient.List(metav1.ListOptions{LabelSelector: prtbOwnerLabel})
	if err != nil || currentCRBs == nil {
		return err
	}

	// Search for the ClusterRoleBindings that are needed, all others should be removed.
	for _, currentCRB := range currentCRBs.Items {
		if desiredCRB, ok := desiredCRBs[currentCRB.Name]; ok {
			if rbac.AreClusterRoleBindingContentsSame(&currentCRB, desiredCRB) {
				delete(desiredCRBs, desiredCRB.Name)
				continue
			}
		}
		if err = p.crbClient.Delete(currentCRB.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	// Any remaining ClusterRoleBindings in the desiredCRBs get created.
	for _, crb := range desiredCRBs {
		if _, err := p.crbClient.Create(crb); err != nil {
			return err
		}
	}
	return nil
}

// doesRoleTemplateHavePromotedRules checks if the PRTB's RoleTemplate has a ClusterRole for promoted rules.
func (p *prtbHandler) doesRoleTemplateHavePromotedRules(rt *v3.RoleTemplate) (bool, error) {
	_, err := p.crClient.Get(rbac.PromotedClusterRoleNameFor(rt.Name), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	return !apierrors.IsNotFound(err), nil
}

// getNamespacesFromProject Lists all namespaces within a project.
func (p *prtbHandler) getNamespacesFromProject(prtb *v3.ProjectRoleTemplateBinding) (*corev1.NamespaceList, error) {
	_, projectId := rbac.GetClusterAndProjectNameFromPRTB(prtb)
	return p.nsClient.List(metav1.ListOptions{
		LabelSelector: projectIDAnnotation + "=" + projectId,
	})
}

// ensureOnlyDesiredRoleBindingExists finds any RoleBindings owned by the PRTB, and removes them if they don't match the desired RoleBinding.
// If the desired RoleBinding isn't found, it creates it.
func (p *prtbHandler) ensureOnlyDesiredRoleBindingExists(desiredRB *rbacv1.RoleBinding, prtbOwnerLabel string) error {
	// Check if any Role Bindings exist already.
	currentRBs, err := p.rbClient.List(desiredRB.Namespace, metav1.ListOptions{LabelSelector: prtbOwnerLabel})
	if err != nil || currentRBs == nil {
		return err
	}

	var matchingRB *rbacv1.RoleBinding
	// Search for the RoleBindings that is needed, all others should be removed.
	for _, currentRB := range currentRBs.Items {
		if areRoleBindingsSame(&currentRB, desiredRB) && matchingRB == nil {
			matchingRB = &currentRB
		} else {
			if err = p.rbClient.Delete(desiredRB.Namespace, currentRB.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}

	// If the desired RoleBinding doesn't exist, create it.
	if matchingRB == nil {
		if _, err := p.rbClient.Create(desiredRB); err != nil {
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
