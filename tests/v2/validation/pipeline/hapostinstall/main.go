package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	b64 "encoding/base64"

	"github.com/rancher/rancher/tests/v2/actions/pipeline"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/token"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/file"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

var (
	adminPassword = os.Getenv("ADMIN_PASSWORD")
	host          = os.Getenv("HA_HOST")

	clusterID = "local"

	configFileName       = file.Name("cattle-config.yaml")
	environmentsFileName = "environments.groovy"

	tokenEnvironmentKey      = "HA_TOKEN"
	kubeconfigEnvironmentKey = "HA_KUBECONFIG"
)

type wrappedConfig struct {
	Configuration *rancher.Config `yaml:"rancher"`
}

func main() {
	rancherConfig := new(rancher.Config)
	rancherConfig.Host = host
	isCleanupEnabled := false
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
		logrus.Fatalf("error creating admin token: %v", err)
	}
	rancherConfig.AdminToken = adminToken.Token

	//create config file
	configWrapped := &wrappedConfig{
		Configuration: rancherConfig,
	}
	configData, err := yaml.Marshal(configWrapped)
	if err != nil {
		logrus.Fatalf("error marshaling: %v", err)
	}
	_, err = configFileName.NewFile(configData)
	if err != nil {
		logrus.Fatalf("error writing yaml: %v", err)
	}
	err = configFileName.SetEnvironmentKey(config.ConfigEnvironmentKey)
	if err != nil {
		logrus.Fatalf("error while setting environment path: %v", err)
	}

	//generate kubeconfig
	session := session.NewSession()
	client, err := rancher.NewClient("", session)
	if err != nil {
		logrus.Fatalf("error creating client: %v", err)
	}

	err = pipeline.UpdateEULA(client)
	if err != nil {
		logrus.Fatalf("error updating EULA: %v", err)
	}

	cluster, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		logrus.Fatalf("error getting cluster: %v", err)
	}
	kubeconfig, err := client.Management.Cluster.ActionGenerateKubeconfig(cluster)
	if err != nil {
		logrus.Fatalf("error getting kubeconfig: %v", err)
	}

	//create groovy environments file
	kubeconfigb64 := b64.StdEncoding.EncodeToString([]byte(kubeconfig.Config))
	kubeconfigEnvironment := newGroovyEnvStr(kubeconfigEnvironmentKey, kubeconfigb64)
	tokenEnvironment := newGroovyEnvStr(tokenEnvironmentKey, adminToken.Token)
	environmentsData := strings.Join([]string{tokenEnvironment, kubeconfigEnvironment}, "\n")
	err = os.WriteFile(environmentsFileName, []byte(environmentsData), 0644)
	if err != nil {
		logrus.Fatalf("error writing yaml: %v", err)
	}

}

func newGroovyEnvStr(key, value string) string {
	prefix := "env"
	return fmt.Sprintf("%v.%v='%v'", prefix, key, value)
}
