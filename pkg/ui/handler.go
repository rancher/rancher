package ui

import (
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/steve/pkg/ui"
)

var (
	vue = newHandler(settings.UIDashboardIndex.Get,
		settings.UIDashboardPath.Get,
		settings.UIOfflinePreferred.Get)
	vueIndex = vue.IndexFile()
)

func newHandler(
	indexSetting func() string,
	pathSetting func() string,
	offlineSetting func() string) *ui.Handler {
	return ui.NewUIHandler(&ui.Options{
		Index:               indexSetting,
		Offline:             offlineSetting,
		Path:                pathSetting,
		ReleaseSetting:      settings.IsRelease,
		APIUIVersionSetting: settings.APIUIVersion.Get,
	})
}
