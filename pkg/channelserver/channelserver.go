package channelserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/rancher/channelserver/pkg/config"
	"github.com/rancher/channelserver/pkg/model"
	"github.com/rancher/channelserver/pkg/server"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/data"
	"github.com/rancher/wrangler/v3/pkg/schemas"
	"github.com/sirupsen/logrus"
)

var (
	configs     map[string]*config.Config
	configsInit sync.Once
)

func GetURLAndInterval() (string, time.Duration) {
	val := map[string]interface{}{}
	if err := json.Unmarshal([]byte(settings.RkeMetadataConfig.Get()), &val); err != nil {
		logrus.Errorf("failed to parse %s value: %v", settings.RkeMetadataConfig.Name, err)
		return "", 0
	}
	url := data.Object(val).String("url")
	minutes, _ := strconv.Atoi(data.Object(val).String("refresh-interval-minutes"))
	if minutes <= 0 {
		minutes = 1440
	}

	return url, time.Duration(minutes) * time.Minute
}

// getChannelServerArg will return with an argument to pass to channel server
// to indicate the server version that is running. If the current version is
// not a proper release version, the argument will be set to the dev version.
func getChannelServerArg() string {
	serverVersion := settings.ServerVersion.Get()
	if !settings.IsReleaseServerVersion(serverVersion) {
		return settings.RancherVersionDev
	}
	return serverVersion
}

func Refresh(ctx context.Context) error {
	var errs error
	for _, runtime := range []string{"k3s", "rke2"} {
		cfg := GetReleaseConfigByRuntime(ctx, runtime)
		if cfg == nil {
			errs = errors.Join(errs, fmt.Errorf("no config found for %s", runtime))
			continue
		}
		if err := cfg.LoadConfig(ctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to reload configuration for %s: %w", runtime, err))
		} else {
			logrus.Infof("reloaded configuration for %s", runtime)
		}
	}

	return errs
}

type DynamicSource struct{}

func (d *DynamicSource) URL() string {
	url, _ := GetURLAndInterval()
	return url
}

func GetReleaseConfigByRuntimeAndVersion(ctx context.Context, runtime, kubernetesVersion string) model.Release {
	fallBack := model.Release{
		AgentArgs:  map[string]schemas.Field{},
		ServerArgs: map[string]schemas.Field{},
	}
	cfg := GetReleaseConfigByRuntime(ctx, runtime)
	if cfg == nil {
		logrus.Errorf("no release config for %s", runtime)
		return fallBack
	}
	for _, releaseData := range cfg.ReleasesConfig().Releases {
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
		urls := []config.Source{
			&DynamicSource{},
			config.StringSource("/var/lib/rancher-data/driver-metadata/data.json"),
		}
		configs = map[string]*config.Config{
			"k3s":  config.NewConfigNoLoad(ctx, "k3s", getChannelServerArg(), "rancher", "", urls),
			"rke2": config.NewConfigNoLoad(ctx, "rke2", getChannelServerArg(), "rancher", "", urls),
		}
		for name, cfg := range configs {
			if err := cfg.LoadConfig(ctx); err != nil {
				logrus.Errorf("failed to load initial config for %s: %v", name, err)
			}
		}
	})

	return configs[runtime]
}

// NewHandler creates an HTTP handler for serving k3s and rke2 release metadata.
// Note: Initial loading might fail, in which case the configs will have missing KDM data.
// The handler gracefully handles this case by returning empty responses.
func NewHandler(ctx context.Context) http.Handler {
	k3sConfig := GetReleaseConfigByRuntime(ctx, "k3s")
	rke2Config := GetReleaseConfigByRuntime(ctx, "rke2")
	return server.NewHandler(map[string]*config.Config{
		"v1-k3s-release":  k3sConfig,
		"v1-rke2-release": rke2Config,
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
	if config == nil {
		return "", fmt.Errorf("no release config for %s", runtime)
	}
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
	if config == nil {
		logrus.Errorf("no release config for %s", runtime)
		return ""
	}
	for _, c := range config.ChannelsConfig().Channels {
		if c.Name == channelName {
			return c.Latest
		}
	}
	return ""
}
