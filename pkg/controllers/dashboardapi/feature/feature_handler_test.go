package feature

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReconcileFeatures(t *testing.T) {
	assert := assert.New(t)

	// testing a non-dynamic feature
	mockFeature := v3.Feature{
		ObjectMeta: v1.ObjectMeta{
			Name: "multi-cluster-management-agent",
		},
	}

	feature := features.GetFeatureByName(mockFeature.Name)
	assert.Equal(false, feature.Enabled())

	needsRestart := ReconcileFeatures(&mockFeature)
	assert.False(needsRestart)
	assert.Equal(false, feature.Enabled())

	mockFeatureWithTrueValue := mockFeature.DeepCopy()
	trueVal := true
	mockFeatureWithTrueValue.Spec.Value = &trueVal
	needsRestart = ReconcileFeatures(mockFeatureWithTrueValue)
	assert.True(needsRestart)
	assert.Equal(false, feature.Enabled())

	mockFeatureWithTrueLockedValue := mockFeature.DeepCopy()
	mockFeatureWithTrueLockedValue.Status.LockedValue = &trueVal
	needsRestart = ReconcileFeatures(mockFeatureWithTrueLockedValue)
	assert.True(needsRestart)
	assert.Equal(false, feature.Enabled())

	// testing a dynamic feature
	mockFeature = v3.Feature{
		ObjectMeta: v1.ObjectMeta{
			Name: "istio-virtual-service-ui",
		},
		Spec: v3.FeatureSpec{
			Value: &trueVal,
		},
	}

	feature = features.GetFeatureByName(mockFeature.Name)
	assert.Equal(true, feature.Enabled())

	needsRestart = ReconcileFeatures(&mockFeature)
	assert.False(needsRestart)
	assert.Equal(true, feature.Enabled())

	falseValue := false
	mockFeatureWithFalseValue := mockFeature.DeepCopy()
	mockFeatureWithFalseValue.Spec.Value = &falseValue
	needsRestart = ReconcileFeatures(mockFeatureWithFalseValue)
	assert.False(needsRestart)
	assert.Equal(false, feature.Enabled())

}
