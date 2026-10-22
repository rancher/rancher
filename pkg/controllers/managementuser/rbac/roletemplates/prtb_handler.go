package roletemplates

import (
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/rancher/rancher/pkg/apis/management.cattle.io"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/impersonation"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	rbacAuth "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"
)

const (
	projectIDAnnotation = "field.cattle.io/projectId"
	namespaceReadOnly   = "namespaces-readonly"
	namespaceEdit       = "namespaces-edit"
	namespaceManage     = "namespaces-manage"
	namespacesCreate    = "create-ns"
	updatePSAVerb       = "updatepsa"
)

type prtbHandler struct {
	impersonationHandler impersonationHandler
	crClient             wrbacv1.ClusterRoleController
	crbClient            wrbacv1.ClusterRoleBindingController
	rtClient             mgmtv3.RoleTemplateController
	nsClient             wcorev1.NamespaceController
	rbClient             wrbacv1.RoleBindingClient
	prtbCache            mgmtv3.ProjectRoleTemplateBindingCache
	clusterName          string
}

func newPRTBHandler(uc *config.UserContext) (*prtbHandler, error) {
	impersonator, err := impersonation.ForCluster(uc)
	if err != nil {
		return nil, err
	}
	return &prtbHandler{
		impersonationHandler: impersonationHandler{
			clusterName:  uc.ClusterName,
			impersonator: impersonator,
			crClient:     uc.RBACw.ClusterRole(),
			crtbCache:    uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
			prtbCache:    uc.Management.Wrangler.Mgmt.ProjectRoleTemplateBinding().Cache(),
		},
		crClient:    uc.RBACw.ClusterRole(),
		crbClient:   uc.RBACw.ClusterRoleBinding(),
		rtClient:    uc.Management.Wrangler.Mgmt.RoleTemplate(),
		nsClient:    uc.Corew.Namespace(),
		rbClient:    uc.RBACw.RoleBinding(),
		prtbCache:   uc.Management.Wrangler.Mgmt.ProjectRoleTemplateBinding().Cache(),
		clusterName: uc.ClusterName,
	}, nil
}

// OnChange ensures a Role Binding exists in every project namespace to the RoleTemplate ClusterRole.
// If there are promoted rules, it creates a second Role Binding in each namespace to the promoted ClusterRole
func (p *prtbHandler) OnChange(_ string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	if prtb == nil || prtb.DeletionTimestamp != nil || !features.AggregatedRoleTemplates.Enabled() {
		return nil, nil
	}

	// Only run this controller if the PRTB is for this cluster
	clusterName, _ := rbac.GetClusterAndProjectNameFromPRTB(prtb)
	if clusterName != p.clusterName {
		return nil, nil
	}

	// Create bindings
	if err := errors.Join(p.reconcileClusterRoleBindings(prtb), p.reconcileBindings(prtb)); err != nil {
		return nil, err
	}

	// Ensure a service account impersonator exists on the cluster.
	if prtb.UserName != "" {
		if err := p.impersonationHandler.ensureServiceAccountImpersonator(prtb.UserName); err != nil {
			return nil, fmt.Errorf("error ensuring service account impersonator for %s: %w", prtb.UserName, err)
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
				Name: rbac.NameForRoleBinding(namespace.Name, roleRef, subject),
				Labels: map[string]string{
					rbac.GetPRTBOwnerLabel(prtb.Name): "true",
					rbac.AggregationFeatureLabel:      "true",
				},
				Namespace: namespace.Name,
			},
			RoleRef:  roleRef,
			Subjects: []rbacv1.Subject{subject},
		}

		if err := p.ensureOnlyDesiredRoleBindingExists(rb, prtb); err != nil {
			return err
		}
	}

	return nil
}

// reconcileClusterRoleBindings handles the promoted, cluster scoped, and namespace Cluster Role Bindings for a PRTB.
// Promoted CRBs are for the implicit set of global resources that are non-namespace scoped that are given by the PRTB.
// Cluster scoped CRBs are for the rules a user explicitly declares in the RoleTemplate's ClusterScopedRules field.
// Namespace CRBs are to give the user access to the namespaces within the project, see ProjectNamespaceRoles. Primarily used by the UI.
func (p *prtbHandler) reconcileClusterRoleBindings(prtb *v3.ProjectRoleTemplateBinding) error {
	crbs := []*rbacv1.ClusterRoleBinding{}

	// If the RoleTemplate doesn't exist yet, there's no way to tell if Promoted or Namespace Rules exist
	rt, err := p.rtClient.Get(prtb.RoleTemplateName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	// Create cluster role bindings for promoted rules and cluster scoped rules, if they exist.
	for _, clusterRoleNameFunc := range []func(string) string{
		rbac.PromotedClusterRoleNameFor,
		rbac.ClusterScopedClusterRoleNameFor,
	} {
		crb, err := p.buildClusterRoleBindingForRules(prtb, rt, clusterRoleNameFunc)
		if err != nil {
			return err
		}
		if crb == nil {
			continue
		}
		crbs = append(crbs, crb)
	}

	// Namespace rules always need to be created.
	namespaceCRBs, err := p.buildNamespaceBindings(prtb)
	if err != nil {
		return err
	}

	crbs = append(crbs, namespaceCRBs...)

	return p.ensureOnlyDesiredClusterRoleBindingsExists(crbs, prtb)
}

// buildNamespaceBindings builds the Cluster Role Bindings used to provide access to the project's namespaces.
func (p *prtbHandler) buildNamespaceBindings(prtb *v3.ProjectRoleTemplateBinding) ([]*rbacv1.ClusterRoleBinding, error) {
	cr, err := p.crClient.Get(rbac.AggregatedClusterRoleNameFor(prtb.RoleTemplateName), metav1.GetOptions{})
	// With no CR the namespace bindings can't be created
	if apierrors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	neededCRBs := []*rbacv1.ClusterRoleBinding{}
	_, projectName := rbac.GetClusterAndProjectNameFromPRTB(prtb)

	for _, roleName := range ProjectNamespaceRoles(projectName, cr.Rules) {
		namespaceCRB, err := rbac.BuildClusterRoleBindingFromRTB(prtb, roleName)
		if err != nil {
			return nil, err
		}
		neededCRBs = append(neededCRBs, namespaceCRB)
	}

	// check if any of the aggregated CR grant updatepsa permission
	psaCRName := projectName + "-namespaces-psa"
	psaRec := authorizer.AttributesRecord{
		Verb:            updatePSAVerb,
		APIGroup:        management.GroupName,
		Resource:        v3.ProjectResourceName,
		Name:            prtb.ProjectName,
		ResourceRequest: true,
	}
	if rbacAuth.RulesAllow(psaRec, cr.Rules...) {
		// if rules allow user to use updatepsa,
		// then we create a dedicated ClusterRole for the project.
		psaCR := rbac.BuildClusterRole(psaCRName, prtb.RoleTemplateName, []rbacv1.PolicyRule{
			{
				APIGroups:     []string{management.GroupName},
				Verbs:         []string{updatePSAVerb},
				Resources:     []string{v3.ProjectResourceName},
				ResourceNames: []string{projectName},
			},
		})
		if _, err := p.crClient.Create(psaCR); err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("failed to create namespace PSA cluster role %s: %w", psaCR.Name, err)
		}

		namespacePSACRB, err := rbac.BuildClusterRoleBindingFromRTB(prtb, psaCR.Name)
		if err != nil {
			return nil, err
		}
		neededCRBs = append(neededCRBs, namespacePSACRB)
	}

	return neededCRBs, nil
}

// ProjectNamespaceRoles returns the names of the ClusterRoles a project member needs to be bound to for access to the
// project's namespaces, based on the rules of their RoleTemplates:
//   - "*" on namespaces gets the project's namespaces-edit role, which grants every verb on the project's namespaces.
//   - every member without the namespaces-edit role gets the project's namespaces-readonly role.
//   - create on namespaces gets create-ns, so the member can create namespaces.
//   - create, update, patch or "*" on namespaces gets the project's namespaces-manage role, so the member can create
//     namespaces in, or move namespaces into, the project.
//
// Any other verbs on the project's namespaces come from the RoleBindings in each project namespace, as Kubernetes
// applies the namespaces rules of a RoleBinding to the namespace the RoleBinding is in.
func ProjectNamespaceRoles(projectName string, rules []rbacv1.PolicyRule) []string {
	allows := func(verb string) bool {
		attributes := authorizer.AttributesRecord{
			Verb:            verb,
			APIGroup:        "",
			Resource:        "namespaces",
			ResourceRequest: true,
		}
		for _, rule := range rules {
			if len(rule.ResourceNames) == 0 && rbacAuth.RulesAllow(attributes, rule) {
				return true
			}
		}
		return false
	}

	var roles []string
	canCreate := allows("create")
	if canCreate {
		roles = append(roles, namespacesCreate)
	}

	// Asking for "*" only matches rules that list "*" as a verb, not rules that list every verb individually.
	if allows(rbacv1.VerbAll) {
		roles = append(roles, projectName+"-"+namespaceEdit)
	} else {
		roles = append(roles, projectName+"-"+namespaceReadOnly)
	}

	// Rules with "*" as a verb allow create, so they're covered by canCreate.
	if canCreate || allows("update") || allows("patch") {
		roles = append(roles, projectName+"-"+namespaceManage)
	}
	return roles
}

// ensureOnlyDesiredClusterRoleBindingsExists takes a list of ClusterRoleBindings and ensures they are the only CRBs that exist for this PRTB.
// Deletes any CRBs with the prtbOwnerLabel that aren't in the given list, and adopts or prunes CRBs created for the PRTB by legacy RBAC.
func (p *prtbHandler) ensureOnlyDesiredClusterRoleBindingsExists(crbs []*rbacv1.ClusterRoleBinding, prtb *v3.ProjectRoleTemplateBinding) error {
	// Turn the slice into a map for easier operations.
	desiredCRBs := map[string]*rbacv1.ClusterRoleBinding{}
	for _, crb := range crbs {
		desiredCRBs[crb.Name] = crb
	}

	// Make sure the aggregation label is set on all the desired CRBs so they can be filtered on later.
	for _, crb := range crbs {
		AddAggregationFeatureLabel(crb)
	}

	labelSelector := labels.Set{
		rbac.GetPRTBOwnerLabel(prtb.Name): "true",
		rbac.AggregationFeatureLabel:      "true",
	}
	aggregationCRBs, err := p.crbClient.List(metav1.ListOptions{LabelSelector: labelSelector.AsSelector().String()})
	if err != nil || aggregationCRBs == nil {
		return err
	}

	// Delete any ClusterRoleBindings that were created for this PRTB before the aggregation changes.
	// TODO: Remove this once roletemplate aggregation is the only enabled RBAC model. https://github.com/rancher/rancher/issues/53743
	legacyCRBs, err := p.crbClient.List(metav1.ListOptions{LabelSelector: rtbOwnerLabel + "=" + rbac.GetRTBLabel(prtb.ObjectMeta)})
	if err != nil || legacyCRBs == nil {
		return err
	}

	currentCRBs := append(aggregationCRBs.Items, legacyCRBs.Items...)

	// Search for the ClusterRoleBindings that are needed, all others should be removed.
	deletedCRBs := map[string]bool{}
	for _, currentCRB := range currentCRBs {
		if desiredCRB, ok := desiredCRBs[currentCRB.Name]; ok {
			if rbac.IsClusterRoleBindingContentSame(&currentCRB, desiredCRB) {
				delete(desiredCRBs, desiredCRB.Name)
				continue
			}
		}
		// Subject and RoleRef can't be updated, so we need to delete and recreate them.
		if err = rbac.DeleteResource(currentCRB.Name, p.crbClient); err != nil {
			return err
		}
		deletedCRBs[currentCRB.Name] = true
	}

	// Adopt or prune the project cluster-wide ClusterRoleBindings legacy RBAC created for this PRTB.
	// TODO: Remove this once roletemplate aggregation is the only enabled RBAC model. https://github.com/rancher/rancher/issues/53743
	legacyProjectCRBs, err := p.listLegacyProjectClusterRoleBindings(prtb)
	if err != nil {
		return err
	}
	for _, legacyCRB := range legacyProjectCRBs {
		if deletedCRBs[legacyCRB.Name] {
			continue
		}
		if err := p.migrateLegacyProjectClusterRoleBinding(&legacyCRB, prtb, desiredCRBs); err != nil {
			return err
		}
	}

	// Any remaining ClusterRoleBindings in the desiredCRBs get created.
	for _, crb := range desiredCRBs {
		// It's possible the CRB was already created, so ignore AlreadyExists errors
		if _, err := p.crbClient.Create(crb); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create cluster role binding %s: %w", crb.Name, err)
		}
	}
	return nil
}

// listLegacyProjectClusterRoleBindings lists the project cluster-wide ClusterRoleBindings (create-ns, <project>-namespaces-*,
// *-promoted) that legacy RBAC created for the PRTB. Legacy RBAC labels these <ns>_<name>: owner-user, so one binding can
// be shared by several PRTBs.
func (p *prtbHandler) listLegacyProjectClusterRoleBindings(prtb *v3.ProjectRoleTemplateBinding) ([]rbacv1.ClusterRoleBinding, error) {
	selector := labels.Set{rbac.GetRTBLabel(prtb.ObjectMeta): rbac.LegacyProjectCRBOwnerValue}.AsSelector().String()
	list, err := p.crbClient.List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("failed to list legacy cluster role bindings: %w", err)
	}
	if list == nil {
		return nil, nil
	}
	return list.Items, nil
}

// migrateLegacyProjectClusterRoleBinding removes the PRTB's legacy owner label from a legacy project ClusterRoleBinding.
// If the binding is still desired, it is adopted in place by adding the aggregation labels, so access isn't interrupted.
// Otherwise it is deleted, unless another PRTB still owns it. Adopted bindings are removed from desiredCRBs.
func (p *prtbHandler) migrateLegacyProjectClusterRoleBinding(crb *rbacv1.ClusterRoleBinding, prtb *v3.ProjectRoleTemplateBinding, desiredCRBs map[string]*rbacv1.ClusterRoleBinding) error {
	crb = crb.DeepCopy()
	delete(crb.Labels, rbac.GetRTBLabel(prtb.ObjectMeta))

	if desiredCRB, ok := desiredCRBs[crb.Name]; ok && rbac.IsClusterRoleBindingContentSame(crb, desiredCRB) {
		maps.Copy(crb.Labels, desiredCRB.Labels)
		if _, err := p.crbClient.Update(crb); err != nil {
			return fmt.Errorf("failed to adopt legacy cluster role binding %s: %w", crb.Name, err)
		}
		delete(desiredCRBs, crb.Name)
		return nil
	}

	owned, err := p.hasRemainingOwner(crb)
	if err != nil {
		return err
	}
	if !owned {
		return rbac.DeleteResource(crb.Name, p.crbClient)
	}
	if _, err := p.crbClient.Update(crb); err != nil {
		return fmt.Errorf("failed to remove legacy owner label from cluster role binding %s: %w", crb.Name, err)
	}
	return nil
}

// hasRemainingOwner reports whether a ClusterRoleBinding is still owned by an RTB, either through an aggregated owner
// label or through a legacy owner label whose PRTB still exists.
func (p *prtbHandler) hasRemainingOwner(crb *rbacv1.ClusterRoleBinding) (bool, error) {
	for k, v := range crb.Labels {
		if strings.HasPrefix(k, rbac.PrtbOwnerLabel) || strings.HasPrefix(k, rbac.CrtbOwnerLabel) || k == rtbOwnerLabel {
			return true, nil
		}
		if v != rbac.LegacyProjectCRBOwnerValue {
			continue
		}
		// The label key is a GetRTBLabel, which may be truncated, so it is resolved through the index.
		prtbs, err := p.prtbCache.GetByIndex(rbac.PRTBByRTBLabelIndex, k)
		if err != nil {
			return false, fmt.Errorf("failed to look up owner %s of cluster role binding %s: %w", k, crb.Name, err)
		}
		if len(prtbs) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// buildClusterRoleBindingForRules builds a binding when the RoleTemplate has an aggregate role for the given rule type.
func (p *prtbHandler) buildClusterRoleBindingForRules(prtb *v3.ProjectRoleTemplateBinding, rt *v3.RoleTemplate, clusterRoleNameFunc func(string) string) (*rbacv1.ClusterRoleBinding, error) {
	hasRules, err := p.doesRoleTemplateHaveRules(rt, clusterRoleNameFunc)
	if err != nil || !hasRules {
		return nil, err
	}
	return rbac.BuildAggregatingClusterRoleBindingFromRTB(prtb, clusterRoleNameFunc(prtb.RoleTemplateName))
}

// doesRoleTemplateHaveRules checks if the RoleTemplate has an aggregate ClusterRole for the given rule type.
func (p *prtbHandler) doesRoleTemplateHaveRules(rt *v3.RoleTemplate, clusterRoleNameFunc func(string) string) (bool, error) {
	_, err := p.crClient.Get(rbac.AggregatedClusterRoleNameFor(clusterRoleNameFunc(rt.Name)), metav1.GetOptions{})
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
func (p *prtbHandler) ensureOnlyDesiredRoleBindingExists(desiredRB *rbacv1.RoleBinding, prtb *v3.ProjectRoleTemplateBinding) error {
	labelSelector := labels.Set{
		rbac.GetPRTBOwnerLabel(prtb.Name): "true",
		rbac.AggregationFeatureLabel:      "true",
	}
	aggregationRBs, err := p.rbClient.List(desiredRB.Namespace, metav1.ListOptions{LabelSelector: labelSelector.AsSelector().String()})
	if err != nil || aggregationRBs == nil {
		return err
	}

	// Delete any RoleBindings that were created for this PRTB before the aggregation changes.
	// TODO: Remove this once roletemplate aggregation is the only enabled RBAC model. https://github.com/rancher/rancher/issues/53743
	legacyRBs, err := p.rbClient.List(desiredRB.Namespace, metav1.ListOptions{LabelSelector: rtbOwnerLabel + "=" + rbac.GetRTBLabel(prtb.ObjectMeta)})
	if err != nil || legacyRBs == nil {
		return err
	}

	currentRBs := append(aggregationRBs.Items, legacyRBs.Items...)

	var matchingRB *rbacv1.RoleBinding
	// Search for the RoleBindings that is needed, all others should be removed.
	for _, currentRB := range currentRBs {
		if rbac.IsRoleBindingContentSame(&currentRB, desiredRB) && matchingRB == nil {
			matchingRB = &currentRB
		} else {
			if err = rbac.DeleteNamespacedResource(desiredRB.Namespace, currentRB.Name, p.rbClient); err != nil {
				return err
			}
		}
	}

	// If the desired RoleBinding doesn't exist, create it.
	if matchingRB == nil {
		if _, err := p.rbClient.Create(desiredRB); err != nil {
			return fmt.Errorf("failed to create role binding %s/%s: %w", desiredRB.Namespace, desiredRB.Name, err)
		}
	}
	return nil
}
