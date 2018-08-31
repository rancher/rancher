package resourcequota

import (
	"encoding/json"
	"strings"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	api "k8s.io/kubernetes/pkg/apis/core"
)

func convertResourceListToLimit(rList api.ResourceList) (*v3.ResourceQuotaLimit, error) {
	converted, err := convert.EncodeToMap(rList)
	if err != nil {
		return nil, err
	}

	convertedMap := map[string]string{}
	for key, value := range converted {
		convertedMap[key] = convert.ToString(value)
	}

	toReturn := &v3.ResourceQuotaLimit{}
	err = convert.ToObj(convertedMap, toReturn)

	return toReturn, err
}

func convertResourceLimitResourceQuotaSpec(limit *v3.ResourceQuotaLimit) (*corev1.ResourceQuotaSpec, error) {
	converted, err := convertProjectResourceLimitToResourceList(limit)
	if err != nil {
		return nil, err
	}
	quotaSpec := &corev1.ResourceQuotaSpec{
		Hard: converted,
	}
	return quotaSpec, err
}

func convertProjectResourceLimitToResourceList(limit *v3.ResourceQuotaLimit) (corev1.ResourceList, error) {
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

func getNamespaceResourceQuota(ns *corev1.Namespace) string {
	if ns.Annotations == nil {
		return ""
	}
	return ns.Annotations[resourceQuotaAnnotation]
}

func getProjectResourceQuotaLimit(ns *corev1.Namespace, projectLister v3.ProjectLister) (*v3.ResourceQuotaLimit, string, error) {
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

func getProjectNamespaceDefaultQuota(ns *corev1.Namespace, projectLister v3.ProjectLister) (*v3.NamespaceResourceQuota, error) {
	projectID := getProjectID(ns)
	if projectID == "" {
		return nil, nil
	}
	projectNamespace, projectName := getProjectNamespaceName(projectID)
	if projectName == "" {
		return nil, nil
	}
	project, err := projectLister.Get(projectNamespace, projectName)
	if err != nil || project.Spec.ResourceQuota == nil {
		return nil, err
	}
	return project.Spec.NamespaceDefaultResourceQuota, nil
}

func getNamespaceLimit(ns *corev1.Namespace) (*v3.ResourceQuotaLimit, error) {
	value := getNamespaceResourceQuota(ns)
	if value == "" {
		return nil, nil
	}
	var nsQuota v3.NamespaceResourceQuota
	err := json.Unmarshal([]byte(convert.ToString(value)), &nsQuota)
	if err != nil {
		return nil, err
	}
	return &nsQuota.Limit, err
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
