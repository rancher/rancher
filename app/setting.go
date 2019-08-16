package app

import (
	"sort"

	"github.com/rancher/rke/util"

	"github.com/pborman/uuid"
	metadata "github.com/rancher/kontainer-driver-metadata/rke"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/settings"
)

func addSetting() error {
	if err := settings.InstallUUID.SetIfUnset(uuid.NewRandom().String()); err != nil {
		return err
	}
	return addK8sVersionData()
}

func addK8sVersionData() error {
	k8sCurrVersions := map[string]interface{}{}

	rancherVersion := kd.GetRancherVersion()
	k8sVersionToRKESystemImages, _, k8sVersionToSvcOptions := kd.GetK8sVersionInfo(metadata.DriverData.K8sVersionRKESystemImages,
		metadata.DriverData.K8sVersionWindowsSystemImages,
		metadata.DriverData.K8sVersionServiceOptions,
		metadata.DriverData.K8sVersionInfo)

	for k := range k8sVersionToRKESystemImages {
		k8sCurrVersions[k] = nil
	}

	var keys []string
	for k := range k8sVersionToRKESystemImages {
		keys = append(keys, util.GetTagMajorVersion(k))
	}
	sort.Strings(keys)

	return kd.SaveSettings(k8sCurrVersions, k8sVersionToSvcOptions, metadata.DriverData.RancherDefaultK8sVersions, rancherVersion, keys)
}
