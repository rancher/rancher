package kontainerdrivermetadata

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rke/util"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetRKESystemImages(k8sVersion string, sysImageLister v3.RKEK8sSystemImageLister, sysImages v3.RKEK8sSystemImageInterface) (v3.RKESystemImages, error) {
	name := getName(k8sVersion)
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

func GetRKEWindowsSystemImages(k8sVersion string, sysImageLister v3.RKEK8sWindowsSystemImageLister, sysImages v3.RKEK8sWindowsSystemImageInterface) (v3.WindowsSystemImages, error) {
	name := getWindowsName(k8sVersion)
	sysImage, err := sysImageLister.Get(namespace.GlobalNamespace, name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return v3.WindowsSystemImages{}, err
		}
		sysImage, err = sysImages.GetNamespaced(namespace.GlobalNamespace, name, metav1.GetOptions{})
		if err != nil {
			return v3.WindowsSystemImages{}, err
		}
	}
	return sysImage.SystemImages, err
}

func GetRKEAddonTemplate(k8sVersion string, addonName string, addonLister v3.RKEAddonLister, addons v3.RKEAddonInterface) (string, error) {
	names := []string{
		fmt.Sprintf("%s-%s", strings.ToLower(addonName), getName(k8sVersion)),
		fmt.Sprintf("%s-%s", strings.ToLower(addonName), getName(util.GetTagMajorVersion(k8sVersion))),
		fmt.Sprintf("%s-%s", strings.ToLower(addonName), "default"),
	}

	for _, name := range names {
		addon, err := addonLister.Get(namespace.GlobalNamespace, name)
		if err != nil {
			if !errors.IsNotFound(err) {
				return "", err
			}
			continue
		}
		if addon.Labels[sendRKELabel] == "false" {
			logrus.Infof("sendRKELabel false addonName %s, k8sVersion %s", addonName, k8sVersion)
			return "", nil
		}
		return addon.Template, err
	}

	for _, name := range names {
		addon, err := addons.GetNamespaced(namespace.GlobalNamespace, name, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return "", err
			}
			continue
		}
		if addon.Labels[sendRKELabel] == "false" {
			logrus.Infof("sendRKELabel false addonName %s, k8sVersion %s", addonName, k8sVersion)
			return "", nil
		}
		return addon.Template, err
	}
	return "", nil
}

func GetRKEK8sServiceOptions(k8sVersion string, svcOptionLister v3.RKEK8sServiceOptionLister, svcOptions v3.RKEK8sServiceOptionInterface) (*v3.KubernetesServicesOptions, error) {
	names := []string{
		getName(k8sVersion),
		getName(util.GetTagMajorVersion(k8sVersion)),
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

func GetK8sVersionInfo(rancherVersion string, rkeSysImages map[string]v3.RKESystemImages,
	winSysImages map[string]v3.WindowsSystemImages, svcOptions map[string]v3.KubernetesServicesOptions,
	rancherVersions map[string]v3.K8sVersionInfo) (map[string]v3.RKESystemImages, map[string]v3.WindowsSystemImages, map[string]v3.KubernetesServicesOptions) {

	k8sVersionRKESystemImages := map[string]v3.RKESystemImages{}
	k8sVersionWinSystemImages := map[string]v3.WindowsSystemImages{}
	k8sVersionSvcOptions := map[string]v3.KubernetesServicesOptions{}

	//rancherVersion := getRancherVersion()
	maxVersionForMajorK8sVersion := map[string]string{}
	for k8sVersion := range rkeSysImages {
		if rancherVersionInfo, ok := rancherVersions[k8sVersion]; ok {
			greaterThanMax := rancherVersionInfo.MaxRancherVersion != "" && rancherVersion > rancherVersionInfo.MaxRancherVersion
			lowerThanMin := rancherVersionInfo.MinRancherVersion != "" && rancherVersion < rancherVersionInfo.MinRancherVersion
			if greaterThanMax || lowerThanMin {
				continue
			}
		}
		majorVersion := util.GetTagMajorVersion(k8sVersion)
		if curr, ok := maxVersionForMajorK8sVersion[majorVersion]; !ok || k8sVersion > curr {
			maxVersionForMajorK8sVersion[majorVersion] = k8sVersion
		}
	}
	for majorVersion, k8sVersion := range maxVersionForMajorK8sVersion {
		k8sVersionRKESystemImages[k8sVersion] = rkeSysImages[k8sVersion]
		k8sVersionWinSystemImages[k8sVersion] = winSysImages[k8sVersion]
		k8sVersionSvcOptions[k8sVersion] = svcOptions[majorVersion]
	}
	return k8sVersionRKESystemImages, k8sVersionWinSystemImages, k8sVersionSvcOptions
}

func GetRKEK8sServiceOptionsWindows(k8sVersion string, svcOptionLister v3.RKEK8sServiceOptionLister, svcOptions v3.RKEK8sServiceOptionInterface) (*v3.KubernetesServicesOptions, error) {
	names := []string{
		getWindowsName(k8sVersion),
		getWindowsName(util.GetTagMajorVersion(k8sVersion)),
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
		return &obj.ServiceOptions, nil
	}
	return k8sSvcOption, nil
}
