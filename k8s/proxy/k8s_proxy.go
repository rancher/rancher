package proxy

import (
	"net/http"

	"github.com/rancher/netes/router"
	"github.com/rancher/netes/types"
	"github.com/rancher/rancher/k8s/lookup"
	"github.com/rancher/types/config"
)

func New(managementContext *config.ManagementContext) (http.Handler, error) {
	return router.New(&types.GlobalConfig{
		Lookup: lookup.New(managementContext),
	}), nil
}
