package external

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/rancher/rancher/pkg/controllers/managementuser/nodesyncer"
	"github.com/rancher/rancher/pkg/image"
	"github.com/sirupsen/logrus"
)

type Source string

const (
	K3S  Source = "k3s"
	RKE2 Source = "rke2"
)

func GetExternalImages(rancherVersion string, externalData map[string]interface{}, source Source, minimumKubernetesVersion *semver.Version, osType image.OSType) ([]string, error) {
	if source != K3S && source != RKE2 {
		return nil, fmt.Errorf("invalid source provided: %s", source)
	}

	logrus.Infof("generating %s image list...", source)
	externalImagesMap := make(map[string]bool)
	releases, _ := externalData["releases"].([]interface{})

	var compatibleReleases []string

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

			versionGTMin, err := nodesyncer.IsNewerVersion(minVersion, rancherVersion)
			if err != nil {
				continue
			}
			if rancherVersion != minVersion && !versionGTMin {
				// Rancher version not equal to or greater than minimum supported rancher version.
				continue
			}

			versionLTMax, err := nodesyncer.IsNewerVersion(rancherVersion, maxVersion)
			if err != nil {
				continue
			}
			if rancherVersion != maxVersion && !versionLTMax {
				// Rancher version not equal to or greater than maximum supported rancher version.
				continue
			}
		}

		logrus.Debugf("[%s] adding compatible release: %s", source, version)
		compatibleReleases = append(compatibleReleases, version)
	}

	if compatibleReleases == nil || len(compatibleReleases) < 1 {
		logrus.Infof("skipping image generation since no compatible releases were found for version: %s", rancherVersion)
		return nil, nil
	}

	for _, release := range compatibleReleases {
		// Registries don't allow "+", so image names will have these substituted.
		upgradeImage := fmt.Sprintf("rancher/%s-upgrade:%s", source, strings.ReplaceAll(release, "+", "-"))
		externalImagesMap[upgradeImage] = true
		systemAgentInstallerImage := fmt.Sprintf("%s%s:%s", "rancher/system-agent-installer-", source, strings.ReplaceAll(release, "+", "-"))
		externalImagesMap[systemAgentInstallerImage] = true

		images, err := downloadExternalSupportingImages(release, source, osType)
		if err != nil {
			logrus.Infof("could not find supporting images for %s release [%s]: %v", source, release, err)
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
