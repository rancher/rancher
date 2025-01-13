package roletemplates

import (
	"errors"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type prtbHandler struct {
	userMGR        user.Manager
	userController mgmtv3.UserController
	rtController   mgmtv3.RoleTemplateController
	crController   crbacv1.ClusterRoleController
	crbController  crbacv1.ClusterRoleBindingController
}

func newPRTBHandler(management *config.ManagementContext) *prtbHandler {
	return &prtbHandler{
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
	if prtb == nil {
		return nil, nil
	}
	prtb, err := p.reconcileSubject(prtb)
	if err != nil {
		return nil, err
	}

	if err := p.reconcileMembershipBindings(prtb); err != nil {
		return nil, err
	}

	return prtb, p.reconcileBindings(prtb)
}

// OnRemove deletes Cluster Role Bindings that are owned by the PRTB. It also removes the membership binding if no other PRTBs give membership access.
func (p *prtbHandler) OnRemove(_ string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	if prtb == nil {
		return nil, nil
	}

	returnErr := deleteMembershipBinding(prtb, p.crbController)

	currentCRBs, err := p.crbController.List(metav1.ListOptions{LabelSelector: rbac.GetPRTBOwnerLabel(prtb.Name)})
	if err != nil {
		return nil, errors.Join(returnErr, err)
	}

	for _, crb := range currentCRBs.Items {
		returnErr = errors.Join(returnErr, rbac.DeleteResource(crb.Name, p.crbController))
	}

	return prtb, returnErr
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
			return binding, err
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

// reconcileMembershipBindings ensures that the user is given the right membership binding to the project.
func (p *prtbHandler) reconcileMembershipBindings(prtb *v3.ProjectRoleTemplateBinding) error {
	rt, err := p.rtController.Get(prtb.RoleTemplateName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	_, err = createOrUpdateMembershipBinding(prtb, rt, p.crbController)
	return err
}

// reconcileBindings ensures the right CRB exists for the project management plane role. It deletes any additional unwanted CRBs.
func (p *prtbHandler) reconcileBindings(prtb *v3.ProjectRoleTemplateBinding) error {
	var crb *rbacv1.ClusterRoleBinding

	projectManagementRoleName := rbac.ProjectManagementPlaneClusterRoleNameFor(prtb.RoleTemplateName)

	// If there is no project management plane role, no need to create a binding for it
	_, err := p.crController.Get(projectManagementRoleName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	crb, err = rbac.BuildClusterRoleBindingFromRTB(prtb, projectManagementRoleName)
	if err != nil {
		return err
	}

	currentCRBs, err := p.crbController.List(metav1.ListOptions{LabelSelector: rbac.GetPRTBOwnerLabel(prtb.Name)})
	if err != nil {
		return err
	}

	var prtbHasBinding bool
	for _, currentCRB := range currentCRBs.Items {
		if rbac.AreClusterRoleBindingContentsSame(&currentCRB, crb) {
			prtbHasBinding = true
			continue
		}
		if err := rbac.DeleteResource(currentCRB.Name, p.crbController); err != nil {
			return err
		}
	}

	if !prtbHasBinding {
		if _, err := p.crbController.Create(crb); err != nil {
			return err
		}
	}
	return nil
}
