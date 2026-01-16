package resourcequota

import (
	"go.uber.org/mock/gomock"
	"reflect"
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestSetValidated(t *testing.T) {
	t.Run("setup changes, second identical not", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		nsMock := fake.NewMockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl)
		nsMock.EXPECT().Update(gomock.Any()).DoAndReturn(func(ns *corev1.Namespace) (*corev1.Namespace, error) {
			return ns, nil
		}).Times(1)
		sc := SyncController{Namespaces: nsMock}

		// setup of the condition, single call to client
		ns := &corev1.Namespace{}
		ns, err := sc.setValidated(ns, true, "test")
		assert.NotNil(t, ns)
		assert.NoError(t, err)

		// second call makes no difference, does not call client
		_, err = sc.setValidated(ns, true, "test")
		assert.NoError(t, err)
	})
}

func TestUpdateResourceQuota(t *testing.T) {
	specA := corev1.ResourceQuotaSpec{
		Hard: corev1.ResourceList{
			"configmaps": resource.MustParse("1"),
		},
	}
	specB := corev1.ResourceQuotaSpec{
		Hard: corev1.ResourceList{
			"configmaps":        resource.MustParse("1"),
			"ephemeral-storage": resource.MustParse("14"),
		},
	}

	t.Run("no update if no change", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		rqMock := fake.NewMockControllerInterface[*corev1.ResourceQuota, *corev1.ResourceQuotaList](ctrl)

		rqMock.EXPECT().Update(gomock.Any()).Return(nil, nil).Times(0)
		sc := SyncController{ResourceQuotas: rqMock}

		err := sc.updateResourceQuota(&corev1.ResourceQuota{Spec: specA}, &specA)
		assert.NoError(t, err)
	})

	t.Run("update for changes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		rqMock := fake.NewMockControllerInterface[*corev1.ResourceQuota, *corev1.ResourceQuotaList](ctrl)

		rqMock.EXPECT().Update(gomock.Any()).Return(nil, nil)
		sc := SyncController{ResourceQuotas: rqMock}

		err := sc.updateResourceQuota(&corev1.ResourceQuota{Spec: specA}, &specB)
		assert.NoError(t, err)
	})
}

func TestUpdateDefaultLimitRange(t *testing.T) {
	specA := corev1.LimitRangeSpec{
		Limits: []corev1.LimitRangeItem{
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
	}
	specB := corev1.LimitRangeSpec{
		Limits: []corev1.LimitRangeItem{
			{
				Type: corev1.LimitTypePod,
				Default: corev1.ResourceList{
					corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI),
				},
				DefaultRequest: corev1.ResourceList{
					corev1.ResourceCPU: *resource.NewMilliQuantity(1000, resource.DecimalSI),
				},
			},
			{
				Type: corev1.LimitTypePod,
				Default: corev1.ResourceList{
					corev1.ResourceMemory: *resource.NewQuantity(1, resource.DecimalSI),
				},
				DefaultRequest: corev1.ResourceList{
					corev1.ResourceMemory: *resource.NewQuantity(1, resource.DecimalSI),
				},
			},
		},
	}

	t.Run("no update if no change", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lrMock := fake.NewMockControllerInterface[*corev1.LimitRange, *corev1.LimitRangeList](ctrl)

		lrMock.EXPECT().Update(gomock.Any()).Return(nil, nil).Times(0)
		sc := SyncController{LimitRange: lrMock}

		err := sc.updateDefaultLimitRange(&corev1.LimitRange{Spec: specA}, &specA)
		assert.NoError(t, err)
	})

	t.Run("update for changes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lrMock := fake.NewMockControllerInterface[*corev1.LimitRange, *corev1.LimitRangeList](ctrl)

		lrMock.EXPECT().Update(gomock.Any()).Return(nil, nil)
		sc := SyncController{LimitRange: lrMock}

		err := sc.updateDefaultLimitRange(&corev1.LimitRange{Spec: specA}, &specB)
		assert.NoError(t, err)
	})
}

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

func TestZeroOutResourceQuotaLimit(t *testing.T) {
	t.Run("zeroOutResourceQuotaLimit, zero all", func(t *testing.T) {
		out, err := zeroOutResourceQuotaLimit(
			&v32.ResourceQuotaLimit{
				ConfigMaps:             "10",
				LimitsCPU:              "10",
				LimitsMemory:           "10",
				PersistentVolumeClaims: "10",
				Pods:                   "10",
				ReplicationControllers: "10",
				RequestsCPU:            "10",
				RequestsMemory:         "10",
				RequestsStorage:        "10",
				Secrets:                "10",
				Services:               "10",
				ServicesLoadBalancers:  "10",
				ServicesNodePorts:      "10",
				Extended: map[string]string{
					"ephemeral-storage": "10",
				},
			},
			corev1.ResourceList{
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
		assert.Equal(t, &v32.ResourceQuotaLimit{
			ConfigMaps:             "0",
			LimitsCPU:              "0",
			LimitsMemory:           "0",
			PersistentVolumeClaims: "0",
			Pods:                   "0",
			ReplicationControllers: "0",
			RequestsCPU:            "0",
			RequestsMemory:         "0",
			RequestsStorage:        "0",
			Secrets:                "0",
			Services:               "0",
			ServicesLoadBalancers:  "0",
			ServicesNodePorts:      "0",
			Extended: map[string]string{
				"ephemeral-storage": "0",
			},
		}, out)
	})
}
