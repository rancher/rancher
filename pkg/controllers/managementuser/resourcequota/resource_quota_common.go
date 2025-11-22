package resourcequota

import (
	"encoding/json"

	"github.com/rancher/norman/types/convert"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wmgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/resourcequota"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
)

const extendedKey = resourcequota.ExtendedKey

func convertResourceListToLimit(rList corev1.ResourceList) (*apiv3.ResourceQuotaLimit, error) {
	converted, err := convert.EncodeToMap(rList)
	if err != nil {
		return nil, err
	}

	extended := map[string]string{}
	convertedMap := map[string]any{}
	for key, value := range converted {
		if val, ok := resourceQuotaReturnConversion[key]; ok {
			key = val
		} else {
			extended[key] = convert.ToString(value)
		}
		convertedMap[key] = convert.ToString(value)
	}
	if len(extended) > 0 {
		convertedMap[extendedKey] = extended
	}

	toReturn := &apiv3.ResourceQuotaLimit{}
	err = convert.ToObj(convertedMap, toReturn)

	return toReturn, err
}

func convertResourceLimitResourceQuotaSpec(limit *apiv3.ResourceQuotaLimit) (*corev1.ResourceQuotaSpec, error) {
	converted, err := convertProjectResourceLimitToResourceList(limit)
	if err != nil {
		return nil, err
	}
	quotaSpec := &corev1.ResourceQuotaSpec{
		Hard: converted,
	}
	return quotaSpec, err
}

// convertProjectResourceLimitToResourceList tries to convert a Rancher-defined resource quota limit to its native Kubernetes notation.
func convertProjectResourceLimitToResourceList(limit *apiv3.ResourceQuotaLimit) (corev1.ResourceList, error) {
	return resourcequota.ConvertLimitToResourceList(limit)
}

func convertContainerResourceLimitToResourceList(limit *apiv3.ContainerResourceLimit) (corev1.ResourceList, corev1.ResourceList, error) {
	in, err := json.Marshal(limit)
	if err != nil {
		return nil, nil, err
	}
	limitsMap := map[string]string{}
	err = json.Unmarshal(in, &limitsMap)
	if err != nil {
		return nil, nil, err
	}

	if len(limitsMap) == 0 {
		return nil, nil, nil
	}

	limits := corev1.ResourceList{}
	requests := corev1.ResourceList{}
	for key, value := range limitsMap {
		var resourceName corev1.ResourceName
		request := false
		if val, ok := limitRangerRequestConversion[key]; ok {
			resourceName = corev1.ResourceName(val)
			request = true
		} else if val, ok := limitRangerLimitConversion[key]; ok {
			resourceName = corev1.ResourceName(val)
		}
		if resourceName == "" {
			continue
		}

		resourceQuantity, err := resource.ParseQuantity(value)
		if err != nil {
			return nil, nil, err
		}
		if request {
			requests[resourceName] = resourceQuantity
		} else {
			limits[resourceName] = resourceQuantity
		}

	}
	return requests, limits, nil
}

var limitRangerRequestConversion = map[string]string{
	"requestsCpu":    "cpu",
	"requestsMemory": "memory",
}

var limitRangerLimitConversion = map[string]string{
	"limitsCpu":    "cpu",
	"limitsMemory": "memory",
}

// Also see resourcequota.resourceQuotaConversion for the table governing the forward conversion
var resourceQuotaReturnConversion = map[string]string{
	"configmaps":             "configMaps",
	"limits.cpu":             "limitsCpu",
	"limits.memory":          "limitsMemory",
	"persistentvolumeclaims": "persistentVolumeClaims",
	"pods":                   "pods",
	"replicationcontrollers": "replicationControllers",
	"requests.cpu":           "requestsCpu",
	"requests.memory":        "requestsMemory",
	"requests.storage":       "requestsStorage",
	"secrets":                "secrets",
	"services":               "services",
	"services.loadbalancers": "servicesLoadBalancers",
	"services.nodeports":     "servicesNodePorts",
}

func getNamespaceResourceQuota(ns *corev1.Namespace) string {
	if ns.Annotations == nil {
		return ""
	}
	return ns.Annotations[resourceQuotaAnnotation]
}

func getNamespaceContainerDefaultResourceLimit(ns *corev1.Namespace) string {
	if ns.Annotations == nil {
		return ""
	}
	return ns.Annotations[limitRangeAnnotation]
}

func getProjectResourceQuotaLimit(ns *corev1.Namespace, projectGetter wmgmtv3.ProjectCache) (*apiv3.ResourceQuotaLimit, string, error) {
	projectID := getProjectID(ns)
	if projectID == "" {
		return nil, "", nil
	}
	projectNamespace, projectName := ref.Parse(projectID)
	if projectName == "" {
		return nil, "", nil
	}
	project, err := projectGetter.Get(projectNamespace, projectName)
	if err != nil || project.Spec.ResourceQuota == nil {
		if errors.IsNotFound(err) {
			// If Rancher is unaware of a project, we should ignore trying to get the resource quota limit
			// A non-existent project is likely managed by another Rancher (e.g. Hosted Rancher)
			return nil, "", nil
		}
		return nil, "", err
	}
	return &project.Spec.ResourceQuota.Limit, projectID, nil
}

func getProjectNamespaceDefaultQuota(ns *corev1.Namespace, projectGetter wmgmtv3.ProjectCache) (*apiv3.NamespaceResourceQuota, error) {
	projectID := getProjectID(ns)
	if projectID == "" {
		return nil, nil
	}
	projectNamespace, projectName := ref.Parse(projectID)
	if projectName == "" {
		return nil, nil
	}
	project, err := projectGetter.Get(projectNamespace, projectName)
	if err != nil || project.Spec.ResourceQuota == nil {
		if errors.IsNotFound(err) {
			// If Rancher is unaware of a project, we should ignore trying to get the default namespace quota
			// A non-existent project is likely managed by another Rancher (e.g. Hosted Rancher)
			return nil, nil
		}
		return nil, err
	}
	return project.Spec.NamespaceDefaultResourceQuota, nil
}

func getProjectContainerDefaultLimit(ns *corev1.Namespace, projectGetter wmgmtv3.ProjectCache) (*apiv3.ContainerResourceLimit, error) {
	projectID := getProjectID(ns)
	if projectID == "" {
		return nil, nil
	}
	projectNamespace, projectName := ref.Parse(projectID)
	if projectName == "" {
		return nil, nil
	}
	project, err := projectGetter.Get(projectNamespace, projectName)
	if err != nil {
		if errors.IsNotFound(err) {
			// If Rancher is unaware of a project, we should ignore trying to get the default container limit
			// A non-existent project is likely managed by another Rancher (e.g. Hosted Rancher)
			return nil, nil
		}
		return nil, err
	}
	return project.Spec.ContainerDefaultResourceLimit, nil
}

func getNamespaceResourceQuotaLimit(ns *corev1.Namespace) (*apiv3.ResourceQuotaLimit, error) {
	value := getNamespaceResourceQuota(ns)
	if value == "" {
		return nil, nil
	}
	var nsQuota apiv3.NamespaceResourceQuota
	err := json.Unmarshal([]byte(convert.ToString(value)), &nsQuota)
	if err != nil {
		return nil, err
	}
	return &nsQuota.Limit, err
}

func getNamespaceContainerResourceLimit(ns *corev1.Namespace) (*apiv3.ContainerResourceLimit, error) {
	value := getNamespaceContainerDefaultResourceLimit(ns)
	// rework after api framework change is done
	// when annotation field is passed as null, the annotation should be removed
	// instead of being updated with the null value
	if value == "" || value == "null" {
		return nil, nil
	}
	var nsLimit apiv3.ContainerResourceLimit
	err := json.Unmarshal([]byte(convert.ToString(value)), &nsLimit)
	if err != nil {
		return nil, err
	}
	return &nsLimit, err
}

func getProjectID(ns *corev1.Namespace) string {
	if ns.Annotations != nil {
		return ns.Annotations[projectIDAnnotation]
	}
	return ""
}

func convertPodResourceLimitToLimitRangeSpec(podResourceLimit *apiv3.ContainerResourceLimit) (*corev1.LimitRangeSpec, error) {
	request, limit, err := convertContainerResourceLimitToResourceList(podResourceLimit)
	if err != nil {
		return nil, err
	}
	if request == nil && limit == nil {
		return nil, nil
	}

	item := corev1.LimitRangeItem{
		Type:           corev1.LimitTypeContainer,
		Default:        limit,
		DefaultRequest: request,
	}
	limits := []corev1.LimitRangeItem{item}
	limitRangeSpec := &corev1.LimitRangeSpec{
		Limits: limits,
	}
	return limitRangeSpec, err
}
