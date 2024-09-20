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
	"github.com/rancher/shepherd/clients/rancher/catalog"
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
	namespace      = "fleet-default"
	containerImage = "nginx"
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

	f.clustersConfig = new(etcdsnapshot.Config)
	config.LoadConfig(etcdsnapshot.ConfigurationFileKey, f.clustersConfig)

}

func (f *FleetWithSnapshotTestSuite) TestSnapshotRestore() {
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
		client       *rancher.Client
	}{
		{" Restore cluster config Kubernetes version and etcd", snapshotRestoreAll, f.client},
	}

	for _, tt := range tests {
		defer f.session.Cleanup()

		fleetVersion, err := fleet.GetDeploymentVersion(f.client, fleet.FleetControllerName, fleet.LocalName)
		require.NoError(f.T(), err)

		chartVersion, err := f.client.Catalog.GetLatestChartVersion(fleet.FleetName, catalog.RancherChartRepo)
		require.NoError(f.T(), err)

		// fleet chart version may contain chart version info that is a superset of the version reported by the fleet deployment.
		require.Contains(f.T(), chartVersion, fleetVersion[1:])

		urlQuery, err := url.ParseQuery(fmt.Sprintf("labelSelector=%s=%s", "cattle.io/os", "windows"))
		require.NoError(f.T(), err)

		steveClient, err := f.client.Steve.ProxyDownstream(f.clusterID)
		require.NoError(f.T(), err)

		winsNodeList, err := steveClient.SteveType("node").List(urlQuery)
		require.NoError(f.T(), err)

		if len(winsNodeList.Data) > 0 {
			f.fleetGitRepo.Spec.Paths = []string{fleet.GitRepoPathWindows}
			f.fleetGitRepo.Name += "windows"
			tt.name += "windows"
		}

		f.client, err = f.client.ReLogin()
		require.NoError(f.T(), err)

		f.Run(fleet.FleetName+" "+fleetVersion+tt.name, func() {
			var isRKE1 = false

			clusterObject, _, _ := extensionscluster.GetProvisioningClusterByName(f.client, f.client.RancherConfig.ClusterName, namespace)
			if clusterObject == nil {
				_, err := f.client.Management.Cluster.ByID(f.clusterID)
				require.NoError(f.T(), err)

				isRKE1 = true
			}

			containerTemplate := workloads.NewContainer(containerImage, containerImage, corev1.PullAlways, []corev1.VolumeMount{}, []corev1.EnvFromSource{}, nil, nil, nil)

			podTemplate := workloads.NewPodTemplate([]corev1.Container{containerTemplate}, []corev1.Volume{}, []corev1.LocalObjectReference{}, nil, map[string]string{})
			deploymentTemplate := workloads.NewDeploymentTemplate(etcdsnapshot.InitialWorkload, "default", podTemplate, true, nil)

			var gitRepoObject *steveV1.SteveAPIObject

			logrus.Info("deploying fleet post-snapshot to test persistance after restore is complete")
			if isRKE1 {
				cluster, snapshotName, postDeploymentResp, postServiceResp := etcdsnapshot.SnapshotRKE1(f.T(), f.client, &podTemplate, deploymentTemplate, f.client.RancherConfig.ClusterName, f.clusterID, tt.etcdSnapshot, isRKE1)

				logrus.Info("Deploying public fleet gitRepo")
				gitRepoObject, err = extensionsfleet.CreateFleetGitRepo(f.client, f.fleetGitRepo)
				require.NoError(f.T(), err)

				err = fleet.VerifyGitRepo(f.client, gitRepoObject.ID, f.clusterID, fleet.Namespace+"/"+f.client.RancherConfig.ClusterName)
				require.NoError(f.T(), err)

				etcdsnapshot.RestoreRKE1(f.T(), f.client, snapshotName, tt.etcdSnapshot, cluster, f.clusterID)

				_, err = steveClient.SteveType(etcdsnapshot.DeploymentSteveType).ByID(postDeploymentResp.ID)
				require.Error(f.T(), err)

				_, err = steveClient.SteveType("service").ByID(postServiceResp.ID)
				require.Error(f.T(), err)

			} else {
				cluster, snapshotName, postDeploymentResp, postServiceResp := etcdsnapshot.SnapshotV2Prov(f.T(), f.client, &podTemplate, deploymentTemplate, f.client.RancherConfig.ClusterName, f.clusterID, tt.etcdSnapshot, isRKE1)

				logrus.Info("Deploying public fleet gitRepo")
				gitRepoObject, err = extensionsfleet.CreateFleetGitRepo(f.client, f.fleetGitRepo)
				require.NoError(f.T(), err)

				err = fleet.VerifyGitRepo(f.client, gitRepoObject.ID, f.clusterID, fleet.Namespace+"/"+f.client.RancherConfig.ClusterName)
				require.NoError(f.T(), err)

				etcdsnapshot.RestoreV2Prov(f.T(), f.client, snapshotName, tt.etcdSnapshot, cluster, f.clusterID)

				_, err = steveClient.SteveType(etcdsnapshot.DeploymentSteveType).ByID(postDeploymentResp.ID)
				require.Error(f.T(), err)

				_, err = steveClient.SteveType("service").ByID(postServiceResp.ID)
				require.Error(f.T(), err)
			}

			// fleet gitRepo should persist after restore, because fleet is a rancher-local-cluster level resource. So it will redeploy after restore.
			err = fleet.VerifyGitRepo(f.client, gitRepoObject.ID, f.clusterID, fleet.Namespace+"/"+f.client.RancherConfig.ClusterName)
			require.NoError(f.T(), err)

			// running normal restore afterared to verify that both post-snapshot-restore and pre-snapshot-restore will keep fleet resources
			logrus.Info("keeping fleet deployed pre-snapshot to test fleet as a pre-snapshot resource")
			etcdsnapshot.SnapshotRestore(f.T(), f.client, f.client.RancherConfig.ClusterName, tt.etcdSnapshot, containerImage)

			err = fleet.VerifyGitRepo(f.client, gitRepoObject.ID, f.clusterID, fleet.Namespace+"/"+f.client.RancherConfig.ClusterName)
			require.NoError(f.T(), err)

		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSnapshotRestoreWithFleetTestSuite(t *testing.T) {
	suite.Run(t, new(FleetWithSnapshotTestSuite))
}
