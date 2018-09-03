package api

import (
	"github.com/rancher/norman/api"
	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/server/apiui"
)

func NewServer(schemas *types.Schemas) (*api.Server, error) {
	server := normanapi.NewAPIServer()
	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}
	apiui.AddAPIUIWriter(server)
	return server, nil
}
