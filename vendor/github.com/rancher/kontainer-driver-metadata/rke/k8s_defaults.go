package rke

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rancher/kontainer-driver-metadata/rke/templates"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
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

	K8sVersionWindowsSystemImages   map[string]v3.WindowsSystemImages
	K8sVersionWindowsServiceOptions map[string]v3.KubernetesServicesOptions
}

var (
	DriverData Data
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

	DriverData.K8sVersionServiceOptions = loadK8sVersionServiceOptions()

	DriverData.K8sVersionInfo = loadK8sVersionInfo()

	DriverData.K8sVersionedTemplates = templates.LoadK8sVersionedTemplates()

	DriverData.RKEDefaultK8sVersions = loadRKEDefaultK8sVersions()

	for _, defaultK8s := range DriverData.RKEDefaultK8sVersions {
		if _, ok := DriverData.K8sVersionRKESystemImages[defaultK8s]; !ok {
			panic(fmt.Sprintf("Default K8s version %v is not found in the K8sVersionToRKESystemImages", defaultK8s))
		}
	}

	// init Windows versions
	DriverData.K8sVersionWindowsSystemImages = loadK8sVersionWindowsSystemimages()
	DriverData.K8sVersionWindowsServiceOptions = loadK8sVersionWindowsServiceOptions()
	DriverData.K8sVersionDockerInfo = loadK8sVersionDockerInfo()

	DriverData.RancherDefaultK8sVersions = loadRancherDefaultK8sVersions()

}

func GenerateData() {
	if len(os.Args) == 2 {
		splitStr := strings.SplitN(os.Args[1], "=", 2)
		if len(splitStr) == 2 {
			if splitStr[0] == "--write-data" && splitStr[1] == "true" {
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
