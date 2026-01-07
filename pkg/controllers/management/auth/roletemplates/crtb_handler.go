package roletemplates

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/status"
	"github.com/rancher/rancher/pkg/features"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
)

type crtbHandler struct {
	s              *status.Status
	userMGR        user.Manager
	userController mgmtv3.UserController
	rtController   mgmtv3.RoleTemplateController
	rbController   wrbacv1.RoleBindingController
	crController   wrbacv1.ClusterRoleController
	crbController  wrbacv1.ClusterRoleBindingController
	crtbCache      mgmtv3.ClusterRoleTemplateBindingCache
	crtbClient     mgmtv3.ClusterRoleTemplateBindingController
}

func newCRTBHandler(management *config.ManagementContext) *crtbHandler {
	return &crtbHandler{
		s:              status.NewStatus(),
		userMGR:        management.UserManager,
		userController: management.Wrangler.Mgmt.User(),
		rtController:   management.Wrangler.Mgmt.RoleTemplate(),
		rbController:   management.Wrangler.RBAC.RoleBinding(),
		crController:   management.Wrangler.RBAC.ClusterRole(),
		crbController:  management.Wrangler.RBAC.ClusterRoleBinding(),
		crtbCache:      management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
		crtbClient:     management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
	}
}

// OnChange syncs the required resources used to implement a CRTB. It does the following:
//   - Create the specified user if it doesn't already exist.
//   - Create the membership bindings to give access to the cluster.
//   - Create a binding to the project and cluster management role if it exists.
func (c *crtbHandler) OnChange(_ string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if crtb == nil || crtb.DeletionTimestamp != nil {
		return nil, nil
	}

	if !features.AggregatedRoleTemplates.Enabled() {
		return crtb, c.removeRoleBindings(crtb)
	}

	var localConditions []metav1.Condition
	var err error
	crtb, err = c.reconcileSubject(crtb, &localConditions)
	if err != nil {
		return crtb, errors.Join(err, c.updateStatus(crtb, localConditions))
	}

	if err := c.reconcileMembershipBindings(crtb, &localConditions); err != nil {
		return crtb, errors.Join(err, c.updateStatus(crtb, localConditions))
	}

	return crtb, errors.Join(c.reconcileBindings(crtb, &localConditions), c.updateStatus(crtb, localConditions))
}

// reconcileSubject ensures that the user referenced by the role template binding exists
func (c *crtbHandler) reconcileSubject(binding *v3.ClusterRoleTemplateBinding, localConditions *[]metav1.Condition) (*v3.ClusterRoleTemplateBinding, error) {
	condition := metav1.Condition{Type: reconcileSubject}
	if binding.GroupName != "" || binding.GroupPrincipalName != "" || (binding.UserPrincipalName != "" && binding.UserName != "") {
		c.s.AddCondition(localConditions, condition, subjectExists, nil)
		return binding, nil
	}

	if binding.UserPrincipalName == "" && binding.UserName == "" {
		c.s.AddCondition(localConditions, condition, crtbHasNoSubject, fmt.Errorf("CRTB has no subject"))
		return binding, fmt.Errorf("ClusterRoleTemplateBinding %v has no subject", binding.Name)
	}

	if binding.UserName == "" {
		displayName := binding.Annotations["auth.cattle.io/principal-display-name"]
		user, err := c.userMGR.EnsureUser(binding.UserPrincipalName, displayName)
		if err != nil {
			c.s.AddCondition(localConditions, condition, failedToCreateUser, err)
			return binding, err
		}

		binding.UserName = user.Name
		c.s.AddCondition(localConditions, condition, subjectExists, nil)
	}

	if binding.UserPrincipalName == "" {
		u, err := c.userController.Get(binding.UserName, metav1.GetOptions{})
		if err != nil {
			c.s.AddCondition(localConditions, condition, failedToGetUser, err)
			return binding, fmt.Errorf("failed to get user %s for binding %s: %w", binding.UserName, binding.Name, err)
		}
		for _, p := range u.PrincipalIDs {
			if strings.HasSuffix(p, binding.UserName) {
				binding.UserPrincipalName = p
				break
			}
		}
		c.s.AddCondition(localConditions, condition, subjectExists, nil)
	}

	c.s.AddCondition(localConditions, condition, subjectExists, nil)
	return binding, nil
}

// reconcileMemberShipBindings ensures that any needed membership bindings for the cluster exist
func (c *crtbHandler) reconcileMembershipBindings(crtb *v3.ClusterRoleTemplateBinding, localCondition *[]metav1.Condition) error {
	condition := metav1.Condition{Type: reconcileMembershipBindings}

	// to determine if a user is a member or an owner, we need to check the aggregated cluster role to see if it inherited the "own" verb on projects/clusters
	clusterRole, err := c.crController.Get(rbac.AggregatedClusterRoleNameFor(crtb.RoleTemplateName), metav1.GetOptions{})
	if err != nil {
		c.s.AddCondition(localCondition, condition, failedToGetClusterRole, err)
		return err
	}
	isClusterOwner := false
	for _, rule := range clusterRole.Rules {
		if slices.Contains(rule.Verbs, "own") && slices.Contains(rule.Resources, "clusters") {
			isClusterOwner = true
			break
		}
	}

	// Create membership binding
	if err := createOrUpdateClusterMembershipBinding(crtb, c.crbController, isClusterOwner); err != nil {
		c.s.AddCondition(localCondition, condition, failedToCreateOrUpdateMembershipBinding, err)
		return err
	}

	c.s.AddCondition(localCondition, condition, membershipBindingExists, nil)
	return nil
}

// reconcileBindings Ensures that all bindings required to provide access to the CRTB either exist or get created.
// It deletes any existing role bindings with the CRTB owner label that are not needed.
func (c *crtbHandler) reconcileBindings(crtb *v3.ClusterRoleTemplateBinding, localConditions *[]metav1.Condition) error {
	condition := metav1.Condition{Type: reconcileBindings}
	desiredRBs, err := c.getDesiredRoleBindings(crtb)
	if err != nil {
		c.s.AddCondition(localConditions, condition, failedToGetDesiredRoleBindings, err)
		return err
	}

	currentRBs, err := c.rbController.List(crtb.Namespace, metav1.ListOptions{LabelSelector: rbac.GetCRTBOwnerLabel(crtb.Name)})
	if err != nil {
		c.s.AddCondition(localConditions, condition, failedToListExistingRoleBindings, err)
		return err
	}

	for _, currentRB := range currentRBs.Items {
		if rb, ok := desiredRBs[currentRB.Name]; ok {
			if rbac.IsRoleBindingContentSame(&currentRB, rb) {
				// If the role binding already exists with the right contents, we can skip creating it.
				delete(desiredRBs, rb.Name)
				continue
			}
		}
		// If the role binding is not a member of the desired role bindings or has different contents, delete it.
		if err := rbac.DeleteNamespacedResource(currentRB.Namespace, currentRB.Name, c.rbController); err != nil {
			c.s.AddCondition(localConditions, condition, failedToDeleteRoleBinding, err)
			return err
		}
	}

	// For any role bindings that don't exist, create them
	for _, rb := range desiredRBs {
		if _, err := c.rbController.Create(rb); err != nil && !apierrors.IsAlreadyExists(err) {
			c.s.AddCondition(localConditions, condition, failedToCreateRoleBinding, err)
			return fmt.Errorf("failed to create role binding %s: %w", rb.Name, err)
		}
	}

	c.s.AddCondition(localConditions, condition, bindingsExists, nil)
	return nil
}

// getDesiredRoleBindings checks for project and cluster management roles, and if they exist, builds and returns the needed RoleBindings
func (c *crtbHandler) getDesiredRoleBindings(crtb *v3.ClusterRoleTemplateBinding) (map[string]*rbacv1.RoleBinding, error) {
	desiredRBs := map[string]*rbacv1.RoleBinding{}
	// Check if there is a project management role to bind to
	projectManagementRoleName := rbac.ProjectManagementPlaneClusterRoleNameFor(crtb.RoleTemplateName)
	cr, err := c.crController.Get(rbac.AggregatedClusterRoleNameFor(projectManagementRoleName), metav1.GetOptions{})
	if err == nil && cr != nil {
		rb, err := rbac.BuildAggregatingRoleBindingFromRTB(crtb, projectManagementRoleName)
		if err != nil {
			return nil, err
		}
		desiredRBs[rb.Name] = rb
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	// Check if there is a cluster management role to bind to
	clusterManagementRoleName := rbac.ClusterManagementPlaneClusterRoleNameFor(crtb.RoleTemplateName)
	cr, err = c.crController.Get(rbac.AggregatedClusterRoleNameFor(clusterManagementRoleName), metav1.GetOptions{})
	if err == nil && cr != nil {
		rb, err := rbac.BuildAggregatingRoleBindingFromRTB(crtb, clusterManagementRoleName)
		if err != nil {
			return nil, err
		}
		desiredRBs[rb.Name] = rb
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	return desiredRBs, nil
}

// OnRemove deletes Cluster Role Bindings that are owned by the CRTB and the membership binding if no other CRTBs give membership access.
func (c *crtbHandler) OnRemove(_ string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if crtb == nil || !features.AggregatedRoleTemplates.Enabled() {
		return nil, nil
	}

	condition := metav1.Condition{Type: clusterRoleTemplateBindingDelete}

	err := deleteClusterMembershipBinding(crtb, c.crbController)
	c.s.AddCondition(&crtb.Status.LocalConditions, condition, clusterMembershipBindingDeleted, err)

	err = removeAuthV2Permissions(crtb, c.rbController)
	c.s.AddCondition(&crtb.Status.LocalConditions, condition, authv2ProvisioningBindingDeleted, err)

	return crtb, errors.Join(err, c.removeRoleBindings(crtb))
}

// removeClusterRoleBindings removes all bindings owned by the CRTB
func (c *crtbHandler) removeRoleBindings(crtb *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: removeRoleBindings}

	// Collect all RoleBindings owned by this ClusterRoleTemplateBinding
	set := labels.Set(map[string]string{
		rbac.GetCRTBOwnerLabel(crtb.Name): "true",
		rbac.AggregationFeatureLabel:      "true",
	})
	currentRBs, err := c.rbController.List(crtb.Namespace, metav1.ListOptions{LabelSelector: set.AsSelector().String()})
	if err != nil {
		c.s.AddCondition(&crtb.Status.LocalConditions, condition, failedToListExistingRoleBindings, err)
		return err
	}

	var returnErr error
	for _, rb := range currentRBs.Items {
		err = rbac.DeleteNamespacedResource(crtb.Namespace, rb.Name, c.rbController)
		if err != nil {
			c.s.AddCondition(&crtb.Status.LocalConditions, condition, failedToDeleteRoleBinding, err)
			returnErr = errors.Join(returnErr, err)
		}
	}

	c.s.AddCondition(&crtb.Status.LocalConditions, condition, roleBindingDeleted, returnErr)
	return returnErr
}

var timeNow = func() time.Time {
	return time.Now()
}

func (c *crtbHandler) updateStatus(crtb *v3.ClusterRoleTemplateBinding, localConditions []metav1.Condition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crtbFromCluster, err := c.crtbCache.Get(crtb.Namespace, crtb.Name)
		if err != nil {
			return err
		}
		if status.CompareConditions(crtbFromCluster.Status.LocalConditions, localConditions) {
			return nil
		}

		crtbFromCluster.Status.SummaryLocal = status.SummaryCompleted
		if crtbFromCluster.Status.SummaryRemote == status.SummaryCompleted {
			crtbFromCluster.Status.Summary = status.SummaryCompleted
		}
		for _, c := range localConditions {
			if c.Status != metav1.ConditionTrue {
				crtbFromCluster.Status.Summary = status.SummaryError
				crtbFromCluster.Status.SummaryLocal = status.SummaryError
				break
			}
		}

		crtbFromCluster.Status.LastUpdateTime = timeNow().Format(time.RFC3339)
		crtbFromCluster.Status.ObservedGenerationLocal = crtb.ObjectMeta.Generation
		crtbFromCluster.Status.LocalConditions = localConditions
		_, err = c.crtbClient.UpdateStatus(crtbFromCluster)
		if err != nil {
			return err
		}

		return nil
	})
}
