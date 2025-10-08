package feature

import (
	"context"
	"os"
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

	newVal, needsRestart := ReconcileFeatures(obj)
	if needsRestart {
		time.Sleep(3 * time.Second)
		logrus.Infof("feature flag [%s] value has changed (new value=%v), rancher must be restarted", obj.Name, newVal)
		os.Exit(1)
	}

	return obj, nil
}

// ReconcileFeatures updates the feature stored in-memory from the feature that
// is in etcd.
//
// It returns whether Rancher must be restarted. This is the case when (1) the
// feature is non-dynamic and (2) the value was changed.
func ReconcileFeatures(obj *v3.Feature) (*bool, bool) {
	feature := features.GetFeatureByName(obj.Name)

	// possible feature watch renamed, or no longer used by rancher
	if feature == nil {
		return nil, false
	}

	if features.RequireRestarts(feature, obj) {
		return nil, true
	}

	newVal := obj.Status.LockedValue
	if newVal == nil {
		newVal = obj.Spec.Value
	}
	if newVal == nil {
		feature.Unset()
	} else {
		feature.Set(*newVal)
	}

	return newVal, false
}
