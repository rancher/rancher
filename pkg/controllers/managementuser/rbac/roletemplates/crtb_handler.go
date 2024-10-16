package roletemplates

import (
	"errors"
	"fmt"
	"reflect"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	rbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	crtbOwnerLabel = "authz.cluster.cattle.io/crtb-owner"
)

type crtbHandler struct {
	impersonationHandler *impersonationHandler
	crbClient            rbacv1.ClusterRoleBindingController
}

func newCRTBHandler(uc *config.UserContext) *crtbHandler {
	return &crtbHandler{
		impersonationHandler: &impersonationHandler{
			userContext: uc,
			crClient:    uc.Management.Wrangler.RBAC.ClusterRole(),
			// TODO I don't think these crtb and prtb clients get the local cluster which is where prtbs/crtbs live
			crtbClient: uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
			prtbClient: uc.Management.Wrangler.Mgmt.ProjectRoleTemplateBinding(),
		},
		crbClient: uc.Management.Wrangler.RBAC.ClusterRoleBinding(),
	}
}

// OnChange ensures that the correct ClusterRoleBinding exists for the ClusterRoleTemplateBinding
func (c *crtbHandler) OnChange(key string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	crb, err := buildClusterRoleBinding(crtb)
	if err != nil {
		return nil, err
	}

	ownerLabel := createCRTBOwnerLabel(crtb.Name)
	currentCRBs, err := c.crbClient.List(metav1.ListOptions{LabelSelector: ownerLabel})
	if err != nil {
		return nil, err
	}

	// Find if there is a CRB that already exists and delete all excess CRBs
	var matchingCRB *v1.ClusterRoleBinding
	for _, currentCRB := range currentCRBs.Items {
		if areClusterRoleBindingsSame(crb, &currentCRB) && matchingCRB == nil {
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
		LabelSelector: createCRTBOwnerLabel(crtb.Name),
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

// buildClusterRoleBinding returns the ClusterRoleBinding needed for a CRTB
func buildClusterRoleBinding(crtb *v3.ClusterRoleTemplateBinding) (*v1.ClusterRoleBinding, error) {
	ownerLabel := createCRTBOwnerLabel(crtb.Name)
	roleRef := v1.RoleRef{
		Kind: "ClusterRole",
		Name: aggregatedClusterRoleNameFor(crtb.RoleTemplateName),
	}

	subject, err := rbac.BuildSubjectFromRTB(crtb)
	if err != nil {
		return nil, err
	}

	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "crb-",
			Labels:       map[string]string{ownerLabel: "true"},
		},
		RoleRef:  roleRef,
		Subjects: []v1.Subject{subject},
	}, nil

}

// areRoleBindingsSame compares the Subjects and RoleRef fields of two Cluster Role Bindings.
func areClusterRoleBindingsSame(crb1, crb2 *v1.ClusterRoleBinding) bool {
	return reflect.DeepEqual(crb1.Subjects, crb2.Subjects) &&
		reflect.DeepEqual(crb1.RoleRef, crb2.RoleRef)
}

// createCRTBOwnerLabel creates an owner label given a CRTB name
func createCRTBOwnerLabel(crtbName string) string {
	return name.SafeConcatName(crtbOwnerLabel, crtbName)
}
