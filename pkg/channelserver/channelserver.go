package channelserver

import (
	"context"
	"encoding/json"
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

const (
	// RefreshTimeout is the maximum time to wait for a config refresh to start
	RefreshTimeout = 30 * time.Second
	// ConfigProcessingDelay is the time to wait after a refresh signal for the config to fully process the new data
	ConfigProcessingDelay = 500 * time.Millisecond
	// FallbackRefreshDelay is the time to wait in the fallback path when RefreshAndWait fails
	FallbackRefreshDelay = 2 * time.Second
)

var (
	configs       map[string]*config.Config
	configsInit   sync.Once
	action        chan string
	refreshDoneMu sync.Mutex
	refreshDone   map[string]chan struct{}
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

type DynamicInterval struct {
	subKey string
}

func (d *DynamicInterval) Wait(ctx context.Context) bool {
	start := time.Now()
	for {
		select {
		case <-time.After(time.Second):
			_, duration := GetURLAndInterval()
			if start.Add(duration).Before(time.Now()) {
				return true
			}
			continue
		case msg := <-action:
			if msg == d.subKey {
				logrus.Infof("getReleaseConfig: reloading config for %s", d.subKey)
				// Signal that this runtime is about to reload
				refreshDoneMu.Lock()
				if refreshDone != nil {
					if ch, ok := refreshDone[d.subKey]; ok {
						close(ch)
						delete(refreshDone, d.subKey)
					}
				}
				refreshDoneMu.Unlock()
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

// RefreshAndWait signals the channelserver configs to refresh and waits for the refresh to complete.
// This ensures that subsequent calls to Get*Config methods will return fresh data.
func RefreshAndWait(ctx context.Context) error {
	refreshDoneMu.Lock()
	if refreshDone == nil {
		refreshDone = make(map[string]chan struct{})
	}
	k3sDone := make(chan struct{})
	rke2Done := make(chan struct{})
	refreshDone["k3s"] = k3sDone
	refreshDone["rke2"] = rke2Done
	refreshDoneMu.Unlock()

	// Cleanup function to remove channels from map if they weren't already removed
	defer func() {
		refreshDoneMu.Lock()
		delete(refreshDone, "k3s")
		delete(refreshDone, "rke2")
		refreshDoneMu.Unlock()
	}()

	// Send refresh signals with non-blocking sends to avoid deadlock if channel is full
	select {
	case action <- "k3s":
	default:
		return fmt.Errorf("failed to send k3s refresh signal: action channel is full")
	}

	select {
	case action <- "rke2":
	default:
		return fmt.Errorf("failed to send rke2 refresh signal: action channel is full")
	}

	// Use a deadline-based approach to ensure consistent timeout for both runtimes
	deadline := time.Now().Add(RefreshTimeout)

	// Wait for k3s refresh to complete
	k3sTimeout := time.Until(deadline)
	if k3sTimeout <= 0 {
		return fmt.Errorf("timeout waiting for k3s config to refresh")
	}
	select {
	case <-k3sDone:
	case <-time.After(k3sTimeout):
		return fmt.Errorf("timeout waiting for k3s config to refresh")
	case <-ctx.Done():
		return ctx.Err()
	}

	// Wait for rke2 refresh to complete
	rke2Timeout := time.Until(deadline)
	if rke2Timeout <= 0 {
		return fmt.Errorf("timeout waiting for rke2 config to refresh")
	}
	select {
	case <-rke2Done:
	case <-time.After(rke2Timeout):
		return fmt.Errorf("timeout waiting for rke2 config to refresh")
	case <-ctx.Done():
		return ctx.Err()
	}

	// Give the config.Config internal reload goroutine a moment to actually process the data
	// The DynamicInterval.Wait() returning triggers the reload, but the reload itself takes a bit of time
	time.Sleep(ConfigProcessingDelay)

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
		urls := []config.Source{
			&DynamicSource{},
			config.StringSource("/var/lib/rancher-data/driver-metadata/data.json"),
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
