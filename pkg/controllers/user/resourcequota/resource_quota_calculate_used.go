package resourcequota

import (
	"reflect"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientcache "k8s.io/client-go/tools/cache"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/quota"
)

/*
collectController is responsible for calculate the combined limit set on the project's namespaces,
and setting this information in the project
*/
type calculateLimitController struct {
	projectLister               v3.ProjectLister
	projects                    v3.ProjectInterface
	nsIndexer                   clientcache.Indexer
	resourceQuotaTemplateLister v3.ResourceQuotaTemplateLister
	clusterName                 string
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

	templates, err := c.resourceQuotaTemplateLister.List(c.clusterName, labels.NewSelector())
	if err != nil {
		return err
	}
	templatesMap := map[string]*v3.ResourceQuotaTemplate{}
	for _, template := range templates {
		templatesMap[formatTemplateID(template)] = template
	}
	nssResourceList := api.ResourceList{}
	for _, n := range namespaces {
		ns := n.(*corev1.Namespace)
		templateID := getTemplateID(ns)
		if templateID == "" {
			continue
		}
		nsLimit, err := getNamespaceLimit(ns, templatesMap, false)
		if err != nil {
			return err
		}
		nsResourceList, err := convertLimitToResourceList(nsLimit)
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

func convertResourceListToLimit(rList api.ResourceList) (*v3.ProjectResourceLimit, error) {
	converted, err := convert.EncodeToMap(rList)
	if err != nil {
		return nil, err
	}

	convertedMap := map[string]string{}
	for key, value := range converted {
		convertedMap[key] = convert.ToString(value)
	}

	toReturn := &v3.ProjectResourceLimit{}
	err = convert.ToObj(convertedMap, toReturn)

	return toReturn, err
}
