package rke

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/rancher/kontainer-driver-metadata/rke/templates"
	"github.com/rancher/types/image"
	"github.com/rancher/types/kdm"
	"github.com/sirupsen/logrus"
)

const (
	rkeDataFilePath = "./data/data.json"
)

var (
	DriverData     kdm.Data
	TemplateData   map[string]map[string]string
	MissedTemplate map[string][]string
	m              = image.Mirror
)

func initData() {
	DriverData = kdm.Data{
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

	// CIS
	DriverData.CisConfigParams = loadCisConfigParams()
	DriverData.CisBenchmarkVersionInfo = loadCisBenchmarkVersionInfo()
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
	MissedTemplate = map[string][]string{}
	for k8sVersion := range DriverData.K8sVersionRKESystemImages {
		toMatch, err := semver.Make(k8sVersion[1:])
		if err != nil {
			panic(fmt.Sprintf("k8sVersion not sem-ver %s %v", k8sVersion, err))
		}
		TemplateData[k8sVersion] = map[string]string{}
		for plugin, pluginData := range DriverData.K8sVersionedTemplates {
			if plugin == kdm.TemplateKeys {
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
				if val, ok := MissedTemplate[plugin]; ok {
					val = append(val, k8sVersion)
					MissedTemplate[plugin] = val
				} else {
					MissedTemplate[plugin] = []string{k8sVersion}
				}
				continue
			}
			TemplateData[k8sVersion][plugin] = fmt.Sprintf("range=%s key=%s", matchedRange, matchedKey)
		}
	}
}

func GenerateData() {
	initData()

	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")

	if err := enc.Encode(TemplateData); err != nil {
		panic(fmt.Sprintf("error encoding template data %v", err))
	}
	fmt.Println(buf.String())

	if len(MissedTemplate) != 0 {
		logrus.Warnf("found k8s versions without a template")
		for plugin, data := range MissedTemplate {
			logrus.Warnf("no %s template for k8sVersions %v \n", plugin, data)
		}
	}

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
