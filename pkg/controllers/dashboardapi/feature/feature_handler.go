package feature

import (
	"context"
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func Register(ctx context.Context, features managementv3.FeatureController) {
	features.OnChange(ctx, "features-restart-handler", sync)
}

func sync(_ string, obj *v3.Feature) (*v3.Feature, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	val := getEffectiveValue(obj)
	if err := ReconcileFeatures(obj, val); err != nil {
		time.Sleep(3 * time.Second)
		logrus.Fatalf("%v", err)
	}

	return obj, nil
}

// getEffectiveValue considers a feature's default, value, and locked value to determine
// its effective value.
func getEffectiveValue(obj *v3.Feature) bool {
	val := obj.Status.Default
	if obj.Spec.Value != nil {
		val = *obj.Spec.Value
	}
	if obj.Status.LockedValue != nil {
		val = *obj.Status.LockedValue
	}
	return val
}

// ReconcileFeatures returns an error if the feature value in memory does
// not match the feature value in etcd AND the feature is non-dynamic.
// Otherwise, the feature value in memory is reconciled and no error is
// returned.
func ReconcileFeatures(obj *v3.Feature, newVal bool) error {
	feature := features.GetFeatureByName(obj.Name)

	// possible feature watch renamed, or no longer used by rancher
	if feature == nil {
		return nil
	}

	if newVal == feature.Enabled() {
		return nil
	}

	if !feature.Dynamic() {
		return fmt.Errorf("feature flag [%s] value has changed, rancher must be restarted", obj.Name)
	}

	feature.Set(newVal)

	return nil
}
