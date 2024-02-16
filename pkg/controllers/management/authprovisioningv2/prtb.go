package authprovisioningv2

import (
	"crypto/sha256"
	"encoding/base32"
	"strings"
	"time"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v2/pkg/name"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const PRTBRoleBindingID = "auth-prov-v2-prtb-rolebinding"

func (h *handler) OnPRTB(key string, prtb *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	if prtb == nil || prtb.DeletionTimestamp != nil || prtb.RoleTemplateName == "" || prtb.ProjectName == "" || prtb.ServiceAccount != "" {
		return prtb, nil
	}

	parts := strings.SplitN(prtb.ProjectName, ":", 2)
	if len(parts) < 2 {
		return prtb, errors.Errorf("cannot determine project and cluster from %v", prtb.ProjectName)
	}

	clusterName := parts[0]

	clusters, err := h.clusters.GetByIndex(byClusterName, clusterName)
	if err != nil {
		return prtb, err
	}

	if len(clusters) == 0 {
		// When no provisioning cluster is found, enqueue the PRTB to wait for
		// the provisioning cluster to be created. If we don't try again
		// permissions for the provisioning objects won't be created until an
		// update to the PRTB happens again.
		logrus.Debugf("[auth-prov-v2-prtb] No provisioning cluster found for cluster %v, enqueuing PRTB %v ", clusterName, prtb.Name)
		h.projectRoleTemplateBindingController.EnqueueAfter(prtb.Namespace, prtb.Name, 10*time.Second)
		return prtb, nil
	}

	cluster := clusters[0]

	err = h.ensureClusterViewBinding(cluster, prtb)

	return prtb, err
}

func (h *handler) ensureClusterViewBinding(cluster *v1.Cluster, prtb *v3.ProjectRoleTemplateBinding) error {
	subject, err := rbac.BuildSubjectFromRTB(prtb)
	if err != nil {
		return err
	}

	// The roleBinding name format: r-cluster-<cluster name>-view-<prtb namespace>-<prtb name>-<hashed subject>
	// Example: r-cluster1-view-prtb-bar-foo-wn5d5n7udr
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name.SafeConcatName(clusterViewName(cluster), prtb.Namespace, prtb.Name, hashSubject(subject)),
			Namespace:   cluster.Namespace,
			Annotations: map[string]string{clusterNameLabel: cluster.GetName(), clusterNamespaceLabel: cluster.GetNamespace()},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: cluster.APIVersion,
					Kind:       cluster.Kind,
					Name:       cluster.Name,
					UID:        cluster.UID,
				},
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     clusterViewName(cluster),
		},
	}

	roleBinding.Subjects = []rbacv1.Subject{subject}

	return h.roleBindingApply.
		WithListerNamespace(cluster.Namespace).
		WithSetID(PRTBRoleBindingID).
		WithOwner(prtb).
		ApplyObjects(roleBinding)
}

func clusterViewName(cluster *v1.Cluster) string {
	return name.SafeConcatName("r-cluster", cluster.Name, "view")
}

func hashSubject(subject rbacv1.Subject) string {
	h := sha256.New()
	h.Write([]byte(subject.String()))
	return strings.ToLower(base32.StdEncoding.WithPadding(-1).EncodeToString(h.Sum(nil))[:10])
}
