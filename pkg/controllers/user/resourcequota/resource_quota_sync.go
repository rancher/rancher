package resourcequota

import (
	"encoding/json"
	"strings"
	"time"

	"reflect"

	"github.com/rancher/rancher/pkg/controllers/user/rbac"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	projectIDAnnotation = "field.cattle.io/projectId"
	resourceQuotaLabel  = "resourcequota.management.cattle.io/default-resource-quota"
)

/*
syncController takes care of creating Kubernetes resource quota based on the resource limits
defined in namespace.resourceQuotaTemplateId
*/
type syncController struct {
	projectLister               v3.ProjectLister
	namespaces                  v1.NamespaceInterface
	namespaceLister             v1.NamespaceLister
	resourceQuotas              v1.ResourceQuotaInterface
	resourceQuotaLister         v1.ResourceQuotaLister
	resourceQuotaTemplateLister v3.ResourceQuotaTemplateLister
}

func (c *syncController) syncResourceQuota(key string, ns *corev1.Namespace) error {
	if ns == nil || ns.DeletionTimestamp != nil {
		return nil
	}

	return c.createResourceQuota(key, ns)
}

func (c *syncController) createResourceQuota(key string, ns *corev1.Namespace) error {
	existing, err := c.getExistingResourceQuota(ns)
	if err != nil {
		return err
	}

	setDefault := false
	if existing == nil {
		projectLimit, _, err := getProjectLimit(ns, c.projectLister)
		if err != nil {
			return err
		}
		setDefault = projectLimit != nil
	}

	quotaSpec, err := c.getNamespaceResourceQuota(ns, setDefault)
	if err != nil {
		return err
	}

	operation := "none"
	if existing == nil {
		if quotaSpec != nil {
			operation = "create"
		}
	} else {
		if quotaSpec == nil {
			operation = "delete"
		} else if !reflect.DeepEqual(existing.Spec.Hard, quotaSpec.Hard) {
			operation = "update"
		}
	}

	switch operation {
	case "create":
		err = c.createDefaultResourceQuota(ns, quotaSpec)
	case "update":
		err = c.updateResourceQuota(existing, quotaSpec)
	case "delete":
		err = c.deleteResourceQuota(existing)
	}
	if err == nil {
		set, err := rbac.IsNamespaceConditionSet(ns, rbac.ResourceQuotaInitCondition, true)
		if err != nil || set {
			return err
		}
		toUpdate := ns.DeepCopy()
		rbac.SetNamespaceCondition(toUpdate, time.Second*1, rbac.ResourceQuotaInitCondition, true, "")
		_, err = c.namespaces.Update(toUpdate)
	}

	return err
}

func (c *syncController) updateResourceQuota(quota *corev1.ResourceQuota, spec *corev1.ResourceQuotaSpec) error {
	toUpdate := quota.DeepCopy()
	toUpdate.Spec = *spec
	logrus.Infof("Updating default resource quota for namespace %v", toUpdate.Namespace)
	_, err := c.resourceQuotas.Update(toUpdate)
	return err
}

func (c *syncController) deleteResourceQuota(quota *corev1.ResourceQuota) error {
	logrus.Infof("Deleting default resource quota for namespace %v", quota.Namespace)
	return c.resourceQuotas.DeleteNamespaced(quota.Namespace, quota.Name, &metav1.DeleteOptions{})
}

func (c *syncController) getExistingResourceQuota(ns *corev1.Namespace) (*corev1.ResourceQuota, error) {
	set := labels.Set(map[string]string{resourceQuotaLabel: "true"})
	quota, err := c.resourceQuotaLister.List(ns.Name, set.AsSelector())
	if err != nil {
		return nil, err
	}
	if len(quota) == 0 {
		return nil, nil
	}
	return quota[0], nil
}

func (c *syncController) getNamespaceResourceQuota(ns *corev1.Namespace, setDefault bool) (*corev1.ResourceQuotaSpec, error) {
	templateID := getAppliedTemplateID(ns)
	var limit *v3.ProjectResourceLimit
	if templateID == "" {
		if setDefault {
			limit = defaultResourceLimit
		} else {
			return nil, nil
		}
	} else {
		splitted := strings.Split(templateID, ":")
		template, err := c.resourceQuotaTemplateLister.Get(splitted[0], splitted[1])
		if err != nil {
			return nil, err
		}
		limit = &template.Limit
	}

	return convertResourceLimitResourceQuotaSpec(limit)
}

func convertResourceLimitResourceQuotaSpec(limit *v3.ProjectResourceLimit) (*corev1.ResourceQuotaSpec, error) {
	converted, err := convertProjectResourceLimitToResourceList(limit)
	if err != nil {
		return nil, err
	}
	quotaSpec := &corev1.ResourceQuotaSpec{
		Hard: converted,
	}
	return quotaSpec, err
}

func convertProjectResourceLimitToResourceList(limit *v3.ProjectResourceLimit) (corev1.ResourceList, error) {
	in, err := json.Marshal(limit)
	if err != nil {
		return nil, err
	}
	limitsMap := map[string]string{}
	err = json.Unmarshal(in, &limitsMap)
	if err != nil {
		return nil, err
	}

	limits := corev1.ResourceList{}
	for key, value := range limitsMap {
		var resourceName corev1.ResourceName
		if val, ok := conversion[key]; ok {
			resourceName = corev1.ResourceName(val)
		} else {
			resourceName = corev1.ResourceName(key)
		}

		resourceQuantity, err := resource.ParseQuantity(value)
		if err != nil {
			return nil, err
		}

		limits[resourceName] = resourceQuantity
	}
	return limits, nil
}

var conversion = map[string]string{
	"replicationControllers": "replicationcontrollers",
	"configMaps":             "configmaps",
	"persistentVolumeClaims": "persistentvolumeclaims",
	"servicesNodePorts":      "services.nodeports",
	"servicesLoadBalancers":  "services.loadbalancers",
	"requestsCpu":            "requests.cpu",
	"requestsMemory":         "requests.memory",
	"requestsStorage":        "requests.storage",
	"limitsCpu":              "limits.cpu",
	"limitsMemory":           "limits.memory",
}

var defaultResourceLimit = &v3.ProjectResourceLimit{
	Pods:                   "0",
	Services:               "0",
	ReplicationControllers: "0",
	Secrets:                "0",
	ConfigMaps:             "0",
	PersistentVolumeClaims: "0",
	ServicesNodePorts:      "0",
	ServicesLoadBalancers:  "0",
	RequestsCPU:            "0",
	RequestsMemory:         "0",
	RequestsStorage:        "0",
	LimitsCPU:              "0",
	LimitsMemory:           "0",
}

func getProjectLimit(ns *corev1.Namespace, projectLister v3.ProjectLister) (*v3.ProjectResourceLimit, string, error) {
	projectID := getProjectID(ns)
	if projectID == "" {
		return nil, "", nil
	}
	projectNamespace, projectName := getProjectNamespaceName(projectID)
	if projectName == "" {
		return nil, "", nil
	}
	project, err := projectLister.Get(projectNamespace, projectName)
	if err != nil || project.Spec.ResourceQuota == nil {
		return nil, "", err
	}
	return &project.Spec.ResourceQuota.Limit, projectID, nil
}

func getProjectID(ns *corev1.Namespace) string {
	if ns.Annotations != nil {
		return ns.Annotations[projectIDAnnotation]
	}
	return ""
}

func getProjectNamespaceName(projectID string) (string, string) {
	if projectID == "" {
		return "", ""
	}
	parts := strings.Split(projectID, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func (c *syncController) createDefaultResourceQuota(ns *corev1.Namespace, spec *corev1.ResourceQuotaSpec) error {
	resourceQuota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "default-",
			Namespace:    ns.Name,
			Labels:       map[string]string{resourceQuotaLabel: "true"},
		},
		Spec: *spec,
	}
	logrus.Infof("Creating default resource quota for namespace %v", ns.Name)
	_, err := c.resourceQuotas.Create(resourceQuota)
	return err
}
