package plugin

import (
	"context"
	"fmt"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	plugincontroller "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func Register(
	ctx context.Context,
	systemNamespace string,
	plugin plugincontroller.UIPluginController,
	pluginCache plugincontroller.UIPluginCache,
) {
	h := &handler{
		systemNamespace: systemNamespace,
		plugin:          plugin,
		pluginCache:     pluginCache,
	}
	plugin.OnChange(ctx, "on-plugin-change", h.OnPluginChange)
}

type handler struct {
	systemNamespace string
	plugin          plugincontroller.UIPluginController
	pluginCache     plugincontroller.UIPluginCache
}

func (h *handler) OnPluginChange(key string, plugin *v1.UIPlugin) (*v1.UIPlugin, error) {
	cachedPlugins, err := h.pluginCache.List(h.systemNamespace, labels.Everything())
	if err != nil {
		return plugin, fmt.Errorf("failed to list plugins from cache. %w", err)
	}
	err = Index.Generate(cachedPlugins)
	if err != nil {
		return plugin, fmt.Errorf("failed to generate index with cached plugins. %w", err)
	}
	var anonymousCachedPlugins []*v1.UIPlugin
	for _, cachedPlugin := range cachedPlugins {
		if cachedPlugin.Spec.Plugin.NoAuth {
			anonymousCachedPlugins = append(anonymousCachedPlugins, cachedPlugin)
		}
	}
	err = AnonymousIndex.Generate(anonymousCachedPlugins)
	if err != nil {
		return plugin, fmt.Errorf("failed to generate anonymous index with cached plugins. %w", err)
	}
	pattern := FSCacheRootDir + "/*/*"
	fsCacheFiles, err := fsCacheFilepathGlob(pattern)
	if err != nil {
		return plugin, fmt.Errorf("failed to get files from filesystem cache. %w", err)
	}
	FsCache.SyncWithIndex(&Index, fsCacheFiles)
	if plugin == nil {
		return plugin, nil
	}
	defer h.plugin.UpdateStatus(plugin)
	if plugin.Spec.Plugin.NoCache {
		plugin.Status.CacheState = Disabled
	} else {
		plugin.Status.CacheState = Pending
	}
	err = FsCache.SyncWithControllersCache(cachedPlugins)
	if err != nil {
		return plugin, fmt.Errorf("failed to sync filesystem cache with controller cache. %w", err)
	}
	if !plugin.Spec.Plugin.NoCache {
		plugin.Status.CacheState = Cached
	}

	return plugin, nil
}
