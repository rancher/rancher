package auth

import (
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	authSettingController = "mgmt-auth-settings-controller"
)

type SettingController struct {
	settings v3.SettingInterface
}

func newAuthSettingController(mgmt *config.ManagementContext) *SettingController {
	n := &SettingController{
		settings: mgmt.Management.Settings(""),
	}
	return n
}

//sync is called periodically and on real updates
func (n *SettingController) sync(key string, obj *v3.Setting) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	switch obj.Name {
	case "auth-user-info-resync-cron":
		providerrefresh.UpdateRefreshCronTime(obj.Value)
	case "auth-user-info-max-age-seconds":
		providerrefresh.UpdateRefreshMaxAge(obj.Value)
	}

	return nil, nil
}
