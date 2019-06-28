package kontainerdrivermetadata

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/rancher/kontainer-driver-metadata/rke"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/util"

	"github.com/rancher/rancher/pkg/namespace"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	APIVersion                = "management.cattle.io/v3"
	RancherVersionDev         = "2.3"
	sendRKELabel              = "send"
	rkeSystemImageKind        = "RkeK8sSystemImage"
	rkeServiceOptionKind      = "RkeK8sServiceOption"
	rkeAddonKind              = "RkeAddon"
	rkeWindowsSystemImageKind = "RkeK8sWindowsSystemImage"
)

var existLabel = map[string]string{sendRKELabel: "false"}

func (md *MetadataController) createOrUpdateMetadata(data Data) error {
	if err := md.saveSystemImages(data); err != nil {
		return err
	}
	if err := md.saveServiceOptions(data); err != nil {
		return err
	}
	if err := md.saveAddons(data); err != nil {
		return err
	}
	if err := md.saveWindowsInfo(data); err != nil {
		return err
	}
	return nil
}

func (md *MetadataController) saveSystemImages(data Data) error {
	maxVersionForMajorK8sVersion := map[string]string{}
	rancherVersion := settings.ServerVersion.Get()
	if rancherVersion == "dev" || rancherVersion == "master" {
		rancherVersion = RancherVersionDev
	}
	for k8sVersion, systemImages := range data.K8sVersionRKESystemImages {
		rancherVersionInfo, ok := data.K8sVersionInfo[k8sVersion]
		if ok {
			if rancherVersionInfo.DeprecateRancherVersion != "" && rancherVersion >= rancherVersionInfo.DeprecateRancherVersion {
				// don't store
				continue
			}
			lowerThanMin := rancherVersionInfo.MinRancherVersion != "" && rancherVersion < rancherVersionInfo.MinRancherVersion
			// only respect min versions, even if max is present - we need to support upgraded clusters
			if lowerThanMin {
				continue
			}
		}
		if err := md.createOrUpdateSystemImageCRD(k8sVersion, systemImages); err != nil {
			return err
		}
		majorVersion := util.GetTagMajorVersion(k8sVersion)
		majorVersionInfo, ok := data.K8sVersionInfo[majorVersion]
		if ok {
			// include in K8sVersionCurrent only if less then max version
			greaterThanMax := majorVersionInfo.MaxRancherVersion != "" && rancherVersion > majorVersionInfo.MaxRancherVersion
			if greaterThanMax {
				continue
			}
		}
		if curr, ok := maxVersionForMajorK8sVersion[majorVersion]; !ok || k8sVersion > curr {
			maxVersionForMajorK8sVersion[majorVersion] = k8sVersion
		}
	}
	updateSettings(maxVersionForMajorK8sVersion, rancherVersion, data)
	return nil
}

func (md *MetadataController) saveServiceOptions(data Data) error {
	rkeDataKeys := getRKEVendorOptions()
	logrus.Debugf("svcOptions rkeDataKeys %v", rkeDataKeys)
	for k8sVersion, serviceOptions := range data.K8sVersionServiceOptions {
		if err := md.createOrUpdateServiceOptionCRD(k8sVersion, serviceOptions, rkeDataKeys); err != nil {
			return err
		}
	}
	return nil
}

func (md *MetadataController) saveAddons(data Data) error {
	for addon, templateData := range data.K8sVersionedTemplates {
		if err := md.createOrUpdateAddonCRD(addon, templateData); err != nil {
			return err
		}
	}
	return nil
}

func (md *MetadataController) saveWindowsInfo(data Data) error {
	for k8sVersion, sysImages := range data.K8sVersionWindowsSystemImages {
		if err := md.createOrUpdateWindowsSystemImageCRD(k8sVersion, sysImages, true); err != nil {
			return err
		}
	}
	for k8sVersion, serviceOptions := range data.K8sVersionWindowsServiceOptions {
		if err := md.createOrUpdateWindowsServiceOptionCRD(k8sVersion, serviceOptions); err != nil {
			return err
		}
	}
	return nil
}

func (md *MetadataController) createOrUpdateSystemImageCRD(k8sVersion string, systemImages v3.RKESystemImages) error {
	sysImage, err := md.getRKESystemImage(k8sVersion)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		sysImage = &v3.RKEK8sSystemImage{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getName(k8sVersion),
				Namespace: namespace.GlobalNamespace,
			},
			SystemImages: systemImages,
			TypeMeta: metav1.TypeMeta{
				Kind:       rkeSystemImageKind,
				APIVersion: APIVersion,
			},
		}
		if _, err := md.SystemImages.Create(sysImage); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
		return nil

	}
	if reflect.DeepEqual(sysImage.SystemImages, systemImages) {
		return nil
	}
	sysImageCopy := sysImage.DeepCopy()
	sysImageCopy.SystemImages = systemImages
	// todo: add retry
	if _, err := md.SystemImages.Update(sysImageCopy); err != nil {
		return err
	}
	return nil
}

func (md *MetadataController) createOrUpdateWindowsSystemImageCRD(k8sVersion string, systemImages v3.WindowsSystemImages, windows bool) error {
	sysImage, err := md.getRKEWindowsSystemImage(k8sVersion)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		sysImage = &v3.RKEK8sWindowsSystemImage{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getWindowsName(k8sVersion),
				Namespace: namespace.GlobalNamespace,
			},
			SystemImages: systemImages,
			TypeMeta: metav1.TypeMeta{
				Kind:       rkeWindowsSystemImageKind,
				APIVersion: APIVersion,
			},
		}
		if _, err := md.WindowsSystemImages.Create(sysImage); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
		return nil

	}
	if reflect.DeepEqual(sysImage.SystemImages, systemImages) {
		return nil
	}
	sysImageCopy := sysImage.DeepCopy()
	sysImageCopy.SystemImages = systemImages
	// todo: add retry
	if _, err := md.WindowsSystemImages.Update(sysImageCopy); err != nil {
		return err
	}
	return nil
}

func (md *MetadataController) createOrUpdateServiceOptionCRD(k8sVersion string, serviceOptions v3.KubernetesServicesOptions, rkeDataKeys map[string]bool) error {
	svcOption, err := md.getRKEServiceOption(k8sVersion)
	_, exists := rkeDataKeys[k8sVersion]
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		svcOption = &v3.RKEK8sServiceOption{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getName(k8sVersion),
				Namespace: namespace.GlobalNamespace,
			},
			ServiceOptions: serviceOptions,
			TypeMeta: metav1.TypeMeta{
				Kind:       rkeServiceOptionKind,
				APIVersion: APIVersion,
			},
		}
		if exists {
			svcOption.Labels = existLabel
		}
		if _, err := md.ServiceOptions.Create(svcOption); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
		return nil
	}
	var svcOptionCopy *v3.RKEK8sServiceOption
	if reflect.DeepEqual(svcOption.ServiceOptions, serviceOptions) {
		if reflect.DeepEqual(svcOption.Labels, existLabel) && exists {
			return nil
		}
		svcOptionCopy = svcOption.DeepCopy()
		if exists {
			svcOptionCopy.Labels = existLabel
		} else {
			logrus.Infof("delete?")
			delete(svcOptionCopy.Labels, sendRKELabel)
		}
	} else {
		svcOptionCopy = svcOption.DeepCopy()
		svcOptionCopy.ServiceOptions = serviceOptions
	}
	if svcOptionCopy != nil {
		if _, err := md.ServiceOptions.Update(svcOptionCopy); err != nil {
			return err
		}
	}
	return nil
}

func (md *MetadataController) createOrUpdateWindowsServiceOptionCRD(k8sVersion string, serviceOptions v3.KubernetesServicesOptions) error {
	svcOption, err := md.getRKEServiceOption(k8sVersion)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		svcOption = &v3.RKEK8sServiceOption{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getWindowsName(k8sVersion),
				Namespace: namespace.GlobalNamespace,
			},
			ServiceOptions: serviceOptions,
			TypeMeta: metav1.TypeMeta{
				Kind:       rkeServiceOptionKind,
				APIVersion: APIVersion,
			},
		}
		if _, err := md.ServiceOptions.Create(svcOption); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
		return nil
	}
	if reflect.DeepEqual(svcOption.ServiceOptions, serviceOptions) {
		return nil
	}
	svcOptionCopy := svcOption.DeepCopy()
	svcOptionCopy.ServiceOptions = serviceOptions
	if svcOptionCopy != nil {
		if _, err := md.ServiceOptions.Update(svcOptionCopy); err != nil {
			return err
		}
	}
	return nil
}

func (md *MetadataController) createOrUpdateAddonCRD(addonName string, templateData map[string]string) error {
	rkeDataKeys := getRKEVendorData(addonName)
	logrus.Debugf("addons %s rkeDataKeys %v", addonName, rkeDataKeys)
	for k8sVersion, template := range templateData {
		_, exists := rkeDataKeys[k8sVersion]
		name := fmt.Sprintf("%s-%s", strings.ToLower(addonName), getName(k8sVersion))
		addon, err := md.getRKEAddon(name)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
			addon = &v3.RKEAddon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace.GlobalNamespace,
				},
				Template: template,
				TypeMeta: metav1.TypeMeta{
					Kind:       rkeAddonKind,
					APIVersion: APIVersion,
				},
			}
			if exists {
				addon.Labels = existLabel
			}
			if _, err := md.Addons.Create(addon); err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
			return nil
		}
		var addonCopy *v3.RKEAddon
		if reflect.DeepEqual(addon.Template, template) {
			if reflect.DeepEqual(addon.Labels, existLabel) && exists {
				return nil
			}
			addonCopy = addon.DeepCopy()
			if exists {
				addonCopy.Labels = existLabel
			} else {
				delete(addonCopy.Labels, sendRKELabel)
			}
		}
		if addonCopy != nil {
			if _, err := md.Addons.Update(addonCopy); err != nil {
				return err
			}
		}
	}
	return nil
}

func getRKEVendorData(addonName string) map[string]bool {
	keys := map[string]bool{}
	templateData, ok := rke.DriverData.K8sVersionedTemplates[addonName]
	if !ok {
		return keys
	}
	for k8sVersion := range templateData {
		keys[k8sVersion] = true
	}
	// for testing,
	delete(keys, "v1.14")
	return keys
}

func getRKEVendorOptions() map[string]bool {
	keys := map[string]bool{}
	for k8sVersion := range rke.DriverData.K8sVersionServiceOptions {
		keys[k8sVersion] = true
	}
	// for testing,
	delete(keys, "v1.14")
	return keys
}

func (md *MetadataController) getRKEAddon(name string) (*v3.RKEAddon, error) {
	return md.AddonsLister.Get(namespace.GlobalNamespace, name)
}

func (md *MetadataController) getRKEServiceOption(k8sVersion string) (*v3.RKEK8sServiceOption, error) {
	return md.ServiceOptionsLister.Get(namespace.GlobalNamespace, getName(k8sVersion))
}

func (md *MetadataController) getRKESystemImage(k8sVersion string) (*v3.RKEK8sSystemImage, error) {
	return md.SystemImagesLister.Get(namespace.GlobalNamespace, getName(k8sVersion))
}

func (md *MetadataController) getRKEWindowsSystemImage(k8sVersion string) (*v3.RKEK8sWindowsSystemImage, error) {
	return md.WindowsSystemImagesLister.Get(namespace.GlobalNamespace, getWindowsName(k8sVersion))
}

func getName(str string) string {
	return strings.Replace(str, ".", "x", 1)
}

func getWindowsName(str string) string {
	return fmt.Sprintf("w%s", getName(str))
}

func updateSettings(maxVersionForMajorK8sVersion map[string]string, rancherVersion string, data Data) error {
	k8sVersionRKESystemImages := map[string]interface{}{}
	k8sVersionSvcOptions := map[string]v3.KubernetesServicesOptions{}

	for majorVersion, k8sVersion := range maxVersionForMajorK8sVersion {
		k8sVersionRKESystemImages[k8sVersion] = nil
		k8sVersionSvcOptions[k8sVersion] = data.K8sVersionServiceOptions[majorVersion]
	}
	return SaveSettings(k8sVersionRKESystemImages, k8sVersionSvcOptions, data.RancherDefaultK8sVersions, rancherVersion)
}

func SaveSettings(k8sCurrVersions map[string]interface{},
	k8sVersionSvcOptions map[string]v3.KubernetesServicesOptions, rancherDefaultK8sVersions map[string]string,
	rancherVersion string) error {
	k8sCurrVersionData, err := marshal(k8sCurrVersions)
	if err != nil {
		return err
	}
	if err := settings.KubernetesVersionToSystemImages.Set(k8sCurrVersionData); err != nil {
		return err
	}

	k8sSvcOptionData, err := marshal(k8sVersionSvcOptions)
	if err != nil {
		return err
	}
	if err := settings.KubernetesVersionToServiceOptions.Set(k8sSvcOptionData); err != nil {
		return err
	}
	defaultK8sVersion, ok := rancherDefaultK8sVersions[rancherVersion]
	if !ok || defaultK8sVersion == "" {
		defaultK8sVersion = rancherDefaultK8sVersions["default"]
	}
	return settings.KubernetesVersion.Set(defaultK8sVersion)
}

func marshal(data interface{}) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
