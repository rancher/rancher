package pipelineutils

import (
	"fmt"
	"time"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/token"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	clusterName = "local"
)

func CreateAdminToken(password string, rancherConfig *rancher.Config) (string, error) {
	adminUser := &management.User{
		Username: "admin",
		Password: password,
	}

	hostURL := rancherConfig.Host
	var userToken *management.Token
	err := kwait.Poll(500*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		userToken, err = token.GenerateUserToken(adminUser, hostURL)
		if err != nil {
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		return "", err
	}

	return userToken.Token, nil
}

func PostRancherInstall(adminClient *rancher.Client, adminPassword string) error {
	clusterID, err := clusters.GetClusterIDByName(adminClient, clusterName)
	if err != nil {
		return err
	}

	steveClient, err := adminClient.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	timeStamp := time.Now().Format(time.RFC3339)
	settingEULA := v3.Setting{
		ObjectMeta: metav1.ObjectMeta{
			Name: "eula-agreed",
		},
		Default: timeStamp,
		Value:   timeStamp,
	}

	urlSetting := &v3.Setting{}

	_, err = steveClient.SteveType("management.cattle.io.setting").Create(settingEULA)
	if err != nil {
		return err
	}

	urlSettingResp, err := steveClient.SteveType("management.cattle.io.setting").ByID("server-url")
	if err != nil {
		return err
	}

	err = v1.ConvertToK8sType(urlSettingResp.JSONResp, urlSetting)
	if err != nil {
		return err
	}

	urlSetting.Value = fmt.Sprintf("https://%s", adminClient.RancherConfig.Host)

	_, err = steveClient.SteveType("management.cattle.io.setting").Update(urlSettingResp, urlSetting)
	if err != nil {
		return err
	}

	userList, err := adminClient.Management.User.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"username": "admin",
		},
	})
	if err != nil {
		return err
	} else if len(userList.Data) == 0 {
		return fmt.Errorf("admin user not found")
	}

	adminUser := &userList.Data[0]
	setPasswordInput := management.SetPasswordInput{
		NewPassword: adminPassword,
	}
	_, err = adminClient.Management.User.ActionSetpassword(adminUser, &setPasswordInput)

	return err
}
