package resourcequota

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/user/rbac"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/cache"
	clientcache "k8s.io/client-go/tools/cache"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/quota"
)

const (
	resourceQuotaTemplateIDAnnotation        = "field.cattle.io/resourceQuotaTemplateId"
	resourceQuotaAppliedTemplateIDAnnotation = "field.cattle.io/resourceQuotaAppliedTemplateId"
	resourceQuotaValidatedCondition          = "ResourceQuotaValidated"
)

var (
	projectLockCache = cache.NewLRUExpireCache(1000)
)

/*
validationController listens on namespace creation, and if the namespace has resourceQuotaTemplate set,
the quota will be validated against the project's quota
*/
type validationController struct {
	namespaces                  v1.NamespaceInterface
	nsIndexer                   clientcache.Indexer
	resourceQuotaLister         v1.ResourceQuotaLister
	projectLister               v3.ProjectLister
	resourceQuotaTemplateLister v3.ResourceQuotaTemplateLister
	clusterName                 string
}

func (c *validationController) validateTemplate(key string, ns *corev1.Namespace) error {
	if ns == nil || ns.DeletionTimestamp != nil {
		return nil
	}

	// get project limit
	projectLimit, projectID, err := getProjectLimit(ns, c.projectLister)
	if err != nil {
		return err
	}

	if projectLimit == nil {
		return c.setAppliedTemplateID(ns, err)
	}

	// validate resource quota
	isFit, msg, err := c.validateResourceQuotaTemplate(ns, projectID, projectLimit)
	if err != nil {
		return err
	}

	if isFit {
		return nil
	}
	set, err := rbac.IsNamespaceConditionSet(ns, resourceQuotaValidatedCondition, false)
	if set || err != nil {
		return err
	}

	toUpdate := ns.DeepCopy()
	err = rbac.SetNamespaceCondition(toUpdate, time.Second*1, resourceQuotaValidatedCondition, false, msg)
	if err != nil {
		return err
	}
	_, err = c.namespaces.Update(toUpdate)

	return err
}

func getProjectLock(projectID string) sync.Mutex {
	val, ok := projectLockCache.Get(projectID)
	if !ok {
		projectLockCache.Add(projectID, sync.Mutex{}, time.Hour)
		val, _ = projectLockCache.Get(projectID)
	}
	mu := val.(sync.Mutex)
	return mu
}

func (c *validationController) validateResourceQuotaTemplate(ns *corev1.Namespace, projectID string,
	projectLimit *v3.ProjectResourceLimit) (bool, string, error) {
	mu := getProjectLock(projectID)
	mu.Lock()
	defer mu.Unlock()

	templateIDToUpdate, err := c.getTemplateIDToUpdate(ns)
	if err != nil {
		return false, "", err
	}
	finalNs := *ns
	if templateIDToUpdate != "" {
		toUpdate := ns.DeepCopy()
		if toUpdate.Annotations == nil {
			toUpdate.Annotations = map[string]string{}
		}
		toUpdate.Annotations[resourceQuotaTemplateIDAnnotation] = templateIDToUpdate
		toUpdate, err = c.namespaces.Update(toUpdate)
		if err != nil {
			return false, "", err
		}
		finalNs = *toUpdate
	}

	isFit, msg, err := c.isQuotaFit(&finalNs, projectID, projectLimit)
	if err != nil {
		return false, "", err
	}

	if isFit {
		return true, "", c.setAppliedTemplateID(&finalNs, err)
	}
	return false, msg, nil
}

func (c *validationController) setAppliedTemplateID(ns *corev1.Namespace, err error) error {
	templateID := getTemplateID(ns)
	validatedTemplateID := getAppliedTemplateID(ns)
	if templateID == validatedTemplateID {
		return nil
	}
	toUpdate := ns.DeepCopy()
	toUpdate.Annotations[resourceQuotaAppliedTemplateIDAnnotation] = templateID
	_, err = c.namespaces.Update(toUpdate)
	return err
}

func (c *validationController) getTemplateIDToUpdate(ns *corev1.Namespace) (string, error) {
	templateID := getTemplateID(ns)
	if templateID != "" {
		return "", nil
	}

	templates, err := c.resourceQuotaTemplateLister.List(c.clusterName, labels.NewSelector())
	if err != nil {
		return "", err
	}

	for _, t := range templates {
		if t.IsDefault {
			return formatTemplateID(t), nil
		}
	}
	return "", nil
}

func (c *validationController) isQuotaFit(ns *corev1.Namespace, projectID string, projectLimit *v3.ProjectResourceLimit) (bool, string, error) {
	templates, err := c.resourceQuotaTemplateLister.List(c.clusterName, labels.NewSelector())
	if err != nil {
		return false, "", err
	}
	templatesMap := map[string]*v3.ResourceQuotaTemplate{}
	for _, template := range templates {
		templatesMap[formatTemplateID(template)] = template
	}
	nssResourceList := api.ResourceList{}
	nsLimit := getNamespaceLimit(ns, templatesMap, false)
	// add itself on create
	nsResourceList, err := convertLimitToResourceList(nsLimit)
	if err != nil {
		return false, "", err
	}
	nssResourceList = quota.Add(nssResourceList, nsResourceList)

	// get other namespaces
	namespaces, err := c.nsIndexer.ByIndex(nsByProjectIndex, projectID)
	if err != nil {
		return false, "", err
	}

	for _, n := range namespaces {
		other := n.(*corev1.Namespace)
		if other.Name == ns.Name {
			continue
		}
		nsLimit := getNamespaceLimit(other, templatesMap, true)
		nsResourceList, err := convertLimitToResourceList(nsLimit)
		if err != nil {
			return false, "", err
		}
		nssResourceList = quota.Add(nssResourceList, nsResourceList)
	}

	projectResourceList, err := convertLimitToResourceList(projectLimit)
	if err != nil {
		return false, "", err
	}

	allowed, exceeded := quota.LessThanOrEqual(nssResourceList, projectResourceList)
	if allowed {
		return true, "", nil
	}
	failedHard := quota.Mask(nssResourceList, exceeded)
	return false, fmt.Sprintf("Resource quota [%v] exceeds project limit ", prettyPrint(failedHard)), nil
}

func prettyPrint(item api.ResourceList) string {
	parts := []string{}
	keys := []string{}
	for key := range item {
		keys = append(keys, string(key))
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := item[api.ResourceName(key)]
		constraint := key + "=" + value.String()
		parts = append(parts, constraint)
	}
	return strings.Join(parts, ",")
}

func formatTemplateID(template *v3.ResourceQuotaTemplate) string {
	return fmt.Sprintf("%s:%s", template.Namespace, template.Name)
}

func getNamespaceLimit(ns *corev1.Namespace, templates map[string]*v3.ResourceQuotaTemplate, applied bool) *v3.ProjectResourceLimit {
	templateID := ""
	if applied {
		templateID = getAppliedTemplateID(ns)
	} else {
		templateID = getTemplateID(ns)
	}
	if templateID == "" {
		return nil
	}
	template := templates[templateID]
	if template == nil {
		return nil
	}
	return &template.Limit
}

func convertLimitToResourceList(limit *v3.ProjectResourceLimit) (api.ResourceList, error) {
	toReturn := api.ResourceList{}
	converted, err := convert.EncodeToMap(limit)
	if err != nil {
		return nil, err
	}
	for key, value := range converted {
		q, err := resource.ParseQuantity(convert.ToString(value))
		if err != nil {
			return nil, err
		}
		toReturn[api.ResourceName(key)] = q
	}
	return toReturn, nil
}

func getAppliedTemplateID(ns *corev1.Namespace) string {
	if ns.Annotations != nil {
		return ns.Annotations[resourceQuotaAppliedTemplateIDAnnotation]
	}
	return ""
}

func getTemplateID(ns *corev1.Namespace) string {
	if ns.Annotations != nil {
		return ns.Annotations[resourceQuotaTemplateIDAnnotation]
	}
	return ""
}
