package feature

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	management.Management.Features("").AddHandler(ctx, "features-restart-handler", sync)
}

func sync(key string, obj *v3.Feature) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	val := obj.Status.Default
	if obj.Spec.Value != nil {
		val = *obj.Spec.Value
	}

	if err := ReconcileFeatures(obj, val); err != nil {
		logrus.Fatalf("%v", err)
	}

	return obj, nil
}

// ReconcileFeatures returns an error if the feature value in memory does
// not match the feature value in etcd AND the feature is non-dynamic.
// Otherwise, the feature value in memory is reconciled and no error is
// returned.
func ReconcileFeatures(obj *v3.Feature, newVal bool) error {
	feature := features.GetFeatureByName(obj.Name)

	if newVal == feature.Enabled() {
		return nil
	}

	if !feature.Dynamic() {
		return fmt.Errorf("feature flag [%s] value has changed, rancher must be restarted", obj.Name)
	}

	feature.Set(newVal)

	return nil
}
