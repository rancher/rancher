package roletemplates

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TODO: Use safeconcat for names
// TODO: Need to create/delete impersonator account on downstream cluster. More info: https://github.com/rancher/rancher/pull/33592

const (
	rtbOwnerLabel = "authz.cluster.cattle.io/rtb-owner"
)

type crtbLifecycle struct {
	crbClient typesrbacv1.ClusterRoleBindingInterface
}

func newCRTBLifecycle(uc *config.UserContext) *crtbLifecycle {
	return &crtbLifecycle{
		crbClient: uc.RBAC.ClusterRoleBindings(""),
	}
}

// Create creates a single ClusterRoleBinding to the aggregated cluster role created by the RoleTemplate
func (c *crtbLifecycle) Create(crtb *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	crb, err := c.buildClusterRoleBinding(crtb)
	if err != nil {
		return nil, err
	}
	_, err = c.crbClient.Create(crb)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	return crtb, nil
}

func (c *crtbLifecycle) Updated(crtb *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	return nil, nil
}

func (c *crtbLifecycle) Remove(crtb *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	// if the CRB name is not known deterministically, we may have to use a cache to list by label
	return nil, nil
}

func (c *crtbLifecycle) buildClusterRoleBinding(crtb *v3.ClusterRoleTemplateBinding) (*v1.ClusterRoleBinding, error) {
	subject, err := rbac.BuildSubjectFromRTB(crtb)
	if err != nil {
		return nil, err
	}

	roleRef := v1.RoleRef{
		Kind: "ClusterRole",
		Name: crtb.RoleTemplateName + "-aggregator",
	}

	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			// TODO: Could use a deterministic name like crtb.Name-crtb.Username
			// but crtb.Username may not be filled (could be a principal or a group)
			GenerateName: "crb-",
			// TODO: Not sure if we even need a label or annotation
			Labels: map[string]string{rtbOwnerLabel: crtb.Name},
		},
		RoleRef:  roleRef,
		Subjects: []v1.Subject{subject},
	}, nil
}
