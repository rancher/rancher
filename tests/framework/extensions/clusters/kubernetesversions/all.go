package kubernetesversions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
)

const (
	rancherVersionSetting = "server-version"

	rke1VersionsSetting = "k8s-versions-current"
	rke2ReleasePath     = "v1-rke2-release/releases"
	k3sReleasePath      = "v1-k3s-release/releases"
	gkeVersionPath      = "meta/gkeVersions"
	aksVersionPath      = "meta/aksVersions"
	eksVersionsFileURL  = "raw.githubusercontent.com/rancher/ui/master/lib/shared/addon/utils/amazon.js"

	eksVersionsSliceRegex      = `EKS_VERSIONS = \[\s*(.*?)\s*\]\;`
	eksVersionsSliceItemsRegex = `(?s)'(.*?)'`
)

// ListRKE1AllVersions is a function that uses the management client to list and return all RKE1 versions.
func ListRKE1AllVersions(client *rancher.Client) (allAvailableVersions []string, err error) {
	setting, err := client.Management.Setting.ByID(rke1VersionsSetting)
	if err != nil {
		return
	}
	allAvailableVersions = strings.Split(setting.Value, ",")

	sort.Strings(allAvailableVersions)

	return
}

// ListRKE2AllVersions is a function that uses the management client and releases endpoint to list and return all RKE2 versions.
func ListRKE2AllVersions(client *rancher.Client) (allAvailableVersions []string, err error) {
	setting, err := client.Management.Setting.ByID(rancherVersionSetting)
	if err != nil {
		return
	}
	rancherVersion, err := semver.NewVersion(setting.Value)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s://%s/%s", "http", client.RancherConfig.Host, rke2ReleasePath)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	bearerToken := fmt.Sprintf("Bearer %s", client.RancherConfig.AdminToken)
	req.Header.Add("Authorization", bearerToken)

	bodyBytes, err := getRequest(req, client)
	if err != nil {
		return
	}

	var mapResponse map[string]interface{}
	if err = json.Unmarshal([]byte(bodyBytes), &mapResponse); err != nil {
		return
	}

	releases := mapResponse["data"].([]interface{})

	allAvailableVersions = sortReleases(rancherVersion, releases)

	sort.Strings(allAvailableVersions)

	return
}

// ListK3SAllVersions is a function that uses the management client and releases endpoint to list and return all K3s versions.
func ListK3SAllVersions(client *rancher.Client) (allAvailableVersions []string, err error) {
	setting, err := client.Management.Setting.ByID(rancherVersionSetting)
	if err != nil {
		return
	}
	rancherVersion, err := semver.NewVersion(setting.Value)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s://%s/%s", "http", client.RancherConfig.Host, k3sReleasePath)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	bearerToken := fmt.Sprintf("Bearer %s", client.RancherConfig.AdminToken)
	req.Header.Add("Authorization", bearerToken)

	bodyBytes, err := getRequest(req, client)
	if err != nil {
		return
	}

	var mapResponse map[string]interface{}
	if err = json.Unmarshal([]byte(bodyBytes), &mapResponse); err != nil {
		return
	}

	releases := mapResponse["data"].([]interface{})

	allAvailableVersions = sortReleases(rancherVersion, releases)

	sort.Strings(allAvailableVersions)

	return
}

// ListGKEAllVersions is a function that uses the management client base and gke meta endpoint to list and return all GKE versions.
func ListGKEAllVersions(client *rancher.Client, projectID, cloudCredentialID, zone, region string) (availableVersions []string, err error) {
	url := fmt.Sprintf("%s://%s/%s", "https", client.RancherConfig.Host, gkeVersionPath)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Add("Authorization", "Bearer "+client.RancherConfig.AdminToken)

	q := req.URL.Query()
	q.Add("cloudCredentialId", cloudCredentialID)

	if zone != "" {
		q.Add("zone", zone)
	} else if region != "" {
		q.Add("region", region)
	}

	q.Add("projectId", projectID)
	req.URL.RawQuery = q.Encode()

	bodyBytes, err := getRequest(req, client)
	if err != nil {
		return
	}

	var mapResponse map[string]interface{}
	if err = json.Unmarshal([]byte(bodyBytes), &mapResponse); err != nil {
		return
	}

	validMasterVersionsResponse := mapResponse["validMasterVersions"].([]interface{})

	for _, version := range validMasterVersionsResponse {
		availableVersions = append(availableVersions, version.(string))
	}

	return
}

// ListAKSAllVersions is a function that uses the management client base and aks meta endpoint to list and return all AKS versions.
func ListAKSAllVersions(client *rancher.Client, cloudCredentialID, region string) (allAvailableVersions []string, err error) {
	url := fmt.Sprintf("%s://%s/%s", "https", client.RancherConfig.Host, aksVersionPath)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Add("Authorization", "Bearer "+client.RancherConfig.AdminToken)

	q := req.URL.Query()
	q.Add("cloudCredentialId", cloudCredentialID)
	q.Add("region", region)
	req.URL.RawQuery = q.Encode()

	bodyBytes, err := getRequest(req, client)
	if err != nil {
		return
	}

	var versionsSlice []interface{}
	if err = json.Unmarshal([]byte(bodyBytes), &versionsSlice); err != nil {
		return
	}

	for _, version := range versionsSlice {
		allAvailableVersions = append(allAvailableVersions, version.(string))
	}

	return
}

// ListEKSAllVersions is a function that uses the management client base and rancher/UI repository to list and return all AKS versions.
func ListEKSAllVersions(client *rancher.Client) (allAvailableVersions []string, err error) {
	url := fmt.Sprintf("%s://%s", "https", eksVersionsFileURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	bodyBytes, err := getRequest(req, client)
	if err != nil {
		return
	}

	r := regexp.MustCompile(eksVersionsSliceRegex)
	match := r.FindStringSubmatch(string(bodyBytes))
	if len(match) == 0 {
		return
	}
	versions := match[1]
	rx := regexp.MustCompile(eksVersionsSliceItemsRegex)
	out := rx.FindAllStringSubmatch(versions, -1)

	for _, version := range out {
		if len(version) < 1 {
			continue
		}

		allAvailableVersions = append(allAvailableVersions, version[1])
	}

	return
}

// sortReleases is a private function that sorts release structs that are used for K3S and RKE2.
// Sorted versions determined by these conditions:
//  1. Current rancher version is between min and max channel versions
//  2. Release struct has serverArgs and agentArgs not empty fields
//  3. Possible newest version of the minimum channel version
func sortReleases(rancherVersion *semver.Version, releases []interface{}) (allAvailableVersions []string) {
	availableVersionsMap := map[string]semver.Version{}

	for _, release := range releases {
		_, serverArgsOk := release.(map[string]interface{})["serverArgs"].(map[string]interface{})
		_, agentArgsOk := release.(map[string]interface{})["agentArgs"].(map[string]interface{})

		if !serverArgsOk || !agentArgsOk {
			continue
		}

		maxVersion := release.(map[string]interface{})["maxChannelServerVersion"].(string)
		minVersion := release.(map[string]interface{})["minChannelServerVersion"].(string)
		kubernetesVersion := release.(map[string]interface{})["version"].(string)

		maxRancherVersion, err := semver.NewVersion(strings.TrimPrefix(maxVersion, "v"))
		if err != nil {
			continue
		}
		minRancherVersion, err := semver.NewVersion(strings.TrimPrefix(minVersion, "v"))
		if err != nil {
			continue
		}

		releaseKubernetesVersion, err := semver.NewVersion(strings.TrimPrefix(kubernetesVersion, "v"))
		if err != nil {
			continue
		}

		if !rancherVersion.GreaterThan(minRancherVersion) && !rancherVersion.LessThan(maxRancherVersion) {
			continue
		}

		value, ok := availableVersionsMap[minRancherVersion.String()]

		if !ok || value.LessThan(releaseKubernetesVersion) {
			availableVersionsMap[minRancherVersion.String()] = *releaseKubernetesVersion
		}
	}

	for _, v := range availableVersionsMap {
		allAvailableVersions = append(allAvailableVersions, fmt.Sprintf("v"+v.String()))
	}

	return
}

// getRequest is a private function that used to reach external endpoints when the clients aren't usable.
func getRequest(request *http.Request, client *rancher.Client) (bodyBytes []byte, err error) {
	resp, err := client.Management.APIBaseClient.Ops.Client.Do(request)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	return
}
