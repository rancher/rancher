package k3smetadata

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/rancher/rancher/pkg/channelserver"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/pkg/data"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	forceRefreshAnnotation = "field.cattle.io/lastForceRefresh"
)

type handler struct {
	url             string
	interval        string
	lastForceReresh string
	ctx             context.Context
}

func Register(ctx context.Context, scaled *config.ScaledContext) {
	url, interval := channelserver.GetURLAndInterval()

	h := &handler{
		url:      url,
		interval: interval,
		ctx:      ctx,
	}
	scaled.Management.Settings("").AddHandler(ctx, "channelserver-restart-handler", h.sync)
}

func (h *handler) sync(key string, setting *v3.Setting) (runtime.Object, error) {
	if setting == nil || setting.DeletionTimestamp != nil {
		return nil, nil
	}

	if setting.Name != settings.RkeMetadataConfig.Name {
		return setting, nil
	}

	val := map[string]interface{}{}
	metadataConfig := setting.Default
	if setting.Value != "" {
		metadataConfig = setting.Value
	}
	if err := json.Unmarshal([]byte(metadataConfig), &val); err != nil {
		return setting, err
	}

	updatedURL := data.Object(val).String("url")

	updatedIntervalMinutes, _ := strconv.Atoi(data.Object(val).String("refresh-interval-minutes"))
	if updatedIntervalMinutes == 0 {
		updatedIntervalMinutes = 1440
	}
	updatedInterval := (time.Duration(updatedIntervalMinutes) * time.Minute).String()

	updatedForceRefresh, _ := setting.Annotations[forceRefreshAnnotation]

	noEffectiveChanges := h.url == updatedURL && h.interval == updatedInterval && h.lastForceReresh == updatedForceRefresh

	if noEffectiveChanges {
		return setting, nil
	}

	persistedURL, persistedInterval := channelserver.GetURLAndInterval()
	if persistedURL != updatedURL && persistedInterval != updatedInterval {
		// channelserver Start() uses the cache and will pass incorrect values if started now
		return setting, fmt.Errorf("rke metadata cache has not been updated yet")
	}

	if err := channelserver.Shutdown(); err != nil {
		return setting, err
	}

	h.url = updatedURL
	h.interval = updatedInterval
	h.lastForceReresh = updatedForceRefresh

	return setting, nil
}
