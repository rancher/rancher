package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"os"
	"sort"

	"regexp"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type RancherK8sVersions struct {
	RancherImageTag             string `json:"rancherImageTag" yaml:"rancherImageTag"`
	RancherImageTagToUpgrade    string `json:"rancherImageTagToUpgrade" yaml:"rancherImageTagToUpgrade"`
	RancherVersion              string `json:"rancherVersion" yaml:"rancherVersion"`
	RancherVersionToUpgrade     string `json:"rancherVersionToUpgrade" yaml:"rancherVersionToUpgrade"`
	RancherRKE2Version          string `json:"rancherRKE2Version" yaml:"rancherRKE2Version"`
	RancherRKE2VersionToUpgrade string `json:"rancherRKE2VersionToUpgrade" yaml:"rancherRKE2VersionToUpgrade"`
	RancherK3sVersion           string `json:"rancherK3sVersion" yaml:"rancherK3sVersion"`
	RancherK3sVersionToUpgrade  string `json:"rancherK3sVersionToUpgrade" yaml:"rancherK3sVersionToUpgrade"`
	RancherRKEVersion           string `json:"rancherRKEVersion" yaml:"rancherRKEVersion"`
	RancherRKEVersionToUpgrade  string `json:"rancherRKEVersionToUpgrade" yaml:"rancherRKEVersionToUpgrade"`
}

var (
	rancherVersion          = os.Getenv("RANCHER_VERSION")
	rancherVersionToUpgrade = os.Getenv("RANCHER_VERSION_TO_UPGRADE")
	communityBaseURL          = "https://github.com/rancher/rancher/releases/download/" + rancherVersion + "/rancher-images.txt"
	communityBaseURLToUpgrade = "https://github.com/rancher/rancher/releases/download/" + rancherVersionToUpgrade + "/rancher-images.txt"
	primeRelease              = true
	primeReleaseToUpgrade     = false
)

const (
	baseURL                 = "https://prime.ribs.rancher.io/"
	rke2K3sVersionFilterURL = "/rancher-images.txt"
	rkeVersionFilterURL     = "/rancher-rke-k8s-versions.txt"
	rke2Filter              = `rancher/hardened-kubernetes:v\d+\.\d+\.\d+(-rke2r\d)?`
	k3sFilter               = `rancher/k3s-upgrade:v\d+\.\d+\.\d+(-k3s\d)`
	fileName                = "rancher-images.txt"
	fileNameUpgradeVersion  = "rancher-images-pre-release-versions.txt"
)

func main() {
	var err error
	urlb := baseURL + "index.html"

	rancherChartVersion, err := extractRancherVersion(urlb, rancherVersion)
	if err != nil {
		return
	}
	if rancherChartVersion == "" {
		rancherChartVersion = rancherVersion
		primeRelease = false
		err = downloadFile(communityBaseURL, fileName)
		if err != nil {
			fmt.Println("Failed to download Rancher images file:", err)
			return
		}
	}
	if rancherVersionToUpgrade != "" {
		err = downloadFile(communityBaseURLToUpgrade, fileNameUpgradeVersion)
		data, err := os.ReadFile(fileNameUpgradeVersion)
		if string(data) == "Not Found" {
			primeReleaseToUpgrade = true
		} else if err != nil {
			return
		}
	}

	logrus.Info("Rancher version: " + rancherChartVersion)

	k8sImageBaseUrl := baseURL + "rancher/" + rancherChartVersion

	rke2K3sK8sVersionsUrl := k8sImageBaseUrl + rke2K3sVersionFilterURL
	rkeK8sVersionsUrl := k8sImageBaseUrl + rkeVersionFilterURL

	rke2K8sVersion, err := extractRke2K3sVersions(rke2K3sK8sVersionsUrl, rke2Filter, primeRelease, primeReleaseToUpgrade)
	if err != nil {
		return
	}

	k3sK8sVersion, err := extractRke2K3sVersions(rke2K3sK8sVersionsUrl, k3sFilter, primeRelease, primeReleaseToUpgrade)
	if err != nil {
		return
	}

	rkeK8sVersion, err := extractRkeVersions(rkeK8sVersionsUrl, primeRelease, primeReleaseToUpgrade)
	if err != nil {
		return
	}

	rancherK8sVersions := RancherK8sVersions{}

	if rancherVersionToUpgrade != "" {
		rancherK8sVersions.RancherVersionToUpgrade = rancherVersionToUpgrade
		rancherK8sVersions.RancherImageTagToUpgrade = rancherVersionToUpgrade
	}

	rancherK8sVersions.RancherImageTag = rancherChartVersion
	rancherK8sVersions.RancherVersion = strings.Trim(rancherChartVersion, "v")
	rancherK8sVersions.RancherRKE2Version = rke2K8sVersion[0]
	rancherK8sVersions.RancherRKE2VersionToUpgrade = rke2K8sVersion[1]
	rancherK8sVersions.RancherK3sVersion = k3sK8sVersion[0]
	rancherK8sVersions.RancherK3sVersionToUpgrade = k3sK8sVersion[1]
	rancherK8sVersions.RancherRKEVersion = rkeK8sVersion[0]
	rancherK8sVersions.RancherRKEVersionToUpgrade = rkeK8sVersion[1]

	logrus.Info("Here are the rancher versions ", rancherK8sVersions)

	err = writeToConfigFile(rancherK8sVersions)
	if err != nil {
		logrus.Error("error writiing test run config: ", err)
	}

}

// getResponseData extracts the data from the given URL and returns the response body as a byte slice.
func getResponseData(urlb string) ([]byte, error) {
	resp, err := http.Get(urlb)
	if err != nil {
		fmt.Printf("Error fetching URL: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	responseData, err := io.ReadAll(resp.Body)

	return responseData, err
}

// getRke2K3sVersionMap extracts the rke2 and k3s version map from the rancher-images.txt for a community release and 
// prime.ribs.rancher.io for a prime released version of rancher.
func getRke2K3sVersionMap(urlb, matchFilter, file string, primeRelease bool) (latestVersions []*version.Version, err error) {
	var k8sVersionData []byte
	versionFilter := regexp.MustCompile(matchFilter)
	if primeRelease {
		k8sVersionData, err = getResponseData(urlb)
		if err != nil {
			return nil, err
		}
	} else {
		k8sVersionData, err = os.ReadFile(file)
		if k8sVersionData == nil {
			return nil, errors.New("Community version file is empty")
		}
	}
	versions := versionFilter.FindAllStringSubmatch(string(k8sVersionData), -1)

	if len(versions) == 0 {
		fmt.Println("No versions found in the response.")
		return nil, err
	}

	majorMinorVersionGroups := make(map[string][]*version.Version)
	for _, ver := range versions {
		ver := strings.Split(ver[0], ":")
		if len(ver) > 1 {
			finalVersion, err := version.NewVersion(ver[1])
			if err != nil {
				return nil, err
			}
			majorMinorVersion := finalVersion.Segments()
			if len(majorMinorVersion) >= 2 {
				majorMinor := fmt.Sprintf("%d.%d", majorMinorVersion[0], majorMinorVersion[1])
				majorMinorVersionGroups[majorMinor] = append(majorMinorVersionGroups[majorMinor], finalVersion)
			}
		}
	}

	for _, group := range majorMinorVersionGroups {
		sort.Sort(version.Collection(group))
		latestVersions = append(latestVersions, group[len(group)-1])
	}
	sort.Sort(version.Collection(latestVersions))

	return latestVersions, nil
}

// extractRke2K3sVersions is a helper that returns the rke2 and k3s versions from the provided rancher release versions
func extractRke2K3sVersions(urlb, matchFilter string, primeRelease, primeReleaseToUpgrade bool) (k8sVersions []string, err error) {
	latestVersions, err := getRke2K3sVersionMap(urlb, matchFilter, fileName, primeRelease)
	var k8sVersionUpgrade []*version.Version

	if rancherVersionToUpgrade != "" {
		re := regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+`)
		urlb = re.ReplaceAllString(urlb, rancherVersionToUpgrade)
		k8sVersionUpgrade, err = getRke2K3sVersionMap(urlb, matchFilter, fileNameUpgradeVersion, primeReleaseToUpgrade)
		if err != nil {
			return nil, err
		}
		k8sVersions = append(k8sVersions, latestVersions[len(latestVersions)-1].Original(), k8sVersionUpgrade[len(k8sVersionUpgrade)-1].Original())
	} else {
		k8sVersions = append(k8sVersions, latestVersions[len(latestVersions)-2].Original(), latestVersions[len(latestVersions)-1].Original())
	}

	return k8sVersions, nil
}

// getRkeVersionMap is a helper that obtains the rke1 k8s versions from the downloaded rancher-images.txt for a community version
// and extracts the versions for a prime release from prime.ribs.rancher.io
func getRkeVersionMap(urlb, file string, primeRelease bool) ([]string, error) {
	var versions []string
	if primeRelease {
		k8sVersionData, err := getResponseData(urlb)
		if err != nil {
			return nil, err
		}
		if len(k8sVersionData) <= 0 {
			return nil, fmt.Errorf("Empty list")
		}

		k8sVersions := strings.TrimSpace(string(k8sVersionData))

		versions = strings.Split(k8sVersions, "\n")
		if len(versions) > 4 {
			return nil, fmt.Errorf("Unexpected count of versions %s", versions)
		}
	} else {
		k8sVersions, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		versionFilter := regexp.MustCompile("rancher/hyperkube:(v[0-9]+.[0-9]+.[0-9]+-rancher[0-9]+)")
		allRKE1Versions := versionFilter.FindAllStringSubmatch(string(k8sVersions), -1)
		if len(allRKE1Versions) == 0 {
			return nil, errors.New("No RKE1 versions found.")
		}
		for _, ver := range allRKE1Versions {
			if len(ver) > 1 {
				versions = append(versions, ver[1])
			}
		}
	}
	return versions, nil
}

// extractRkeVersions is a helper that returns the RKE1 k8s versions 
func extractRkeVersions(urlb string, primeRelease, primeReleaseToUpgrade bool) (k8sVersions []string, err error) {
	versions, err := getRkeVersionMap(urlb, fileName, primeRelease)
	if err != nil {
		return nil, err
	}
	var k8sVersionToUpgrade []string
	if rancherVersionToUpgrade != "" {
		re := regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+`)
		urlb = re.ReplaceAllString(urlb, rancherVersionToUpgrade)
		k8sVersionToUpgrade, err = getRkeVersionMap(urlb, fileNameUpgradeVersion, primeReleaseToUpgrade)
		if err != nil {
			return nil, err
		}
		k8sVersions = append(k8sVersions, versions[len(versions)-1], k8sVersionToUpgrade[len(k8sVersionToUpgrade)-1])

	} else {
		k8sVersions = append(k8sVersions, versions[len(versions)-2], versions[len(versions)-1])
	}

	return k8sVersions, err
}

// extractRancherVersion is a helper uses prime.ribs.rancher.io to get the latest if no version provided, returns empty 
// if the provided version is a community release 
func extractRancherVersion(urlb, rancherVersion string) (string, error) {
	rancherVersionData, err := getResponseData(urlb)
	if err != nil {
		return "", err
	}

	versionFilter := regexp.MustCompile(`<b class="release-title-tag">(v\d+\.\d+\.\d+)</b>`)
	ids := versionFilter.FindAllStringSubmatch(string(rancherVersionData), -1)

	if len(ids) == 0 {
		fmt.Println("No matching `id` fields found in the response.")
		return "", err
	}
	var versions []*version.Version
	for _, id := range ids {
		if len(id) > 1 {
			ver, err := version.NewVersion(id[1])
			if err != nil {
				return "", err
			}
			if rancherVersion == ver.Original() {
				return ver.Original(), nil
			}
			versions = append(versions, ver)
		}
	}

	sort.Sort(version.Collection(versions))
	if rancherVersion == "" {
		rancherVersion = "v" + versions[len(versions)-1].String()
	}
	return "", nil
}

// downloadFile is a helper that downloads the community release images and copies the contents to a file named rancher-images.txt
// copies the contents to a file named rancher-images-pre-release-versions.txt for rancher pre-release versions.
func downloadFile(url, file string) error {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error fetching URL: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(file)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// writeToConfigFile is a helper that adds the struct RancherK8sVersions to a file testrancherk8s.yaml.
func writeToConfigFile(config RancherK8sVersions) error {
	yamlConfig, err := yaml.Marshal(config)

	dir, dirErr := os.Getwd()
	if dirErr != nil {
		logrus.Errorf("Failed to get current directory: %v", dirErr)
	} else {
		logrus.Infof("Current directory: %s", dir)
	}

	if err != nil {
		return err
	}
	file := "testrancherk8s.yaml"

	absFilePath, absErr := filepath.Abs(file)
	if absErr != nil {
		logrus.Warnf("Failed to resolve absolute file path: %v", absErr)
		logrus.Infof("File created at (relative): %s", file)
	} else {
		logrus.Infof("YAML file successfully created at: %s", absFilePath)
	}

	return os.WriteFile(file, yamlConfig, 0644)
}
