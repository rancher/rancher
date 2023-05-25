package apiui

import (
	"github.com/rancher/rancher/pkg/settings"
	steve "github.com/rancher/steve/pkg/server"
)

func Register(server *steve.Server) {
	server.APIServer.CustomAPIUIResponseWriter(cssURL, jsURL, settings.APIUIVersion.Get)
}

func cssURL() string {
	switch settings.UIOfflinePreferred.Get() {
	case "dynamic":
		if !settings.IsRelease() {
			return ""
		}
	case "false":
		return ""
	}
	return "/api-ui/ui.min.css"
}

func jsURL() string {
	switch settings.UIOfflinePreferred.Get() {
	case "dynamic":
		if !settings.IsRelease() {
			return ""
		}
	case "false":
		return ""
	}
	return "/api-ui/ui.min.js"
}
