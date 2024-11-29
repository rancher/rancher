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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type prtbHandler struct {
	s              *status.Status
	userMGR        user.Manager
	userController mgmtv3.UserController
	rtController   mgmtv3.RoleTemplateController
	crController   crbacv1.ClusterRoleController
	crbController  crbacv1.ClusterRoleBindingController
}

func newPRTBHandler(management *config.ManagementContext) *prtbHandler {
	return &prtbHandler{
		s:              status.NewStatus(),
		userMGR:        management.UserManager,
		userController: management.Wrangler.Mgmt.User(),
		rtController:   management.Wrangler.Mgmt.RoleTemplate(),
		crController:   management.Wrangler.RBAC.ClusterRole(),
		crbController:  management.Wrangler.RBAC.ClusterRoleBinding(),
	}
}

// OnChange syncs the required resources used to implement a PRTB. It does the following:
//   - Create the specified user if it doesn't already exist.
//   - Create the membership bindings to give access to the cluster.
//   - Create a binding to the project management role if it exists.
func (p *prtbHandler) OnChange(_ string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	// Create user
	prtb, err := p.reconcileSubject(prtb)
	if err != nil {
		return nil, err
	}

	rt, err := p.rtController.Get(prtb.RoleTemplateName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if err := createOrUpdateMembershipBinding(prtb, rt, p.crbController); err != nil {
		return nil, err
	}

	var crb *rbacv1.ClusterRoleBinding
	ownerLabel := rbac.CreatePRTBOwnerLabel(prtb.Name)

	projectManagementRoleName := rbac.ProjectManagementPlaneClusterRoleNameFor(prtb.RoleTemplateName)
	cr, err := p.crController.Get(projectManagementRoleName, metav1.GetOptions{})
	if err == nil && cr != nil {
		crb, err = rbac.BuildClusterRoleBindingFromRTB(prtb, ownerLabel, projectManagementRoleName)
		if err != nil {
			return nil, err
		}
	}

	currentCRBs, err := p.crbController.List(metav1.ListOptions{LabelSelector: ownerLabel})
	if err != nil {
		return nil, err
	}

	var prtbHasBinding bool
	for _, currentCRB := range currentCRBs.Items {
		if rbac.AreClusterRoleBindingsSame(&currentCRB, crb) {
			prtbHasBinding = true
			continue
		}
		if err := p.crbController.Delete(currentCRB.Name, &metav1.DeleteOptions{}); err != nil {
			return nil, err
		}
	}

	if !prtbHasBinding {
		if _, err := p.crbController.Create(crb); err != nil {
			return nil, err
		}
	}

	return prtb, nil
}

// OnRemove deletes Cluster Role Bindings that are owned by the PRTB. It also removes the membership binding if no other PRTBs give membership access.
func (p *prtbHandler) OnRemove(_ string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	returnErr := deleteMembershipBinding(prtb, p.crbController)

	ownerLabel := rbac.CreatePRTBOwnerLabel(prtb.Name)
	currentCRBs, err := p.crbController.List(metav1.ListOptions{LabelSelector: ownerLabel})
	if err != nil {
		return nil, errors.Join(returnErr, err)
	}

	for _, crb := range currentCRBs.Items {
		errors.Join(returnErr, p.crbController.Delete(crb.Name, &metav1.DeleteOptions{}))
	}

	return prtb, returnErr
}

// reconcileSubject ensures that both the UserPrincipalName and UserName are set, creating the user if UserPrincipalName is set but not UserName.
func (p *prtbHandler) reconcileSubject(binding *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	if binding.GroupName != "" || binding.GroupPrincipalName != "" || (binding.UserPrincipalName != "" && binding.UserName != "") {
		return binding, nil
	}

	if binding.UserPrincipalName != "" && binding.UserName == "" {
		displayName := binding.Annotations["auth.cattle.io/principal-display-name"]
		user, err := p.userMGR.EnsureUser(binding.UserPrincipalName, displayName)
		if err != nil {
			return binding, err
		}

		binding.UserName = user.Name
		return binding, nil
	}

	if binding.UserPrincipalName == "" && binding.UserName != "" {
		u, err := p.userController.Get(binding.UserName, metav1.GetOptions{})
		if err != nil {
			return binding, err
		}
		for _, p := range u.PrincipalIDs {
			if strings.HasSuffix(p, binding.UserName) {
				binding.UserPrincipalName = p
				break
			}
		}
		return binding, nil
	}

	return nil, fmt.Errorf("binding %v has no subject", binding.Name)
}
