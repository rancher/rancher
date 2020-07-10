package resourcequota

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"

	"fmt"

	validate "github.com/rancher/rancher/pkg/resourcequota"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	namespaceutil "github.com/rancher/rancher/pkg/types/namespace"
	corev1 "k8s.io/api/core/v1"
	clientcache "k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/quota/v1"
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

func (c *calculateLimitController) calculateResourceQuotaUsed(key string, ns *corev1.Namespace) (runtime.Object, error) {
	if ns == nil {
		return nil, nil
	}
	projectID := getProjectID(ns)
	if projectID == "" {
		return nil, nil
	}
	return nil, c.calculateProjectResourceQuota(projectID)
}

func (c *calculateLimitController) calculateResourceQuotaUsedProject(key string, p *v3.Project) (runtime.Object, error) {
	if p == nil || p.DeletionTimestamp != nil {
		return nil, nil
	}

	return nil, c.calculateProjectResourceQuota(fmt.Sprintf("%s:%s", c.clusterName, p.Name))
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
	nssResourceList := corev1.ResourceList{}
	for _, n := range namespaces {
		ns := n.(*corev1.Namespace)
		if ns.DeletionTimestamp != nil {
			continue
		}
		set, err := namespaceutil.IsNamespaceConditionSet(ns, ResourceQuotaValidatedCondition, true)
		if err != nil {
			return err
		}
		if !set {
			continue
		}
		nsLimit, err := getNamespaceResourceQuotaLimit(ns)
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
