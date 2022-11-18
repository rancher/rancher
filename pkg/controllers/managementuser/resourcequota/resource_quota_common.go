package resourcequota

import (
	"encoding/json"

	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
)

func convertResourceListToLimit(rList corev1.ResourceList) (*v32.ResourceQuotaLimit, error) {
	converted, err := convert.EncodeToMap(rList)
	if err != nil {
		return nil, err
	}

	convertedMap := map[string]string{}
	for key, value := range converted {
		convertedMap[key] = convert.ToString(value)
	}

	toReturn := &v32.ResourceQuotaLimit{}
	err = convert.ToObj(convertedMap, toReturn)

	return toReturn, err
}

func convertResourceLimitResourceQuotaSpec(limit *v32.ResourceQuotaLimit) (*corev1.ResourceQuotaSpec, error) {
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
func convertProjectResourceLimitToResourceList(limit *v32.ResourceQuotaLimit) (corev1.ResourceList, error) {
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
		if val, ok := resourceQuotaConversion[key]; ok {
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

func convertContainerResourceLimitToResourceList(limit *v32.ContainerResourceLimit) (corev1.ResourceList, corev1.ResourceList, error) {
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

var resourceQuotaConversion = map[string]string{
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

func getNamespaceContainerDefaultResourceLimit(ns *corev1.Namespace) string {
	if ns.Annotations == nil {
		return ""
	}
	return ns.Annotations[limitRangeAnnotation]
}

func getProjectResourceQuotaLimit(ns *corev1.Namespace, projectLister v3.ProjectLister) (*v32.ResourceQuotaLimit, string, error) {
	projectID := getProjectID(ns)
	if projectID == "" {
		return nil, "", nil
	}
	projectNamespace, projectName := ref.Parse(projectID)
	if projectName == "" {
		return nil, "", nil
	}
	project, err := projectLister.Get(projectNamespace, projectName)
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

func getProjectNamespaceDefaultQuota(ns *corev1.Namespace, projectLister v3.ProjectLister) (*v32.NamespaceResourceQuota, error) {
	projectID := getProjectID(ns)
	if projectID == "" {
		return nil, nil
	}
	projectNamespace, projectName := ref.Parse(projectID)
	if projectName == "" {
		return nil, nil
	}
	project, err := projectLister.Get(projectNamespace, projectName)
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

func getProjectContainerDefaultLimit(ns *corev1.Namespace, projectLister v3.ProjectLister) (*v32.ContainerResourceLimit, error) {
	projectID := getProjectID(ns)
	if projectID == "" {
		return nil, nil
	}
	projectNamespace, projectName := ref.Parse(projectID)
	if projectName == "" {
		return nil, nil
	}
	project, err := projectLister.Get(projectNamespace, projectName)
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

func getNamespaceResourceQuotaLimit(ns *corev1.Namespace) (*v32.ResourceQuotaLimit, error) {
	value := getNamespaceResourceQuota(ns)
	if value == "" {
		return nil, nil
	}
	var nsQuota v32.NamespaceResourceQuota
	err := json.Unmarshal([]byte(convert.ToString(value)), &nsQuota)
	if err != nil {
		return nil, err
	}
	return &nsQuota.Limit, err
}

func getNamespaceContainerResourceLimit(ns *corev1.Namespace) (*v32.ContainerResourceLimit, error) {
	value := getNamespaceContainerDefaultResourceLimit(ns)
	// rework after api framework change is done
	// when annotation field is passed as null, the annotation should be removed
	// instead of being updated with the null value
	if value == "" || value == "null" {
		return nil, nil
	}
	var nsLimit v32.ContainerResourceLimit
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

func convertPodResourceLimitToLimitRangeSpec(podResourceLimit *v32.ContainerResourceLimit) (*corev1.LimitRangeSpec, error) {
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
