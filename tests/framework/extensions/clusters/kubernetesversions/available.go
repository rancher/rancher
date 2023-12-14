package kubernetesversions

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v3 "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/sirupsen/logrus"
)

// ListRKE1AvailableVersions is a function to list and return only available RKE1 versions for a specific cluster.
func ListRKE1AvailableVersions(client *rancher.Client, cluster *v3.Cluster) (availableVersions []string, err error) {
	var allAvailableVersions []*semver.Version
	allRKE1Versions, err := ListRKE1AllVersions(client)
	if err != nil {
		return
	}

	for _, v := range allRKE1Versions {
		rkeVersion, err := semver.NewVersion(strings.TrimPrefix(v, "v"))
		if err != nil {
			logrus.Errorf("couldn't turn %v to a semantic version", rkeVersion)
			continue
		}

		allAvailableVersions = append(allAvailableVersions, rkeVersion)
	}

	currentVersion, err := semver.NewVersion(strings.TrimPrefix(cluster.RancherKubernetesEngineConfig.Version, "v"))
	if err != nil {
		return
	}

	for _, v := range allAvailableVersions {
		if v.Compare(currentVersion) == 0 || v.Compare(currentVersion) == -1 {
			continue
		}
		availableVersions = append(availableVersions, fmt.Sprint("v", v.String()))
	}

	return
}

// ListRKE1ImportedAvailableVersions is a function to list and return only available imported RKE1 versions for a specific cluster.
func ListRKE1ImportedAvailableVersions(client *rancher.Client, cluster *v3.Cluster) (availableVersions []string, err error) {
	var allAvailableVersions []*semver.Version
	allRKE1Versions, err := ListRKE1AllVersions(client)
	if err != nil {
		return
	}

	for _, v := range allRKE1Versions {
		rkeVersion, err := semver.NewVersion(strings.TrimPrefix(v, "v"))
		if err != nil {
			logrus.Errorf("couldn't turn %v to a semantic version", rkeVersion)
			continue
		}

		allAvailableVersions = append(allAvailableVersions, rkeVersion)
	}

	currentVersion, err := semver.NewVersion(strings.TrimPrefix(cluster.Version.GitVersion, "v"))
	if err != nil {
		return
	}

	for _, v := range allAvailableVersions {
		if v.Compare(currentVersion) == 0 || v.Compare(currentVersion) == -1 {
			continue
		}
		availableVersions = append(availableVersions, fmt.Sprint("v", v.String()))
	}

	return
}

// ListRKE2AvailableVersions is a function to list and return only available RKE2 versions for a specific cluster.
func ListRKE2AvailableVersions(client *rancher.Client, cluster *v1.SteveAPIObject) (availableVersions []string, err error) {
	var allAvailableVersions []*semver.Version
	allRKE2Versions, err := ListRKE2AllVersions(client)
	if err != nil {
		return
	}

	for _, v := range allRKE2Versions {
		rke2Version, err := semver.NewVersion(strings.TrimPrefix(v, "v"))
		if err != nil {
			logrus.Errorf("couldn't turn %v to a semantic version", rke2Version)
			continue
		}

		allAvailableVersions = append(allAvailableVersions, rke2Version)
	}

	clusterSpec := &apiv1.ClusterSpec{}
	err = v1.ConvertToK8sType(cluster.Spec, clusterSpec)
	if err != nil {
		return
	}

	currentVersion, err := semver.NewVersion(strings.TrimPrefix(clusterSpec.KubernetesVersion, "v"))
	if err != nil {
		return
	}

	for _, v := range allAvailableVersions {
		if v.Compare(currentVersion) == 0 || v.Compare(currentVersion) == -1 {
			continue
		}

		availableVersions = append(availableVersions, fmt.Sprint("v", v.String()))
	}

	return
}

// ListNormanRKE2AvailableVersions is a function to list and return only available RKE2 versions for an imported specific cluster.
func ListNormanRKE2AvailableVersions(client *rancher.Client, cluster *v3.Cluster) (availableVersions []string, err error) {
	var allAvailableVersions []*semver.Version
	allRKE2Versions, err := ListRKE2AllVersions(client)
	if err != nil {
		return
	}

	for _, v := range allRKE2Versions {
		k3sVersion, err := semver.NewVersion(strings.TrimPrefix(v, "v"))
		if err != nil {
			logrus.Errorf("couldn't turn %v to a semantic version", k3sVersion)
			continue
		}

		allAvailableVersions = append(allAvailableVersions, k3sVersion)
	}

	currentVersion, err := semver.NewVersion(strings.TrimPrefix(cluster.Rke2Config.Version, "v"))
	if err != nil {
		return
	}

	for _, v := range allAvailableVersions {
		if v.Compare(currentVersion) == 0 || v.Compare(currentVersion) == -1 {
			continue
		}

		availableVersions = append(availableVersions, fmt.Sprint("v", v.String()))
	}

	return
}

// ListK3SAvailableVersions is a function to list and return only available K3S versions for a specific cluster.
func ListK3SAvailableVersions(client *rancher.Client, cluster *v1.SteveAPIObject) (availableVersions []string, err error) {
	var allAvailableVersions []*semver.Version
	allK3sVersions, err := ListK3SAllVersions(client)
	if err != nil {
		return
	}

	for _, v := range allK3sVersions {
		rkeVersion, err := semver.NewVersion(strings.TrimPrefix(v, "v"))
		if err != nil {
			logrus.Errorf("couldn't turn %v to a semantic version", rkeVersion)
			continue
		}

		allAvailableVersions = append(allAvailableVersions, rkeVersion)
	}

	clusterSpec := &apiv1.ClusterSpec{}
	err = v1.ConvertToK8sType(cluster.Spec, clusterSpec)
	if err != nil {
		return
	}

	currentVersion, err := semver.NewVersion(strings.TrimPrefix(clusterSpec.KubernetesVersion, "v"))
	if err != nil {
		return
	}

	for _, v := range allAvailableVersions {
		if v.Compare(currentVersion) == 0 || v.Compare(currentVersion) == -1 {
			continue
		}

		availableVersions = append(availableVersions, fmt.Sprint("v", v.String()))
	}

	return
}

// ListNormanK3SAvailableVersions is a function to list and return only available K3S versions for an imported specific cluster.
func ListNormanK3SAvailableVersions(client *rancher.Client, cluster *v3.Cluster) (availableVersions []string, err error) {
	var allAvailableVersions []*semver.Version
	allK3sVersions, err := ListK3SAllVersions(client)
	if err != nil {
		return
	}

	for _, v := range allK3sVersions {
		rkeVersion, err := semver.NewVersion(strings.TrimPrefix(v, "v"))
		if err != nil {
			logrus.Errorf("couldn't turn %v to a semantic version", rkeVersion)
			continue
		}

		allAvailableVersions = append(allAvailableVersions, rkeVersion)
	}

	currentVersion, err := semver.NewVersion(strings.TrimPrefix(cluster.K3sConfig.Version, "v"))
	if err != nil {
		return
	}

	for _, v := range allAvailableVersions {
		if v.Compare(currentVersion) == 0 || v.Compare(currentVersion) == -1 {
			continue
		}

		availableVersions = append(availableVersions, fmt.Sprint("v", v.String()))
	}

	return
}

// ListGKEAvailableVersions is a function to list and return only available GKE versions for a specific cluster.
func ListGKEAvailableVersions(client *rancher.Client, cluster *v3.Cluster) (availableVersions []string, err error) {
	currentVersion, err := semver.NewVersion(cluster.Version.GitVersion)
	if err != nil {
		return
	}

	if cluster.GKEConfig == nil {
		return nil, errors.Wrapf(err, "cluster %s has no gke config", cluster.Name)
	}

	var validMasterVersions []*semver.Version
	allAvailableVersions, err := ListGKEAllVersions(client, cluster.GKEConfig.ProjectID, cluster.GKEConfig.GoogleCredentialSecret, cluster.GKEConfig.Zone, cluster.GKEConfig.Region)

	for _, version := range allAvailableVersions {
		v, err := semver.NewVersion(version)
		if err != nil {
			continue
		}
		validMasterVersions = append(validMasterVersions, v)
	}

	for _, v := range validMasterVersions {
		if v.Minor()-1 > currentVersion.Minor() || v.Compare(currentVersion) == 0 || v.Compare(currentVersion) == -1 {
			continue
		}
		availableVersions = append(availableVersions, v.String())
	}

	reverseSlice(availableVersions)

	return
}

// ListAKSAvailableVersions is a function to list and return only available AKS versions for a specific cluster.
func ListAKSAvailableVersions(client *rancher.Client, cluster *v3.Cluster) (availableVersions []string, err error) {
	currentVersion, err := semver.NewVersion(cluster.Version.GitVersion)
	if err != nil {
		return
	}

	var validMasterVersions []*semver.Version
	allAvailableVersions, err := ListAKSAllVersions(client, cluster.AKSConfig.AzureCredentialSecret, cluster.AKSConfig.ResourceLocation)

	for _, version := range allAvailableVersions {
		v, err := semver.NewVersion(version)
		if err != nil {
			continue
		}
		validMasterVersions = append(validMasterVersions, v)
	}

	for _, v := range validMasterVersions {
		if v.Minor()-1 > currentVersion.Minor() || v.Compare(currentVersion) == 0 || v.Compare(currentVersion) == -1 {
			continue
		}
		availableVersions = append(availableVersions, v.String())
	}

	return
}

// ListEKSAvailableVersions is a function to list and return only available EKS versions for a specific cluster.
func ListEKSAvailableVersions(client *rancher.Client, cluster *v3.Cluster) (availableVersions []string, err error) {
	currentVersion, err := semver.NewVersion(cluster.Version.GitVersion)
	if err != nil {
		return
	}

	var validMasterVersions []*semver.Version

	allAvailableVersions, err := ListEKSAllVersions(client)
	if err != nil {
		return
	}

	for _, version := range allAvailableVersions {
		v, err := semver.NewVersion(version)
		if err != nil {
			continue
		}

		validMasterVersions = append(validMasterVersions, v)
	}

	for _, v := range validMasterVersions {
		if v.Minor()-1 > currentVersion.Minor() || v.Compare(currentVersion) == 0 || v.Compare(currentVersion) == -1 {
			continue
		}
		version := fmt.Sprintf("%v.%v", v.Major(), v.Minor())
		availableVersions = append(availableVersions, version)
	}

	reverseSlice(availableVersions)

	return
}

// reverseSlice is a private function that used to rever slice of strings.
func reverseSlice(stringSlice []string) []string {
	sort.SliceStable(stringSlice, func(i, j int) bool { return i > j })

	return stringSlice
}
