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
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type crtbHandler struct {
	s              *status.Status
	userMGR        user.Manager
	userController mgmtv3.UserController
	rtController   mgmtv3.RoleTemplateController
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
	if crtb == nil {
		return nil, nil
	}

	var localConditions []metav1.Condition
	crtb, err := c.reconcileSubject(crtb, &localConditions)
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
		return nil, fmt.Errorf("ClusterRoleTemplateBinding %v has no subject", binding.Name)
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
	if _, err := createOrUpdateMembershipBinding(crtb, rt, c.crbController); err != nil {
		c.s.AddCondition(localCondition, condition, failedToCreateOrUpdateMembershipBinding, err)
		return err
	}

	c.s.AddCondition(localCondition, condition, membershipBindingExists, nil)
	return nil
}

// getDesiredClusterRoleBindings checks for project and cluster management roles, and if they exist, builds and returns the needed ClusterRoleBindings
func (c *crtbHandler) getDesiredClusterRoleBindings(crtb *v3.ClusterRoleTemplateBinding) (map[string]*rbacv1.ClusterRoleBinding, error) {
	desiredCRBs := map[string]*rbacv1.ClusterRoleBinding{}
	// Check if there is a project management role to bind to
	projectMagementRoleName := rbac.ProjectManagementPlaneClusterRoleNameFor(crtb.RoleTemplateName)
	cr, err := c.crController.Get(projectMagementRoleName, metav1.GetOptions{})
	if err == nil && cr != nil {
		crb, err := rbac.BuildClusterRoleBindingFromRTB(crtb, projectMagementRoleName)
		if err != nil {
			return nil, err
		}
		desiredCRBs[crb.Name] = crb
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	// Check if there is a cluster management role to bind to
	clusterManagementRoleName := rbac.ClusterManagementPlaneClusterRoleNameFor(crtb.RoleTemplateName)
	cr, err = c.crController.Get(clusterManagementRoleName, metav1.GetOptions{})
	if err != nil && cr != nil {
		crb, err := rbac.BuildClusterRoleBindingFromRTB(crtb, clusterManagementRoleName)
		if err != nil {
			return nil, err
		}
		desiredCRBs[crb.Name] = crb
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	return desiredCRBs, nil
}

// reconcileBindings takes a map of desired ClusterRoleBindings and a slice of already existing ClusterRoleBindings and ensures that all the desired CRBs exist.
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
	return nil
}

// OnRemove deletes Cluster Role Bindings that are owned by the CRTB and the membership binding if no other CRTBs give membership access.
func (c *crtbHandler) OnRemove(_ string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if crtb == nil {
		return nil, nil
	}

	condition := metav1.Condition{Type: clusterRoleTemplateBindingDelete}

	err := deleteMembershipBinding(crtb, c.crbController)
	c.s.AddCondition(&crtb.Status.LocalConditions, condition, clusterMembershipBindingDeleted, err)

	return crtb, errors.Join(err, c.removeClusterRoleBindings(crtb))
}

// removeClusterRoleBindings removes all cluster role bindings owned by the CRTB
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
