package plugin

import (
	"context"
	"fmt"
	"hash/maphash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
	"math/rand"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	plugincontroller "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	minWait = 10 * time.Second
	maxWait = 30 * time.Minute
)

var timeNow = time.Now

func Register(
	ctx context.Context,
	wContext *wrangler.Context,
) {
	h := &handler{
		systemNamespace: namespace.UIPluginNamespace,
		plugin:          wContext.Catalog.UIPlugin(),
		pluginCache:     wContext.Catalog.UIPlugin().Cache(),
	}
	wContext.Catalog.UIPlugin().OnChange(ctx, "on-ui-plugin-change", h.OnPluginChange)
}

type handler struct {
	systemNamespace string
	plugin          plugincontroller.UIPluginController
	pluginCache     plugincontroller.UIPluginCache
}

func (h *handler) OnPluginChange(key string, plugin *v1.UIPlugin) (*v1.UIPlugin, error) {
	triggered := timeNow().UTC()
	forceUpdate := false
	//indexing all plugins
	cachedPlugins, err := h.pluginCache.List(h.systemNamespace, labels.Everything())
	if err != nil {
		return plugin, fmt.Errorf("failed to list plugins from cache: %w", err)
	}
	err = Index.Generate(cachedPlugins)
	if err != nil {
		return plugin, fmt.Errorf("failed to generate index with cached plugins: %w", err)
	}
	var anonymousCachedPlugins []*v1.UIPlugin
	for _, cachedPlugin := range cachedPlugins {
		if cachedPlugin.Spec.Plugin.NoAuth {
			anonymousCachedPlugins = append(anonymousCachedPlugins, cachedPlugin)
		}
	}
	err = AnonymousIndex.Generate(anonymousCachedPlugins)
	if err != nil {
		return plugin, fmt.Errorf("failed to generate anonymous index with cached plugins: %w", err)
	}
	pattern := FSCacheRootDir + "/*/*"
	fsCacheFiles, err := fsCacheFilepathGlob(pattern)
	if err != nil {
		logrus.WithError(err).Error("failed to get files from filesystem cache")
		return plugin, err
	}
	FsCache.SyncWithIndex(&Index, fsCacheFiles)
	//plugin specific logic
	if plugin == nil {
		return plugin, nil
	}
	if plugin.Generation > plugin.Status.ObservedGeneration {
		forceUpdate = true
		plugin.Status.RetryNumber = 0
		plugin.Status.RetryAt = metav1.Time{}
	}
	if !plugin.Status.RetryAt.IsZero() && plugin.Status.RetryAt.After(triggered) {
		return plugin, nil
	}
	defer h.plugin.UpdateStatus(plugin)
	if plugin.Spec.Plugin.NoCache {
		plugin.Status.CacheState = Disabled
	} else {
		plugin.Status.CacheState = Pending
	}

	maxFileSize, err := strconv.ParseInt(settings.MaxUIPluginFileByteSize.Get(), 10, 64)
	if err != nil {
		logrus.Errorf("failed to convert setting MaxUIPluginFileByteSize to int64, using fallback. err: %s", err.Error())
		maxFileSize = settings.DefaultMaxUIPluginFileSizeInBytes
	}

	for _, p := range cachedPlugins {
		err2 := FsCache.SyncWithControllersCache(p, forceUpdate)
		if errors.Is(err2, errMaxFileSizeError) {
			logrus.Errorf("one of the files is more than the defaultUIPluginFileByteSize limit %s", strconv.FormatInt(maxFileSize, 10))
			// update CRD to remove cache
			p.Spec.Plugin.NoCache = true
			_, err2 := h.plugin.Update(p)
			if err2 != nil {
				p.Spec.Plugin.NoCache = false
				logrus.Errorf("failed to update plugin [%s] noCache flag: %s", p.Spec.Plugin.Name, err2.Error())
				p.Status.Ready = false
				p.Status.Error = "Failed to cache plugin due to max file size limit"
				continue
			}
			// delete files that were written
			err2 = FsCache.Delete(p.Spec.Plugin.Name, p.Spec.Plugin.Version)
			if err2 != nil {
				p.Spec.Plugin.NoCache = false
				logrus.Error(err2)
				continue
			}
			p.Status.CacheState = Disabled
		} else if err2 != nil {
			p.Status.Ready = false
			p.Status.Error = "Failed to cache plugin"
			err = err2
		} else {
			p.Status.Ready = true
			p.Status.Error = ""
		}
	}
	plugin.Status.ObservedGeneration = plugin.Generation
	if err != nil {
		logrus.WithError(err).Error("failed to sync filesystem cache with controller cache")
		backoff := calculateBackoff(plugin.Status.RetryNumber).Round(time.Second)
		plugin.Status.RetryNumber++
		plugin.Status.RetryAt = metav1.Time{Time: timeNow().UTC().Add(backoff)}
		h.plugin.EnqueueAfter(plugin.Namespace, plugin.Name, backoff)
		return plugin, nil
	}
	if !plugin.Spec.Plugin.NoCache {
		plugin.Status.CacheState = Cached
	}
	if plugin.Status.Ready {
		plugin.Status.Error = ""
		plugin.Status.RetryNumber = 0
		plugin.Status.RetryAt = metav1.Time{}
	}
	return plugin, nil
}

// calculateBackoff gets the amount of time to wait for the next call.
// Reference: https://github.com/oras-project/oras-go/blob/main/registry/remote/retry/policy.go#L95
func calculateBackoff(numberOfRetries int) time.Duration {
	var h maphash.Hash
	h.SetSeed(maphash.MakeSeed())
	rand := rand.New(rand.NewSource(int64(h.Sum64())))
	temp := float64(minWait) * math.Pow(2, float64(numberOfRetries))
	backoff := time.Duration(temp*(1-0.2)) + time.Duration(rand.Int63n(int64(2*0.2*temp)))
	if backoff < minWait {
		return minWait
	}
	if backoff > maxWait {
		return maxWait
	}
	return backoff
}
