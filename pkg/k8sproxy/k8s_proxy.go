package k8sproxy

import (
	"net/http"

	"github.com/rancher/rancher/pkg/clusterrouter"
	"github.com/rancher/rancher/pkg/k8slookup"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/dialer"
)

func New(scaledContext *config.ScaledContext, dialer dialer.Factory) http.Handler {
	return clusterrouter.New(scaledContext.LocalConfig, k8slookup.New(scaledContext, false), dialer,
		scaledContext.Management.Clusters("").Controller().Lister())
}
