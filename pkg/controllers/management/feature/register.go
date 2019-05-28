package feature

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

func RegisterEarly(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	s := &SettingController{
		management.Management.Settings(""),
	}

	management.Management.Settings("").AddHandler(ctx, "feat-kontainer-driver", s.sync)
}

type SettingController struct {
	settings v3.SettingInterface
}

func (s *SettingController) sync(key string, obj *v3.Setting) (runtime.Object, error) {
	if !features.SettingExists(key) {
		return nil, nil
	}
	if obj.Value != "" {
		if err := features.SetFeature(key, obj.Value); err != nil {
			return nil, fmt.Errorf("could not set feature setting %s: %v", key, err)
		}
	} else {
		if err := features.SetFeature(key, obj.Default); err != nil {
			return nil, fmt.Errorf("could not set feature setting %s: %v", key, err)
		}
	}

	return nil, nil
}
