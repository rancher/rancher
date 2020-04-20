package rke

import (
	"encoding/json"
	"fmt"
	"github.com/blang/semver"
	"github.com/rancher/kontainer-driver-metadata/rke/templates"
	"github.com/sirupsen/logrus"
	"os"
	"strings"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/image"
)

const (
	rkeDataFilePath = "./data/data.json"
)

// Data to be written in dataFilePath, dynamically populated on init() with the latest versions
type Data struct {

	// K8sVersionServiceOptions - service options per k8s version
	K8sVersionServiceOptions  map[string]v3.KubernetesServicesOptions
	K8sVersionRKESystemImages map[string]v3.RKESystemImages

	// Addon Templates per K8s version ("default" where nothing changes for k8s version)
	K8sVersionedTemplates map[string]map[string]string

	// K8sVersionInfo - min/max RKE+Rancher versions per k8s version
	K8sVersionInfo map[string]v3.K8sVersionInfo

	//Default K8s version for every rancher version
	RancherDefaultK8sVersions map[string]string

	//Default K8s version for every rke version
	RKEDefaultK8sVersions map[string]string

	K8sVersionDockerInfo map[string][]string

	// K8sVersionWindowsServiceOptions - service options per windows k8s version
	K8sVersionWindowsServiceOptions map[string]v3.KubernetesServicesOptions
}

var (
	DriverData     Data
	MissedTemplate map[string][]string
	m              = image.Mirror
)

func init() {
	DriverData = Data{
		K8sVersionRKESystemImages: loadK8sRKESystemImages(),
	}

	for version, images := range DriverData.K8sVersionRKESystemImages {
		longName := "rancher/hyperkube:" + version
		if !strings.HasPrefix(longName, images.Kubernetes) {
			panic(fmt.Sprintf("For K8s version %s, the Kubernetes image tag should be a substring of %s, currently it is %s", version, version, images.Kubernetes))
		}
	}

	DriverData.RKEDefaultK8sVersions = loadRKEDefaultK8sVersions()
	DriverData.RancherDefaultK8sVersions = loadRancherDefaultK8sVersions()

	validateDefaultPresent(DriverData.RKEDefaultK8sVersions)

	DriverData.K8sVersionedTemplates = templates.LoadK8sVersionedTemplates()

	validateTemplateMatch()

	DriverData.K8sVersionServiceOptions = loadK8sVersionServiceOptions()

	DriverData.K8sVersionInfo = loadK8sVersionInfo()

	// init Windows versions
	DriverData.K8sVersionWindowsServiceOptions = loadK8sVersionWindowsServiceOptions()
	DriverData.K8sVersionDockerInfo = loadK8sVersionDockerInfo()

}

func validateDefaultPresent(versions map[string]string) {
	for _, defaultK8s := range versions {
		if _, ok := DriverData.K8sVersionRKESystemImages[defaultK8s]; !ok {
			panic(fmt.Sprintf("Default K8s version %v is not found in the K8sVersionToRKESystemImages", defaultK8s))
		}
	}
}

func validateTemplateMatch() {
	MissedTemplate = map[string][]string{}
	for k8sVersion := range DriverData.K8sVersionRKESystemImages {
		toMatch, err := semver.Make(k8sVersion[1:])
		if err != nil {
			panic(fmt.Sprintf("k8sVersion not sem-ver %s %v", k8sVersion, err))
		}
		for plugin, pluginData := range DriverData.K8sVersionedTemplates {
			if plugin == templates.TemplateKeys {
				continue
			}
			matchedKey := ""
			matchedRange := ""
			for toTestRange, key := range pluginData {
				testRange, err := semver.ParseRange(toTestRange)
				if err != nil {
					panic(fmt.Sprintf("range for %s not sem-ver %v %v", plugin, testRange, err))
				}
				if testRange(toMatch) {
					// only one range should be matched
					if matchedKey != "" {
						panic(fmt.Sprintf("k8sVersion %s for plugin %s passing range %s, conflict range matching with %s",
							k8sVersion, plugin, toTestRange, matchedRange))
					}
					matchedKey = key
					matchedRange = toTestRange
				}
			}

			// no template found
			if matchedKey == "" {
				// check if plugin was introduced later
				if templateRanges, ok := templates.TemplateIntroducedRanges[plugin]; ok {
					// as we want to use the logic outside this loop, we check every range and if its matched, we set pluginCheck to true
					// in the end, we check if any of the ranges have matched, if so, we dont skip the logic to add a missing template (because every version matched in the range should have a template)
					var pluginCheck bool
					// plugin has ranges configured
					for _, toTestRange := range templateRanges {
						testRange, err := semver.ParseRange(toTestRange)
						if err != nil {
							panic(fmt.Sprintf("range for %s not sem-ver %v %v", plugin, testRange, err))
						}
						if testRange(toMatch) {
							pluginCheck = true
						}
					}
					if !pluginCheck {
						// logrus.Warnf("skipping %s for %s", k8sVersion, plugin)
						continue
					}

				}

				// if version not already found for that plugin, append it, else create it
				if val, ok := MissedTemplate[plugin]; ok {
					val = append(val, k8sVersion)
					MissedTemplate[plugin] = val
				} else {
					MissedTemplate[plugin] = []string{k8sVersion}
				}
				continue
			}
		}
	}
}

func GenerateData() {
	if len(os.Args) == 2 {
		splitStr := strings.SplitN(os.Args[1], "=", 2)
		if len(splitStr) == 2 {
			if splitStr[0] == "--write-data" && splitStr[1] == "true" {
				if len(MissedTemplate) != 0 {
					logrus.Warnf("found k8s versions without a template")
					for plugin, data := range MissedTemplate {
						logrus.Warnf("no %s template for k8sVersions %v \n", plugin, data)
					}
				}

				//todo: zip file
				strData, _ := json.MarshalIndent(DriverData, "", " ")
				jsonFile, err := os.Create(rkeDataFilePath)
				if err != nil {
					panic(fmt.Errorf("err creating data file %v", err))
				}
				defer jsonFile.Close()
				_, err = jsonFile.Write(strData)
				if err != nil {
					panic(fmt.Errorf("err writing jsonFile %v", err))
				}
				fmt.Println("finished generating data.json")
			}
		}
	}
}
