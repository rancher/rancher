package kontainerdrivermetadata

import (
	"strings"

	mVersion "github.com/mcuadros/go-version"
	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"

	"github.com/sirupsen/logrus"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rke/util"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetRKESystemImages(k8sVersion string, sysImageLister v3.RKEK8sSystemImageLister, sysImages v3.RKEK8sSystemImageInterface) (v3.RKESystemImages, error) {
	name := k8sVersion
	sysImage, err := sysImageLister.Get(namespace.GlobalNamespace, name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return v3.RKESystemImages{}, err
		}
		sysImage, err = sysImages.GetNamespaced(namespace.GlobalNamespace, name, metav1.GetOptions{})
		if err != nil {
			return v3.RKESystemImages{}, err
		}
	}
	return sysImage.SystemImages, err
}

func GetRKEAddonTemplate(addonName string, addonLister v3.RKEAddonLister, addons v3.RKEAddonInterface) (string, error) {
	addon, err := addonLister.Get(namespace.GlobalNamespace, addonName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return "", err
		}
		addon, err = addons.GetNamespaced(namespace.GlobalNamespace, addonName, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
	}
	if addon.Labels[sendRKELabel] == "false" {
		return "", nil
	}
	return addon.Template, err
}

func GetRKEK8sServiceOptions(k8sVersion string, svcOptionLister v3.RKEK8sServiceOptionLister, svcOptions v3.RKEK8sServiceOptionInterface, osType OSType) (*v3.KubernetesServicesOptions, error) {
	names := []string{
		getVersionNameWithOsType(k8sVersion, osType),
		getVersionNameWithOsType(util.GetTagMajorVersion(k8sVersion), osType),
	}
	var k8sSvcOption *v3.KubernetesServicesOptions
	for _, name := range names {
		obj, err := svcOptionLister.Get(namespace.GlobalNamespace, name)
		if err != nil {
			if !errors.IsNotFound(err) {
				return k8sSvcOption, err
			}
			continue
		}
		if obj.Labels[sendRKELabel] == "false" {
			logrus.Infof("svcOption false k8sVersion %s", k8sVersion)
			return k8sSvcOption, nil
		}
		return &obj.ServiceOptions, nil
	}

	for _, name := range names {
		obj, err := svcOptions.GetNamespaced(namespace.GlobalNamespace, name, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return k8sSvcOption, err
			}
			continue
		}
		if obj.Labels[sendRKELabel] == "false" {
			logrus.Infof("svcOption false k8sVersion %s", k8sVersion)
			return k8sSvcOption, nil
		}
		return &obj.ServiceOptions, nil
	}
	return k8sSvcOption, nil
}

func GetK8sVersionInfo(
	rancherVersion string,
	rkeSysImages map[string]v3.RKESystemImages,
	linuxSvcOptions map[string]v3.KubernetesServicesOptions,
	windowsSvcOptions map[string]v3.KubernetesServicesOptions,
	rancherVersions map[string]v3.K8sVersionInfo,
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
		// windows has been supported since v1.14,
		// the following logic would not find `< v1.14` service options
		if svcOptions, exist := windowsSvcOptions[majorVersion]; exist {
			// only keep the related images for windows
			windowsSysImgs := v3.RKESystemImages{
				NginxProxy:                sysImgs.NginxProxy,
				CertDownloader:            sysImgs.CertDownloader,
				KubernetesServicesSidecar: sysImgs.KubernetesServicesSidecar,
				Kubernetes:                sysImgs.Kubernetes,
				WindowsPodInfraContainer:  sysImgs.WindowsPodInfraContainer,
			}

			windowsInfo.RKESystemImages[k8sVersion] = windowsSysImgs
			windowsInfo.KubernetesServicesOptions[k8sVersion] = svcOptions
		}
		if svcOptions, exist := linuxSvcOptions[majorVersion]; exist {
			// clean the unrelated images for linux
			sysImgs.WindowsPodInfraContainer = ""

			linuxInfo.RKESystemImages[k8sVersion] = sysImgs
			linuxInfo.KubernetesServicesOptions[k8sVersion] = svcOptions
		}
	}

	return linuxInfo, windowsInfo
}

type VersionInfo struct {
	RKESystemImages           map[string]v3.RKESystemImages
	KubernetesServicesOptions map[string]v3.KubernetesServicesOptions
}

func newVersionInfo() *VersionInfo {
	return &VersionInfo{
		RKESystemImages:           map[string]v3.RKESystemImages{},
		KubernetesServicesOptions: map[string]v3.KubernetesServicesOptions{},
	}
}

func GetRancherVersion() string {
	rancherVersion := settings.ServerVersion.Get()
	if strings.HasPrefix(rancherVersion, "dev") || strings.HasPrefix(rancherVersion, "master") {
		return RancherVersionDev
	}
	if strings.HasPrefix(rancherVersion, "v") {
		return rancherVersion[1:]
	}
	return rancherVersion
}
