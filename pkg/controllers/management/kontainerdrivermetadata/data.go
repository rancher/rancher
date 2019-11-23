package kontainerdrivermetadata

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/blang/semver"
	mVersion "github.com/mcuadros/go-version"
	"github.com/rancher/kontainer-driver-metadata/rke"
	"github.com/rancher/kontainer-driver-metadata/rke/templates"
	"github.com/rancher/norman/types/convert"
	setting2 "github.com/rancher/rancher/pkg/api/store/setting"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/util"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OSType int

const (
	Linux OSType = iota
	Windows
)

const (
	APIVersion           = "management.cattle.io/v3"
	RancherVersionDev    = "2.4"
	sendRKELabel         = "io.cattle.rke_store"
	svcOptionLinuxKey    = "service-option-linux-key"
	svcOptionWindowsKey  = "service-option-windows-key"
	rkeSystemImageKind   = "RkeK8sSystemImage"
	rkeServiceOptionKind = "RkeK8sServiceOption"
	rkeAddonKind         = "RkeAddon"
)

var existLabel = map[string]string{sendRKELabel: "false"}

//settings corresponding to keys in setting2.MetadataSettings
var userUpdateSettingMap = map[string]settings.Setting{
	settings.KubernetesVersion.Name:            settings.KubernetesVersion,
	settings.KubernetesVersionsCurrent.Name:    settings.KubernetesVersionsCurrent,
	settings.KubernetesVersionsDeprecated.Name: settings.KubernetesVersionsDeprecated,
}

var rancherUpdateSettingMap = map[string]settings.Setting{
	settings.KubernetesVersion.Name:                 settings.KubernetesVersion,
	settings.KubernetesVersionsCurrent.Name:         settings.KubernetesVersionsCurrent,
	settings.KubernetesVersionsDeprecated.Name:      settings.KubernetesVersionsDeprecated,
	settings.UIKubernetesDefaultVersion.Name:        settings.UIKubernetesDefaultVersion,
	settings.UIKubernetesSupportedVersions.Name:     settings.UIKubernetesSupportedVersions,
	settings.KubernetesVersionToSystemImages.Name:   settings.KubernetesVersionToSystemImages,
	settings.KubernetesVersionToServiceOptions.Name: settings.KubernetesVersionToServiceOptions,
}

func (md *MetadataController) createOrUpdateMetadata(data Data) error {
	if err := md.saveSystemImages(data.K8sVersionRKESystemImages, data.K8sVersionedTemplates,
		data.K8sVersionInfo, data.K8sVersionServiceOptions, data.K8sVersionWindowsServiceOptions, data.RancherDefaultK8sVersions); err != nil {
		return err
	}
	if err := md.saveAllServiceOptions(data.K8sVersionServiceOptions, data.K8sVersionWindowsServiceOptions); err != nil {
		return err
	}
	if err := md.saveAddons(data.K8sVersionedTemplates); err != nil {
		return err
	}
	return nil
}

func (md *MetadataController) createOrUpdateMetadataDefaults() error {
	if err := md.saveSystemImages(rke.DriverData.K8sVersionRKESystemImages, rke.DriverData.K8sVersionedTemplates,
		rke.DriverData.K8sVersionInfo, rke.DriverData.K8sVersionServiceOptions, rke.DriverData.K8sVersionWindowsServiceOptions, rke.DriverData.RancherDefaultK8sVersions); err != nil {
		return err
	}
	if err := md.saveAllServiceOptions(rke.DriverData.K8sVersionServiceOptions, rke.DriverData.K8sVersionWindowsServiceOptions); err != nil {
		return err
	}
	if err := md.saveAddons(rke.DriverData.K8sVersionedTemplates); err != nil {
		return err
	}
	return nil
}

func (md *MetadataController) saveSystemImages(K8sVersionRKESystemImages map[string]v3.RKESystemImages,
	AddonsData map[string]map[string]string,
	K8sVersionInfo map[string]v3.K8sVersionInfo,
	ServiceOptions map[string]v3.KubernetesServicesOptions,
	ServiceOptionsWindows map[string]v3.KubernetesServicesOptions,
	DefaultK8sVersions map[string]string) error {
	maxVersionForMajorK8sVersion := map[string]string{}
	deprecatedMap := map[string]bool{}
	rancherVersion := GetRancherVersion()
	var maxIgnore []string
	for k8sVersion, systemImages := range K8sVersionRKESystemImages {
		rancherVersionInfo, minorOk := K8sVersionInfo[k8sVersion]
		if minorOk && toIgnoreForAllK8s(rancherVersionInfo, rancherVersion) {
			deprecatedMap[k8sVersion] = true
			continue
		}
		majorVersion := util.GetTagMajorVersion(k8sVersion)
		majorVersionInfo, majorOk := K8sVersionInfo[majorVersion]
		if majorOk && toIgnoreForAllK8s(majorVersionInfo, rancherVersion) {
			deprecatedMap[k8sVersion] = true
			continue
		}
		labelsMap, err := getLabelMap(k8sVersion, AddonsData, ServiceOptions, ServiceOptionsWindows)
		if err != nil {
			return err
		}
		if err := md.createOrUpdateSystemImageCRD(k8sVersion, systemImages, labelsMap); err != nil {
			return err
		}
		if minorOk && toIgnoreForK8sCurrent(rancherVersionInfo, rancherVersion) {
			maxIgnore = append(maxIgnore, k8sVersion)
			continue
		}
		if majorOk && toIgnoreForK8sCurrent(majorVersionInfo, rancherVersion) {
			maxIgnore = append(maxIgnore, k8sVersion)
			continue
		}
		if curr, ok := maxVersionForMajorK8sVersion[majorVersion]; !ok || mVersion.Compare(k8sVersion, curr, ">") {
			maxVersionForMajorK8sVersion[majorVersion] = k8sVersion
		}
	}
	logrus.Debugf("driverMetadata deprecated %v max incompatible versions %v", deprecatedMap, maxIgnore)

	return md.updateSettings(maxVersionForMajorK8sVersion, rancherVersion, ServiceOptions, DefaultK8sVersions, deprecatedMap)
}

func toIgnoreForAllK8s(rancherVersionInfo v3.K8sVersionInfo, rancherVersion string) bool {
	if rancherVersionInfo.DeprecateRancherVersion != "" && mVersion.Compare(rancherVersion, rancherVersionInfo.DeprecateRancherVersion, ">=") {
		return true
	}
	if rancherVersionInfo.MinRancherVersion != "" && mVersion.Compare(rancherVersion, rancherVersionInfo.MinRancherVersion, "<") {
		// only respect min versions, even if max is present - we need to support upgraded clusters
		return true
	}
	return false
}

func toIgnoreForK8sCurrent(majorVersionInfo v3.K8sVersionInfo, rancherVersion string) bool {
	if majorVersionInfo.MaxRancherVersion != "" && mVersion.Compare(rancherVersion, majorVersionInfo.MaxRancherVersion, ">") {
		// include in K8sVersionCurrent only if less then max version
		return true
	}
	return false
}

func (md *MetadataController) saveAllServiceOptions(linuxSvcOptions map[string]v3.KubernetesServicesOptions, windowsSvcOptions map[string]v3.KubernetesServicesOptions) error {
	// save linux options
	if err := md.saveServiceOptions(linuxSvcOptions, Linux); err != nil {
		return err
	}
	// save windows options
	if err := md.saveServiceOptions(windowsSvcOptions, Windows); err != nil {
		return err
	}
	return nil
}

func (md *MetadataController) saveServiceOptions(K8sVersionServiceOptions map[string]v3.KubernetesServicesOptions, osType OSType) error {
	rkeDataKeys := getRKEVendorOptions(osType)
	for k8sVersion, serviceOptions := range K8sVersionServiceOptions {
		if err := md.createOrUpdateServiceOptionCRD(k8sVersion, serviceOptions, rkeDataKeys, osType); err != nil {
			return err
		}
	}
	return nil
}

func (md *MetadataController) saveAddons(K8sVersionedTemplates map[string]map[string]string) error {
	rkeAddonKeys := getRKEVendorData()
	for addon, template := range K8sVersionedTemplates[templates.TemplateKeys] {
		if err := md.createOrUpdateAddonCRD(addon, template, rkeAddonKeys); err != nil {
			return err
		}
	}
	return nil
}

func (md *MetadataController) createOrUpdateSystemImageCRD(k8sVersion string, systemImages v3.RKESystemImages, pluginsMap map[string]string) error {
	sysImage, err := md.getRKESystemImage(k8sVersion)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		sysImage = &v3.RKEK8sSystemImage{
			ObjectMeta: metav1.ObjectMeta{
				Name:      k8sVersion,
				Namespace: namespace.GlobalNamespace,
				Labels:    pluginsMap,
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
	if reflect.DeepEqual(sysImage.SystemImages, systemImages) && reflect.DeepEqual(sysImage.Labels, pluginsMap) {
		return nil
	}
	sysImageCopy := sysImage.DeepCopy()
	sysImageCopy.SystemImages = systemImages
	for k, v := range pluginsMap {
		sysImageCopy.Labels[k] = v
	}
	if _, err := md.SystemImages.Update(sysImageCopy); err != nil {
		return err
	}
	return nil
}

func (md *MetadataController) createOrUpdateServiceOptionCRD(k8sVersion string, serviceOptions v3.KubernetesServicesOptions, rkeDataKeys map[string]bool, osType OSType) error {
	svcOption, err := md.getRKEServiceOption(k8sVersion, osType)
	_, exists := rkeDataKeys[k8sVersion]
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		svcOption = &v3.RKEK8sServiceOption{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getVersionNameWithOsType(k8sVersion, osType),
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
	if reflect.DeepEqual(svcOption.ServiceOptions, serviceOptions) && labelEqual(svcOption.Labels, exists) {
		return nil
	}
	svcOptionCopy = svcOption.DeepCopy()
	if !exists {
		svcOptionCopy.ServiceOptions = serviceOptions
	}
	updateLabel(svcOptionCopy.Labels, exists)
	if svcOptionCopy != nil {
		if _, err := md.ServiceOptions.Update(svcOptionCopy); err != nil {
			return err
		}
	}
	return nil
}

func (md *MetadataController) createOrUpdateAddonCRD(addonName, template string, rkeAddonKeys map[string]bool) error {
	_, exists := rkeAddonKeys[addonName]
	addon, err := md.getRKEAddon(addonName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		addon = &v3.RKEAddon{
			ObjectMeta: metav1.ObjectMeta{
				Name:      addonName,
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
	if reflect.DeepEqual(addon.Template, template) && labelEqual(addon.Labels, exists) {
		return nil
	}
	addonCopy = addon.DeepCopy()
	if !exists {
		addonCopy.Template = template
	}
	updateLabel(addonCopy.Labels, exists)
	if addonCopy != nil {
		if _, err := md.Addons.Update(addonCopy); err != nil {
			return err
		}
	}
	return nil
}

func getLabelMap(k8sVersion string, data map[string]map[string]string,
	svcOption map[string]v3.KubernetesServicesOptions, svcOptionWindows map[string]v3.KubernetesServicesOptions) (map[string]string, error) {
	toMatch, err := semver.Make(k8sVersion[1:])
	if err != nil {
		return nil, fmt.Errorf("k8sVersion not sem-ver %s %v", k8sVersion, err)
	}
	labelMap := map[string]string{}
	for addon, addonData := range data {
		if addon == templates.TemplateKeys {
			continue
		}
		found := false
		for k8sRange, key := range addonData {
			testRange, err := semver.ParseRange(k8sRange)
			if err != nil {
				logrus.Errorf("getPluginData: range for %s not sem-ver %v %v", addon, testRange, err)
				continue
			}
			if testRange(toMatch) {
				labelMap[addon] = key
				found = true
				break
			}
		}
		if !found {
			logrus.Debugf("getPluginData: no template found for k8sVersion %s plugin %s", k8sVersion, addon)
		}
	}
	// store service options
	majorKey := util.GetTagMajorVersion(k8sVersion)
	if _, ok := svcOption[k8sVersion]; ok {
		labelMap[svcOptionLinuxKey] = getVersionNameWithOsType(k8sVersion, Linux)
	} else if _, ok := svcOption[majorKey]; ok {
		labelMap[svcOptionLinuxKey] = getVersionNameWithOsType(majorKey, Linux)
	}

	if _, ok := svcOptionWindows[k8sVersion]; ok {
		labelMap[svcOptionWindowsKey] = getVersionNameWithOsType(k8sVersion, Windows)
	} else if _, ok := svcOptionWindows[majorKey]; ok {
		labelMap[svcOptionWindowsKey] = getVersionNameWithOsType(majorKey, Windows)
	}

	return labelMap, nil
}

func getRKEVendorData() map[string]bool {
	keys := map[string]bool{}
	templateData, ok := rke.DriverData.K8sVersionedTemplates[templates.TemplateKeys]
	if !ok {
		return keys
	}
	for templateKey := range templateData {
		keys[templateKey] = true
	}
	return keys
}

func getRKEVendorOptions(osType OSType) map[string]bool {
	options := rke.DriverData.K8sVersionServiceOptions
	if osType == Windows {
		options = rke.DriverData.K8sVersionWindowsServiceOptions
	}

	keys := map[string]bool{}
	for k8sVersion := range options {
		keys[k8sVersion] = true
	}
	return keys
}

func (md *MetadataController) getRKEAddon(name string) (*v3.RKEAddon, error) {
	return md.AddonsLister.Get(namespace.GlobalNamespace, name)
}

func (md *MetadataController) getRKEServiceOption(k8sVersion string, osType OSType) (*v3.RKEK8sServiceOption, error) {
	return md.ServiceOptionsLister.Get(namespace.GlobalNamespace, getVersionNameWithOsType(k8sVersion, osType))
}

func (md *MetadataController) getRKESystemImage(k8sVersion string) (*v3.RKEK8sSystemImage, error) {
	return md.SystemImagesLister.Get(namespace.GlobalNamespace, k8sVersion)
}

func getVersionNameWithOsType(str string, osType OSType) string {
	if osType == Windows {
		return getWindowsName(str)
	}
	return str
}

func getWindowsName(str string) string {
	return fmt.Sprintf("w%s", str)
}

func (md *MetadataController) updateSettings(maxVersionForMajorK8sVersion map[string]string, rancherVersion string,
	K8sVersionServiceOptions map[string]v3.KubernetesServicesOptions, DefaultK8sVersions map[string]string,
	deprecated map[string]bool) error {

	userSettings, userUpdated, err := md.getUserSettings()
	if err != nil {
		return err
	}

	updateSettings, err := toUpdate(maxVersionForMajorK8sVersion, deprecated, DefaultK8sVersions, rancherVersion, K8sVersionServiceOptions)
	if err != nil {
		return err
	}

	if err := md.updateSettingDefaultFromFields(updateSettings); err != nil {
		return err
	}

	if !userUpdated {
		if err := md.updateSettingFromFields(updateSettings, map[string]string{}); err != nil {
			return err
		}
	} else {
		userMaxVersionForMajorK8sVersion, userDeprecated, err := getUserSettings(userSettings, DefaultK8sVersions)
		if err != nil {
			return err
		}

		if len(userMaxVersionForMajorK8sVersion) == 0 {
			userMaxVersionForMajorK8sVersion = maxVersionForMajorK8sVersion
		}

		if len(userDeprecated) == 0 {
			userDeprecated = deprecated
		}

		userUpdateSettings, err := toUpdate(userMaxVersionForMajorK8sVersion, userDeprecated, DefaultK8sVersions, rancherVersion, K8sVersionServiceOptions)
		if err != nil {
			return err
		}

		if err := md.updateSettingFromFields(userUpdateSettings, userSettings); err != nil {
			return err
		}
	}

	return nil
}

func (md *MetadataController) getUserSettings() (map[string]string, bool, error) {
	userSettings := map[string]string{}
	get := func(key string) string {
		if setting, ok := userUpdateSettingMap[key]; ok {
			return setting.Get()
		}
		return ""
	}
	for key := range userUpdateSettingMap {
		setting, err := md.SettingLister.Get("", key)
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, false, fmt.Errorf("driverMetadata: error getting setting %s: %v", key, err)
			}
			setting, err = md.Settings.Get(key, metav1.GetOptions{})
			if err != nil {
				return nil, false, fmt.Errorf("driverMetadata: error getting setting %s: %v", key, err)
			}
		}
		if val, ok := setting.Labels[setting2.UserUpdateLabel]; ok && convert.ToString(val) == "true" {
			userSettings[key] = get(key)
		}
	}
	logrus.Debugf("driverMetadata: userSettings %v", userSettings)
	if len(userSettings) > 0 {
		return userSettings, true, nil
	}
	return userSettings, false, nil
}

func toUpdate(maxVersionForMajorK8sVersion map[string]string, deprecated map[string]bool,
	defaultK8sVersions map[string]string, rancherVersion string, k8sVersionServiceOptions map[string]v3.KubernetesServicesOptions) (map[string]string, error) {

	var k8sVersionsCurrent []string
	var maxVersions []string
	for k, v := range maxVersionForMajorK8sVersion {
		if !deprecated[k] {
			k8sVersionsCurrent = append(k8sVersionsCurrent, v)
			maxVersions = append(maxVersions, k)
		}
	}
	if len(maxVersions) == 0 {
		return nil, fmt.Errorf("driverMetadata: no max version %v", maxVersionForMajorK8sVersion)
	}
	sort.Strings(k8sVersionsCurrent)
	sort.Strings(maxVersions)

	defaultK8sVersion, err := getDefaultK8sVersion(defaultK8sVersions, k8sVersionsCurrent, rancherVersion)
	if err != nil {
		return nil, err
	}

	k8sVersionRKESystemImages := map[string]interface{}{}
	k8sVersionSvcOptions := map[string]v3.KubernetesServicesOptions{}

	for majorVersion, k8sVersion := range maxVersionForMajorK8sVersion {
		if !deprecated[k8sVersion] {
			k8sVersionRKESystemImages[k8sVersion] = nil
			k8sVersionSvcOptions[k8sVersion] = k8sVersionServiceOptions[majorVersion]
		}
	}

	k8sCurrRKEdata, err := marshal(k8sVersionRKESystemImages)
	if err != nil {
		return nil, err
	}

	k8sSvcOptionData, err := marshal(k8sVersionSvcOptions)
	if err != nil {
		return nil, err
	}

	deprecatedData, err := marshal(deprecated)
	if err != nil {
		return nil, err
	}

	minVersion := maxVersions[0]
	maxVersion := util.GetTagMajorVersion(defaultK8sVersion)
	uiSupported := fmt.Sprintf(">=%s.x <=%s.x", minVersion, maxVersion)
	uiDefaultRange := fmt.Sprintf("<=%s.x", maxVersion)

	return map[string]string{
		settings.KubernetesVersionsCurrent.Name:         strings.Join(k8sVersionsCurrent, ","),
		settings.KubernetesVersion.Name:                 defaultK8sVersion,
		settings.KubernetesVersionsDeprecated.Name:      deprecatedData,
		settings.UIKubernetesDefaultVersion.Name:        uiDefaultRange,
		settings.UIKubernetesSupportedVersions.Name:     uiSupported,
		settings.KubernetesVersionToSystemImages.Name:   k8sCurrRKEdata,
		settings.KubernetesVersionToServiceOptions.Name: k8sSvcOptionData,
	}, nil
}

func (md *MetadataController) updateSettingFromFields(updateField map[string]string, skip map[string]string) error {
	for key, setting := range rancherUpdateSettingMap {
		if _, ok := skip[key]; ok {
			continue
		}
		if _, ok := updateField[key]; !ok {
			return fmt.Errorf("driverMetadata: updated value not present for setting %s", key)
		}
		oldVal := setting.Get()
		newVal := updateField[key]
		if oldVal != newVal {
			if err := setting.Set(newVal); err != nil {
				return err
			}
		}
	}
	return nil
}

func (md *MetadataController) updateSettingDefaultFromFields(updateField map[string]string) error {
	for key := range rancherUpdateSettingMap {
		val, ok := updateField[key]
		if !ok {
			return fmt.Errorf("driverMetadata: default value not present for setting %s", key)
		}
		setting, err := md.SettingLister.Get("", key)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
			setting, err = md.Settings.Get(key, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}
		if setting.Default == val {
			continue
		}
		settingCopy := setting.DeepCopy()
		settingCopy.Default = val
		_, err = md.Settings.Update(settingCopy)
		if err != nil {
			return err
		}
	}
	return nil

}

func getUserSettings(userSettings map[string]string, defaultK8sVersions map[string]string) (map[string]string, map[string]bool, error) {
	userMaxVersionForMajorK8sVersion := map[string]string{}
	if val, ok := userSettings[settings.KubernetesVersionsCurrent.Name]; ok {
		versions := strings.Split(val, ",")
		for _, version := range versions {
			userMaxVersionForMajorK8sVersion[util.GetTagMajorVersion(version)] = version
		}
	}

	userDeprecated := map[string]bool{}
	if val, ok := userSettings[settings.KubernetesVersionsDeprecated.Name]; ok {
		deprecatedVersions := make(map[string]bool)
		if val != "" {
			if err := json.Unmarshal([]byte(val), &deprecatedVersions); err != nil {
				return nil, nil, err
			}
		}
		for key, val := range deprecatedVersions {
			userDeprecated[key] = val
		}
	}

	if val, ok := userSettings[settings.KubernetesVersion.Name]; ok {
		defaultK8sVersions["user"] = val
	}

	return userMaxVersionForMajorK8sVersion, userDeprecated, nil
}

func getDefaultK8sVersion(rancherDefaultK8sVersions map[string]string, k8sCurrVersions []string, rancherVersion string) (string, error) {
	defaultK8sVersion, ok := rancherDefaultK8sVersions["user"]
	if defaultK8sVersion != "" {
		found := false
		for _, k8sVersion := range k8sCurrVersions {
			if k8sVersion == defaultK8sVersion {
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("driverMetadata: unable to find default k8s version in current k8s %s %v", defaultK8sVersion, k8sCurrVersions)
		}
		return defaultK8sVersion, nil
	}

	defaultK8sVersionRange, ok := rancherDefaultK8sVersions[rancherVersion]
	if !ok || defaultK8sVersionRange == "" {
		defaultK8sVersionRange = rancherDefaultK8sVersions["default"]
	}
	// get matching default k8s from k8s curr
	toMatch := util.GetTagMajorVersion(defaultK8sVersionRange)

	for _, k8sCurr := range k8sCurrVersions {
		toTest := util.GetTagMajorVersion(k8sCurr)
		if toTest == toMatch {
			defaultK8sVersion = k8sCurr
			break
		}
	}
	if defaultK8sVersion == "" {
		return "", fmt.Errorf("driverMetadata: unable to find default k8s version in current k8s %s %v", defaultK8sVersionRange, k8sCurrVersions)
	}
	return defaultK8sVersion, nil
}

func marshal(data interface{}) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func labelEqual(labels map[string]string, exists bool) bool {
	toSendValue := "true"
	if exists {
		toSendValue = "false"
	}
	return toSendValue == labels[sendRKELabel]
}

func updateLabel(labels map[string]string, exists bool) {
	if exists {
		labels[sendRKELabel] = "false"
	} else {
		delete(labels, sendRKELabel)
	}

}
