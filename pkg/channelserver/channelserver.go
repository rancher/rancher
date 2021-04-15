package channelserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/rancher/channelserver/pkg/config"
	"github.com/rancher/channelserver/pkg/server"
	"github.com/rancher/rancher/pkg/catalog/utils"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/sirupsen/logrus"
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
// not a proper release version, the argument will be empty.
func getChannelServerArg() string {
	serverVersion := settings.ServerVersion.Get()
	if !utils.ReleaseServerVersion(serverVersion) {
		return ""
	}
	return serverVersion
}

type DynamicInterval struct{}

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
		case <-ctx.Done():
			return false
		}
	}
}

type DynamicSource struct{}

func (d *DynamicSource) URL() string {
	url, _ := GetURLAndInterval()
	return url
}

func NewHandler(ctx context.Context) http.Handler {
	interval := &DynamicInterval{}
	urls := []config.Source{
		&DynamicSource{},
		config.StringSource("/var/lib/rancher-data/driver-metadata/data.json"),
	}
	return server.NewHandler(map[string]*config.Config{
		"v1-k3s-release":  config.NewConfig(ctx, "k3s", interval, getChannelServerArg(), urls),
		"v1-rke2-release": config.NewConfig(ctx, "rke2", interval, getChannelServerArg(), urls),
	})
}
