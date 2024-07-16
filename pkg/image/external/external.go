package external

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/rancher/rancher/pkg/controllers/management/k3sbasedupgrade"
	"github.com/rancher/rancher/pkg/image"
	"github.com/sirupsen/logrus"
)

type Source string

// DistributionVersion is a utility struct which represents a specific RKE2/K3s version and
// is used to compare versions against one another.
type DistributionVersion struct {
	VersionString string
	SemVer        *semver.Version
	ReleaseNumber int
}

const (
	K3S  Source = "k3s"
	RKE2 Source = "rke2"
)

func GetExternalImagesForVersions(source Source, osType image.OSType, versions []DistributionVersion) ([]string, error) {
	if source != K3S && source != RKE2 {
		return nil, fmt.Errorf("invalid source provided: %s", source)
	}

	logrus.Infof("generating %s image list...", source)

	if versions == nil || len(versions) < 1 {
		logrus.Infof("skipping image generation as no versions were provided")
		return nil, nil
	}

	externalImagesMap := make(map[string]bool)
	for _, release := range versions {
		// Registries don't allow "+", so image names will have these substituted.
		upgradeImage := fmt.Sprintf("rancher/%s-upgrade:%s", source, strings.ReplaceAll(release.VersionString, "+", "-"))
		externalImagesMap[upgradeImage] = true
		systemAgentInstallerImage := fmt.Sprintf("%s%s:%s", "rancher/system-agent-installer-", source, strings.ReplaceAll(release.VersionString, "+", "-"))
		externalImagesMap[systemAgentInstallerImage] = true

		images, err := downloadExternalSupportingImages(release.VersionString, source, osType)
		if err != nil {
			logrus.Infof("could not find supporting images for %s release [%s]: %v", source, release.VersionString, err)
			continue
		}

		supportingImages := strings.Split(images, "\n")
		if supportingImages[len(supportingImages)-1] == "" {
			supportingImages = supportingImages[:len(supportingImages)-1]
		}

		for _, imageName := range supportingImages {
			imageName = strings.TrimPrefix(imageName, "docker.io/")
			externalImagesMap[imageName] = true
		}
	}

	var externalImages []string
	for imageName := range externalImagesMap {
		logrus.Debugf("[%s] adding image: %s", source, imageName)
		externalImages = append(externalImages, imageName)
	}

	sort.Strings(externalImages)
	logrus.Infof("finished generating %s image list...", source)
	return externalImages, nil
}

// GetLatestPatchesForSupportedVersions iterates over KDM data to gather the latest
// RKE2 or K3s patch versions for each minor SemVer of RKE2 or K3s supported by the given rancherVersion.
func GetLatestPatchesForSupportedVersions(rancherVersion string, externalData map[string]interface{}, source Source, minimumKubernetesVersion *semver.Version) ([]DistributionVersion, error) {
	if source != K3S && source != RKE2 {
		return nil, fmt.Errorf("invalid source provided: %s", source)
	}

	releases, _ := externalData["releases"].([]interface{})
	distroReleaseMap := make(map[string]DistributionVersion)
	for _, release := range releases {
		releaseMap, _ := release.(map[string]interface{})
		version, _ := releaseMap["version"].(string)
		if version == "" {
			continue
		}

		// Skip the release if a minimum Kubernetes version is provided and is not met.
		if minimumKubernetesVersion != nil {
			versionSemVer, err := semver.NewVersion(strings.TrimPrefix(version, "v"))
			if err != nil {
				continue
			}
			if versionSemVer.LessThan(*minimumKubernetesVersion) {
				continue
			}
		}

		if rancherVersion != "dev" {
			maxVersion, _ := releaseMap["maxChannelServerVersion"].(string)
			maxVersion = strings.TrimPrefix(maxVersion, "v")
			if maxVersion == "" {
				continue
			}
			minVersion, _ := releaseMap["minChannelServerVersion"].(string)
			minVersion = strings.Trim(minVersion, "v")
			if minVersion == "" {
				continue
			}

			versionGTMin, err := k3sbasedupgrade.IsNewerVersion(minVersion, rancherVersion)
			if err != nil {
				continue
			}
			if rancherVersion != minVersion && !versionGTMin {
				// Rancher version not equal to or greater than minimum supported rancher version.
				continue
			}

			versionLTMax, err := k3sbasedupgrade.IsNewerVersion(rancherVersion, maxVersion)
			if err != nil {
				continue
			}
			if rancherVersion != maxVersion && !versionLTMax {
				// Rancher version not equal to or greater than maximum supported rancher version.
				continue
			}
		}

		distroVersion, err := stringToDistroVersion(version, source)
		if err != nil {
			logrus.Errorf("error encountered converting version %s to a DistributionVersion: %v", version, err)
			continue
		}

		minor := fmt.Sprintf("%s.%s", strconv.FormatInt(distroVersion.SemVer.Major, 10), strconv.FormatInt(distroVersion.SemVer.Minor, 10))

		// update the map with the latest patch version for each minor
		existingVersion, ok := distroReleaseMap[minor]
		if !ok || existingVersion.SemVer.LessThan(*distroVersion.SemVer) || distroVersion.ReleaseNumber > existingVersion.ReleaseNumber {
			if !ok {
				logrus.Debugf("Adding new patch version for minor %s:  %s", minor, version)
			} else {
				logrus.Debugf("Found new version for release %s, %s", existingVersion.VersionString, version)
			}
			distroReleaseMap[minor] = DistributionVersion{
				VersionString: version,
				SemVer:        distroVersion.SemVer,
				ReleaseNumber: distroVersion.ReleaseNumber,
			}
		}
	}

	var latestPatchVersions []DistributionVersion
	for _, v := range distroReleaseMap {
		latestPatchVersions = append(latestPatchVersions, v)
	}

	logrus.Infof("Latest patch versions %v", latestPatchVersions)

	return latestPatchVersions, nil
}

// stringToDistroVersion converts the string representation of an rke2/k3s version (e.g. v1.30.1+rke2r1) to a DistributionVersion struct
func stringToDistroVersion(v string, source Source) (DistributionVersion, error) {
	minorPatchSplit := strings.Split(v, "+")
	if len(minorPatchSplit) != 2 {
		return DistributionVersion{}, fmt.Errorf("provided version %s did not split cleanly, expected a version format of either v1.X.Y+rke2rZ, or v1.X.Y+k3sZ", v)
	}

	k8sVersion := strings.ReplaceAll(minorPatchSplit[0], "v", "")
	releaseVersion := minorPatchSplit[1]

	var releaseNumber string
	switch source {
	case RKE2:
		releaseNumber = strings.ReplaceAll(releaseVersion, "rke2r", "")
	case K3S:
		releaseNumber = strings.ReplaceAll(releaseVersion, "k3s", "")
	default:
		return DistributionVersion{}, fmt.Errorf("invalid source")
	}

	releaseNumberInt, err := strconv.Atoi(releaseNumber)
	if err != nil {
		return DistributionVersion{}, fmt.Errorf("failed to cast %s release number string (%s) to int: %v", source, releaseNumber, err)
	}

	sv, err := semver.NewVersion(k8sVersion)
	if err != nil {
		return DistributionVersion{}, fmt.Errorf("failed to cast version string (%s) to semver.Version: %v", k8sVersion, err)
	}

	return DistributionVersion{
		VersionString: v,
		SemVer:        sv,
		ReleaseNumber: releaseNumberInt,
	}, nil
}

// downloadExternalSupportingImages downloads the list of images used by a Source from GitHub releases.
// The osType parameter is only used by RKE2 since K3s is not currently available for Windows containers.
func downloadExternalSupportingImages(release string, source Source, osType image.OSType) (string, error) {
	switch source {
	case RKE2:
		// FIXME: Support s390x and arm64 images lists.
		// rke2 publishes a list of images for s390x but not for arm64 at the moment.
		var externalImageURL string
		switch osType {
		case image.Linux:
			externalImageURL = fmt.Sprintf("https://github.com/rancher/rke2/releases/download/%s/rke2-images-all.linux-amd64.txt", release)
		case image.Windows:
			externalImageURL = fmt.Sprintf("https://github.com/rancher/rke2/releases/download/%s/rke2-images.windows-amd64.txt", release)
		default:
			return "", fmt.Errorf("could not download external supporting images: unsupported os type %v", osType)
		}
		images, err := downloadExternalImageListFromURL(externalImageURL)
		if err != nil {
			return "", err
		}
		return images, nil
	case K3S:
		// k3s does not support Windows containers at the moment.
		return downloadExternalImageListFromURL(fmt.Sprintf("https://github.com/k3s-io/k3s/releases/download/%s/k3s-images.txt", release))
	default:
		// This function should never be called with an invalid source, but we will anticipate this
		// error for safety.
		return "", fmt.Errorf("invalid source provided for download: %s", source)
	}
}

func downloadExternalImageListFromURL(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to get url: %v", string(body))
	}
	defer resp.Body.Close()

	if err != nil {
		return "", err
	}

	return string(body), nil
}
