//go:build validation

package snapshot

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/etcdsnapshot"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionscluster "github.com/rancher/shepherd/extensions/clusters"
	extensionsfleet "github.com/rancher/shepherd/extensions/fleet"
	"github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	containerImage        = "nginx"
	windowsContainerImage = "mcr.microsoft.com/windows/servercore/iis"
)

type FleetWithSnapshotTestSuite struct {
	suite.Suite
	client         *rancher.Client
	session        *session.Session
	fleetGitRepo   *v1alpha1.GitRepo
	clustersConfig *etcdsnapshot.Config
	clusterID      string
}

func (f *FleetWithSnapshotTestSuite) TearDownSuite() {
	f.session.Cleanup()
}

func (f *FleetWithSnapshotTestSuite) SetupSuite() {
	f.session = session.NewSession()

	client, err := rancher.NewClient("", f.session)
	require.NoError(f.T(), err)

	f.client = client

	f.fleetGitRepo = &v1alpha1.GitRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fleet.FleetMetaName + namegenerator.RandStringLower(5),
			Namespace: fleet.Namespace,
		},
		Spec: v1alpha1.GitRepoSpec{
			Repo:            fleet.ExampleRepo,
			Branch:          fleet.BranchName,
			Paths:           []string{fleet.GitRepoPathLinux},
			CorrectDrift:    &v1alpha1.CorrectDrift{},
			ImageScanCommit: v1alpha1.CommitSpec{AuthorName: "", AuthorEmail: ""},
			Targets:         []v1alpha1.GitTarget{{ClusterName: f.client.RancherConfig.ClusterName}},
		},
	}

	f.client, err = f.client.ReLogin()
	require.NoError(f.T(), err)

	userConfig := new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, userConfig)

	clusterObject, _, _ := extensionscluster.GetProvisioningClusterByName(f.client, f.client.RancherConfig.ClusterName, fleet.Namespace)
	if clusterObject != nil {
		status := &provv1.ClusterStatus{}
		err := steveV1.ConvertToK8sType(clusterObject.Status, status)
		require.NoError(f.T(), err)

		f.clusterID = status.ClusterName
	} else {
		f.clusterID, err = extensionscluster.GetClusterIDByName(f.client, f.client.RancherConfig.ClusterName)
		require.NoError(f.T(), err)
	}

	podErrors := pods.StatusPods(f.client, f.clusterID)
	require.Empty(f.T(), podErrors)

}

func (f *FleetWithSnapshotTestSuite) TestSnapshotThenFleetRestore() {
	snapshotRestoreAll := &etcdsnapshot.Config{
		UpgradeKubernetesVersion:     "",
		SnapshotRestore:              "all",
		ControlPlaneConcurrencyValue: "15%",
		ControlPlaneUnavailableValue: "3",
		WorkerConcurrencyValue:       "20%",
		WorkerUnavailableValue:       "15%",
		RecurringRestores:            1,
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
	}{
		{" Restore cluster config Kubernetes version and etcd", snapshotRestoreAll},
	}

	for _, tt := range tests {
		testSession := session.NewSession()
		defer testSession.Cleanup()
		client, err := f.client.WithSession(testSession)
		require.NoError(f.T(), err)

		fleetVersion, err := fleet.GetDeploymentVersion(client, fleet.FleetControllerName, fleet.LocalName)
		require.NoError(f.T(), err)

		urlQuery, err := url.ParseQuery(fmt.Sprintf("labelSelector=%s=%s", "cattle.io/os", "windows"))
		require.NoError(f.T(), err)

		steveClient, err := client.Steve.ProxyDownstream(f.clusterID)
		require.NoError(f.T(), err)

		winsNodeList, err := steveClient.SteveType("node").List(urlQuery)
		require.NoError(f.T(), err)

		image := containerImage

		if len(winsNodeList.Data) > 0 {
			f.fleetGitRepo.Spec.Paths = []string{fleet.GitRepoPathWindows}
			f.fleetGitRepo.Name += "windows"
			tt.name += "windows"
			image = windowsContainerImage
		}

		client, err = client.ReLogin()
		require.NoError(f.T(), err)

		f.Run(fleet.FleetName+" "+fleetVersion+tt.name, func() {
			var isRKE1 = false

			clusterObject, _, _ := extensionscluster.GetProvisioningClusterByName(client, client.RancherConfig.ClusterName, fleet.Namespace)
			if clusterObject == nil {
				_, err := client.Management.Cluster.ByID(f.clusterID)
				require.NoError(f.T(), err)

				isRKE1 = true
			}

			require.False(f.T(), isRKE1, "rke1 is not supported at this time. ")

			containerTemplate := workloads.NewContainer(containerImage, image, corev1.PullAlways, []corev1.VolumeMount{}, []corev1.EnvFromSource{}, nil, nil, nil)

			podTemplate := workloads.NewPodTemplate([]corev1.Container{containerTemplate}, []corev1.Volume{}, []corev1.LocalObjectReference{}, nil, map[string]string{})
			deploymentTemplate := workloads.NewDeploymentTemplate(etcdsnapshot.InitialWorkload, "default", podTemplate, true, nil)

			var gitRepoObject *steveV1.SteveAPIObject

			logrus.Info("deploying fleet post-snapshot to test persistance after restore is complete")

			cluster, snapshotName, postDeploymentResp, postServiceResp, err := etcdsnapshot.CreateAndValidateSnapshotV2Prov(client, &podTemplate, deploymentTemplate, client.RancherConfig.ClusterName, f.clusterID, tt.etcdSnapshot, isRKE1)
			require.NoError(f.T(), err)

			logrus.Info("Deploying public fleet gitRepo")
			gitRepoObject, err = extensionsfleet.CreateFleetGitRepo(client, f.fleetGitRepo)
			require.NoError(f.T(), err)

			err = fleet.VerifyGitRepo(client, gitRepoObject.ID, f.clusterID, fleet.Namespace+"/"+client.RancherConfig.ClusterName)
			require.NoError(f.T(), err)

			err = etcdsnapshot.RestoreAndValidateSnapshotV2Prov(client, snapshotName, tt.etcdSnapshot, cluster, f.clusterID)
			require.NoError(f.T(), err)

			_, err = steveClient.SteveType(etcdsnapshot.DeploymentSteveType).ByID(postDeploymentResp.ID)
			require.Error(f.T(), err)

			_, err = steveClient.SteveType("service").ByID(postServiceResp.ID)
			require.Error(f.T(), err)

			err = fleet.VerifyGitRepo(client, gitRepoObject.ID, f.clusterID, fleet.Namespace+"/"+client.RancherConfig.ClusterName)
			require.NoError(f.T(), err)

		})
	}
}

func (f *FleetWithSnapshotTestSuite) TestFleetThenSnapshotRestore() {
	snapshotRestoreAll := &etcdsnapshot.Config{
		UpgradeKubernetesVersion:     "",
		SnapshotRestore:              "all",
		ControlPlaneConcurrencyValue: "15%",
		ControlPlaneUnavailableValue: "3",
		WorkerConcurrencyValue:       "20%",
		WorkerUnavailableValue:       "15%",
		RecurringRestores:            1,
	}

	tests := []struct {
		name         string
		etcdSnapshot *etcdsnapshot.Config
	}{
		{" Restore cluster config Kubernetes version and etcd", snapshotRestoreAll},
	}

	for _, tt := range tests {
		testSession := session.NewSession()
		defer testSession.Cleanup()
		client, err := f.client.WithSession(testSession)
		require.NoError(f.T(), err)

		fleetVersion, err := fleet.GetDeploymentVersion(client, fleet.FleetControllerName, fleet.LocalName)
		require.NoError(f.T(), err)

		urlQuery, err := url.ParseQuery(fmt.Sprintf("labelSelector=%s=%s", "cattle.io/os", "windows"))
		require.NoError(f.T(), err)

		steveClient, err := client.Steve.ProxyDownstream(f.clusterID)
		require.NoError(f.T(), err)

		winsNodeList, err := steveClient.SteveType("node").List(urlQuery)
		require.NoError(f.T(), err)

		image := containerImage
		if len(winsNodeList.Data) > 0 {
			f.fleetGitRepo.Spec.Paths = []string{fleet.GitRepoPathWindows}
			f.fleetGitRepo.Name += "windows"
			tt.name += "windows"
			image = windowsContainerImage
		}

		client, err = client.ReLogin()
		require.NoError(f.T(), err)

		f.Run(fleet.FleetName+" "+fleetVersion+tt.name, func() {

			var gitRepoObject *steveV1.SteveAPIObject
			logrus.Info("Deploying public fleet gitRepo")
			gitRepoObject, err = extensionsfleet.CreateFleetGitRepo(client, f.fleetGitRepo)
			require.NoError(f.T(), err)

			err = fleet.VerifyGitRepo(client, gitRepoObject.ID, f.clusterID, fleet.Namespace+"/"+client.RancherConfig.ClusterName)
			require.NoError(f.T(), err)

			logrus.Info("having fleet deployed pre-snapshot to test fleet as a pre-snapshot resource")
			err = etcdsnapshot.CreateAndValidateSnapshotRestore(client, client.RancherConfig.ClusterName, tt.etcdSnapshot, image)
			require.NoError(f.T(), err)

			err = fleet.VerifyGitRepo(client, gitRepoObject.ID, f.clusterID, fleet.Namespace+"/"+client.RancherConfig.ClusterName)
			require.NoError(f.T(), err)

		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSnapshotRestoreWithFleetTestSuite(t *testing.T) {
	suite.Run(t, new(FleetWithSnapshotTestSuite))
}
