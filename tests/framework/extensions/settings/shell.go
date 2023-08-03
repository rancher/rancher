package settings

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
)

// ShellVersion is a helper that gets the shell setting json based on the ID and return the shell image value.
func ShellVersion(client *rancher.Client, clusterID, resourceName string) (string, error) {
	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return "", err
	}

	shellSetting := &v3.Setting{}
	shellSettingResp, err := steveClient.SteveType("management.cattle.io.setting").ByID("shell-image")
	if err != nil {
		return "", err
	}

	err = v1.ConvertToK8sType(shellSettingResp.JSONResp, shellSetting)
	if err != nil {
		return "", err
	}
	image := shellSetting.Value

	return image, nil

}
