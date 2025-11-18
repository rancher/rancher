package resourcequota

import (
	"fmt"
	"sync"
	"time"

	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/cache"
	quota "k8s.io/apiserver/pkg/quota/v1"
)

const ExtendedKey = "extended"

var (
	projectLockCache        = cache.NewLRUExpireCache(1000)
	resourceQuotaConversion = map[string]string{
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
)

func GetProjectLock(projectID string) *sync.Mutex {
	val, ok := projectLockCache.Get(projectID)
	if !ok {
		projectLockCache.Add(projectID, &sync.Mutex{}, time.Hour)
		val, _ = projectLockCache.Get(projectID)
	}
	mu := val.(*sync.Mutex)
	return mu
}

func IsQuotaFit(nsLimit *v32.ResourceQuotaLimit, nsLimits []*v32.ResourceQuotaLimit, projectLimit *v32.ResourceQuotaLimit) (bool, api.ResourceList, error) {
	nssResourceList := api.ResourceList{}
	nsResourceList, err := ConvertLimitToResourceList(nsLimit)
	if err != nil {
		return false, nil, fmt.Errorf("checking quota fit: %w", err)
	}
	nssResourceList = quota.Add(nssResourceList, nsResourceList)

	for _, nsLimit := range nsLimits {
		nsResourceList, err := ConvertLimitToResourceList(nsLimit)
		if err != nil {
			return false, nil, fmt.Errorf("checking namespace limits: %w", err)
		}
		nssResourceList = quota.Add(nssResourceList, nsResourceList)
	}

	projectResourceList, err := ConvertLimitToResourceList(projectLimit)
	if err != nil {
		return false, nil, fmt.Errorf("checking project limits: %w", err)
	}

	_, exceeded := quota.LessThanOrEqual(nssResourceList, projectResourceList)
	// Include resources with negative values among exceeded resources.
	exceeded = append(exceeded, quota.IsNegative(nsResourceList)...)
	if len(exceeded) == 0 {
		return true, nil, nil
	}
	failedHard := quota.Mask(nssResourceList, exceeded)
	return false, failedHard, nil
}

func ConvertLimitToResourceList(limit *v32.ResourceQuotaLimit) (api.ResourceList, error) {
	// TECH DEBT: Any change here has to be reflected in rancher/webhook
	//   pkg/resources/management.cattle.io/v3/project/quota_validate.go
	// until such time as both places are unified in a single function shared between r/r and r/w

	toReturn := api.ResourceList{}
	converted, err := convert.EncodeToMap(limit)
	if err != nil {
		return nil, err
	}

	// convert the extended set first, ...
	if extended, ok := converted[ExtendedKey]; ok {
		delete(converted, ExtendedKey)
		for key, value := range extended.(map[string]any) {
			resourceName := api.ResourceName(key)
			resourceQuantity, err := resource.ParseQuantity(value.(string))
			if err != nil {
				return nil, fmt.Errorf("failed to parse value for key %q: %w", key, err)
			}

			toReturn[resourceName] = resourceQuantity
		}
	}

	// then place the fixed data. this order ensures that in case of
	// conflicts between arbitrary and fixed data the fixed data wins.
	for key, value := range converted {
		var resourceName api.ResourceName
		if val, ok := resourceQuotaConversion[key]; ok {
			resourceName = api.ResourceName(val)
		} else {
			resourceName = api.ResourceName(key)
		}
		resourceQuantity, err := resource.ParseQuantity(convert.ToString(value))
		if err != nil {
			return nil, fmt.Errorf("parsing quantity %q: %w", key, err)
		}
		toReturn[resourceName] = resourceQuantity
	}
	return toReturn, nil
}
