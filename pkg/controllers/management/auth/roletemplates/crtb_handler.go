package roletemplates

import (
	"errors"
	"fmt"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/status"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const (
	crtTokenReaderRole = "crt-token-reader"
)

type crtbHandler struct {
	s              *status.Status
	userMGR        user.Manager
	userController mgmtv3.UserController
	rtController   mgmtv3.RoleTemplateController
	rbController   wrbacv1.RoleBindingController
	rbCache        wrbacv1.RoleBindingCache
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
		rbCache:        management.Wrangler.RBAC.RoleBinding().Cache(),
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
			return binding, err
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
	rt, err := c.rtController.Get(crtb.RoleTemplateName, metav1.GetOptions{})
	if err != nil {
		c.s.AddCondition(localCondition, condition, failedToGetRoleTemplate, err)
		return err
	}

	// Create membership binding
	if err := createOrUpdateClusterMembershipBinding(crtb, rt, c.crbController); err != nil {
		c.s.AddCondition(localCondition, condition, failedToCreateOrUpdateMembershipBinding, err)
		return err
	}

	c.s.AddCondition(localCondition, condition, membershipBindingExists, nil)
	return nil
}

// reconcileBindings Ensures that all bindings required to provide access to the CRTB either exist or get created.
// It deletes any of the existing CRBs that are not needed.
func (c *crtbHandler) reconcileBindings(crtb *v3.ClusterRoleTemplateBinding, localConditions *[]metav1.Condition) error {
	condition := metav1.Condition{Type: reconcileBindings}
	desiredCRBs, err := c.getDesiredClusterRoleBindings(crtb)
	if err != nil {
		c.s.AddCondition(localConditions, condition, failedToGetDesiredClusterRoleBindings, err)
		return err
	}

	currentCRBs, err := c.crbController.List(metav1.ListOptions{LabelSelector: rbac.GetCRTBOwnerLabel(crtb.Name)})
	if err != nil {
		c.s.AddCondition(localConditions, condition, failedToListExistingClusterRoleBindings, err)
		return err
	}

	for _, currentCRB := range currentCRBs.Items {
		if crb, ok := desiredCRBs[currentCRB.Name]; ok {
			if rbac.AreClusterRoleBindingContentsSame(&currentCRB, crb) {
				// If the cluster role binding already exists with the right contents, we can skip creating it.
				delete(desiredCRBs, crb.Name)
				continue
			}
		}
		// If the CRB is not a member of the desired CRBs or has different contents, delete it.
		if err := c.crbController.Delete(currentCRB.Name, &metav1.DeleteOptions{}); err != nil {
			c.s.AddCondition(localConditions, condition, failedToDeleteClusterRoleBinding, err)
			return err
		}
	}

	// For any CRBs that don't exist, create them
	for _, crb := range desiredCRBs {
		if _, err := c.crbController.Create(crb); err != nil {
			c.s.AddCondition(localConditions, condition, failedToCreateClusterRoleBinding, err)
			return err
		}
	}

	c.s.AddCondition(localConditions, condition, bindingsExists, nil)

	// Reconcile the CRT token reader RoleBinding for aggregated mode
	return c.reconcileCRTTokenReaderRoleBinding(crtb)
}

// reconcileCRTTokenReaderRoleBinding ensures the crt-token-reader RoleBinding exists when the
// cluster management role grants CRT access, and removes it when it does not.
func (c *crtbHandler) reconcileCRTTokenReaderRoleBinding(crtb *v3.ClusterRoleTemplateBinding) error {
	clusterManagementRoleName := rbac.ClusterManagementPlaneClusterRoleNameFor(crtb.RoleTemplateName)
	cr, err := c.crController.Get(rbac.AggregatedClusterRoleNameFor(clusterManagementRoleName), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return c.removeCRTTokenReaderRoleBinding(crtb)
	}
	if err != nil {
		return err
	}

	if grantsCRTAccess(cr) {
		desired, err := c.buildCRTTokenReaderRoleBinding(crtb)
		if err != nil {
			return err
		}
		existing, err := c.rbCache.Get(desired.Namespace, desired.Name)
		if apierrors.IsNotFound(err) {
			_, err = c.rbController.Create(desired)
			return err
		}
		if err != nil {
			return err
		}
		if !rbac.AreRoleBindingContentsSame(existing, desired) {
			if err := c.rbController.Delete(existing.Namespace, existing.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			_, err = c.rbController.Create(desired)
			return err
		}
		return nil
	}
	return c.removeCRTTokenReaderRoleBinding(crtb)
}

// removeCRTTokenReaderRoleBinding removes the crt-token-reader RoleBinding owned by the CRTB if it exists.
func (c *crtbHandler) removeCRTTokenReaderRoleBinding(crtb *v3.ClusterRoleTemplateBinding) error {
	subject, err := rbac.BuildSubjectFromRTB(crtb)
	if err != nil {
		return err
	}
	name := rbac.NameForRoleBinding(crtb.Namespace, rbacv1.RoleRef{Kind: "Role", Name: crtTokenReaderRole}, subject)
	existing, err := c.rbCache.Get(crtb.Namespace, name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return c.rbController.Delete(existing.Namespace, existing.Name, &metav1.DeleteOptions{})
}

// grantsCRTAccess checks if a ClusterRole has rules that grant read or create access to
// clusterregistrationtokens. Read verbs (get, list, watch) and create are included because
// those principals legitimately need to read the token value. Pure write/delete verbs
// (update, patch, delete, deletecollection) do not warrant Secret read access.
func grantsCRTAccess(cr *rbacv1.ClusterRole) bool {
	readOrCreateVerbs := map[string]bool{
		"get":    true,
		"list":   true,
		"watch":  true,
		"create": true,
		"*":      true,
	}
	for _, rule := range cr.Rules {
		for _, resource := range rule.Resources {
			if resource != "clusterregistrationtokens" && resource != "*" {
				continue
			}
			for _, apiGroup := range rule.APIGroups {
				if apiGroup != "management.cattle.io" && apiGroup != "*" {
					continue
				}
				for _, verb := range rule.Verbs {
					if readOrCreateVerbs[verb] {
						return true
					}
				}
			}
		}
	}
	return false
}

// buildCRTTokenReaderRoleBinding creates a RoleBinding to the crt-token-reader Role.
func (c *crtbHandler) buildCRTTokenReaderRoleBinding(crtb *v3.ClusterRoleTemplateBinding) (*rbacv1.RoleBinding, error) {
	subject, err := rbac.BuildSubjectFromRTB(crtb)
	if err != nil {
		return nil, err
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbac.NameForRoleBinding(crtb.Namespace, rbacv1.RoleRef{Kind: "Role", Name: crtTokenReaderRole}, subject),
			Namespace: crtb.Namespace,
			Labels: map[string]string{
				rbac.GetCRTBOwnerLabel(crtb.Name): "true",
			},
		},
		Subjects: []rbacv1.Subject{subject},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     crtTokenReaderRole,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	return rb, nil
}

// getDesiredClusterRoleBindings checks for project and cluster management roles, and if they exist, builds and returns the needed ClusterRoleBindings
func (c *crtbHandler) getDesiredClusterRoleBindings(crtb *v3.ClusterRoleTemplateBinding) (map[string]*rbacv1.ClusterRoleBinding, error) {
	desiredCRBs := map[string]*rbacv1.ClusterRoleBinding{}
	// Check if there is a project management role to bind to
	projectManagementRoleName := rbac.ProjectManagementPlaneClusterRoleNameFor(crtb.RoleTemplateName)
	cr, err := c.crController.Get(rbac.AggregatedClusterRoleNameFor(projectManagementRoleName), metav1.GetOptions{})
	if err == nil && cr != nil {
		crb, err := rbac.BuildAggregatingClusterRoleBindingFromRTB(crtb, projectManagementRoleName)
		if err != nil {
			return nil, err
		}
		desiredCRBs[crb.Name] = crb
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	// Check if there is a cluster management role to bind to
	clusterManagementRoleName := rbac.ClusterManagementPlaneClusterRoleNameFor(crtb.RoleTemplateName)
	cr, err = c.crController.Get(rbac.AggregatedClusterRoleNameFor(clusterManagementRoleName), metav1.GetOptions{})
	if err == nil && cr != nil {
		crb, err := rbac.BuildAggregatingClusterRoleBindingFromRTB(crtb, clusterManagementRoleName)
		if err != nil {
			return nil, err
		}
		desiredCRBs[crb.Name] = crb
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	return desiredCRBs, nil
}

// OnRemove deletes Cluster Role Bindings that are owned by the CRTB and the membership binding if no other CRTBs give membership access.
func (c *crtbHandler) OnRemove(_ string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if crtb == nil {
		return nil, nil
	}

	condition := metav1.Condition{Type: clusterRoleTemplateBindingDelete}

	err := deleteClusterMembershipBinding(crtb, c.crbController)
	c.s.AddCondition(&crtb.Status.LocalConditions, condition, clusterMembershipBindingDeleted, err)

	err = removeAuthV2Permissions(crtb, c.rbController)
	c.s.AddCondition(&crtb.Status.LocalConditions, condition, authv2ProvisioningBindingDeleted, err)

	// Clean up the crt-token-reader RoleBinding. Log but don't fail — the CRTB is being deleted
	// and the RoleBinding has an owner label, so any failure here is not critical.
	if cleanupErr := c.removeCRTTokenReaderRoleBinding(crtb); cleanupErr != nil {
		logrus.Warnf("[roletemplates] failed to remove crt-token-reader RoleBinding for CRTB %s: %v", crtb.Name, cleanupErr)
	}

	return crtb, errors.Join(err, c.removeClusterRoleBindings(crtb))
}

// removeClusterRoleBindings removes all bindings owned by the CRTB
func (c *crtbHandler) removeClusterRoleBindings(crtb *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: removeClusterRoleBindings}
	currentCRBs, err := c.crbController.List(metav1.ListOptions{LabelSelector: rbac.GetCRTBOwnerLabel(crtb.Name)})
	if err != nil {
		c.s.AddCondition(&crtb.Status.LocalConditions, condition, failedToListExistingClusterRoleBindings, err)
		return err
	}

	var returnErr error
	for _, crb := range currentCRBs.Items {
		err = rbac.DeleteResource(crb.Name, c.crbController)
		if err != nil {
			c.s.AddCondition(&crtb.Status.LocalConditions, condition, failedToDeleteClusterRoleBinding, err)
			returnErr = errors.Join(returnErr, err)
		}
	}

	c.s.AddCondition(&crtb.Status.LocalConditions, condition, clusterRoleBindingDeleted, returnErr)
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
