package resourcequota

import (
	"maps"
	"slices"
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestZeroOutResourceList(t *testing.T) {
	t.Run("ZeroOutResourceList, zero all", func(t *testing.T) {
		out := ZeroOutResourceList(
			api.ResourceList{
				"configmaps":             resource.MustParse("1"),
				"ephemeral-storage":      resource.MustParse("1"),
				"limits.cpu":             resource.MustParse("1"),
				"limits.memory":          resource.MustParse("1"),
				"persistentvolumeclaims": resource.MustParse("1"),
				"pods":                   resource.MustParse("1"),
				"replicationcontrollers": resource.MustParse("1"),
				"requests.cpu":           resource.MustParse("1"),
				"requests.memory":        resource.MustParse("1"),
				"requests.storage":       resource.MustParse("1"),
				"secrets":                resource.MustParse("1"),
				"services":               resource.MustParse("1"),
				"services.loadbalancers": resource.MustParse("1"),
				"services.nodeports":     resource.MustParse("1"),
			},
			[]api.ResourceName{
				"configmaps",
				"ephemeral-storage",
				"limits.cpu",
				"limits.memory",
				"persistentvolumeclaims",
				"pods",
				"replicationcontrollers",
				"requests.cpu",
				"requests.memory",
				"requests.storage",
				"secrets",
				"services",
				"services.loadbalancers",
				"services.nodeports",
			})
		assert.Equal(t, api.ResourceList{
			"configmaps":             resource.MustParse("0"),
			"ephemeral-storage":      resource.MustParse("0"),
			"limits.cpu":             resource.MustParse("0"),
			"limits.memory":          resource.MustParse("0"),
			"persistentvolumeclaims": resource.MustParse("0"),
			"pods":                   resource.MustParse("0"),
			"replicationcontrollers": resource.MustParse("0"),
			"requests.cpu":           resource.MustParse("0"),
			"requests.memory":        resource.MustParse("0"),
			"requests.storage":       resource.MustParse("0"),
			"secrets":                resource.MustParse("0"),
			"services":               resource.MustParse("0"),
			"services.loadbalancers": resource.MustParse("0"),
			"services.nodeports":     resource.MustParse("0"),
		}, out)
	})

	t.Run("ZeroOutResourceList, zero none", func(t *testing.T) {
		out := ZeroOutResourceList(
			api.ResourceList{
				"configmaps":             resource.MustParse("1"),
				"ephemeral-storage":      resource.MustParse("11"),
				"limits.cpu":             resource.MustParse("2"),
				"limits.memory":          resource.MustParse("4"),
				"persistentvolumeclaims": resource.MustParse("17"),
				"pods":                   resource.MustParse("41"),
				"replicationcontrollers": resource.MustParse("71"),
				"requests.cpu":           resource.MustParse("81"),
				"requests.memory":        resource.MustParse("9"),
				"requests.storage":       resource.MustParse("5"),
				"secrets":                resource.MustParse("10"),
				"services":               resource.MustParse("122"),
				"services.loadbalancers": resource.MustParse("6"),
				"services.nodeports":     resource.MustParse("101"),
			},
			[]api.ResourceName{})
		assert.Equal(t, api.ResourceList{
			"configmaps":             resource.MustParse("1"),
			"ephemeral-storage":      resource.MustParse("11"),
			"limits.cpu":             resource.MustParse("2"),
			"limits.memory":          resource.MustParse("4"),
			"persistentvolumeclaims": resource.MustParse("17"),
			"pods":                   resource.MustParse("41"),
			"replicationcontrollers": resource.MustParse("71"),
			"requests.cpu":           resource.MustParse("81"),
			"requests.memory":        resource.MustParse("9"),
			"requests.storage":       resource.MustParse("5"),
			"secrets":                resource.MustParse("10"),
			"services":               resource.MustParse("122"),
			"services.loadbalancers": resource.MustParse("6"),
			"services.nodeports":     resource.MustParse("101"),
		}, out)
	})

	t.Run("ZeroOutResourceList, zero none, nil", func(t *testing.T) {
		out := ZeroOutResourceList(
			api.ResourceList{
				"configmaps":             resource.MustParse("1"),
				"ephemeral-storage":      resource.MustParse("11"),
				"limits.cpu":             resource.MustParse("2"),
				"limits.memory":          resource.MustParse("4"),
				"persistentvolumeclaims": resource.MustParse("17"),
				"pods":                   resource.MustParse("41"),
				"replicationcontrollers": resource.MustParse("71"),
				"requests.cpu":           resource.MustParse("81"),
				"requests.memory":        resource.MustParse("9"),
				"requests.storage":       resource.MustParse("5"),
				"secrets":                resource.MustParse("10"),
				"services":               resource.MustParse("122"),
				"services.loadbalancers": resource.MustParse("6"),
				"services.nodeports":     resource.MustParse("101"),
			}, nil)
		assert.Equal(t, api.ResourceList{
			"configmaps":             resource.MustParse("1"),
			"ephemeral-storage":      resource.MustParse("11"),
			"limits.cpu":             resource.MustParse("2"),
			"limits.memory":          resource.MustParse("4"),
			"persistentvolumeclaims": resource.MustParse("17"),
			"pods":                   resource.MustParse("41"),
			"replicationcontrollers": resource.MustParse("71"),
			"requests.cpu":           resource.MustParse("81"),
			"requests.memory":        resource.MustParse("9"),
			"requests.storage":       resource.MustParse("5"),
			"secrets":                resource.MustParse("10"),
			"services":               resource.MustParse("122"),
			"services.loadbalancers": resource.MustParse("6"),
			"services.nodeports":     resource.MustParse("101"),
		}, out)
	})

	t.Run("ZeroOutResourceList, zero in part", func(t *testing.T) {
		out := ZeroOutResourceList(
			api.ResourceList{
				"configmaps":             resource.MustParse("1"),
				"ephemeral-storage":      resource.MustParse("11"),
				"limits.cpu":             resource.MustParse("2"),
				"limits.memory":          resource.MustParse("4"),
				"persistentvolumeclaims": resource.MustParse("17"),
				"pods":                   resource.MustParse("41"),
				"replicationcontrollers": resource.MustParse("71"),
				"requests.cpu":           resource.MustParse("81"),
				"requests.memory":        resource.MustParse("9"),
				"requests.storage":       resource.MustParse("5"),
				"secrets":                resource.MustParse("10"),
				"services":               resource.MustParse("122"),
				"services.loadbalancers": resource.MustParse("6"),
				"services.nodeports":     resource.MustParse("101"),
			},
			[]api.ResourceName{
				"ephemeral-storage",
				"requests.cpu",
				"secrets",
			})
		assert.Equal(t, api.ResourceList{
			"configmaps":             resource.MustParse("1"),
			"ephemeral-storage":      resource.MustParse("0"),
			"limits.cpu":             resource.MustParse("2"),
			"limits.memory":          resource.MustParse("4"),
			"persistentvolumeclaims": resource.MustParse("17"),
			"pods":                   resource.MustParse("41"),
			"replicationcontrollers": resource.MustParse("71"),
			"requests.cpu":           resource.MustParse("0"),
			"requests.memory":        resource.MustParse("9"),
			"requests.storage":       resource.MustParse("5"),
			"secrets":                resource.MustParse("0"),
			"services":               resource.MustParse("122"),
			"services.loadbalancers": resource.MustParse("6"),
			"services.nodeports":     resource.MustParse("101"),
		}, out)
	})
}

func TestIsQuotaFit(t *testing.T) {
	projectLimit := &v32.ResourceQuotaLimit{LimitsMemory: "4000Mi"}

	currentLimitOk := &v32.ResourceQuotaLimit{LimitsMemory: "2000Mi"}
	currentLimitExceed := &v32.ResourceQuotaLimit{LimitsMemory: "6000Mi"}
	currentLimitNegative := &v32.ResourceQuotaLimit{LimitsMemory: "-2000Mi"}

	peerLimitOk := &v32.ResourceQuotaLimit{LimitsMemory: "1000Mi"}
	peerLimitExceed := &v32.ResourceQuotaLimit{LimitsMemory: "3000Mi"}
	peerLimitNegative := &v32.ResourceQuotaLimit{LimitsMemory: "-8000Mi"}

	failedResource := []api.ResourceName{api.ResourceLimitsMemory}

	testcases := map[string]struct {
		wantErr      error
		wantExceeds  []api.ResourceName
		wantNegative []api.ResourceName
		wantFit      bool
		current      *v32.ResourceQuotaLimit
		peer         *v32.ResourceQuotaLimit
	}{
		"negative for current, ok peer, fail": {
			wantNegative: failedResource,
			current:      currentLimitNegative,
			peer:         peerLimitOk,
		},
		"negative for current, negative peer, fail": {
			wantNegative: failedResource,
			current:      currentLimitNegative,
			peer:         peerLimitNegative,
		},
		"ok current, negative peer is ignored, ok": {
			wantFit: true,
			current: currentLimitOk,
			peer:    peerLimitNegative,
		},
		"ok current, exceeds through peer, fail": {
			wantExceeds: failedResource,
			current:     currentLimitOk,
			peer:        peerLimitExceed,
		},
		"ok current, ok peer, ok": {
			wantFit: true,
			current: currentLimitOk,
			peer:    peerLimitOk,
		},
		"exceeds through current, ok peer, fail": {
			wantExceeds: failedResource,
			current:     currentLimitExceed,
			peer:        peerLimitOk,
		},
		"exceeds through current, negative peer does not compensate, fail": {
			wantExceeds: failedResource,
			current:     currentLimitExceed,
			peer:        peerLimitNegative,
		},
		"exceeds through current and peer, fail": {
			wantExceeds: failedResource,
			current:     currentLimitExceed,
			peer:        peerLimitExceed,
		},
	}
	for name, spec := range testcases {
		t.Run(name, func(t *testing.T) {
			fit, exceeded, negatives, err := IsQuotaFit(spec.current,
				[]*v32.ResourceQuotaLimit{spec.peer}, projectLimit)

			assert.Equal(t, spec.wantFit, fit)
			if spec.wantErr != nil {
				assert.Error(t, spec.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
			if spec.wantExceeds != nil {
				assert.NotNil(t, exceeded)
				assert.Equal(t, spec.wantExceeds, slices.Sorted(maps.Keys(exceeded)))
			} else {
				assert.Nil(t, exceeded)
			}
			if spec.wantNegative != nil {
				assert.NotNil(t, negatives)
				assert.Equal(t, spec.wantNegative, slices.Sorted(maps.Keys(negatives)))
			} else {
				assert.Nil(t, negatives)
			}
		})
	}
}
