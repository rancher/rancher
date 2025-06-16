package kontainerdrivermetadata

import (
	mVersion "github.com/mcuadros/go-version"
	rketypes "github.com/rancher/rke/types"
	"github.com/rancher/rke/util"
)

func GetK8sVersionInfo(
	rancherVersion string,
	rkeSysImages map[string]rketypes.RKESystemImages,
	linuxSvcOptions map[string]rketypes.KubernetesServicesOptions,
	windowsSvcOptions map[string]rketypes.KubernetesServicesOptions,
	rancherVersions map[string]rketypes.K8sVersionInfo,
) (linuxInfo, windowsInfo *VersionInfo) {

	linuxInfo = newVersionInfo()
	windowsInfo = newVersionInfo()

	maxVersionForMajorK8sVersion := map[string]string{}
	for k8sVersion := range rkeSysImages {
		if rancherVersionInfo, ok := rancherVersions[k8sVersion]; ok && toIgnoreForAllK8s(rancherVersionInfo, rancherVersion) {
			continue
		}
		majorVersion := util.GetTagMajorVersion(k8sVersion)
		if majorVersionInfo, ok := rancherVersions[majorVersion]; ok && toIgnoreForK8sCurrent(majorVersionInfo, rancherVersion) {
			continue
		}
		if curr, ok := maxVersionForMajorK8sVersion[majorVersion]; !ok || mVersion.Compare(k8sVersion, curr, ">") {
			maxVersionForMajorK8sVersion[majorVersion] = k8sVersion
		}
	}

	for majorVersion, k8sVersion := range maxVersionForMajorK8sVersion {
		sysImgs, exist := rkeSysImages[k8sVersion]
		if !exist {
			continue
		}

		if svcOptions, exist := linuxSvcOptions[majorVersion]; exist {
			linuxInfo.RKESystemImages[k8sVersion] = sysImgs
			linuxInfo.KubernetesServicesOptions[k8sVersion] = svcOptions
		}

		if svcOptions, exist := windowsSvcOptions[majorVersion]; exist {
			windowsInfo.RKESystemImages[k8sVersion] = sysImgs
			windowsInfo.KubernetesServicesOptions[k8sVersion] = svcOptions
		}
	}

	return linuxInfo, windowsInfo
}

type VersionInfo struct {
	RKESystemImages           map[string]rketypes.RKESystemImages
	KubernetesServicesOptions map[string]rketypes.KubernetesServicesOptions
}

func newVersionInfo() *VersionInfo {
	return &VersionInfo{
		RKESystemImages:           map[string]rketypes.RKESystemImages{},
		KubernetesServicesOptions: map[string]rketypes.KubernetesServicesOptions{},
	}
}
