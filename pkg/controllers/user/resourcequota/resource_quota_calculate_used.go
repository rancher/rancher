package resourcequota

import (
	"reflect"

	namespaceutil "github.com/rancher/rancher/pkg/namespace"
	validate "github.com/rancher/rancher/pkg/resourcequota"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	clientcache "k8s.io/client-go/tools/cache"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/quota"
)

/*
collectController is responsible for calculate the combined limit set on the project's Namespaces,
and setting this information in the project
*/
type calculateLimitController struct {
	projectLister v3.ProjectLister
	projects      v3.ProjectInterface
	nsIndexer     clientcache.Indexer
	clusterName   string
}

func (c *calculateLimitController) calculateResourceQuotaUsed(key string, ns *corev1.Namespace) error {
	if ns == nil {
		return nil
	}
	projectID := getProjectID(ns)
	if projectID == "" {
		return nil
	}
	return c.calculateProjectResourceQuota(projectID)
}

func (c *calculateLimitController) calculateProjectResourceQuota(projectID string) error {
	projectNamespace, projectName := getProjectNamespaceName(projectID)
	project, err := c.projectLister.Get(projectNamespace, projectName)
	if err != nil || project.Spec.ResourceQuota == nil {
		return err
	}

	namespaces, err := c.nsIndexer.ByIndex(nsByProjectIndex, projectID)
	if err != nil {
		return err
	}
	nssResourceList := api.ResourceList{}
	for _, n := range namespaces {
		ns := n.(*corev1.Namespace)
		set, err := namespaceutil.IsNamespaceConditionSet(ns, resourceQuotaValidatedCondition, true)
		if err != nil {
			return err
		}
		if !set {
			continue
		}
		nsLimit, err := getNamespaceLimit(ns)
		if err != nil {
			return err
		}
		nsResourceList, err := validate.ConvertLimitToResourceList(nsLimit)
		if err != nil {
			return err
		}
		nssResourceList = quota.Add(nssResourceList, nsResourceList)
	}
	limit, err := convertResourceListToLimit(nssResourceList)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(project.Spec.ResourceQuota.UsedLimit, limit) {
		return nil
	}
	toUpdate := project.DeepCopy()
	toUpdate.Spec.ResourceQuota.UsedLimit = *limit
	_, err = c.projects.Update(toUpdate)
	return err
}
