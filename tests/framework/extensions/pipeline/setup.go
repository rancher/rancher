package pipeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/token"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	clusterName = "local"
)

// CreateAdminToken is a function that creates a new admin token
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

// PostRancherInstall is a function that updates EULA after the rancher installation
// and sets new admin password to the admin user
func PostRancherInstall(adminClient *rancher.Client, adminPassword string) error {
	clusterID, err := clusters.GetClusterIDByName(adminClient, clusterName)
	if err != nil {
		return err
	}

	UpdateEULA(adminClient, clusterID)

	var userList *management.UserCollection
	err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		userList, err = adminClient.Management.User.List(&types.ListOpts{
			Filters: map[string]interface{}{
				"username": "admin",
			},
		})
		if err != nil {
			return false, err
		} else if len(userList.Data) == 0 {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	adminUser := &userList.Data[0]
	setPasswordInput := management.SetPasswordInput{
		NewPassword: adminPassword,
	}
	_, err = adminClient.Management.User.ActionSetpassword(adminUser, &setPasswordInput)

	return err
}

// UpdateEULA is a function that updates EULA after the rancher installation
func UpdateEULA(adminClient *rancher.Client, clusterID string) error {
	clusterID, err := clusters.GetClusterIDByName(adminClient, clusterID)
	if err != nil {
		return err
	}

	steveClient, err := adminClient.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	urlSetting := &v3.Setting{}
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

	timeStamp := time.Now().Format(time.RFC3339)
	settingEULA := v3.Setting{
		ObjectMeta: metav1.ObjectMeta{
			Name: "eula-agreed",
		},
		Default: timeStamp,
		Value:   timeStamp,
	}

	_, err = steveClient.SteveType("management.cattle.io.setting").Create(settingEULA)
	if err != nil && strings.Contains(err.Error(), "409 Conflict") {
		return nil
	}

	return err
}
