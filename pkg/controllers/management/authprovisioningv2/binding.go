package authprovisioningv2

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/pkg/name"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *handler) OnCRBT(key string, crbt *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if crbt == nil || (crbt.UserName == "" && crbt.GroupName == "") || crbt.RoleTemplateName == "" {
		return nil, nil
	}

	rt, err := h.roleTemplates.Get(crbt.RoleTemplateName)
	if err != nil {
		return crbt, err
	}

	indexed, err := h.isClusterIndexed(rt)
	if err != nil || !indexed {
		return crbt, err
	}

	clusters, err := h.clusters.GetByIndex(byClusterName, crbt.ClusterName)
	if err != nil {
		return nil, err
	}

	if len(clusters) == 0 {
		return crbt, nil
	}

	cluster := clusters[0]

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.SafeConcatName(roleTemplateRoleName(crbt.RoleTemplateName, cluster.Name), crbt.RoleTemplateName),
			Namespace: cluster.Namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     roleTemplateRoleName(crbt.RoleTemplateName, cluster.Name),
		},
	}
	if crbt.UserName != "" {
		roleBinding.Subjects = append(roleBinding.Subjects, rbacv1.Subject{
			Kind:     "User",
			APIGroup: rbacv1.GroupName,
			Name:     crbt.UserName,
		})
	}
	if crbt.GroupName != "" {
		roleBinding.Subjects = append(roleBinding.Subjects, rbacv1.Subject{
			Kind:     "Group",
			APIGroup: rbacv1.GroupName,
			Name:     crbt.GroupName,
		})
	}

	return crbt, h.roleBindingApply.
		WithOwner(crbt).
		WithListerNamespace(cluster.Namespace).
		WithSetOwnerReference(true, true).
		ApplyObjects(roleBinding)
}

func (h *handler) isClusterIndexed(rt *v3.RoleTemplate) (bool, error) {
	for _, rule := range rt.Rules {
		if len(rule.NonResourceURLs) > 0 || len(rule.ResourceNames) > 0 {
			continue
		}
		matches, err := h.getMatchingClusterIndexedTypes(rule)
		if err != nil {
			return false, err
		}
		if len(matches) > 0 {
			return true, nil
		}
	}
	return false, nil
}
