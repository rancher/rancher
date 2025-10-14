package feature

import (
	"context"
	"fmt"
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
		logrus.Infof("feature flag [%s] value has changed (new value=%v), rancher must be restarted", obj.Name, ptrBoolToString(newVal))
		os.Exit(0)
	}

	return obj, nil
}

func ptrBoolToString(val *bool) string {
	if val == nil {
		return "nil"
	}
	return fmt.Sprintf("%v", *val)
}

// ReconcileFeatures updates the feature stored in-memory from the feature that
// is in etcd.
//
// It returns the new value and whether Rancher must be restarted. This is the
// case when (1) the feature is non-dynamic and (2) the value was changed.
func ReconcileFeatures(obj *v3.Feature) (*bool, bool) {
	feature := features.GetFeatureByName(obj.Name)

	// possible feature watch renamed, or no longer used by rancher
	if feature == nil {
		return nil, false
	}

	newVal := obj.Status.LockedValue
	if newVal == nil {
		newVal = obj.Spec.Value
	}

	if features.RequireRestarts(feature, obj) {
		return newVal, true
	}

	if newVal == nil {
		feature.Unset()
	} else {
		feature.Set(*newVal)
	}

	return newVal, false
}
