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
//   - Create a binding to the project management role if it exists.
//   - Create a binding to the cluster management role if it exists.
//
// TODO add statuses
func (c *crtbHandler) OnChange(_ string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	var localConditions []metav1.Condition

	// Create user
	crtb, err := c.reconcileSubject(crtb, &localConditions)
	if err != nil {
		return nil, err
	}

	// Create membership binding
	if err := createOrUpdateMembershipBinding(crtb, c.rtController, c.crbController); err != nil {
		return nil, err
	}

	rtName := crtb.RoleTemplateName
	crbsNeeded := map[*rbacv1.ClusterRoleBinding]bool{}

	// Check if there is a project management role to bind to
	projectMagementRoleName := rbac.ProjectManagementPlaneClusterRoleNameFor(rtName)
	cr, err := c.crController.Get(projectMagementRoleName, metav1.GetOptions{})
	if err == nil && cr != nil {
		crb, err := rbac.BuildClusterRoleBindingFromCRTB(crtb, projectMagementRoleName)
		if err != nil {
			return nil, err
		}
		crbsNeeded[crb] = true
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	// Check if there is a cluster management role to bind to
	clusterManagementRoleName := rbac.ClusterManagementPlaneClusterRoleNameFor(rtName)
	cr, err = c.crController.Get(clusterManagementRoleName, metav1.GetOptions{})
	if err != nil && cr != nil {
		crb, err := rbac.BuildClusterRoleBindingFromCRTB(crtb, clusterManagementRoleName)
		if err != nil {
			return nil, err
		}
		crbsNeeded[crb] = true
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	ownerLabel := rbac.CreateCRTBOwnerLabel(crtb.Name)
	currentCRBs, err := c.crbController.List(metav1.ListOptions{LabelSelector: ownerLabel})
	if err != nil {
		return nil, err
	}

	// Find if CRBs already exists
currentLoop:
	for _, currentCRB := range currentCRBs.Items {
		for desiredCRB := range crbsNeeded {
			if rbac.AreClusterRoleBindingsSame(&currentCRB, desiredCRB) {
				crbsNeeded[desiredCRB] = false
				continue currentLoop
			}
			if err := c.crbController.Delete(currentCRB.Name, &metav1.DeleteOptions{}); err != nil {
				return nil, err
			}
		}
	}

	// For any CRBs that don't exist, create them
	for crb, needed := range crbsNeeded {
		if needed {
			if _, err := c.crbController.Create(crb); err != nil {
				return nil, err
			}
		}
	}

	return crtb, nil
}

func (c *crtbHandler) OnRemove(_ string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	returnErr := deleteMembershipBinding(crtb, c.crbController)

	ownerLabel := rbac.CreateCRTBOwnerLabel(crtb.Name)
	currentCRBs, err := c.crbController.List(metav1.ListOptions{LabelSelector: ownerLabel})
	if err != nil {
		return nil, err
	}

	for _, crb := range currentCRBs.Items {
		errors.Join(returnErr, c.crbController.Delete(crb.Name, &metav1.DeleteOptions{}))
	}

	return crtb, nil
}

// reconcileSubject ensures that the user referenced by the role template binding exists
func (c *crtbHandler) reconcileSubject(binding *v3.ClusterRoleTemplateBinding, localConditions *[]metav1.Condition) (*v3.ClusterRoleTemplateBinding, error) {
	condition := metav1.Condition{Type: subjectExists}
	if binding.GroupName != "" || binding.GroupPrincipalName != "" || (binding.UserPrincipalName != "" && binding.UserName != "") {
		c.s.AddCondition(localConditions, condition, subjectExists, nil)
		return binding, nil
	}

	if binding.UserPrincipalName != "" && binding.UserName == "" {
		displayName := binding.Annotations["auth.cattle.io/principal-display-name"]
		user, err := c.userMGR.EnsureUser(binding.UserPrincipalName, displayName)
		if err != nil {
			c.s.AddCondition(localConditions, condition, failedToCreateUser, err)
			return binding, err
		}

		binding.UserName = user.Name
		c.s.AddCondition(localConditions, condition, subjectExists, nil)
		return binding, nil
	}

	if binding.UserPrincipalName == "" && binding.UserName != "" {
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
		return binding, nil
	}

	c.s.AddCondition(localConditions, condition, crtbHasNoSubject, fmt.Errorf("CRTB has no subject"))

	return nil, fmt.Errorf("ClusterRoleTemplateBinding %v has no subject", binding.Name)
}
