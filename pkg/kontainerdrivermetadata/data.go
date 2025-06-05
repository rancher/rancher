package kontainerdrivermetadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/blang/semver"
	"github.com/rancher/norman/types/convert"
	setting2 "github.com/rancher/rancher/pkg/api/norman/store/setting"
	"github.com/rancher/rancher/pkg/channelserver"
	"github.com/rancher/rancher/pkg/settings"
	rketypes "github.com/rancher/rke/types"
	"github.com/rancher/rke/types/kdm"
	"github.com/rancher/rke/util"
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
	DataJSONLocation     = "/var/lib/rancher-data/driver-metadata/data.json"
	sendRKELabel         = "io.cattle.rke_store"
	svcOptionLinuxKey    = "service-option-linux-key"
	svcOptionWindowsKey  = "service-option-windows-key"
	rkeSystemImageKind   = "RkeK8sSystemImage"
	rkeServiceOptionKind = "RkeK8sServiceOption"
	rkeAddonKind         = "RkeAddon"
)

var existLabel = map[string]string{sendRKELabel: "false"}

// settings corresponding to keys in setting2.MetadataSettings
var userUpdateSettingMap = map[string]settings.Setting{
	settings.KubernetesVersion.Name:            settings.KubernetesVersion,
	settings.KubernetesVersionsCurrent.Name:    settings.KubernetesVersionsCurrent,
	settings.KubernetesVersionsDeprecated.Name: settings.KubernetesVersionsDeprecated,
}

var rancherUpdateSettingMap = map[string]settings.Setting{
	settings.Rke2DefaultVersion.Name: settings.Rke2DefaultVersion,
	settings.K3sDefaultVersion.Name:  settings.K3sDefaultVersion,
}

func (md *MetadataController) loadDataFromLocal() (kdm.Data, error) {
	if os.Getenv("CATTLE_DEV_MODE") != "" {
		return kdm.Data{}, nil
	}
	logrus.Infof("Retrieve data.json from local path %v", DataJSONLocation)
	data, err := ioutil.ReadFile(DataJSONLocation)
	if err != nil {
		return kdm.Data{}, err
	}
	return kdm.FromData(data)
}

func (md *MetadataController) createOrUpdateMetadata(data kdm.Data) error {
	_, err := md.loadDataFromLocal()
	if err != nil {
		return err
	}

	return nil
}

func (md *MetadataController) createOrUpdateMetadataFromLocal() error {
	_, err := md.loadDataFromLocal()
	if err != nil {
		return err
	}

	return nil
}

func getLabelMap(k8sVersion string, data map[string]map[string]string,
	svcOption map[string]rketypes.KubernetesServicesOptions, svcOptionWindows map[string]rketypes.KubernetesServicesOptions) (map[string]string, error) {
	toMatch, err := semver.Make(k8sVersion[1:])
	if err != nil {
		return nil, fmt.Errorf("k8sVersion not sem-ver %s %v", k8sVersion, err)
	}
	labelMap := map[string]string{"cattle.io/creator": "norman"}
	for addon, addonData := range data {
		if addon == kdm.TemplateKeys {
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

	return labelMap, nil
}

func getRKEVendorData(templates map[string]map[string]string) map[string]bool {
	keys := map[string]bool{}
	if templates == nil {
		return keys
	}
	templateData, ok := templates[kdm.TemplateKeys]
	if !ok {
		return keys
	}
	for templateKey := range templateData {
		keys[templateKey] = true
	}
	return keys
}

func (md *MetadataController) updateSettings(maxVersionForMajorK8sVersion map[string]string, rancherVersion string,
	K8sVersionServiceOptions map[string]rketypes.KubernetesServicesOptions, DefaultK8sVersions map[string]string,
	deprecated map[string]bool) error {

	userSettings, userUpdated, err := md.getUserSettings()
	if err != nil {
		return err
	}

	updateSettings, err := toUpdate(maxVersionForMajorK8sVersion, deprecated, DefaultK8sVersions, rancherVersion, K8sVersionServiceOptions)
	if err != nil {
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
		setting, err := md.Settings.Get(key, metav1.GetOptions{})
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
	defaultK8sVersions map[string]string, rancherVersion string, k8sVersionServiceOptions map[string]rketypes.KubernetesServicesOptions) (map[string]string, error) {

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
	k8sVersionSvcOptions := map[string]rketypes.KubernetesServicesOptions{}

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

	rke2DefaultVersion := channelserver.GetDefaultByRuntimeAndServerVersion(context.TODO(), "rke2", rancherVersion)
	k3sDefaultVersion := channelserver.GetDefaultByRuntimeAndServerVersion(context.TODO(), "k3s", rancherVersion)

	return map[string]string{
		settings.KubernetesVersionsCurrent.Name:         strings.Join(k8sVersionsCurrent, ","),
		settings.KubernetesVersion.Name:                 defaultK8sVersion,
		settings.KubernetesVersionsDeprecated.Name:      deprecatedData,
		settings.UIKubernetesDefaultVersion.Name:        uiDefaultRange,
		settings.UIKubernetesSupportedVersions.Name:     uiSupported,
		settings.KubernetesVersionToSystemImages.Name:   k8sCurrRKEdata,
		settings.KubernetesVersionToServiceOptions.Name: k8sSvcOptionData,
		settings.Rke2DefaultVersion.Name:                rke2DefaultVersion,
		settings.K3sDefaultVersion.Name:                 k3sDefaultVersion,
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
	if ok && defaultK8sVersion != "" {
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
	if _, ok := labels[sendRKELabel]; !ok {
		return toSendValue == "true"
	}
	return toSendValue == labels[sendRKELabel]
}

func updateLabel(labels map[string]string, exists bool) map[string]string {
	if exists {
		if labels == nil {
			return map[string]string{
				sendRKELabel: "false",
			}
		}
		labels[sendRKELabel] = "false"
		return labels
	}
	delete(labels, sendRKELabel)
	return labels
}
