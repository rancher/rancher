package resourcequota

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/resource"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/quota"
)

func IsQuotaFit(nsLimit *v3.ResourceQuotaLimit, nsLimits []*v3.ResourceQuotaLimit, projectLimit *v3.ResourceQuotaLimit) (bool, string, error) {
	nssResourceList := api.ResourceList{}
	nsResourceList, err := ConvertLimitToResourceList(nsLimit)
	if err != nil {
		return false, "", err
	}
	nssResourceList = quota.Add(nssResourceList, nsResourceList)

	for _, nsLimit := range nsLimits {
		nsResourceList, err := ConvertLimitToResourceList(nsLimit)
		if err != nil {
			return false, "", err
		}
		nssResourceList = quota.Add(nssResourceList, nsResourceList)
	}

	projectResourceList, err := ConvertLimitToResourceList(projectLimit)
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

func ConvertLimitToResourceList(limit *v3.ResourceQuotaLimit) (api.ResourceList, error) {
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
