package rke

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/blang/semver"
	"github.com/rancher/kontainer-driver-metadata/rke/templates"
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
	DriverData Data
	TemplateData  map[string]map[string]string
	m          = image.Mirror
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
	TemplateData = map[string]map[string]string{}
	for k8sVersion := range DriverData.K8sVersionRKESystemImages {
		toMatch, err := semver.Make(k8sVersion[1:])
		if err != nil {
			panic(fmt.Sprintf("k8sVersion not sem-ver %s %v", k8sVersion, err))
		}
		TemplateData[k8sVersion] = map[string]string{}
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
			if matchedKey == "" {
				panic(fmt.Sprintf("no template found for k8sVersion %s plugin %s", k8sVersion, plugin))
			}
			TemplateData[k8sVersion][plugin] = fmt.Sprintf("range=%s key=%s", matchedRange, matchedKey)
		}
	}
}

func GenerateData() {
	if len(os.Args) == 2 {
		splitStr := strings.SplitN(os.Args[1], "=", 2)
		if len(splitStr) == 2 {
			if splitStr[0] == "--write-data" && splitStr[1] == "true" {
				
				buf := new(bytes.Buffer)
				enc := json.NewEncoder(buf)
				enc.SetEscapeHTML(false)
				enc.SetIndent("", " ")

				if err := enc.Encode(TemplateData); err != nil {
					panic(fmt.Sprintf("error encoding template data %v", err))
				}
				fmt.Println(buf.String())

				fmt.Println("generating data.json")
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
			}
		}
	}
}
