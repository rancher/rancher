package server

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	managementapi "github.com/rancher/management-api/server"
	"github.com/rancher/rancher/server/ui"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/config"
)

func New(ctx context.Context, management *config.ManagementContext) (http.Handler, error) {
	managementAPI, err := managementapi.New(ctx, management)
	if err != nil {
		return nil, err
	}

	router := mux.NewRouter()

	router.Handle("/", ui.UI(managementAPI))
	router.PathPrefix("/meta").Handler(managementAPI)
	router.PathPrefix(managementSchema.Version.Path).Handler(managementAPI)
	router.NotFoundHandler = ui.UI(http.NotFoundHandler())

	return router, nil
}
