package resourcequota

import (
	"maps"
	"slices"
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	api "k8s.io/api/core/v1"
)

func TestIsQuotaFitRejectsNegativePeerLimit(t *testing.T) {
	projectLimit := &v32.ResourceQuotaLimit{LimitsMemory: "4000Mi"}
	currentLimit := &v32.ResourceQuotaLimit{LimitsMemory: "2000Mi"}
	peerNegativeLimit := &v32.ResourceQuotaLimit{LimitsMemory: "-8000Mi"}

	fit, failedHard, err := IsQuotaFit(currentLimit, []*v32.ResourceQuotaLimit{
		peerNegativeLimit,
	}, projectLimit)
	assert.NoError(t, err)
	assert.False(t, fit)
	assert.NotNil(t, failedHard)
	assert.Len(t, failedHard, 1)
	assert.Equal(t, []api.ResourceName{
		api.ResourceLimitsMemory,
	}, slices.Sorted(maps.Keys(failedHard)))
}

func TestIsQuotaFitAllowsPositivePeerLimitWithinProjectLimit(t *testing.T) {
	projectLimit := &v32.ResourceQuotaLimit{LimitsMemory: "4000Mi"}
	currentLimit := &v32.ResourceQuotaLimit{LimitsMemory: "2000Mi"}
	peerLimit := &v32.ResourceQuotaLimit{LimitsMemory: "1000Mi"}

	fit, failedHard, err := IsQuotaFit(currentLimit, []*v32.ResourceQuotaLimit{
		peerLimit,
	}, projectLimit)
	assert.NoError(t, err)
	assert.True(t, fit)
	assert.Nil(t, failedHard)
}
