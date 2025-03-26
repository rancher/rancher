package resourcequota

import (
	"reflect"
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCompleteLimit(t *testing.T) {
	type input struct {
		nsValues      *v32.ContainerResourceLimit
		projectValues *v32.ContainerResourceLimit
	}

	type expected struct {
		expected *v32.ContainerResourceLimit
		err      error
	}

	testCases := []struct {
		name     string
		input    input
		expected expected
	}{
		{
			name: "limits not set in project",
			input: input{
				nsValues: &v32.ContainerResourceLimit{
					LimitsCPU:    "1000m",
					LimitsMemory: "256Mi",
				},
				projectValues: &v32.ContainerResourceLimit{},
			},
			expected: expected{
				expected: &v32.ContainerResourceLimit{
					LimitsCPU:    "1000m",
					LimitsMemory: "256Mi",
				},
				err: nil,
			},
		},
		{
			name: "limits set in project - namespace setting equal values",
			input: input{
				nsValues: &v32.ContainerResourceLimit{
					LimitsCPU:    "1000m",
					LimitsMemory: "256Mi",
				},
				projectValues: &v32.ContainerResourceLimit{
					LimitsCPU:    "1000m",
					LimitsMemory: "256Mi",
				},
			},
			expected: expected{
				expected: nil,
				err:      nil,
			},
		},
		{
			name: "limits set in namespace and in project",
			input: input{
				nsValues: &v32.ContainerResourceLimit{
					LimitsCPU:    "1000m",
					LimitsMemory: "256Mi",
				},
				projectValues: &v32.ContainerResourceLimit{
					LimitsCPU:    "1200m",
					LimitsMemory: "512Mi",
				},
			},
			expected: expected{
				expected: &v32.ContainerResourceLimit{
					LimitsCPU:    "1000m",
					LimitsMemory: "256Mi",
				},
				err: nil,
			},
		},
		{
			name: "limits set in namespace and requests set in project",
			input: input{
				nsValues: &v32.ContainerResourceLimit{
					LimitsCPU:    "1000m",
					LimitsMemory: "256Mi",
				},
				projectValues: &v32.ContainerResourceLimit{
					RequestsCPU:    "100m",
					RequestsMemory: "128Mi",
				},
			},
			expected: expected{
				expected: &v32.ContainerResourceLimit{
					RequestsCPU:    "100m",
					RequestsMemory: "128Mi",
					LimitsCPU:      "1000m",
					LimitsMemory:   "256Mi",
				},
				err: nil,
			},
		},
		{
			name: "requests set in namespace and limits in project",
			input: input{
				nsValues: &v32.ContainerResourceLimit{
					RequestsCPU:    "100m",
					RequestsMemory: "128Mi",
				},
				projectValues: &v32.ContainerResourceLimit{
					LimitsCPU:    "1000m",
					LimitsMemory: "256Mi",
				},
			},
			expected: expected{
				expected: &v32.ContainerResourceLimit{
					RequestsCPU:    "100m",
					RequestsMemory: "128Mi",
					LimitsCPU:      "1000m",
					LimitsMemory:   "256Mi",
				},
				err: nil,
			},
		},
		{
			name: "requests and limits set in both",
			input: input{
				nsValues: &v32.ContainerResourceLimit{
					RequestsCPU:    "100m",
					RequestsMemory: "128Mi",
					LimitsCPU:      "2000m",
					LimitsMemory:   "1Gi",
				},
				projectValues: &v32.ContainerResourceLimit{
					RequestsCPU:    "200m",
					RequestsMemory: "256Mi",
					LimitsCPU:      "1000m",
					LimitsMemory:   "512Mi",
				},
			},
			expected: expected{
				expected: &v32.ContainerResourceLimit{
					RequestsCPU:    "100m",
					RequestsMemory: "128Mi",
					LimitsCPU:      "2000m",
					LimitsMemory:   "1Gi",
				},
				err: nil,
			},
		},
		{
			name: "project values are null",
			input: input{
				nsValues: &v32.ContainerResourceLimit{
					RequestsCPU:    "100m",
					RequestsMemory: "128Mi",
					LimitsCPU:      "2000m",
					LimitsMemory:   "1Gi",
				},
				projectValues: nil,
			},
			expected: expected{
				expected: nil,
				err:      nil,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			res, err := completeLimit(tt.input.nsValues, tt.input.projectValues)
			if tt.expected.err != nil {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, tt.expected.expected, res)
		})
	}
}

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
		src      *v32.ResourceQuotaLimit
		dst      *v32.ResourceQuotaLimit
		expected bool
	}{
		{
			name:   "compare ResourceQuota using reflect.DeepEqual",
			method: reflect.DeepEqual,
			src: &v32.ResourceQuotaLimit{
				Pods:        "30",
				RequestsCPU: "1000m",
			},
			dst: &v32.ResourceQuotaLimit{
				Pods:        "30",
				RequestsCPU: "1",
			},
			expected: false,
		},
		{
			name:   "compare ResourceQuota using semantic.DeepEqual",
			method: apiequality.Semantic.DeepEqual,
			src: &v32.ResourceQuotaLimit{
				Pods:        "30",
				RequestsCPU: "1000m",
			},
			dst: &v32.ResourceQuotaLimit{
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
