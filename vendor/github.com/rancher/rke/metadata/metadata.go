package metadata

import (
	"context"
	"strings"

	mVersion "github.com/mcuadros/go-version"
	"github.com/rancher/kontainer-driver-metadata/rke"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

var (
	RKEVersion                  string
	DefaultK8sVersion           string
	K8sVersionToTemplates       map[string]map[string]string
	K8sVersionToRKESystemImages map[string]v3.RKESystemImages
	K8sVersionToServiceOptions  map[string]v3.KubernetesServicesOptions
	K8sVersionToDockerVersions  map[string][]string
	K8sVersionsCurrent          []string
	K8sBadVersions              = map[string]bool{}

	K8sVersionToWindowsServiceOptions map[string]v3.KubernetesServicesOptions
)

func InitMetadata(ctx context.Context) error {
	initK8sRKESystemImages()
	initAddonTemplates()
	initServiceOptions()
	initDockerOptions()
	return nil
}

const RKEVersionDev = "v1.0.9"

func initAddonTemplates() {
	K8sVersionToTemplates = rke.DriverData.K8sVersionedTemplates
}

func initServiceOptions() {
	K8sVersionToServiceOptions = interface{}(rke.DriverData.K8sVersionServiceOptions).(map[string]v3.KubernetesServicesOptions)
	K8sVersionToWindowsServiceOptions = rke.DriverData.K8sVersionWindowsServiceOptions
}

func initDockerOptions() {
	K8sVersionToDockerVersions = rke.DriverData.K8sVersionDockerInfo
}

func initK8sRKESystemImages() {
	K8sVersionToRKESystemImages = map[string]v3.RKESystemImages{}
	rkeData := rke.DriverData
	// non released versions
	if RKEVersion == "" {
		RKEVersion = RKEVersionDev
	}
	DefaultK8sVersion = rkeData.RKEDefaultK8sVersions["default"]
	if defaultK8sVersion, ok := rkeData.RKEDefaultK8sVersions[RKEVersion[1:]]; ok {
		DefaultK8sVersion = defaultK8sVersion
	}
	maxVersionForMajorK8sVersion := map[string]string{}
	for k8sVersion, systemImages := range rkeData.K8sVersionRKESystemImages {
		rkeVersionInfo, ok := rkeData.K8sVersionInfo[k8sVersion]
		if ok {
			// RKEVersion = 0.2.4, DeprecateRKEVersion = 0.2.2
			if rkeVersionInfo.DeprecateRKEVersion != "" && mVersion.Compare(RKEVersion, rkeVersionInfo.DeprecateRKEVersion, ">=") {
				K8sBadVersions[k8sVersion] = true
				continue
			}
			// RKEVersion = 0.2.4, MinVersion = 0.2.5, don't store
			lowerThanMin := rkeVersionInfo.MinRKEVersion != "" && mVersion.Compare(RKEVersion, rkeVersionInfo.MinRKEVersion, "<")
			if lowerThanMin {
				continue
			}
		}
		// store all for upgrades
		K8sVersionToRKESystemImages[k8sVersion] = interface{}(systemImages).(v3.RKESystemImages)

		majorVersion := getTagMajorVersion(k8sVersion)
		maxVersionInfo, ok := rkeData.K8sVersionInfo[majorVersion]
		if ok {
			// RKEVersion = 0.2.4, MaxVersion = 0.2.3, don't use in current
			greaterThanMax := maxVersionInfo.MaxRKEVersion != "" && mVersion.Compare(RKEVersion, maxVersionInfo.MaxRKEVersion, ">")
			if greaterThanMax {
				continue
			}
		}
		if curr, ok := maxVersionForMajorK8sVersion[majorVersion]; !ok || mVersion.Compare(k8sVersion, curr, ">") {
			maxVersionForMajorK8sVersion[majorVersion] = k8sVersion
		}
	}
	for _, k8sVersion := range maxVersionForMajorK8sVersion {
		K8sVersionsCurrent = append(K8sVersionsCurrent, k8sVersion)
	}
}

func getTagMajorVersion(tag string) string {
	splitTag := strings.Split(tag, ".")
	if len(splitTag) < 2 {
		return ""
	}
	return strings.Join(splitTag[:2], ".")
}
