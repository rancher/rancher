package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	"github.com/rancher/rancher/tests/framework/extensions/token"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/file"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

type wrappedConfig struct {
	Configuration *rancher.Config `yaml:"rancher"`
}

var (
	adminPassword = os.Getenv("ADMIN_PASSWORD")
	host          = os.Getenv("HA_HOST")

	configFileName = file.Name("cattle-config.yaml")
)

func main() {
	rancherConfig := new(rancher.Config)
	rancherConfig.Host = host
	isCleanupEnabled := true
	rancherConfig.Cleanup = &isCleanupEnabled

	adminUser := &management.User{
		Username: "admin",
		Password: adminPassword,
	}

	//create admin token
	var adminToken *management.Token
	err := kwait.Poll(500*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		adminToken, err = token.GenerateUserToken(adminUser, host)
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		logrus.Errorf("error creating admin token: %v", err)
	}
	rancherConfig.AdminToken = adminToken.Token

	//create config file
	configWrapped := &wrappedConfig{
		Configuration: rancherConfig,
	}
	configData, err := yaml.Marshal(configWrapped)
	if err != nil {
		logrus.Errorf("error marshaling: %v", err)
	}
	_, err = configFileName.NewFile(configData)
	if err != nil {
		logrus.Fatalf("error writing yaml: %v", err)
	}
	err = configFileName.SetEnvironmentKey(config.ConfigEnvironmentKey)
	if err != nil {
		logrus.Fatalf("error while setting environment path: %v", err)
	}

	session := session.NewSession()
	client, err := rancher.NewClient("", session)
	if err != nil {
		logrus.Errorf("error creating client: %v", err)
	}

	clusterList, err := client.Management.Cluster.List(&types.ListOpts{})
	if err != nil {
		logrus.Errorf("error getting cluster list: %v", err)
	}

	for _, c := range clusterList.Data {
		isLocalCluster := c.ID == "local"
		if !isLocalCluster {
			opts := metav1.ListOptions{
				FieldSelector:  "metadata.name=" + c.ID,
				TimeoutSeconds: &defaults.WatchTimeoutSeconds,
			}

			err := client.Management.Cluster.Delete(&c)
			if err != nil {
				logrus.Errorf("error delete cluster call: %v", err)
			}

			watchInterface, err := client.GetManagementWatchInterface(management.ClusterType, opts)
			if err != nil {
				logrus.Errorf("error while getting the watch interface: %v", err)
			}

			wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
				if event.Type == watch.Error {
					return false, fmt.Errorf("there was an error deleting cluster")
				} else if event.Type == watch.Deleted {
					return true, nil
				}
				return false, nil
			})
		}
	}
}
