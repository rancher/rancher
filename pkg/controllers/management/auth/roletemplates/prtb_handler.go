package roletemplates

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type prtbHandler struct {
	userMGR        user.Manager
	userController mgmtv3.UserController
	rtController   mgmtv3.RoleTemplateController
	rbController   crbacv1.RoleBindingController
	crController   crbacv1.ClusterRoleController
	crbController  crbacv1.ClusterRoleBindingController
}

func newPRTBHandler(management *config.ManagementContext) *prtbHandler {
	return &prtbHandler{
		userMGR:        management.UserManager,
		userController: management.Wrangler.Mgmt.User(),
		rtController:   management.Wrangler.Mgmt.RoleTemplate(),
		rbController:   management.Wrangler.RBAC.RoleBinding(),
		crController:   management.Wrangler.RBAC.ClusterRole(),
		crbController:  management.Wrangler.RBAC.ClusterRoleBinding(),
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

	if !features.AggregatedRoleTemplates.Enabled() {
		return prtb, p.deleteRoleBindings(prtb)
	}

	var err error
	prtb, err = p.reconcileSubject(prtb)
	if err != nil {
		return nil, err
	}

	if err := p.reconcileMembershipBindings(prtb); err != nil {
		return nil, err
	}

	return prtb, p.reconcileBindings(prtb)
}

// OnRemove deletes Role Bindings that are owned by the PRTB. It also removes the membership binding if no other PRTBs give membership access.
func (p *prtbHandler) OnRemove(_ string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	if prtb == nil || !features.AggregatedRoleTemplates.Enabled() {
		return nil, nil
	}

	returnErr := errors.Join(deleteClusterMembershipBinding(prtb, p.crbController),
		deleteProjectMembershipBinding(prtb, p.rbController),
		removeAuthV2Permissions(prtb, p.rbController),
		p.deleteRoleBindings(prtb))

	return prtb, returnErr
}

// deleteRoleBindings deletes all Role Bindings in the project namespace made by the PRTB.
func (p *prtbHandler) deleteRoleBindings(prtb *v3.ProjectRoleTemplateBinding) error {
	// Collect all RoleBindings owned by this ProjectRoleTemplateBinding
	set := labels.Set(map[string]string{
		rbac.GetPRTBOwnerLabel(prtb.Name): "true",
		rbac.AggregationFeatureLabel:      "true",
	})
	currentRBs, err := p.rbController.List(prtb.Namespace, metav1.ListOptions{LabelSelector: set.AsSelector().String()})
	if err != nil {
		return err
	}

	var returnErr error
	for _, rb := range currentRBs.Items {
		returnErr = errors.Join(returnErr, rbac.DeleteNamespacedResource(rb.Namespace, rb.Name, p.rbController))
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
