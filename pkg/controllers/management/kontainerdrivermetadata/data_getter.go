package kontainerdrivermetadata

import (
	"fmt"

	mVersion "github.com/mcuadros/go-version"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	rketypes "github.com/rancher/rke/types"
	"github.com/rancher/rke/util"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetRKESystemImages(k8sVersion string, sysImageLister v3.RkeK8sSystemImageLister, sysImages v3.RkeK8sSystemImageInterface) (rketypes.RKESystemImages, error) {
	name := k8sVersion
	sysImage, err := sysImageLister.Get(namespace.GlobalNamespace, name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return rketypes.RKESystemImages{}, err
		}
		sysImage, err = sysImages.GetNamespaced(namespace.GlobalNamespace, name, metav1.GetOptions{})
		if err != nil {
			return rketypes.RKESystemImages{}, err
		}
	}
	return sysImage.SystemImages, err
}

func GetRKEAddonTemplate(addonName string, addonLister v3.RkeAddonLister, addons v3.RkeAddonInterface) (string, error) {
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

func getRKEServiceOption(name string, svcOptionLister v3.RkeK8sServiceOptionLister, svcOptions v3.RkeK8sServiceOptionInterface) (*rketypes.KubernetesServicesOptions, error) {
	var k8sSvcOption *rketypes.KubernetesServicesOptions
	svcOption, err := svcOptionLister.Get(namespace.GlobalNamespace, name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return k8sSvcOption, err
		}
		svcOption, err = svcOptions.GetNamespaced(namespace.GlobalNamespace, name, metav1.GetOptions{})
		if err != nil {
			return k8sSvcOption, err
		}
	}
	if svcOption.Labels[sendRKELabel] == "false" {
		return k8sSvcOption, nil
	}
	logrus.Debugf("getRKEServiceOption: sending svcOption %s", name)
	return &svcOption.ServiceOptions, nil
}

func GetRKEK8sServiceOptions(k8sVersion string, svcOptionLister v3.RkeK8sServiceOptionLister,
	svcOptions v3.RkeK8sServiceOptionInterface, sysImageLister v3.RkeK8sSystemImageLister,
	sysImages v3.RkeK8sSystemImageInterface, osType OSType) (*rketypes.KubernetesServicesOptions, error) {

	var k8sSvcOption *rketypes.KubernetesServicesOptions
	sysImage, err := sysImageLister.Get(namespace.GlobalNamespace, k8sVersion)
	if err != nil {
		if !errors.IsNotFound(err) {
			logrus.Errorf("getSvcOptions: error finding system image for %s %v", k8sVersion, err)
			return k8sSvcOption, err
		}
		sysImage, err = sysImages.GetNamespaced(namespace.GlobalNamespace, k8sVersion, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("getSvcOptions: error finding system image for %s %v", k8sVersion, err)
			return k8sSvcOption, err
		}
	}
	key := svcOptionLinuxKey
	if osType == Windows {
		key = svcOptionWindowsKey
	}
	val, ok := sysImage.Labels[key]
	// It's possible that we have a k8s version with no windows svcOptions. In this case, we just warn and return nil.
	// if we have in fact windows nodes trying to use that version, the error will show in reknodeconfig server.
	if !ok && osType == Windows {
		logrus.Debugf("getSvcOptions: no service-option key present for %s", k8sVersion)
		return k8sSvcOption, nil
	} else if !ok {
		return k8sSvcOption, fmt.Errorf("getSvcOptions: no service-option key present for %s", k8sVersion)
	}
	return getRKEServiceOption(val, svcOptionLister, svcOptions)
}

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
		// windows has been supported since v1.14,
		// the following logic would not find `< v1.14` service options
		if svcOptions, exist := windowsSvcOptions[majorVersion]; exist {
			// only keep the related images for windows
			windowsSysImgs := rketypes.RKESystemImages{
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
	RKESystemImages           map[string]rketypes.RKESystemImages
	KubernetesServicesOptions map[string]rketypes.KubernetesServicesOptions
}

func newVersionInfo() *VersionInfo {
	return &VersionInfo{
		RKESystemImages:           map[string]rketypes.RKESystemImages{},
		KubernetesServicesOptions: map[string]rketypes.KubernetesServicesOptions{},
	}
}
