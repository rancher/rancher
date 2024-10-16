package roletemplates

import (
	"errors"
	"fmt"
	"reflect"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	rbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	prtbOwnerLabel      = "authz.cluster.cattle.io/prtb-owner"
	projectIDAnnotation = "field.cattle.io/projectId"
)

type prtbHandler struct {
	impersonationHandler impersonationHandler
	crClient             rbacv1.ClusterRoleController
	rtClient             mgmtv3.RoleTemplateController
	nsClient             corev1.NamespaceController
	rbClient             rbacv1.RoleBindingClient
}

func newPRTBHandler(uc *config.UserContext) *prtbHandler {
	return &prtbHandler{
		impersonationHandler: impersonationHandler{
			userContext: uc,
			crClient:    uc.Management.Wrangler.RBAC.ClusterRole(),
			// TODO I don't think these crtb and prtb clients get the local cluster which is where prtbs/crtbs live
			crtbClient: uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
			prtbClient: uc.Management.Wrangler.Mgmt.ProjectRoleTemplateBinding(),
		},
		crClient: uc.Management.Wrangler.RBAC.ClusterRole(),
		rtClient: uc.Management.Wrangler.Mgmt.RoleTemplate(),
		nsClient: uc.Management.Wrangler.Core.Namespace(),
		rbClient: uc.Management.Wrangler.RBAC.RoleBinding(),
	}
}

// OnChange ensures a Role Binding exists in every project namespace to the RoleTemplate ClusterRole.
// If there are promoted rules, it creates a second Role Binding in each namaspace to the promoted ClusterRole
func (p *prtbHandler) OnChange(key string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	hasPromotedRule, err := p.doesRoleTemplateHavePromotedRules(prtb)
	if err != nil {
		return nil, err
	}

	ownerLabel := createPRTBOwnerLabel(prtb.Name)

	// TODO how to select namespaces
	namespaces, err := p.nsClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, n := range namespaces.Items {
		if !n.DeletionTimestamp.IsZero() {
			continue
		}

		rb, promotedRB, err := buildRoleBindings(prtb, n.Name, hasPromotedRule)
		if err != nil {
			return nil, err
		}

		// Check if any Role Bindings exist already
		currentRBs, err := p.rbClient.List(n.Name, metav1.ListOptions{LabelSelector: ownerLabel})
		if err != nil {
			return nil, err
		}

		var matchingRB, matchingPromotedRB *v1.RoleBinding
		var rbsToDelete []*v1.RoleBinding
		// Search for RoleBindings that are needed, all others should be removed
		for _, currentRB := range currentRBs.Items {
			if areRoleBindingsSame(&currentRB, rb) && matchingRB == nil {
				matchingRB = &currentRB
			} else if hasPromotedRule && areRoleBindingsSame(&currentRB, promotedRB) && matchingPromotedRB == nil {
				matchingPromotedRB = &currentRB
			} else {
				rbsToDelete = append(rbsToDelete, &currentRB)
			}
		}

		// Remove excess RBs
		for _, rbToDelete := range rbsToDelete {
			if err = p.rbClient.Delete(n.Name, rbToDelete.Name, &metav1.DeleteOptions{}); err != nil {
				return nil, err
			}
		}

		if matchingRB == nil {
			if _, err := p.rbClient.Create(rb); err != nil {
				return nil, err
			}
		}
		if hasPromotedRule && matchingPromotedRB == nil {
			if _, err := p.rbClient.Create(promotedRB); err != nil {
				return nil, err
			}
		}
	}

	// Ensure a service account impersonator exists on the cluster
	if prtb.UserName != "" {
		if err := p.impersonationHandler.ensureServiceAccountImpersonator(prtb.UserName); err != nil {
			return nil, fmt.Errorf("error deleting service account impersonator: %w", err)
		}
	}

	return prtb, nil
}

// OnRemove removes all Role Bindings in each project namespace made by the PRTB
func (p *prtbHandler) OnRemove(key string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	// TODO how to select namespaces
	namespaces, err := p.nsClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	lo := metav1.ListOptions{
		LabelSelector: createPRTBOwnerLabel(prtb.Name),
	}

	var returnError error
	for _, n := range namespaces.Items {
		rbs, err := p.rbClient.List(n.Name, lo)
		if err != nil {
			return nil, err
		}
		for _, crb := range rbs.Items {
			err = p.rbClient.Delete(n.Name, crb.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				returnError = errors.Join(returnError, err)
			}
		}
	}

	if prtb.UserName != "" {
		if err = p.impersonationHandler.deleteServiceAccountImpersonator(prtb.UserName); err != nil {
			return nil, err
		}
	}

	return nil, returnError
}

// doesRoleTemplateHavePromotedRules checks if the PRTB's RoleTemplate has a ClusterRole for promoted rules.
func (p *prtbHandler) doesRoleTemplateHavePromotedRules(prtb *v3.ProjectRoleTemplateBinding) (bool, error) {
	rt, err := p.rtClient.Get(prtb.RoleTemplateName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	_, err = p.crClient.Get(promotedClusterRoleNameFor(rt.Name), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	return !apierrors.IsNotFound(err), nil
}

// buildRoleBindings returns the Role Binding needed for a prtb in a specific namespace.
// If createPromotedRoleBindings is true, it also returns a Role Binding for the promoted Cluster Role.
func buildRoleBindings(prtb *v3.ProjectRoleTemplateBinding, ns string, createPromotedRoleBinding bool) (*v1.RoleBinding, *v1.RoleBinding, error) {
	ownerLabel := createPRTBOwnerLabel(prtb.Name)
	roleRef := v1.RoleRef{
		Kind: "Role",
		Name: aggregatedClusterRoleNameFor(prtb.RoleTemplateName),
	}

	subject, err := rbac.BuildSubjectFromRTB(prtb)
	if err != nil {
		return nil, nil, err
	}

	rb := &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "rb-",
			Namespace:    ns,
			Labels:       map[string]string{ownerLabel: "true"},
		},
		RoleRef:  roleRef,
		Subjects: []v1.Subject{subject},
	}

	var promotedRB *v1.RoleBinding
	if createPromotedRoleBinding {
		roleRef.Name = aggregatedClusterRoleNameFor(promotedClusterRoleNameFor(prtb.RoleTemplateName))
		promotedRB = &v1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "rb-",
				Namespace:    ns,
				Labels:       map[string]string{ownerLabel: "true"},
			},
			RoleRef:  roleRef,
			Subjects: []v1.Subject{subject},
		}
	}
	return rb, promotedRB, nil
}

// areRoleBindingsSame compares the Subjects and RoleRef fields of two Role Bindings.
func areRoleBindingsSame(rb1, rb2 *v1.RoleBinding) bool {
	return reflect.DeepEqual(rb1.Subjects, rb2.Subjects) &&
		reflect.DeepEqual(rb1.RoleRef, rb2.RoleRef)
}

// createPRTBOwnerLabel creates an owner label given a PRTB name.
func createPRTBOwnerLabel(prtbName string) string {
	return name.SafeConcatName(prtbOwnerLabel, prtbName)
}
