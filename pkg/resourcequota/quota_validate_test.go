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

	fit, exceeded, negatives, err := IsQuotaFit(currentLimit, []*v32.ResourceQuotaLimit{
		peerNegativeLimit,
	}, projectLimit)
	assert.NoError(t, err)
	assert.False(t, fit)
	assert.Nil(t, exceeded)
	assert.NotNil(t, negatives)
	assert.Len(t, negatives, 1)
	assert.Equal(t, []api.ResourceName{
		api.ResourceLimitsMemory,
	}, slices.Sorted(maps.Keys(negatives)))
}

func TestIsQuotaFitRejectsCompensatingNegativePeerLimit(t *testing.T) {
	projectLimit := &v32.ResourceQuotaLimit{LimitsMemory: "4000Mi"}
	currentLimit := &v32.ResourceQuotaLimit{LimitsMemory: "2000Mi"}
	peerNegativeLimit := &v32.ResourceQuotaLimit{LimitsMemory: "-2000Mi"}

	fit, exceeded, negatives, err := IsQuotaFit(currentLimit, []*v32.ResourceQuotaLimit{
		peerNegativeLimit,
	}, projectLimit)
	assert.NoError(t, err)
	assert.False(t, fit)
	assert.Nil(t, exceeded)
	assert.NotNil(t, negatives)
	assert.Len(t, negatives, 1)
	assert.Equal(t, []api.ResourceName{
		api.ResourceLimitsMemory,
	}, slices.Sorted(maps.Keys(negatives)))
}

func TestIsQuotaFitRejectsOversubscription(t *testing.T) {
	projectLimit := &v32.ResourceQuotaLimit{LimitsMemory: "4000Mi"}
	currentLimit := &v32.ResourceQuotaLimit{LimitsMemory: "2000Mi"}
	peerLimit := &v32.ResourceQuotaLimit{LimitsMemory: "8000Mi"}

	fit, exceeded, negatives, err := IsQuotaFit(currentLimit, []*v32.ResourceQuotaLimit{
		peerLimit,
	}, projectLimit)
	assert.NoError(t, err)
	assert.False(t, fit)
	assert.Nil(t, negatives)
	assert.NotNil(t, exceeded)
	assert.Len(t, exceeded, 1)
	assert.Equal(t, []api.ResourceName{
		api.ResourceLimitsMemory,
	}, slices.Sorted(maps.Keys(exceeded)))
}

func TestIsQuotaFitAllowsPositivePeerLimitWithinProjectLimit(t *testing.T) {
	projectLimit := &v32.ResourceQuotaLimit{LimitsMemory: "4000Mi"}
	currentLimit := &v32.ResourceQuotaLimit{LimitsMemory: "2000Mi"}
	peerLimit := &v32.ResourceQuotaLimit{LimitsMemory: "1000Mi"}

	fit, exceeded, negatives, err := IsQuotaFit(currentLimit, []*v32.ResourceQuotaLimit{
		peerLimit,
	}, projectLimit)
	assert.NoError(t, err)
	assert.True(t, fit)
	assert.Nil(t, exceeded)
	assert.Nil(t, negatives)
}

func TestIsQuotaFitRejectsNegativeOwnLimitWithinProjectLimit(t *testing.T) {
	projectLimit := &v32.ResourceQuotaLimit{LimitsMemory: "4000Mi"}
	currentLimit := &v32.ResourceQuotaLimit{LimitsMemory: "-2000Mi"}
	peerLimit := &v32.ResourceQuotaLimit{LimitsMemory: "1000Mi"}

	fit, exceeded, negatives, err := IsQuotaFit(currentLimit, []*v32.ResourceQuotaLimit{
		peerLimit,
	}, projectLimit)
	assert.NoError(t, err)
	assert.False(t, fit)
	assert.Nil(t, exceeded)
	assert.NotNil(t, negatives)
	assert.Len(t, negatives, 1)
	assert.Equal(t, []api.ResourceName{
		api.ResourceLimitsMemory,
	}, slices.Sorted(maps.Keys(negatives)))
}
