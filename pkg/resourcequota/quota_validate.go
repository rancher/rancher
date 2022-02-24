package resourcequota

import (
	"sync"
	"time"

	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/cache"
	quota "k8s.io/apiserver/pkg/quota/v1"
)

var (
	projectLockCache = cache.NewLRUExpireCache(1000)
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
		return false, nil, err
	}
	nssResourceList = quota.Add(nssResourceList, nsResourceList)

	for _, nsLimit := range nsLimits {
		nsResourceList, err := ConvertLimitToResourceList(nsLimit)
		if err != nil {
			return false, nil, err
		}
		nssResourceList = quota.Add(nssResourceList, nsResourceList)
	}

	projectResourceList, err := ConvertLimitToResourceList(projectLimit)
	if err != nil {
		return false, nil, err
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
