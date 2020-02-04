package resourcequota

import (
	"reflect"
	"testing"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestLimitsChanged(t *testing.T) {

	tests := []struct {
		name     string
		existing []corev1.LimitRangeItem
		toUpdate []corev1.LimitRangeItem
		expected bool
	}{
		{
			name: "limitsChange using semantic.DeepEqual",
			existing: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypePod,
					Default: corev1.ResourceList{
						corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI),
					},
					DefaultRequest: corev1.ResourceList{
						corev1.ResourceCPU: *resource.NewMilliQuantity(1000, resource.DecimalSI),
					},
				},
			},
			toUpdate: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypePod,
					Default: corev1.ResourceList{
						corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI),
					},
					DefaultRequest: corev1.ResourceList{
						corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI),
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		result := limitsChanged(tt.existing, tt.toUpdate)
		assert.Equal(t, tt.expected, result)
	}
}

func TestSemanticDeepEqual(t *testing.T) {

	tests := []struct {
		name     string
		method   func(x, y interface{}) bool
		src      *v3.ResourceQuotaLimit
		dst      *v3.ResourceQuotaLimit
		expected bool
	}{
		{
			name:   "compare ResourceQuota using reflect.DeepEqual",
			method: reflect.DeepEqual,
			src: &v3.ResourceQuotaLimit{
				Pods:        "30",
				RequestsCPU: "1000m",
			},
			dst: &v3.ResourceQuotaLimit{
				Pods:        "30",
				RequestsCPU: "1",
			},
			expected: false,
		},
		{
			name:   "compare ResourceQuota using semantic.DeepEqual",
			method: apiequality.Semantic.DeepEqual,
			src: &v3.ResourceQuotaLimit{
				Pods:        "30",
				RequestsCPU: "1000m",
			},
			dst: &v3.ResourceQuotaLimit{
				Pods:        "30",
				RequestsCPU: "1",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		srcResourceList, err := convertProjectResourceLimitToResourceList(tt.src)
		if err != nil {
			t.Error(err)
		}

		dstResourceList, err := convertProjectResourceLimitToResourceList(tt.dst)
		if err != nil {
			t.Error(err)
		}

		result := tt.method(srcResourceList, dstResourceList)
		assert.Equal(t, tt.expected, result)
	}

}
