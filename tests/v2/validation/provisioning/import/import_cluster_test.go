//go:build (validation || extended) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !cluster.any && !cluster.custom && !cluster.nodedriver && !sanity && !stress

package imported

import (
	"testing"
	"time"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/sirupsen/logrus"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	k3sImportPackageName = "k3sToImport"
)

type ImportProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	provisioningConfig *provisioninginput.Config
}

func (r *ImportProvisioningTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *ImportProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	r.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(r.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(r.T(), err)

	r.standardUserClient = standardUserClient

	corralConfig := corral.Configurations()
	err = corral.SetupCorralConfig(corralConfig.CorralConfigVars, corralConfig.CorralConfigUser, corralConfig.CorralSSHPath)
	require.NoError(r.T(), err, "error reading corral configs")
}

func (r *ImportProvisioningTestSuite) TestProvisioningImportK3SCluster() {
	corralPackage := corral.PackagesConfig()

	if corralPackage.HasCustomRepo != "" {
		err := corral.SetCustomRepo(corralPackage.HasCustomRepo)
		require.Nil(r.T(), err, "error setting remote repo")
	}

	if len(corralPackage.CorralPackageImages) == 0 {
		r.T().Error("No Corral Packages to Test")
	}

	_, ok := corralPackage.CorralPackageImages[k3sImportPackageName]
	require.True(r.T(), ok, "Please ensure %s package is in corralPackageImages", k3sImportPackageName)

	var importPackageName string

	for packageName, packageImage := range corralPackage.CorralPackageImages {
		newPackageName := namegen.AppendRandomString(packageName)

		if packageName == k3sImportPackageName {
			importPackageName = newPackageName

			corralRun, err := corral.CreateCorral(r.session, newPackageName, packageImage, corralPackage.HasDebug, corralPackage.HasDebug)
			require.NoError(r.T(), err, "error creating corral %v", packageName)

			r.T().Logf("Corral %v created successfully", packageName)
			require.NotNil(r.T(), corralRun, "corral run had no restConfig")
		}

	}

	importCluster := apiv1.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      namegen.AppendRandomString("auto-import"),
			Namespace: "fleet-default",
		},
	}

	backoff := wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1.1,
		Jitter:   0.1,
		Steps:    40,
	}

	decodedKubeConfig := []byte{}
	err := wait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		decodedKubeConfig, err = corral.GetKubeConfig(importPackageName)
		if err != nil {
			return false, nil
		}
		return true, nil
	})

	require.NoError(r.T(), err)

	config, err := clientcmd.RESTConfigFromKubeConfig(decodedKubeConfig)
	require.NoError(r.T(), err)

	_, err = clusters.CreateK3SRKE2Cluster(r.client, &importCluster)
	require.NoError(r.T(), err)

	updatedCluster := new(apiv1.Cluster)

	err = wait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		updatedCluster, _, err = clusters.GetProvisioningClusterByName(r.client, importCluster.Name, importCluster.Namespace)
		require.NoError(r.T(), err)

		if updatedCluster.Status.ClusterName != "" {
			return true, nil
		}

		return false, nil
	})
	require.NoError(r.T(), err)

	logrus.Info(updatedCluster.Status.ClusterName)
	err = clusters.ImportCluster(r.client, updatedCluster, config)
	require.NoError(r.T(), err)

	backoff = wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1.1,
		Jitter:   0.1,
		Steps:    100,
	}

	err = wait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		updatedCluster, _, err = clusters.GetProvisioningClusterByName(r.client, importCluster.Name, importCluster.Namespace)
		require.NoError(r.T(), err)

		if updatedCluster.Status.Ready {
			return true, nil
		}

		return false, nil
	})
	require.NoError(r.T(), err)

	podErrors := pods.StatusPods(r.client, updatedCluster.Status.ClusterName)
	require.Empty(r.T(), podErrors)

}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestImportProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(ImportProvisioningTestSuite))
}
