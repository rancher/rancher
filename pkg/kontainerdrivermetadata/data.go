package kontainerdrivermetadata

import (
	mVersion "github.com/mcuadros/go-version"
	rketypes "github.com/rancher/rke/types"
)

const (
	DataJSONLocation = "/var/lib/rancher-data/driver-metadata/data.json"
)

func toIgnoreForAllK8s(rancherVersionInfo rketypes.K8sVersionInfo, rancherVersion string) bool {
	if rancherVersionInfo.DeprecateRancherVersion != "" && mVersion.Compare(rancherVersion, rancherVersionInfo.DeprecateRancherVersion, ">=") {
		return true
	}
	if rancherVersionInfo.MinRancherVersion != "" && mVersion.Compare(rancherVersion, rancherVersionInfo.MinRancherVersion, "<") {
		// only respect min versions, even if max is present - we need to support upgraded clusters
		return true
	}
	return false
}

func toIgnoreForK8sCurrent(majorVersionInfo rketypes.K8sVersionInfo, rancherVersion string) bool {
	if majorVersionInfo.MaxRancherVersion != "" && mVersion.Compare(rancherVersion, majorVersionInfo.MaxRancherVersion, ">") {
		// include in K8sVersionCurrent only if less then max version
		return true
	}
	return false
}
