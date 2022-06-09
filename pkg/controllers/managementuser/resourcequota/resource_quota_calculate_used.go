package resourcequota

import (
	"fmt"
	"reflect"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	namespaceutil "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	validate "github.com/rancher/rancher/pkg/resourcequota"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	quota "k8s.io/apiserver/pkg/quota/v1"
	clientcache "k8s.io/client-go/tools/cache"
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
	projectNamespace, projectName := ref.Parse(projectID)
	project, err := c.projectLister.Get(projectNamespace, projectName)
	if err != nil || project.Spec.ResourceQuota == nil {
		if errors.IsNotFound(err) {
			// If Rancher is unaware of a project, we should ignore trying to calculate the project resource quota
			// A non-existent project is likely managed by another Rancher (e.g. Hosted Rancher)
			return nil
		}
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
