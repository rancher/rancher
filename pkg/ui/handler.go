package ui

import (
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/steve/pkg/ui"
)

var (
	ember = newHandler(settings.UIIndex.Get,
		settings.UIPath.Get,
		settings.UIOfflinePreferred.Get)
	emberAlwaysOffline = newHandler(settings.UIIndex.Get,
		settings.UIPath.Get,
		func() string { return "true" })
	vue = newHandler(settings.UIDashboardIndex.Get,
		settings.UIDashboardPath.Get,
		settings.UIOfflinePreferred.Get)
	emberIndex = ember.IndexFile()
	vueIndex   = vue.IndexFile()
)

func newHandler(
	indexSetting func() string,
	pathSetting func() string,
	offlineSetting func() string) *ui.Handler {
	return ui.NewUIHandler(&ui.Options{
		Index:          indexSetting,
		Offline:        offlineSetting,
		Path:           pathSetting,
		ReleaseSetting: settings.IsRelease,
	})
}
