package api

import (
	"strings"

	"github.com/rancher/norman/api"
	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/settings"
)

func NewServer(schemas *types.Schemas) (*api.Server, error) {
	server := normanapi.NewAPIServer()
	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}
	server.CustomAPIUIResponseWriter(cssURL, jsURL, settings.APIUIVersion.Get)
	return server, nil
}

func cssURL() string {
	if !strings.HasPrefix(settings.ServerVersion.Get(), "v") {
		return ""
	}
	return "/api-ui/ui.min.css"
}

func jsURL() string {
	if !strings.HasPrefix(settings.ServerVersion.Get(), "v") {
		return ""
	}
	return "/api-ui/ui.min.js"
}
