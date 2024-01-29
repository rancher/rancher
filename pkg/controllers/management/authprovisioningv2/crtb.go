package authprovisioningv2

import (
	"reflect"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v2/pkg/name"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const CRTBRoleBindingID = "auth-prov-v2-crtb-rolebinding"

// OnCRTB create a "membership" binding that gives the subject access to the the cluster custom resource itself
// along with granting any clusterIndexed permissions based on the roleTemplate
func (h *handler) OnCRTB(key string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if crtb == nil || crtb.DeletionTimestamp != nil || crtb.RoleTemplateName == "" || crtb.ClusterName == "" {
		return crtb, nil
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
		h.clusterRoleTemplateBindingController.EnqueueAfter(crtb.Namespace, crtb.Name, 10*time.Second)
		return crtb, nil
	}

	cluster := clusters[0]

	rt, err := h.roleTemplatesCache.Get(crtb.RoleTemplateName)
	if err != nil {
		return crtb, err
	}

	clusterIndexed, provisioningCluster, err := h.isClusterIndexed(rt)
	if err != nil {
		return crtb, err
	}

	subject, err := rbac.BuildSubjectFromRTB(crtb)
	if err != nil {
		return nil, err
	}

	hashedSubject := hashSubject(subject)

	var bindings []runtime.Object

	// Based on the rules in the roleTemplate we need to decide if an additional role binding
	// needs to be created in order to give the subject of the CRTB permissions to view
	// the provisioning cluster. The additional binding is added as oppose to editing the
	// the existing role because not all CRTBs grant clusterIndexed permissions.This will also
	// make troubleshooting permissions assigned to a user easier and makes tying a role
	// back to a roleTemplate easier.
	// clusterIndexed and provisioningCluster are both true: One binding is created for the original CRTB
	// because the roleTemplate already grants permissions to the provisioning cluster.
	// clusterIndexed and provisioningCluster are both false: One binding is created which grants
	// permissions to view the provisioning cluster as the roleTemplate does not grant any clusterIndexed items.
	// clusterIndexed is true and provisioningCluster is false: Two bindings are created, one
	// for the original CRTB and a 2nd one for viewing the provisioning cluster since the original
	// roleTemplate does not grant permission for the provisioning cluster.
	if clusterIndexed {
		// The roleBinding name format: crt-<cluster name>-<crtb name>-<hashed subject>
		// Example: crt-cluster1-creator-cluster-owner-blxbujr34t
		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name.SafeConcatName("crt", cluster.Name, crtb.Name, hashedSubject),
				Namespace:   cluster.Namespace,
				Annotations: map[string]string{clusterNameLabel: cluster.GetName(), clusterNamespaceLabel: cluster.GetNamespace()},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     roleTemplateRoleName(crtb.RoleTemplateName, cluster.Name),
			},
			Subjects: []rbacv1.Subject{subject},
		}
		bindings = append(bindings, roleBinding)
	}

	if !provisioningCluster {
		// The roleBinding name format: r-cluster-<cluster name>-view-<crtb name>-<hashed subject>
		// Example: r-cluster1-view-crtb-foo-wn5d5n7udr
		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name.SafeConcatName(clusterViewName(cluster), crtb.Name, hashedSubject),
				Namespace:   cluster.Namespace,
				Annotations: map[string]string{clusterNameLabel: cluster.GetName(), clusterNamespaceLabel: cluster.GetNamespace()},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     clusterViewName(cluster),
			},
			Subjects: []rbacv1.Subject{subject},
		}
		bindings = append(bindings, roleBinding)
	}

	return crtb, h.roleBindingApply.
		WithListerNamespace(cluster.Namespace).
		WithSetID(CRTBRoleBindingID).
		WithOwner(crtb).
		ApplyObjects(bindings...)
}

func (h *handler) isClusterIndexed(rt *v3.RoleTemplate) (bool, bool, error) {
	var clusterIndexed, provisioningCluster bool

	rules, err := rbac.RulesFromTemplate(h.clusterRoleCache, h.roleTemplatesCache, rt)
	if err != nil {
		return clusterIndexed, provisioningCluster, err
	}

	for _, rule := range rules {
		if len(rule.NonResourceURLs) > 0 || len(rule.ResourceNames) > 0 {
			continue
		}
		matches, err := h.getMatchingClusterIndexedTypes(rule)
		if err != nil {
			return clusterIndexed, provisioningCluster, err
		}
		if len(matches) > 0 {
			clusterIndexed = true
			for _, match := range matches {
				if reflect.DeepEqual(match.GVK, h.provisioningClusterGVK) {
					return clusterIndexed, true, nil
				}
			}
		}

	}
	return clusterIndexed, provisioningCluster, nil
}
