package authprovisioningv2

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/pkg/name"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *handler) OnCRTB(key string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if crtb == nil || (crtb.UserName == "" && crtb.GroupName == "") || crtb.RoleTemplateName == "" {
		return nil, nil
	}

	rt, err := h.roleTemplates.Get(crtb.RoleTemplateName)
	if err != nil {
		return crtb, err
	}

	indexed, err := h.isClusterIndexed(rt)
	if err != nil || !indexed {
		return crtb, err
	}

	clusters, err := h.clusters.GetByIndex(byClusterName, crtb.ClusterName)
	if err != nil {
		return nil, err
	}

	if len(clusters) == 0 {
		return crtb, nil
	}

	cluster := clusters[0]

	// The roleBinding name format: crt-<cluster name>-<roleTemplate name>-<crtb name>
	// Example: crt-cluster1-cluster-member-crtb-aaaaa
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.SafeConcatName(roleTemplateRoleName(crtb.RoleTemplateName, cluster.Name), crtb.Name),
			Namespace: cluster.Namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     roleTemplateRoleName(crtb.RoleTemplateName, cluster.Name),
		},
	}
	if crtb.UserName != "" {
		roleBinding.Subjects = append(roleBinding.Subjects, rbacv1.Subject{
			Kind:     "User",
			APIGroup: rbacv1.GroupName,
			Name:     crtb.UserName,
		})
	}
	if crtb.GroupName != "" {
		roleBinding.Subjects = append(roleBinding.Subjects, rbacv1.Subject{
			Kind:     "Group",
			APIGroup: rbacv1.GroupName,
			Name:     crtb.GroupName,
		})
	}

	return crtb, h.roleBindingApply.
		WithOwner(crtb).
		WithListerNamespace(cluster.Namespace).
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
