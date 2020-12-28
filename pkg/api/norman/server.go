package norman

import (
	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/settings"
)

func NewServer(schemas *types.Schemas) (*normanapi.Server, error) {
	server := normanapi.NewAPIServer()
	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}
	ConfigureAPIUI(server)
	return server, nil
}

func ConfigureAPIUI(server *normanapi.Server) {
	server.CustomAPIUIResponseWriter(cssURL, jsURL, settings.APIUIVersion.Get)
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
