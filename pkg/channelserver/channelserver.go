package channelserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
	var errs []string
	for _, runtime := range []string{"k3s", "rke2"} {
		cfg, err := GetReleaseConfigByRuntime(ctx, runtime)
		if err != nil {
			logrus.Errorf("failed to get config for %s: %v", runtime, err)
			errs = append(errs, fmt.Sprintf("%s: %v", runtime, err))
			continue
		}
		if cfg == nil {
			msg := fmt.Sprintf("no config found for %s", runtime)
			logrus.Error(msg)
			errs = append(errs, msg)
			continue
		}
		if err := cfg.LoadConfig(ctx); err != nil {
			logrus.Errorf("failed to reload configuration for %s: %v", runtime, err)
			errs = append(errs, fmt.Sprintf("%s: %v", runtime, err))
		} else {
			logrus.Infof("reloaded configuration for %s", runtime)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("refresh errors: %s", strings.Join(errs, "; "))
	}
	return nil
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
	cfg, err := GetReleaseConfigByRuntime(ctx, runtime)
	if err != nil {
		logrus.Errorf("failed to get release config for %s: %v", runtime, err)
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

func GetReleaseConfigByRuntime(ctx context.Context, runtime string) (*config.Config, error) {
	var initErr error
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
				initErr = fmt.Errorf("failed to load initial config for %s: %w", name, err)
				return
			}
		}
	})
	if initErr != nil {
		return nil, initErr
	}

	cfg, ok := configs[runtime]
	if !ok {
		return nil, fmt.Errorf("unsupported runtime: %s", runtime)
	}

	return cfg, nil
}

func NewHandler(ctx context.Context) (http.Handler, error) {
	k3sConfig, err := GetReleaseConfigByRuntime(ctx, "k3s")
	if err != nil {
		return nil, fmt.Errorf("failed to get k3s config: %w", err)
	}
	rke2Config, err := GetReleaseConfigByRuntime(ctx, "rke2")
	if err != nil {
		return nil, fmt.Errorf("failed to get rke2 config: %w", err)
	}
	return server.NewHandler(map[string]*config.Config{
		"v1-k3s-release":  k3sConfig,
		"v1-rke2-release": rke2Config,
	}), nil
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
	config, err := GetReleaseConfigByRuntime(ctx, runtime)
	if err != nil {
		return "", fmt.Errorf("failed to get release config for %s: %w", runtime, err)
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
	config, err := GetReleaseConfigByRuntime(ctx, runtime)
	if err != nil {
		logrus.Errorf("failed to get release config for %s: %v", runtime, err)
		return ""
	}
	for _, c := range config.ChannelsConfig().Channels {
		if c.Name == channelName {
			return c.Latest
		}
	}
	return ""
}
