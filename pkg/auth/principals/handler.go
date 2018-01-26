package principals

import (
	"context"
	"github.com/gorilla/mux"
	"net/http"

	"github.com/rancher/types/config"
)

//principalAPIHandler is a wrapper over the mux router serving /v3/principals API
type principalAPIHandler struct {
	principalRouter http.Handler
}

func (h *principalAPIHandler) getRouter() http.Handler {
	return h.principalRouter
}

func (h *principalAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.getRouter().ServeHTTP(w, r)
}

func NewPrincipalAPIHandler(ctx context.Context, mgmtCtx *config.ManagementContext) (http.Handler, error) {
	router, err := newPrincipalRouter(ctx, mgmtCtx)
	if err != nil {
		return nil, err
	}
	return &principalAPIHandler{principalRouter: router}, nil
}

//newPrincipalRouter creates and configures a mux router for /v3/principals APIs
func newPrincipalRouter(ctx context.Context, mgmtCtx *config.ManagementContext) (*mux.Router, error) {
	apiServer, err := newPrincipalAPIServer(ctx, mgmtCtx)
	if err != nil {
		return nil, err
	}

	router := mux.NewRouter().StrictSlash(true)
	// Application routes
	router.Methods("GET").Path("/v3/principals").Handler(http.HandlerFunc(apiServer.listPrincipals))
	router.Methods("POST").Path("/v3/principals").Queries("action", "search").Handler(http.HandlerFunc(apiServer.searchPrincipals))

	return router, nil
}
