package fleet

import (
	"errors"
	"strings"
	"time"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"

	extensionsfleet "github.com/rancher/shepherd/extensions/fleet"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

// The json/yaml config key for the corral package to be build ..
const (
	gitRepoConfigConfigurationFileKey = "gitRepo"

	FleetName          = "fleet"
	LocalName          = "local"
	HarvesterName      = "harvester"
	ExampleRepo        = "https://github.com/rancher/fleet-examples"
	BranchName         = "master"
	MatchKey           = "provider.cattle.io"
	MatchOperator      = "NotIn"
	FleetMetaName      = "automatedrepo-"
	Namespace          = "fleet-default"
	GitRepoPathLinux   = "simple"
	GitRepoPathWindows = "multi-cluster/windows-helm"
	CniCalico          = "calico"
)

// GitRepoConfig is a function that reads in the gitRepo object from the config file
func GitRepoConfig() *v1alpha1.GitRepo {
	var gitRepo v1alpha1.GitRepo
	config.LoadConfig(gitRepoConfigConfigurationFileKey, &gitRepo)
	return &gitRepo
}

const (
	DeploymentResourceSteveType = "apps.deployment"
	FleetClusterResourceType    = "fleet.cattle.io.cluster"
	FleetControllerName         = "cattle-fleet-system/fleet-controller"
)

// VerifyGitRepo will verify that the gitRepo itself comes to an active state along with fleetCluster resources
// and the steve Cluster's resources from said gitRepo come to an active state. This is limited to work with
// a single cluster. If multiple clusters are targeted in the gitRepo, only the specified steve Cluster's
// resources will be validated. However the gitRepo's fleetCluster will still be validated for its targets.
func VerifyGitRepo(client *rancher.Client, gitRepoID, k8sClusterID, steveClusterID string) error {
	backoff := kwait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.1,
		Jitter:   0.1,
		Steps:    20,
	}

	err := kwait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}

		// after checking clusterStatus, check gitRepoStatus. gitRepoStatus starts in a healthy state,
		// so if errors come up during clusterBundle deployments, its status will update to a negative / error state
		gitRepo, err := client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).ByID(gitRepoID)
		if err != nil {
			return false, err
		}

		gitStatus := &v1alpha1.GitRepoStatus{}
		err = steveV1.ConvertToK8sType(gitRepo.Status, gitStatus)
		if err != nil {
			return false, err
		}

		if gitStatus.Display.Error {
			return true, errors.New(gitStatus.Display.Message)
		}

		if gitRepo.State.Error {
			return true, errors.New(gitRepo.State.Message)
		}

		if gitStatus.Summary.NotReady > 0 || gitStatus.Summary.DesiredReady == 0 || gitStatus.ReadyClusters == 0 {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return err
	}

	logrus.Info("waiting for bundles to deploy to ", steveClusterID)
	err = kwait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}

		cluster, err := client.Steve.SteveType(FleetClusterResourceType).ByID(steveClusterID)
		if err != nil {
			return false, err
		}

		status := &v1alpha1.ClusterStatus{}
		err = steveV1.ConvertToK8sType(cluster.Status, status)
		if err != nil {
			return false, err
		}

		for _, nonReadyResource := range status.Summary.NonReadyResources {
			logrus.Info(nonReadyResource.Message)
			if strings.Contains(nonReadyResource.Message, "error") || strings.Contains(nonReadyResource.Message, "Unable to continue") {
				return true, errors.New(nonReadyResource.Message)
			}
		}

		// after checking clusterStatus, check gitRepoStatus. gitRepoStatus can start in a healthy state,
		// so if errors come up during clusterBundle deployments, its status will update to a negative / error state that aren'
		gitRepo, err := client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).ByID(gitRepoID)
		if err != nil {
			return false, err
		}

		gitStatus := &v1alpha1.GitRepoStatus{}
		err = steveV1.ConvertToK8sType(gitRepo.Status, gitStatus)
		if err != nil {
			return false, err
		}

		if gitStatus.Display.Error {
			return true, errors.New(gitStatus.Display.Message)
		}

		if gitRepo.State.Error {
			return true, errors.New(gitRepo.State.Message)
		}

		if status.Summary.NotReady > 0 || status.Summary.DesiredReady == 0 {
			return false, nil
		}

		if gitStatus.Summary.NotReady > 0 || gitStatus.Summary.DesiredReady == 0 || gitStatus.ReadyClusters == 0 {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return err
	}

	// validate all resources on the cluster are actually in a good state, regardless of what fleet is reporting
	podErrors := pods.StatusPods(client, k8sClusterID)
	if len(podErrors) > 0 {
		for _, err := range podErrors {
			logrus.Errorf(err.Error())
		}
		return errors.New("pods are not healthy in " + steveClusterID)
	}
	return err
}

// GetDeploymentVersion is a helper that gets the image version from a deployment ID in a given cluster.
func GetDeploymentVersion(client *rancher.Client, deploymentID, clusterName string) (string, error) {
	var deploymentVersion string

	clusterProxy, err := client.Steve.ProxyDownstream(clusterName)
	if err != nil {
		return deploymentVersion, err
	}

	steveClient := clusterProxy.SteveType(DeploymentResourceSteveType)

	deploymentObject, err := steveClient.ByID(deploymentID)
	if err != nil {
		return deploymentVersion, err
	}

	deploymentSpec := &appsv1.DeploymentSpec{}
	err = steveV1.ConvertToK8sType(deploymentObject.Spec, deploymentSpec)
	if err != nil {
		return deploymentVersion, err
	}

	return strings.Split(deploymentSpec.Template.Spec.Containers[0].Image, ":")[1], nil
}
