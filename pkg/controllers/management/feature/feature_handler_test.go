package feature

import (
	"testing"

	"github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReconcileFeatures(t *testing.T) {
	assert := assert.New(t)

	// testing a non-dynamic feature
	mockFeature := v3.Feature{
		ObjectMeta: v1.ObjectMeta{
			Name: "steve",
		},
	}

	feature := features.GetFeatureByName(mockFeature.Name)
	assert.Equal(false, feature.Enabled())

	err := ReconcileFeatures(&mockFeature, false)
	assert.Nil(err)
	assert.Equal(false, feature.Enabled())

	err = ReconcileFeatures(&mockFeature, true)
	assert.Error(err)
	assert.Equal(false, feature.Enabled())

	// testing a dynamic feature
	mockFeature = v3.Feature{
		ObjectMeta: v1.ObjectMeta{
			Name: "istio-virtual-service-ui",
		},
	}

	feature = features.GetFeatureByName(mockFeature.Name)
	assert.Equal(true, feature.Enabled())

	err = ReconcileFeatures(&mockFeature, true)
	assert.Nil(err)
	assert.Equal(true, feature.Enabled())

	err = ReconcileFeatures(&mockFeature, false)
	assert.Nil(err)
	assert.Equal(false, feature.Enabled())
}
