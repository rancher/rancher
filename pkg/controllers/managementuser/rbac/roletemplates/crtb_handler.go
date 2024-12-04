package roletemplates

import (
	"errors"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	rbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type crtbHandler struct {
	impersonationHandler *impersonationHandler
	crbClient            rbacv1.ClusterRoleBindingController
}

func newCRTBHandler(uc *config.UserContext) *crtbHandler {
	return &crtbHandler{
		impersonationHandler: &impersonationHandler{
			userContext: uc,
			crClient:    uc.RBACw.ClusterRole(),
			crtbClient:  uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
			prtbClient:  uc.Management.Wrangler.Mgmt.ProjectRoleTemplateBinding(),
		},
		crbClient: uc.RBACw.ClusterRoleBinding(),
	}
}

// OnChange ensures that the correct ClusterRoleBinding exists for the ClusterRoleTemplateBinding
func (c *crtbHandler) OnChange(key string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if crtb == nil || crtb.DeletionTimestamp != nil {
		return nil, nil
	}

	ownerLabel := rbac.CreateCRTBOwnerLabel(crtb.Name)
	crb, err := rbac.BuildClusterRoleBindingFromRTB(crtb, ownerLabel, crtb.RoleTemplateName)
	if err != nil {
		return nil, err
	}

	currentCRBs, err := c.crbClient.List(metav1.ListOptions{LabelSelector: ownerLabel})
	if err != nil {
		return nil, err
	}

	// Find if there is a CRB that already exists and delete all excess CRBs
	var matchingCRB *v1.ClusterRoleBinding
	for _, currentCRB := range currentCRBs.Items {
		if rbac.AreClusterRoleBindingContentsSame(crb, &currentCRB) && matchingCRB == nil {
			matchingCRB = &currentCRB
			continue
		}
		if err := c.crbClient.Delete(currentCRB.Name, &metav1.DeleteOptions{}); err != nil {
			return nil, err
		}
	}

	// If we didn't find an existing CRB, create it
	if matchingCRB == nil {
		if _, err := c.crbClient.Create(crb); err != nil {
			return nil, err
		}
	}

	// Ensure a service account impersonator exists on the cluster
	if crtb.UserName != "" {
		if err := c.impersonationHandler.ensureServiceAccountImpersonator(crtb.UserName); err != nil {
			return nil, fmt.Errorf("error deleting service account impersonator: %w", err)
		}
	}

	return crtb, nil
}

// OnRemove deletes all ClusterRoleBindings owned by the ClusterRoleTemplateBinding
func (c *crtbHandler) OnRemove(key string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	lo := metav1.ListOptions{
		LabelSelector: rbac.CreateCRTBOwnerLabel(crtb.Name),
	}

	crbs, err := c.crbClient.List(lo)
	if err != nil {
		return nil, err
	}

	var returnError error
	for _, crb := range crbs.Items {
		err = c.crbClient.Delete(crb.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			returnError = errors.Join(returnError, err)
		}
	}

	if crtb.UserName != "" {
		if err = c.impersonationHandler.deleteServiceAccountImpersonator(crtb.UserName); err != nil {
			return nil, err
		}
	}
	return nil, returnError
}
