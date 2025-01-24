package main

import (
	"fmt"
	"io"
	"net/http"

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
	RancherVersion              string `json:"rancherVersion" yaml:"rancherVersion"`
	RancherRKE2Version          string `json:"rancherRKE2Version" yaml:"rancherRKE2Version"`
	RancherRKE2VersionToUpgrade string `json:"rancherRKE2VersionToUpgrade" yaml:"rancherRKE2VersionToUpgrade"`
	RancherK3sVersion           string `json:"rancherK3sVersion" yaml:"rancherK3sVersion"`
	RancherK3sVersionToUpgrade  string `json:"rancherK3sVersionToUpgrade" yaml:"rancherK3sVersionToUpgrade"`
	RancherRKEVersion           string `json:"rancherRKEVersion" yaml:"rancherRKEVersion"`
	RancherRKEVersionToUpgrade  string `json:"rancherRKEVersionToUpgrade" yaml:"rancherRKEVersionToUpgrade"`
}

const (
	baseURL                 = "https://prime.ribs.rancher.io/"
	rke2K3sVersionFilterURL = "/rancher-images.txt"
	rkeVersionFilterURL     = "/rancher-rke-k8s-versions.txt"
	rke2Filter              = `rancher/hardened-kubernetes:v\d+\.\d+\.\d+(-rke2r\d)?`
	k3sFilter               = `rancher/k3s-upgrade:v\d+\.\d+\.\d+(-k3s\d)`
)

var (
	rancherVersion          = os.Getenv("HA_IMAGE_TAG")
	rancherVersionToUpgrade = os.Getenv("HA_IMAGE_TAG_TO_UPGRADE")
)

func main() {
	var err error
	urlb := baseURL + "index.html"

	if rancherVersion == "" {
		rancherVersion, err = extractRancherVersion(urlb)
		if err != nil {
			return
		}
	}

	logrus.Info("Rancher version: " + rancherVersion)

	k8sImageBaseUrl := baseURL + "rancher/" + rancherVersion

	rke2K3sK8sVersionsUrl := k8sImageBaseUrl + rke2K3sVersionFilterURL
	rkeK8sVersionsUrl := k8sImageBaseUrl + rkeVersionFilterURL

	rke2K8sVersion, err := extractRke2K3sVersions(rke2K3sK8sVersionsUrl, rke2Filter)
	if err != nil {
		return
	}

	k3sK8sVersion, err := extractRke2K3sVersions(rke2K3sK8sVersionsUrl, k3sFilter)
	if err != nil {
		return
	}

	rkeK8sVersion, err := extractRkeVersions(rkeK8sVersionsUrl)
	if err != nil {
		return
	}

	rancherK8sVersions := RancherK8sVersions{}
	rancherK8sVersions.RancherImageTag = rancherVersion
	rancherK8sVersions.RancherVersion = strings.Trim(rancherVersion, "v")
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

func getResponseData(urlb string) (string, error) {
	resp, err := http.Get(urlb)
	if err != nil {
		fmt.Printf("Error fetching URL: %v\n", err)
		return "", err
	}
	defer resp.Body.Close()

	responseData, err := io.ReadAll(resp.Body)

	return string(responseData), err
}

func getRke2K3sVersionMap(urlb, matchFilter string) (latestVersions []*version.Version, err error) {
	k8sVersionData, err := getResponseData(urlb)
	if err != nil {
		return nil, err
	}

	versionFilter := regexp.MustCompile(matchFilter)
	versions := versionFilter.FindAllStringSubmatch(k8sVersionData, -1)

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

func extractRke2K3sVersions(urlb, matchFilter string) (k8sVersions []string, err error) {
	latestVersions, err := getRke2K3sVersionMap(urlb, matchFilter)
	var k8sVersionUpgrade []*version.Version

	if rancherVersionToUpgrade != "" {
		re := regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+`)
		urlb = re.ReplaceAllString(urlb, rancherVersionToUpgrade)
		k8sVersionUpgrade, err = getRke2K3sVersionMap(urlb, matchFilter)
		if err != nil {
			return nil, err
		}
		k8sVersions = append(k8sVersions, latestVersions[len(latestVersions)-1].Original(), k8sVersionUpgrade[len(k8sVersionUpgrade)-1].Original())
	} else {
		k8sVersions = append(k8sVersions, latestVersions[len(latestVersions)-1].Original(), latestVersions[len(latestVersions)-2].Original())
	}

	return k8sVersions, nil
}

func getRkeVersionMap(urlb string) ([]string, error) {
	k8sVersionData, err := getResponseData(urlb)
	if err != nil {
		return nil, err
	}

	var versions []string
	if len(k8sVersionData) <= 0 {
		return nil, fmt.Errorf("Empty list")
	}

	k8sVersionData = strings.TrimSpace(k8sVersionData)

	versions = strings.Split(k8sVersionData, "\n")
	if len(versions) > 4 {
		return nil, fmt.Errorf("Unexpected length of versions %s", versions)
	}
	return versions, err
}
func extractRkeVersions(urlb string) (k8sVersions []string, err error) {
	versions, err := getRkeVersionMap(urlb)
	if err != nil {
		return nil, err
	}
	var k8sVersionToUpgrade []string
	if rancherVersionToUpgrade != "" {
		re := regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+`)
		urlb = re.ReplaceAllString(urlb, rancherVersionToUpgrade)
		k8sVersionToUpgrade, err = getRkeVersionMap(urlb)
		if err != nil {
			return nil, err
		}
		k8sVersions = append(k8sVersions, versions[len(versions)-1], k8sVersionToUpgrade[len(k8sVersionToUpgrade)-1])

	} else {
		k8sVersions = append(k8sVersions, versions[len(versions)-1], versions[len(versions)-2])
	}

	return k8sVersions, err
}

func extractRancherVersion(urlb string) (string, error) {
	rancherVersionData, err := getResponseData(urlb)
	if err != nil {
		return "", err
	}

	versionFilter := regexp.MustCompile(`<b class="release-title-tag">(v\d+\.\d+\.\d+)</b>`)
	ids := versionFilter.FindAllStringSubmatch(rancherVersionData, -1)

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
			versions = append(versions, ver)
		}
	}

	sort.Sort(version.Collection(versions))
	rancherVersion := "v" + versions[len(versions)-1].String()

	return rancherVersion, nil

}

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

	return os.WriteFile(file, yamlConfig, 0644)
}
