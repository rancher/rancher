package authprovisioningv2

import (
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *handler) OnCRTB(key string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if crtb == nil || crtb.DeletionTimestamp != nil || (crtb.UserName == "" && crtb.GroupName == "") || crtb.RoleTemplateName == "" {
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
		// When no provisioning cluster is found, enqueue the CRTB to wait for
		// the provisioning cluster to be created. If we don't try again
		// permissions for the provisioning objects won't be created until an
		// update to the CRTB happens again.
		logrus.Debugf("[auth-prov-v2-crtb] No provisioning cluster found for cluster %v, enqueuing CRTB %v ", crtb.ClusterName, crtb.Name)
		h.clusterRoleTemplateBindingController.EnqueueAfter(crtb.Namespace, crtb.Name, 5*time.Second)
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

	subject, err := rbac.BuildSubjectFromRTB(crtb)
	if err != nil {
		return nil, err
	}

	roleBinding.Subjects = []rbacv1.Subject{subject}

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
