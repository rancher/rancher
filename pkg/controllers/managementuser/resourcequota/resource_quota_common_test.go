package resourcequota

import (
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestConvertResourceListToLimit(t *testing.T) {
	t.Run("convertResourceListToLimit", func(t *testing.T) {
		out, err := convertResourceListToLimit(corev1.ResourceList{
			"configmaps":             resource.MustParse("1"),
			"ephemeral-storage":      resource.MustParse("14"),
			"limits.cpu":             resource.MustParse("2"),
			"limits.memory":          resource.MustParse("3"),
			"persistentvolumeclaims": resource.MustParse("4"),
			"pods":                   resource.MustParse("5"),
			"replicationcontrollers": resource.MustParse("6"),
			"requests.cpu":           resource.MustParse("7"),
			"requests.memory":        resource.MustParse("8"),
			"requests.storage":       resource.MustParse("9"),
			"secrets":                resource.MustParse("10"),
			"services":               resource.MustParse("11"),
			"services.loadbalancers": resource.MustParse("12"),
			"services.nodeports":     resource.MustParse("13"),
		})
		assert.NoError(t, err)
		assert.Equal(t, &apiv3.ResourceQuotaLimit{
			ConfigMaps:             "1",
			LimitsCPU:              "2",
			LimitsMemory:           "3",
			PersistentVolumeClaims: "4",
			Pods:                   "5",
			ReplicationControllers: "6",
			RequestsCPU:            "7",
			RequestsMemory:         "8",
			RequestsStorage:        "9",
			Secrets:                "10",
			Services:               "11",
			ServicesLoadBalancers:  "12",
			ServicesNodePorts:      "13",
			Extended: map[string]string{
				"ephemeral-storage": "14",
			},
		}, out)
	})
}

func TestConvertResourceLimitResourceQuotaSpec(t *testing.T) {
	t.Run("convertResourceLimitResourceQuotaSpec", func(t *testing.T) {
		out, err := convertResourceLimitResourceQuotaSpec(&apiv3.ResourceQuotaLimit{
			ConfigMaps:             "1",
			LimitsCPU:              "2",
			LimitsMemory:           "3",
			PersistentVolumeClaims: "4",
			Pods:                   "5",
			ReplicationControllers: "6",
			RequestsCPU:            "7",
			RequestsMemory:         "8",
			RequestsStorage:        "9",
			Secrets:                "10",
			Services:               "11",
			ServicesLoadBalancers:  "12",
			ServicesNodePorts:      "13",
			Extended: map[string]string{
				"ephemeral-storage": "14",
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, &corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				"configmaps":             resource.MustParse("1"),
				"ephemeral-storage":      resource.MustParse("14"),
				"limits.cpu":             resource.MustParse("2"),
				"limits.memory":          resource.MustParse("3"),
				"persistentvolumeclaims": resource.MustParse("4"),
				"pods":                   resource.MustParse("5"),
				"replicationcontrollers": resource.MustParse("6"),
				"requests.cpu":           resource.MustParse("7"),
				"requests.memory":        resource.MustParse("8"),
				"requests.storage":       resource.MustParse("9"),
				"secrets":                resource.MustParse("10"),
				"services":               resource.MustParse("11"),
				"services.loadbalancers": resource.MustParse("12"),
				"services.nodeports":     resource.MustParse("13"),
			},
		}, out)
	})
}

func TestConvertProjectResourceLimitToResourceList(t *testing.T) {
	t.Run("convertProjectResourceLimitToResourceList", func(t *testing.T) {
		out, err := convertProjectResourceLimitToResourceList(&apiv3.ResourceQuotaLimit{
			ConfigMaps:             "1",
			LimitsCPU:              "2",
			LimitsMemory:           "3",
			PersistentVolumeClaims: "4",
			Pods:                   "5",
			ReplicationControllers: "6",
			RequestsCPU:            "7",
			RequestsMemory:         "8",
			RequestsStorage:        "9",
			Secrets:                "10",
			Services:               "11",
			ServicesLoadBalancers:  "12",
			ServicesNodePorts:      "13",
			Extended: map[string]string{
				"ephemeral-storage": "14",
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, corev1.ResourceList{
			"configmaps":             resource.MustParse("1"),
			"ephemeral-storage":      resource.MustParse("14"),
			"limits.cpu":             resource.MustParse("2"),
			"limits.memory":          resource.MustParse("3"),
			"persistentvolumeclaims": resource.MustParse("4"),
			"pods":                   resource.MustParse("5"),
			"replicationcontrollers": resource.MustParse("6"),
			"requests.cpu":           resource.MustParse("7"),
			"requests.memory":        resource.MustParse("8"),
			"requests.storage":       resource.MustParse("9"),
			"secrets":                resource.MustParse("10"),
			"services":               resource.MustParse("11"),
			"services.loadbalancers": resource.MustParse("12"),
			"services.nodeports":     resource.MustParse("13"),
		}, out)
	})
}

func TestConvertContainerResourceLimitToResourceList(t *testing.T) {
	t.Run("convertContainerResourceLimitToResourceList", func(t *testing.T) {
		requests, limits, err := convertContainerResourceLimitToResourceList(&apiv3.ContainerResourceLimit{
			LimitsCPU:      "2",
			LimitsMemory:   "3",
			RequestsCPU:    "7",
			RequestsMemory: "8",
		})
		assert.NoError(t, err)
		assert.Equal(t, corev1.ResourceList{
			"cpu":    resource.MustParse("7"),
			"memory": resource.MustParse("8"),
		}, requests)
		assert.Equal(t, corev1.ResourceList{
			"cpu":    resource.MustParse("2"),
			"memory": resource.MustParse("3"),
		}, limits)
	})
}
