package feature

import (
	"context"
	"fmt"
	"github.com/coreos/etcd/etcdserver/api/v2http/httptypes"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func RegisterEarly(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {

}

type SettingController struct {
	settings v3.SettingInterface
}

func newFeatSettingController(mgmt *config.ManagementContext) *SettingController {
	s := &SettingController{
		settings: mgmt.Management.Settings(""),
	}
	return s
}

func (s *SettingController) sync(key string, obj *v3.Setting) (runtime.Object, error) {
	if !features.SettingExists(key) {
		return nil, nil
	}

	err := features.SetFeature(key, obj.Value)
	if err != nil {
		return nil, fmt.Errorf("could not set feature setting %s: %v", key, err)
	}
}