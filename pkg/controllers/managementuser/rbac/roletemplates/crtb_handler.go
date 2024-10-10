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

// TODO: Need to create/delete impersonator account on downstream cluster. More info: https://github.com/rancher/rancher/pull/33592

const (
	crtbOwnerLabel = "authz.cluster.cattle.io/crtb-owner"
)

type crtbHandler struct {
	crbClient rbacv1.ClusterRoleBindingController
}

func newCRTBHandler(uc *config.UserContext) *crtbHandler {
	return &crtbHandler{
		crbClient: uc.Management.Wrangler.RBAC.ClusterRoleBinding(),
	}
}

// OnChange ensures that the correct ClusterRoleBinding exists for the ClusterRoleTemplateBinding
func (c *crtbHandler) OnChange(key string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	ownerLabel := createCRTBOwnerLabel(crtb.Name)
	roleRef := v1.RoleRef{
		Kind: "ClusterRole",
		Name: addAggregatorSuffix(crtb.RoleTemplateName),
	}

	subject, err := rbac.BuildSubjectFromRTB(crtb)
	if err != nil {
		return nil, err
	}

	crb := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "crb-",
			Labels:       map[string]string{ownerLabel: "true"},
		},
		RoleRef:  roleRef,
		Subjects: []v1.Subject{subject},
	}

	// function for getting the CRB
	getCRB := func(_ *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
		lo := metav1.ListOptions{
			LabelSelector: createCRTBOwnerLabel(crtb.Name),
		}
		crbs, err := c.crbClient.List(lo)
		if err != nil {
			return nil, err
		}
		if len(crbs.Items) == 0 {
			return nil, nil
		}
		if len(crbs.Items) > 1 {
			return nil, fmt.Errorf("got multiple CRBs for this CRTB when there should only be 1")
		}
		return &crbs.Items[0], nil
	}

	// function for comparing ClusterRoleBindings
	areClusterRoleBindingsSame := func(currentCRB, wantedCRB *v1.ClusterRoleBinding) (bool, *v1.ClusterRoleBinding) {
		same := true
		if !reflect.DeepEqual(currentCRB.Subjects, wantedCRB.Subjects) {
			same = false
			currentCRB.Subjects = wantedCRB.Subjects
		}
		if currentCRB.Labels[ownerLabel] != wantedCRB.Labels[ownerLabel] {
			currentCRB.Labels[ownerLabel] = wantedCRB.Labels[ownerLabel]
		}
		return same, currentCRB
	}

	if err = createOrUpdateResource(crb, c.crbClient, getCRB, areClusterRoleBindingsSame); err != nil {
		return crtb, err
	}

	return crtb, nil
}

// OnRemove deletes all ClusterRoleBindings owned by the ClusterRoleTemplateBinding
func (c *crtbHandler) OnRemove(key string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	lo := metav1.ListOptions{
		LabelSelector: createCRTBOwnerLabel(crtb.Name),
	}

	// this returns a list but it should always be 1 item
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
	return nil, returnError
}

func createCRTBOwnerLabel(s string) string {
	return name.SafeConcatName(crtbOwnerLabel, s)
}
