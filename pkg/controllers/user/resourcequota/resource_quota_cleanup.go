package resourcequota

import (
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
cleanupController watches resource quota resource and cleans up those not created by Rancher
if the resource belongs to a project with resourceQuota set
*/
type cleanupController struct {
	projectLister       v3.ProjectLister
	namespaceLister     v1.NamespaceLister
	resourceQuotas      v1.ResourceQuotaInterface
	resourceQuotaLister v1.ResourceQuotaLister
}

func (c *cleanupController) cleanup(key string, quota *corev1.ResourceQuota) error {
	if quota == nil || quota.DeletionTimestamp != nil {
		return nil
	}
	cleanup, err := c.needToCleanup(quota)
	if err != nil || !cleanup {
		return err
	}

	return c.cleanupResourcequota(key, quota)
}

func (c *cleanupController) needToCleanup(quota *corev1.ResourceQuota) (bool, error) {
	if quota.Labels != nil {
		if _, ok := quota.Labels[resourceQuotaLabel]; ok {
			return false, nil
		}
	}
	ns, err := c.namespaceLister.Get("", quota.Namespace)
	if err != nil {
		return false, err
	}
	projectLimit, _, err := getProjectLimit(ns, c.projectLister)
	if err != nil {
		return false, err
	}
	return projectLimit != nil, nil
}

func (c *cleanupController) cleanupResourcequota(key string, quota *corev1.ResourceQuota) error {
	logrus.Infof("Deleting resource quota [%s] created natively", key)
	return c.resourceQuotas.DeleteNamespaced(quota.Namespace, quota.Name, &metav1.DeleteOptions{})
}
