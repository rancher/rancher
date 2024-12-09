package roletemplates

import (
	"errors"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/status"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	subjectExists      = "SubjectExists"
	failedToCreateUser = "FailedToCreateUser"
	failedToGetUser    = "FailedToGetUser"
	crtbHasNoSubject   = "CRTBHasNoSubject"
)

type crtbHandler struct {
	s              *status.Status
	userMGR        user.Manager
	userController mgmtv3.UserController
	rtController   mgmtv3.RoleTemplateController
	crController   crbacv1.ClusterRoleController
	crbController  crbacv1.ClusterRoleBindingController
}

func newCRTBHandler(management *config.ManagementContext) *crtbHandler {
	return &crtbHandler{
		s:              status.NewStatus(),
		userMGR:        management.UserManager,
		userController: management.Wrangler.Mgmt.User(),
		rtController:   management.Wrangler.Mgmt.RoleTemplate(),
		crController:   management.Wrangler.RBAC.ClusterRole(),
		crbController:  management.Wrangler.RBAC.ClusterRoleBinding(),
	}
}

// OnChange syncs the required resources used to implement a CRTB. It does the following:
//   - Create the specified user if it doesn't already exist.
//   - Create the membership bindings to give access to the cluster.
//   - Create a binding to the project and cluster management role if it exists.
//
// TODO add statuses
func (c *crtbHandler) OnChange(_ string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	var localConditions []metav1.Condition

	// Create user
	crtb, err := c.reconcileSubject(crtb, &localConditions)
	if err != nil {
		return nil, err
	}

	rt, err := c.rtController.Get(crtb.RoleTemplateName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Create membership binding
	if _, err := createOrUpdateMembershipBinding(crtb, rt, c.crbController); err != nil {
		return nil, err
	}

	ownerLabel := rbac.CreateCRTBOwnerLabel(crtb.Name)

	desiredCRBs, err := c.getDesiredClusterRoleBindings(crtb, ownerLabel)
	if err != nil {
		return nil, err
	}

	currentCRBs, err := c.crbController.List(metav1.ListOptions{LabelSelector: ownerLabel})
	if err != nil {
		return nil, err
	}

	err = c.reconcileBindings(desiredCRBs, currentCRBs.Items)

	return crtb, err
}

// getDesiredClusterRoleBindings checks for project and cluster management roles, and if they exist, builds and returns the needed ClusterRoleBindings
func (c *crtbHandler) getDesiredClusterRoleBindings(crtb *v3.ClusterRoleTemplateBinding, ownerLabel string) (map[string]*rbacv1.ClusterRoleBinding, error) {
	desiredCRBs := map[string]*rbacv1.ClusterRoleBinding{}
	// Check if there is a project management role to bind to
	projectMagementRoleName := rbac.ProjectManagementPlaneClusterRoleNameFor(crtb.RoleTemplateName)
	cr, err := c.crController.Get(projectMagementRoleName, metav1.GetOptions{})
	if err == nil && cr != nil {
		crb, err := rbac.BuildClusterRoleBindingFromRTB(crtb, ownerLabel, projectMagementRoleName)
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
		crb, err := rbac.BuildClusterRoleBindingFromRTB(crtb, ownerLabel, clusterManagementRoleName)
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
func (c *crtbHandler) reconcileBindings(desiredCRBs map[string]*rbacv1.ClusterRoleBinding, currentCRBs []rbacv1.ClusterRoleBinding) error {
	for _, currentCRB := range currentCRBs {
		if crb, ok := desiredCRBs[currentCRB.Name]; ok {
			if rbac.AreClusterRoleBindingContentsSame(&currentCRB, crb) {
				// If the cluster role binding already exists with the right contents, we can skip creating it.
				delete(desiredCRBs, crb.Name)
				continue
			}
		}
		// If the CRB is not a member of the desired CRBs or has different contents, delete it.
		if err := c.crbController.Delete(currentCRB.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	// For any CRBs that don't exist, create them
	for _, crb := range desiredCRBs {
		if _, err := c.crbController.Create(crb); err != nil {
			return err
		}
	}

	return nil
}

// OnRemove deletes Cluster Role Bindings that are owned by the CRTB. It also removes the membership binding if no other CRTBs give membership access.
func (c *crtbHandler) OnRemove(_ string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	returnErr := deleteMembershipBinding(crtb, c.crbController)

	ownerLabel := rbac.CreateCRTBOwnerLabel(crtb.Name)
	currentCRBs, err := c.crbController.List(metav1.ListOptions{LabelSelector: ownerLabel})
	if err != nil {
		return nil, errors.Join(returnErr, err)
	}

	for _, crb := range currentCRBs.Items {
		errors.Join(returnErr, c.crbController.Delete(crb.Name, &metav1.DeleteOptions{}))
	}

	return crtb, returnErr
}

// reconcileSubject ensures that the user referenced by the role template binding exists
func (c *crtbHandler) reconcileSubject(binding *v3.ClusterRoleTemplateBinding, localConditions *[]metav1.Condition) (*v3.ClusterRoleTemplateBinding, error) {
	condition := metav1.Condition{Type: subjectExists}
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

	return binding, nil
}
