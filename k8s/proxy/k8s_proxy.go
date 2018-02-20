package proxy

import (
	"net/http"

	"github.com/rancher/rancher/k8s/lookup"
	"github.com/rancher/rancher/pkg/clusterrouter"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/dialer"
)

func New(managementContext *config.ScaledContext, dialer dialer.Factory) http.Handler {
	return clusterrouter.New(lookup.New(managementContext), dialer)
}
