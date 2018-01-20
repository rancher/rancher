package router

import (
	"encoding/json"
	"net/http"

	"github.com/rancher/netes/cluster"
	"github.com/rancher/netes/server"
	"github.com/rancher/netes/types"
	"github.com/rancher/norman/httperror"
)

type Router struct {
	clusterLookup cluster.Lookup
	serverFactory *server.Factory
}

func New(config *types.GlobalConfig) http.Handler {
	return &Router{
		clusterLookup: config.Lookup,
		serverFactory: server.NewFactory(config),
	}
}

func (r *Router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	c, handler, err := r.serverFactory.Get(req)
	if err != nil {
		response(rw, httperror.ServerError, err.Error())
		return
	}

	if c == nil {
		response(rw, httperror.NotFound, "No cluster available")
		return
	}

	ctx := cluster.StoreCluster(req.Context(), c)
	handler.ServeHTTP(rw, req.WithContext(ctx))
}

func response(rw http.ResponseWriter, code httperror.ErrorCode, message string) {
	rw.WriteHeader(code.Status)
	rw.Header().Set("content-type", "application/json")
	json.NewEncoder(rw).Encode(httperror.NewAPIError(code, message))
}
