package kontainerdrivermetadata

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blang/semver"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/channelserver"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

type MetadataController struct {
	Settings        mgmtcontrollers.SettingController
	wranglerContext *wrangler.Context
	ctx             context.Context
}

type Data struct {
	// K3S specific data, opaque and defined by the config file in kdm
	K3S map[string]interface{} `json:"k3s,omitempty"`
	// Rke2 specific data, defined by the config file in kdm
	RKE2 map[string]interface{} `json:"rke2,omitempty"`
}

func Register(ctx context.Context, wContext *wrangler.Context) {
	m := &MetadataController{
		Settings: wContext.Mgmt.Setting(),
		ctx:      ctx,
	}
	wContext.Mgmt.Setting().OnChange(ctx, "rke-metadata-handler", m.sync)
}

func (m *MetadataController) sync(_ string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil || (setting.Name != settings.RkeMetadataConfig.Name) {
		return nil, nil
	}
	if err := m.Refresh(); err != nil {
		return nil, err
	}
	// Enqueue to update settings if data changes on next reload by the interval managed by channelserver's DynamicInterval
	_, interval := channelserver.GetURLAndInterval()
	m.Settings.EnqueueAfter(settings.RkeMetadataConfig.Name, interval)
	return setting, nil
}

func (m *MetadataController) Refresh() error {
	// Refreshes to sync rke2/k3s releases
	channelserver.Refresh()
	// Update settings for rke2/k3s and ui
	return m.updateSettings(m.ctx, settings.GetRancherVersion())
}

func (m *MetadataController) updateSettings(ctx context.Context, rancherVersion string) error {
	update := func(setting settings.Setting, value string) error {
		if setting.Get() != value {
			if err := setting.Set(value); err != nil {
				return err
			}
		}
		return nil
	}
	rke2DefaultVersion := channelserver.GetDefaultByRuntimeAndServerVersion(ctx, "rke2", rancherVersion)
	if err := update(settings.Rke2DefaultVersion, rke2DefaultVersion); err != nil {
		return err
	}
	k3sDefaultVersion := channelserver.GetDefaultByRuntimeAndServerVersion(ctx, "k3s", rancherVersion)
	if err := update(settings.K3sDefaultVersion, k3sDefaultVersion); err != nil {
		return err
	}
	// Assuming rke2 and k3s share the same Kubernetes version range for UI display purposes, calculate the range only based on rke2.
	// uiSupportedK8sRange is used by ui for imported/hosted clusters, of the format `">=v1.30.x <=v1.32.x"`
	uiSupportedK8sRange, err := getKubernetesVersionRange(ctx, "rke2", rancherVersion)
	if err != nil {
		return err
	}
	if err := update(settings.UIKubernetesSupportedVersions, uiSupportedK8sRange); err != nil {
		return err
	}
	return nil
}

func getKubernetesVersionRange(ctx context.Context, runtime, serverVersion string) (string, error) {
	serverVersionParsed, err := semver.ParseTolerant(serverVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse server version %s: %v", serverVersion, err)
	}
	config := channelserver.GetReleaseConfigByRuntime(ctx, runtime).ReleasesConfig()
	if config == nil || len(config.Releases) == 0 {
		return "", fmt.Errorf("no released versions found for %s: %s", runtime, serverVersion)
	}
	var versions []semver.Version
	seenVersions := make(map[string]struct{})
	for _, release := range config.Releases {
		versionParsed, err := semver.ParseTolerant(release.Version)
		if err != nil {
			logrus.Tracef("failed to parse release version %s: %v", release.Version, err)
			continue
		}
		majorMinorKey := fmt.Sprintf("%d.%d", versionParsed.Major, versionParsed.Minor)
		if _, ok := seenVersions[majorMinorKey]; ok {
			continue
		}
		minVersionParsed, err := semver.ParseTolerant(release.ChannelServerMinVersion)
		if err != nil {
			logrus.Tracef("failed to parse ChannelServerMinVersion '%s': %v", release.ChannelServerMinVersion, err)
			continue
		}
		maxVersionParsed, err := semver.ParseTolerant(release.ChannelServerMaxVersion)
		if err != nil {
			logrus.Tracef("failed to parse ChannelServerMaxVersion '%s': %v", release.ChannelServerMaxVersion, err)
			continue
		}
		if serverVersionParsed.LT(minVersionParsed) || serverVersionParsed.GT(maxVersionParsed) {
			continue
		}
		versions = append(versions, versionParsed)
		seenVersions[majorMinorKey] = struct{}{}
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no compatible versions found for server version %s and runtime %s", serverVersion, runtime)
	}
	semver.Sort(versions)
	minVersion := versions[0]
	maxVersion := versions[len(versions)-1]
	uiSupported := fmt.Sprintf(">=v%d.%d.x <=v%d.%d.x", minVersion.Major, minVersion.Minor, maxVersion.Major, maxVersion.Minor)
	return uiSupported, nil
}

func FromData(b []byte) (Data, error) {
	d := &Data{}

	if err := json.Unmarshal(b, d); err != nil {
		return Data{}, err
	}
	return *d, nil
}
