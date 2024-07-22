// We're protecting this file with a build tag because it depends on github.com/containers/image which depends on C
// libraries that we can't and don't want to build unless we're going to run this integration setup program.

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/creasty/defaults"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/k3d"
	rancherClient "github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/token"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

var (
	agentImage = os.Getenv("CATTLE_AGENT_IMAGE")
)

const (
	k3dClusterNameBasename = "k3d-cluster"
)

// main creates a test namespace and cluster for use in integration tests.
func main() {
	rancherConfig := new(rancherClient.Config)

	ipAddress := getOutboundIP()
	hostURL := fmt.Sprintf("%s:443", ipAddress.String())

	var userToken *management.Token
	logrus.Infof("CATTLE AGENT IS %s", agentImage)
	err := kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		userToken, err = token.GenerateUserToken(&management.User{
			Username: "admin",
			Password: "admin",
		}, hostURL)
		if err != nil {
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		logrus.Fatalf("Error with generating admin token: %v", err)
	}

	clusterName := namegen.AppendRandomString(k3dClusterNameBasename)

	cleanup := true
	rancherConfig.AdminToken = userToken.Token
	rancherConfig.Host = hostURL
	rancherConfig.Cleanup = &cleanup
	rancherConfig.ClusterName = clusterName

	if err := defaults.Set(rancherConfig); err != nil {
		logrus.Fatalf("error with setting up config file: %v", err)
	}

	config.WriteConfig(rancherClient.ConfigurationFileKey, rancherConfig)

	testSession := session.NewSession()

	var client *rancherClient.Client

	agentSetting := &v3.Setting{}
	var agentSettingResp *v1.SteveAPIObject
	client, err = rancherClient.NewClient("", testSession)
	if err != nil {
		logrus.Fatalf("error instantiating client: %v", err)
	}

	agentSettingResp, err = client.Steve.SteveType("management.cattle.io.setting").ByID("agent-image")
	if err != nil {
		logrus.Fatalf("error get agent-image setting: %v", err)
	}

	err = v1.ConvertToK8sType(agentSettingResp.JSONResp, agentSetting)
	if err != nil {
		logrus.Fatalf("error converting to k8s type: %v", err)
	}
	agentSetting.Value = agentImage

	_, err = client.Steve.SteveType("management.cattle.io.setting").Update(agentSettingResp, agentSetting)
	if err != nil {
		logrus.Fatalf("error updating agent-image setting: %v", err)
	}

	_, err = k3d.CreateAndImportK3DCluster(client, clusterName, agentImage, "", 1, 0, true)
	if err != nil {
		logrus.Fatalf("error creating and importing a k3d cluster: %v", err)
	}
}

// Get preferred outbound ip of this machine
func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		logrus.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}
