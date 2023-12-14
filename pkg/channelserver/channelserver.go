package channelserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/rancher/channelserver/pkg/config"
	"github.com/rancher/channelserver/pkg/model"
	"github.com/rancher/channelserver/pkg/server"
	"github.com/rancher/rancher/pkg/catalog/utils"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/sirupsen/logrus"
)

var (
	configs     map[string]*config.Config
	configsInit sync.Once
	action      chan string
)

func getMetadataConfig() settings.MetadataConfig {
	config := settings.MetadataConfig{}
	if err := json.Unmarshal([]byte(settings.RkeMetadataConfig.Get()), &config); err != nil {
		logrus.Errorf("failed to parse %s value: %v", settings.RkeMetadataConfig.Name, err)
		return config
	}
	if config.RefreshIntervalMinutes <= 0 {
		config.RefreshIntervalMinutes = 1440
	}

	return config
}

// getChannelServerArg will return with an argument to pass to channel server
// to indicate the server version that is running. If the current version is
// not a proper release version, the argument will be set to the dev version.
func getChannelServerArg() string {
	serverVersion := settings.ServerVersion.Get()
	if !utils.ReleaseServerVersion(serverVersion) {
		return settings.RancherVersionDev
	}
	return serverVersion
}

type DynamicInterval struct {
	subKey string
}

func (d *DynamicInterval) Wait(ctx context.Context) bool {
	start := time.Now()
	for {
		select {
		case <-time.After(time.Second):
			duration := getMetadataConfig().RefreshIntervalMinutes
			if start.Add(time.Minute * time.Duration(duration)).Before(time.Now()) {
				return true
			}
			continue
		case msg := <-action:
			if msg == d.subKey {
				logrus.Infof("getReleaseConfig: reloading config for %s", d.subKey)
				return true
			}
			action <- msg
		case <-ctx.Done():
			return false
		}
	}
}

func Refresh() {
	action <- "k3s"
	action <- "rke2"
}

func GetReleaseConfigByRuntimeAndVersion(ctx context.Context, runtime, kubernetesVersion string) model.Release {
	fallBack := model.Release{
		AgentArgs:  map[string]schemas.Field{},
		ServerArgs: map[string]schemas.Field{},
	}
	for _, releaseData := range GetReleaseConfigByRuntime(ctx, runtime).ReleasesConfig().Releases {
		if releaseData.Version == kubernetesVersion {
			return releaseData
		}
		for k, v := range releaseData.ServerArgs {
			fallBack.ServerArgs[k] = v
		}
		for k, v := range releaseData.AgentArgs {
			fallBack.AgentArgs[k] = v
		}
	}
	return fallBack
}

func GetReleaseConfigByRuntime(ctx context.Context, runtime string) *config.Config {
	configsInit.Do(func() {
		urls := make([]config.Source, 0)
		m := getMetadataConfig()
		if m.URL != "" {
			urls = append(urls, config.StringSource(m.URL))
		}
		if m.Path != "" {
			urls = append(urls, config.StringSource(m.Path))
		}
		configs = map[string]*config.Config{
			"k3s":  config.NewConfig(ctx, "k3s", &DynamicInterval{"k3s"}, getChannelServerArg(), "rancher", urls),
			"rke2": config.NewConfig(ctx, "rke2", &DynamicInterval{"rke2"}, getChannelServerArg(), "rancher", urls),
		}
	})
	return configs[runtime]
}

func NewHandler(ctx context.Context) http.Handler {
	action = make(chan string, 2)
	return server.NewHandler(map[string]*config.Config{
		"v1-k3s-release":  GetReleaseConfigByRuntime(ctx, "k3s"),
		"v1-rke2-release": GetReleaseConfigByRuntime(ctx, "rke2"),
	})
}

func GetDefaultByRuntimeAndServerVersion(ctx context.Context, runtime, serverVersion string) string {
	version, err := getDefaultFromAppDefaultsByRuntimeAndServerVersion(ctx, runtime, serverVersion)
	if err != nil {
		logrus.Debugf("[channelserver] fallback to use the default channel due to: %v", err)
		version = getDefaultFromChannel(ctx, runtime, "default")
	}
	return version
}

func getDefaultFromAppDefaultsByRuntimeAndServerVersion(ctx context.Context, runtime, serverVersion string) (string, error) {
	var defaultVersionRange string
	serverVersionParsed, err := semver.ParseTolerant(serverVersion)
	if err != nil {
		return "", fmt.Errorf("fails to parse the server version: %v", err)
	}
	config := GetReleaseConfigByRuntime(ctx, runtime)
	appDefaults := config.AppDefaultsConfig().AppDefaults
	if len(appDefaults) == 0 {
		return "", fmt.Errorf("no %s appDefaults is found for %s", runtime, serverVersion)
	}
	// We use the first entry from the list. We do not expect the list contains more than one entry.
	for _, appDefault := range appDefaults[0].Defaults {
		avrParsed, err := semver.ParseRange(appDefault.AppVersion)
		if err != nil {
			return "", fmt.Errorf("faild to parse %s appVersionRange for %s: %v", runtime, serverVersion, err)
		}
		if avrParsed(serverVersionParsed) {
			defaultVersionRange = appDefault.DefaultVersion
			continue
		}
	}
	if defaultVersionRange == "" {
		return "", fmt.Errorf("no matching %s defaultVersionRange is found for %s", runtime, serverVersion)
	}
	dvrParsed, err := semver.ParseRange(defaultVersionRange)
	if err != nil {
		return "", fmt.Errorf("faild to parse %s defaultVersionRange for %s: %v", runtime, serverVersion, err)
	}

	var candidate []semver.Version
	for _, release := range config.ReleasesConfig().Releases {
		version, err := semver.ParseTolerant(release.Version)
		if err != nil {
			logrus.Debugf("fails to parse the release version %s: %v", release.Version, err)
			continue
		}
		if dvrParsed(version) {
			candidate = append(candidate, version)
		}
	}
	if len(candidate) == 0 {
		return "", fmt.Errorf("no %s version is found for %s", runtime, serverVersion)
	}
	// the build metadata parts are ignored when sorting versions for now;
	// ideally they should be since k3s/RKE2 releases use it to establish order of precedence
	semver.Sort(candidate)
	return candidate[len(candidate)-1].String(), nil
}

func getDefaultFromChannel(ctx context.Context, runtime, channelName string) string {
	config := GetReleaseConfigByRuntime(ctx, runtime)
	for _, c := range config.ChannelsConfig().Channels {
		if c.Name == channelName {
			return c.Latest
		}
	}
	return ""
}
