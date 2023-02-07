package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/corral"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	aws "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/aws"
	"github.com/rancher/rancher/tests/framework/extensions/token"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	passwordgenerator "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/rancher/rancher/tests/v2/validation/registries"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

var (
	adminPassword = os.Getenv("ADMIN_PASSWORD")
)

const (
	globalCorralName     = "rancherglobalregistry"
	registryEnabledName  = "downstreamregistryenabled"
	registryDisabledName = "downstreamregistrydisabled"
	clusterName          = "local"
)

func main() {
	rancherConfig := new(rancher.Config)
	config.LoadConfig(rancher.ConfigurationFileKey, rancherConfig)

	registryEnabledUsername, err := corral.GetCorralEnvVar(registryEnabledName, "registry_username")
	if err != nil {
		logrus.Fatalf("error getting registry username %v", err)
	}
	logrus.Infof("Registry Enabled Username: %s", registryEnabledUsername)

	registryEnabledPassword, err := corral.GetCorralEnvVar(registryEnabledName, "registry_password")
	if err != nil {
		logrus.Fatalf("error getting registry password %v", err)
	}
	logrus.Infof("Registry Enabled Password: %s", registryEnabledPassword)

	registryEnabledFqdn, err := corral.GetCorralEnvVar(registryEnabledName, "registry_fqdn")
	if err != nil {
		logrus.Fatalf("error getting registry fqdn %v", err)
	}
	logrus.Infof("Enabled FQDN: %s", registryEnabledFqdn)

	registryDisabledFqdn, err := corral.GetCorralEnvVar(registryDisabledName, "registry_fqdn")
	if err != nil {
		logrus.Fatalf("error getting registry fqdn %v", err)
	}
	logrus.Infof("Disabled FQDN: %s", registryDisabledFqdn)

	globalRegistryFqdn, err := corral.GetCorralEnvVar(globalCorralName, "registry_fqdn")
	if err != nil {
		logrus.Fatalf("error getting registry fqdn %v", err)
	}
	logrus.Infof("Global Registry FQDN: %s", globalRegistryFqdn)

	password, err := corral.GetCorralEnvVar(globalCorralName, "bootstrap_password")
	if err != nil {
		logrus.Fatalf("error getting password %v", err)
	}

	token, err := createAdminToken(password, rancherConfig)
	if err != nil {
		logrus.Fatalf("error with generating admin token: %v", err)
	}

	rancherConfig.AdminToken = token
	config.UpdateConfig(rancher.ConfigurationFileKey, rancherConfig)
	//provision clusters for test
	session := session.NewSession()
	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	kubernetesVersions := clustersConfig.RKE1KubernetesVersions
	cnis := clustersConfig.CNIs
	nodesAndRoles := clustersConfig.NodesAndRolesRKE1

	client, err := rancher.NewClient(token, session)
	if err != nil {
		logrus.Fatalf("error creating admin client: %v", err)
	}

	err = postRancherInstall(client)
	if err != nil {
		logrus.Fatalf("error with admin user service account secret: %v", err)
	}

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = passwordgenerator.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	if err != nil {
		logrus.Fatalf("error creating admin client: %v", err)
	}

	newUser.Password = user.Password

	if err != nil {
		logrus.Fatalf("error creating standard user client: %v", err)
	}
	var privateRegistriesNoauth []management.PrivateRegistry
	privateRegistry := management.PrivateRegistry{}
	privateRegistry.URL = registryEnabledFqdn
	privateRegistry.IsDefault = true
	privateRegistry.Password = registryEnabledPassword
	privateRegistry.User = registryEnabledUsername
	privateRegistriesNoauth = append(privateRegistriesNoauth, privateRegistry)

	// create cluster with auth registry
	noauthClusterName, err := createTestCluster(client, client, 1, "noauthregcluster", cnis[0], kubernetesVersions[0], nodesAndRoles, privateRegistriesNoauth)
	if err != nil {
		logrus.Fatalf("error creating cluster: %v", err)
	}

	var privateRegistriesAuth []management.PrivateRegistry
	privateRegistry = management.PrivateRegistry{}
	privateRegistry.URL = registryDisabledFqdn
	privateRegistry.IsDefault = true
	privateRegistry.Password = ""
	privateRegistry.User = ""
	privateRegistriesAuth = append(privateRegistriesAuth, privateRegistry)

	authClusterName, err := createTestCluster(client, client, 1, "authregcluster", cnis[0], kubernetesVersions[0], nodesAndRoles, privateRegistriesAuth)
	if err != nil {
		logrus.Fatalf("error creating cluster: %v", err)
	}

	noAuthCluster := new(registries.Cluster)
	noAuthCluster.Name = noauthClusterName[0]
	noAuthCluster.Auth = false
	noAuthCluster.URL = privateRegistriesNoauth[0].URL

	authCluster := new(registries.Cluster)
	authCluster.Name = authClusterName[0]
	authCluster.Auth = true
	authCluster.URL = privateRegistriesAuth[0].URL

	localCluster := new(registries.Cluster)
	localCluster.Name = "local"
	localCluster.Auth = false
	localCluster.URL = globalRegistryFqdn

	registriesConfig := new(registries.Config)
	clusters := []registries.Cluster{*noAuthCluster, *authCluster, *localCluster}
	registriesConfig.Clusters = clusters

	config.UpdateConfig(registries.RegistriesConfigKey, registriesConfig)
}

func createAdminToken(password string, rancherConfig *rancher.Config) (string, error) {
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

func createTestCluster(client, adminClient *rancher.Client, numClusters int, clusterNameBase, cni, kubeVersion string, nodesAndRoles []nodepools.NodeRoles, privateRegistries []management.PrivateRegistry) ([]string, error) {
	clusterNames := []string{}
	for i := 0; i < numClusters; i++ {
		clusterName := namegen.AppendRandomString(clusterNameBase)
		clusterNames = append(clusterNames, clusterName)
		cluster := clusters.NewRKE1ClusterConfig(clusterName, cni, kubeVersion, client)

		if len(privateRegistries) > 0 {
			cluster.RancherKubernetesEngineConfig.PrivateRegistries = privateRegistries
		}

		clusterResp, err := clusters.CreateRKE1Cluster(client, cluster)
		if err != nil {
			return nil, err
		}

		err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
			cluster, err := client.Management.Cluster.ByID(clusterResp.ID)
			if err != nil {
				return false, nil
			} else if cluster != nil && cluster.ID == clusterResp.ID {
				return true, nil
			}
			return false, nil
		})

		if err != nil {
			return nil, err
		}

		nodeTemplateResp, err := aws.CreateAWSNodeTemplate(client)
		if err != nil {
			return nil, err
		}

		err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
			nodeTemplate, err := client.Management.NodeTemplate.ByID(nodeTemplateResp.ID)
			if err != nil {
				return false, nil
			} else if nodeTemplate != nil && nodeTemplate.ID == nodeTemplateResp.ID {
				return true, nil
			}
			return false, nil
		})

		if err != nil {
			return nil, err
		}

		_, err = nodepools.NodePoolSetup(client, nodesAndRoles, clusterResp.ID, nodeTemplateResp.ID)
		if err != nil {
			return nil, err
		}

		opts := metav1.ListOptions{
			FieldSelector:  "metadata.name=" + clusterResp.ID,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		}
		watchInterface, err := adminClient.GetManagementWatchInterface(management.ClusterType, opts)
		if err != nil {
			return nil, err
		}

		checkFunc := clusters.IsHostedProvisioningClusterReady

		err = wait.WatchWait(watchInterface, checkFunc)
		if err != nil {
			return nil, err
		}
	}
	return clusterNames, nil
}

func postRancherInstall(adminClient *rancher.Client) error {
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
