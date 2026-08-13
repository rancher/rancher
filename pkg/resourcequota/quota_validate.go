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
	zeroQuantity            = resource.MustParse("0")
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

// IsQuotaFit puts the various limits (of the namespace itself, and its
// siblings) together and checks if they still fit into the project. It reports
// the names of all bad resources. A resource is flagged as bad either because
// its sum goes beyond the project limit (oversubscription), or because it is
// negative.  Negative values in the data taken from the sibling namespaces are
// treated as zero, as these are blocked anyway.
func IsQuotaFit(nsLimit *v32.ResourceQuotaLimit, nsLimits []*v32.ResourceQuotaLimit, projectLimit *v32.ResourceQuotaLimit) (bool, api.ResourceList, api.ResourceList, error) {
	nssResourceList := api.ResourceList{}
	nsResourceList, err := ConvertLimitToResourceList(nsLimit)
	if err != nil {
		return false, nil, nil, fmt.Errorf("checking quota fit: %w", err)
	}
	negatives := quota.IsNegative(nsResourceList)
	nssResourceList = quota.Add(nssResourceList, ZeroOutResourceList(nsResourceList, negatives))

	// Detect over-subscription
	for _, nsLimit := range nsLimits {
		nsResourceList, err := ConvertLimitToResourceList(nsLimit)
		if err != nil {
			return false, nil, nil, fmt.Errorf("checking namespace limits: %w", err)
		}
		// zero any negative limits coming from the sibling namespace
		nssResourceList = quota.Add(nssResourceList, ZeroOutResourceList(nsResourceList, quota.IsNegative(nsResourceList)))
	}
	projectResourceList, err := ConvertLimitToResourceList(projectLimit)
	if err != nil {
		return false, nil, nil, fmt.Errorf("checking project limits: %w", err)
	}
	_, exceeded := quota.LessThanOrEqual(nssResourceList, projectResourceList)

	// Compute the full set of bad resources for the current namespace as
	// the union of exceeded and negatives.
	badResources := []api.ResourceName{}
	badResources = append(badResources, exceeded...)
	badResources = append(badResources, negatives...)
	badResources = uniqueResourceNames(badResources)

	if len(badResources) == 0 {
		return true, nil, nil, nil
	}

	// We have problems. Fail the fit, and report the affected resources
	failedExceeded := quota.Mask(nssResourceList, exceeded)
	if len(failedExceeded) == 0 {
		failedExceeded = nil
	}

	failedNegative := quota.Mask(nsResourceList, negatives)
	if len(failedNegative) == 0 {
		failedNegative = nil
	}

	return false, failedExceeded, failedNegative, nil
}

func uniqueResourceNames(resources []api.ResourceName) []api.ResourceName {
	seen := map[api.ResourceName]struct{}{}
	unique := make([]api.ResourceName, 0, len(resources))
	for _, resource := range resources {
		if _, ok := seen[resource]; ok {
			continue
		}
		seen[resource] = struct{}{}
		unique = append(unique, resource)
	}
	return unique
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

// ZeroOutResourceList takes a resource list and a list of bad resources, and
// returns a new list with the bad resources set to zero.
func ZeroOutResourceList(limit api.ResourceList, badResources []api.ResourceName) api.ResourceList {
	// fast path, nothing zero out
	if len(badResources) == 0 {
		return limit
	}
	// copy input, then zero the bad parts
	zeroed := api.ResourceList{}
	for k, v := range limit {
		zeroed[k] = v
	}
	for _, k := range badResources {
		zeroed[k] = zeroQuantity
	}
	return zeroed
}
