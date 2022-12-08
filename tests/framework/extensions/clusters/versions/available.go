package versions

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
)

// ListRKE1AvailableVersions is a function to list and return only available RKE1 versions for a specific cluster.
func ListRKE1AvailableVersions(client *rancher.Client, cluster *v3.Cluster) (availableVersions []string, err error) {
	allAvailableVersions, err := ListRKE1AllVersions(client)
	if err != nil {
		return
	}

	for i, version := range allAvailableVersions {
		if strings.Contains(version, cluster.RancherKubernetesEngineConfig.Version) {
			availableVersions = allAvailableVersions[i+1:]
			break
		}
	}

	return
}

// ListRKE2AvailableVersions is a function to list and return only available RKE2 versions for a specific cluster.
func ListRKE2AvailableVersions(client *rancher.Client, cluster *v1.SteveAPIObject) (availableVersion []string, err error) {
	allAvailableVersions, err := ListRKE2AllVersions(client)
	if err != nil {
		return
	}

	clusterSpec := &apiv1.ClusterSpec{}
	err = v1.ConvertToK8sType(cluster.Spec, clusterSpec)
	if err != nil {
		return
	}

	availableVersion = allAvailableVersions

	for i, version := range allAvailableVersions {
		if strings.Contains(version, clusterSpec.KubernetesVersion) {
			availableVersion = allAvailableVersions[i+1:]
			break
		}
	}

	return
}

// ListNormanRKE2AvailableVersions is a function to list and return only available RKE2 versions for an imported specific cluster.
func ListNormanRKE2AvailableVersions(client *rancher.Client, cluster *v3.Cluster) (availableVersion []string, err error) {
	allAvailableVersions, err := ListRKE2AllVersions(client)
	if err != nil {
		return
	}

	availableVersion = allAvailableVersions

	for i, version := range allAvailableVersions {
		if strings.Contains(version, cluster.Rke2Config.Version) {
			availableVersion = allAvailableVersions[i+1:]
			break
		}
	}

	return
}

// ListK3SAvailableVersions is a function to list and return only available K3S versions for a specific cluster.
func ListK3SAvailableVersions(client *rancher.Client, cluster *v1.SteveAPIObject) (availableVersion []string, err error) {
	allAvailableVersions, err := ListK3SAllVersions(client)
	if err != nil {
		return
	}

	availableVersion = allAvailableVersions

	clusterSpec := &apiv1.ClusterSpec{}
	err = v1.ConvertToK8sType(cluster.Spec, clusterSpec)
	if err != nil {
		return
	}

	for i, version := range allAvailableVersions {
		if strings.Contains(version, clusterSpec.KubernetesVersion) {
			availableVersion = allAvailableVersions[i+1:]
			break
		}
	}

	return
}

// ListNormanK3SAvailableVersions is a function to list and return only available K3S versions for an imported specific cluster.
func ListNormanK3SAvailableVersions(client *rancher.Client, cluster *v3.Cluster) (availableVersion []string, err error) {
	allAvailableVersions, err := ListK3SAllVersions(client)
	if err != nil {
		return
	}

	availableVersion = allAvailableVersions

	for i, version := range allAvailableVersions {
		if strings.Contains(version, cluster.K3sConfig.Version) {
			availableVersion = allAvailableVersions[i+1:]
			break
		}
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
	allAvailableVersions, err := ListAKSAllVersions(client, cluster)

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
