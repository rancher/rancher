package lastlogin

import (
	"fmt"
	"strconv"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	client "github.com/rancher/shepherd/clients/rancher/generated/management/v3"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	lastloginLabel = "cattle.io/last-login"
)

func getLastLoginTime(labels map[string]string) (lastLogin time.Time, err error) {
	value, exists := isLabelPresent(labels)

	if !exists || value == "" {
		return
	}
	epochTime, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return
	}
	lastLogin = convertEpochToTime(epochTime)
	return
}

func convertEpochToTime(epochTime int64) time.Time {
	return time.Unix(epochTime, 0)
}

func isLabelPresent(labels map[string]string) (string, bool) {
	value, exists := labels[lastloginLabel]
	return value, exists
}

func getUserAfterLogin(rancherClient *rancher.Client, user client.User) (userDetails *v3.User, err error) {

	_, err = rancherClient.AsUser(&user)
	if err != nil {
		return
	}
	listOpt := v1.ListOptions{
		FieldSelector: "metadata.name=" + user.ID,
	}
	userList, err := rancherClient.WranglerContext.Mgmt.User().List(listOpt)

	if len(userList.Items) == 0 {
		return nil, fmt.Errorf("User %s not found", user.ID)
	}
	userDetails = &userList.Items[0]

	return
}
