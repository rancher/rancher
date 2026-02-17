package roletemplates

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/features"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type prtbHandler struct {
	userMGR              user.Manager
	userController       mgmtv3.UserController
	rtController         mgmtv3.RoleTemplateController
	rbController         crbacv1.RoleBindingController
	crController         crbacv1.ClusterRoleController
	crbController        crbacv1.ClusterRoleBindingController
	prtbClient           mgmtv3.ProjectRoleTemplateBindingController
	clusterController    mgmtv3.ClusterController
	clusterManager       *clustermanager.Manager
	impersonationHandler *impersonationHandler
}

func newPRTBHandler(management *config.ManagementContext, clusterManager *clustermanager.Manager) *prtbHandler {
	return &prtbHandler{
		userMGR:           management.UserManager,
		userController:    management.Wrangler.Mgmt.User(),
		rtController:      management.Wrangler.Mgmt.RoleTemplate(),
		rbController:      management.Wrangler.RBAC.RoleBinding(),
		crController:      management.Wrangler.RBAC.ClusterRole(),
		crbController:     management.Wrangler.RBAC.ClusterRoleBinding(),
		prtbClient:        management.Wrangler.Mgmt.ProjectRoleTemplateBinding(),
		clusterController: management.Wrangler.Mgmt.Cluster(),
		clusterManager:    clusterManager,
		impersonationHandler: &impersonationHandler{
			crtbCache: management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
			prtbCache: management.Wrangler.Mgmt.ProjectRoleTemplateBinding().Cache(),
		},
	}
}

// OnChange syncs the required resources used to implement a PRTB. It does the following:
//   - Create the specified user if it doesn't already exist.
//   - Create the membership bindings to give access to the cluster.
//   - Create a binding to the project management role if it exists.
func (p *prtbHandler) OnChange(_ string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	if prtb == nil || prtb.DeletionTimestamp != nil {
		return nil, nil
	}

	var err error
	prtb, err = p.handleMigration(prtb)
	if err != nil {
		return prtb, err
	}

	prtb, err = p.reconcileSubject(prtb)
	if err != nil {
		return nil, err
	}

	if err := p.reconcileMembershipBindings(prtb); err != nil {
		return nil, err
	}

	return prtb, p.reconcileBindings(prtb)
}

// handleMigration handles the migration of PRTBs when toggling the AggregatedRoleTemplates feature flag.
// If the feature flag is disabled, it removes the aggregation label and deletes any bindings that were created for aggregation.
// If the feature flag is enabled, it adds the aggregation label and deletes any legacy bindings that were created before aggregation.
// TODO: To be removed once roletemplate aggregation is the only enabled RBAC model. https://github.com/rancher/rancher/issues/53743
func (p *prtbHandler) handleMigration(prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	return handleAggregationMigration(
		prtb,
		prtb.Labels,
		func(resource *v3.ProjectRoleTemplateBinding, labels map[string]string) (*v3.ProjectRoleTemplateBinding, error) {
			prtbCopy := resource.DeepCopy()
			prtbCopy.Labels = labels
			return p.prtbClient.Update(prtbCopy)
		},
		p.deleteRoleBindings,
		p.deleteLegacyBinding,
	)
}

// OnRemove deletes Role Bindings that are owned by the PRTB. It also removes the membership binding if no other PRTBs give membership access.
func (p *prtbHandler) OnRemove(_ string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	if prtb == nil {
		return nil, nil
	}
	if !features.AggregatedRoleTemplates.Enabled() {
		return nil, p.deleteDownstreamResources(prtb, false)
	}

	returnErr := errors.Join(deleteClusterMembershipBinding(prtb, p.crbController),
		deleteProjectMembershipBinding(prtb, p.rbController),
		removeAuthV2Permissions(prtb, p.rbController),
		p.deleteRoleBindings(prtb),
		p.deleteDownstreamResources(prtb, true))

	return prtb, returnErr
}

// deleteRoleBindings deletes all Role Bindings in the project namespace made by the PRTB.
func (p *prtbHandler) deleteRoleBindings(prtb *v3.ProjectRoleTemplateBinding) error {
	// Collect all RoleBindings owned by this ProjectRoleTemplateBinding
	set := labels.Set{
		rbac.GetPRTBOwnerLabel(prtb.Name): "true",
		rbac.AggregationFeatureLabel:      "true",
	}
	currentRBs, err := p.rbController.List(prtb.Namespace, metav1.ListOptions{LabelSelector: set.AsSelector().String()})
	if err != nil {
		return fmt.Errorf("failed to list role bindings in namespace %s: %w", prtb.Namespace, err)
	}

	var returnErr error
	for _, rb := range currentRBs.Items {
		returnErr = errors.Join(returnErr, rbac.DeleteNamespacedResource(rb.Namespace, rb.Name, p.rbController))
	}
	return returnErr
}

// deleteDownstreamResources deletes all Role Bindings and Cluster Role Bindings in the downstream cluster made by the PRTB.
// If deleteImpersonator is true, it also removes the service account impersonator for the user if there are no other PRTBs or CRTBs for the user.
// If the cluster is not found, it assumes it has been deleted and does not re-queue.
func (p *prtbHandler) deleteDownstreamResources(prtb *v3.ProjectRoleTemplateBinding, deleteImpersonator bool) error {
	clusterName, _ := rbac.GetClusterAndProjectNameFromPRTB(prtb)
	cluster, err := p.clusterController.Get(clusterName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		logrus.Infof("Cluster %s not found when deleting downstream resources for PRTB %s. Not re-queuing.", clusterName, prtb.Name)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get cluster %s: %w", clusterName, err)
	}

	userContext, err := p.clusterManager.UserContext(cluster.Name)
	if err != nil {
		return err
	}

	returnErr := errors.Join(
		p.deleteDownstreamRoleBindings(prtb, userContext.RBACw.RoleBinding()),
		p.deleteDownstreamClusterRoleBindings(prtb, userContext.RBACw.ClusterRoleBinding()),
	)

	if deleteImpersonator && prtb.UserName != "" {
		returnErr = errors.Join(returnErr, p.impersonationHandler.deleteServiceAccountImpersonator(clusterName, prtb.UserName, userContext.RBACw.ClusterRole()))
	}

	return returnErr
}

// deleteDownstreamRoleBindings deletes all Role Bindings in the downstream cluster made by the PRTB.
func (p *prtbHandler) deleteDownstreamRoleBindings(prtb *v3.ProjectRoleTemplateBinding, rbController crbacv1.RoleBindingController) error {
	set := labels.Set{
		rbac.GetPRTBOwnerLabel(prtb.Name): "true",
		rbac.AggregationFeatureLabel:      "true",
	}
	currentRBs, err := rbController.List(metav1.NamespaceAll, metav1.ListOptions{LabelSelector: set.AsSelector().String()})
	if err != nil {
		return fmt.Errorf("failed to list role bindings in downstream cluster for PRTB %s: %w", prtb.Name, err)
	}

	var returnErr error
	for _, rb := range currentRBs.Items {
		returnErr = errors.Join(returnErr, rbac.DeleteNamespacedResource(rb.Namespace, rb.Name, rbController))
	}

	return returnErr
}

// deleteDownstreamClusterRoleBindings deletes all Cluster Role Bindings in the downstream cluster made by the PRTB.
func (p *prtbHandler) deleteDownstreamClusterRoleBindings(prtb *v3.ProjectRoleTemplateBinding, crbController crbacv1.ClusterRoleBindingController) error {
	set := labels.Set{
		rbac.GetPRTBOwnerLabel(prtb.Name): "true",
		rbac.AggregationFeatureLabel:      "true",
	}
	currentCRBs, err := crbController.List(metav1.ListOptions{LabelSelector: set.AsSelector().String()})
	if err != nil {
		return fmt.Errorf("failed to list cluster role bindings in downstream cluster for PRTB %s: %w", prtb.Name, err)
	}

	var returnErr error
	for _, crb := range currentCRBs.Items {
		// Check if the CRB is owned by another PRTB
		// This can happen if the CRB is reused like the namespace access CRB
		crbOwnedByAnotherPRTB := false
		delete(crb.Labels, rbac.GetPRTBOwnerLabel(prtb.Name))
		for label := range crb.Labels {
			if strings.HasPrefix(label, rbac.PrtbOwnerLabel) {
				crbOwnedByAnotherPRTB = true
				break
			}
		}
		// In the case where it is shared, only update the CRB with the ownership label removed
		if crbOwnedByAnotherPRTB {
			if _, err = crbController.Update(&crb); err != nil {
				returnErr = errors.Join(returnErr, fmt.Errorf("failed to update cluster role binding %s: %w", crb.Name, err))
			}
			continue
		}
		// If there are no other owners, delete the CRB
		returnErr = errors.Join(returnErr, rbac.DeleteResource(crb.Name, crbController))
	}
	return returnErr
}

// reconcileSubject ensures that both the UserPrincipalName and UserName are set, creating the user if UserPrincipalName is set but not UserName.
func (p *prtbHandler) reconcileSubject(binding *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	if binding.GroupName != "" || binding.GroupPrincipalName != "" || (binding.UserPrincipalName != "" && binding.UserName != "") {
		return binding, nil
	}

	if binding.UserPrincipalName == "" && binding.UserName == "" {
		return nil, fmt.Errorf("binding %v has no subject", binding.Name)
	}

	if binding.UserName == "" {
		displayName := binding.Annotations["auth.cattle.io/principal-display-name"]
		user, err := p.userMGR.EnsureUser(binding.UserPrincipalName, displayName)
		if err != nil {
			return binding, err
		}

		binding.UserName = user.Name
	}

	if binding.UserPrincipalName == "" {
		u, err := p.userController.Get(binding.UserName, metav1.GetOptions{})
		if err != nil {
			return binding, fmt.Errorf("failed to get user %s for binding %s: %w", binding.UserName, binding.Name, err)
		}
		for _, p := range u.PrincipalIDs {
			if strings.HasSuffix(p, binding.UserName) {
				binding.UserPrincipalName = p
				break
			}
		}
	}

	return binding, nil
}

// isInheritedOwner checks if a PolicyRule contains the "own" verb for the given resource. That indicates whether the user should be given owner level access via membership bindings.
func isInheritedOwner(rule rbacv1.PolicyRule, resource string) bool {
	return slices.Contains(rule.Verbs, "own") && slices.Contains(rule.Resources, resource)
}

// reconcileMembershipBindings ensures that the user is given the right membership binding to the project and cluster.
func (p *prtbHandler) reconcileMembershipBindings(prtb *v3.ProjectRoleTemplateBinding) error {
	// to determine if a user is a member or an owner, we need to check the aggregated cluster role to see if it inherited the "own" verb on projects/clusters
	clusterRole, err := p.crController.Get(rbac.AggregatedClusterRoleNameFor(prtb.RoleTemplateName), metav1.GetOptions{})
	if err != nil {
		return err
	}
	isProjectOwner, isClusterOwner := false, false
	for _, rule := range clusterRole.Rules {
		if isInheritedOwner(rule, "projects") {
			isProjectOwner = true
		}
		if isInheritedOwner(rule, "clusters") {
			isClusterOwner = true
		}
	}

	return errors.Join(createOrUpdateClusterMembershipBinding(prtb, p.crbController, isClusterOwner),
		createOrUpdateProjectMembershipBinding(prtb, p.rbController, isProjectOwner))
}

// reconcileBindings ensures the right Role Binding exists for the project management plane role. It deletes any additional unwanted Role Bindings.
func (p *prtbHandler) reconcileBindings(prtb *v3.ProjectRoleTemplateBinding) error {
	projectManagementRoleName := rbac.ProjectManagementPlaneClusterRoleNameFor(prtb.RoleTemplateName)

	// If there is no project management plane role, no need to create a binding for it
	_, err := p.crController.Get(rbac.AggregatedClusterRoleNameFor(projectManagementRoleName), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	rb, err := rbac.BuildAggregatingRoleBindingFromRTB(prtb, projectManagementRoleName)
	if err != nil {
		return err
	}

	currentRBs, err := p.rbController.List(prtb.Namespace, metav1.ListOptions{LabelSelector: rbac.GetPRTBOwnerLabel(prtb.Name)})
	if err != nil {
		return err
	}

	// Remove any excess or incorrect Role Bindings that may exist for this PRTB
	var prtbHasBinding bool
	for _, currentRB := range currentRBs.Items {
		if rbac.IsRoleBindingContentSame(&currentRB, rb) {
			prtbHasBinding = true
			continue
		}
		// RoleRef and Subjects are immutable, so we have to delete and recreate if they are different
		if err := rbac.DeleteNamespacedResource(currentRB.Namespace, currentRB.Name, p.rbController); err != nil {
			return err
		}
	}

	if !prtbHasBinding {
		if _, err := p.rbController.Create(rb); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create role binding %s: %w", rb.Name, err)
		}
	}
	return nil
}

// deleteLegacyBinding deletes the management plane Role Binding in the project namespace that was created for this PRTB before the aggregation feature was enabled.
// TODO: Remove this once roletemplate aggregation is the only enabled RBAC model. https://github.com/rancher/rancher/issues/53743
func (p *prtbHandler) deleteLegacyBinding(prtb *v3.ProjectRoleTemplateBinding) error {
	rbs, err := p.rbController.List(prtb.Namespace, metav1.ListOptions{LabelSelector: labels.Everything().String()})
	if err != nil {
		return fmt.Errorf("failed to list role bindings in cluster namespace %s: %w", prtb.Namespace, err)
	}
	var returnErr error
	for _, rb := range rbs.Items {
		if strings.HasPrefix(rb.Name, prtb.Name) {
			returnErr = errors.Join(returnErr, rbac.DeleteNamespacedResource(prtb.Namespace, fmt.Sprintf("%s-%s", prtb.Name, prtb.RoleTemplateName), p.rbController))
		}
	}
	return returnErr
}
