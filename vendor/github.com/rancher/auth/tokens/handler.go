package tokens

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/types/config"
)

//tokenAndIdentityAPIHandler is a wrapper over the mux router serving /token and /identities API
type tokenAPIHandler struct {
	tokenRouter http.Handler
}

func (h *tokenAPIHandler) getRouter() http.Handler {
	return h.tokenRouter
}

func (h *tokenAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.getRouter().ServeHTTP(w, r)
}

func NewTokenAPIHandler(ctx context.Context, mgmtCtx *config.ManagementContext) (http.Handler, error) {
	router, err := newTokenRouter(ctx, mgmtCtx)
	if err != nil {
		return nil, err
	}
	return &tokenAPIHandler{tokenRouter: router}, nil
}

//newTokenRouter creates and configures a mux router for /v3/tokens APIs
func newTokenRouter(ctx context.Context, mgmtCtx *config.ManagementContext) (*mux.Router, error) {
	apiServer, err := newTokenAPIServer(ctx, mgmtCtx)
	if err != nil {
		return nil, err
	}

	router := mux.NewRouter().StrictSlash(true)
	// Application routes
	router.Methods("POST").Path("/v3/tokens").Queries("action", "login").Handler(http.HandlerFunc(apiServer.login))
	router.Methods("POST").Path("/v3/tokens").Queries("action", "logout").Handler(http.HandlerFunc(apiServer.logout))
	router.Methods("POST").Path("/v3/tokens").Handler(http.HandlerFunc(apiServer.deriveToken))
	router.Methods("GET").Path("/v3/tokens/{tokenId}").Handler(http.HandlerFunc(apiServer.getToken))
	router.Methods("GET").Path("/v3/tokens").Handler(http.HandlerFunc(apiServer.listTokens))
	router.Methods("DELETE").Path("/v3/tokens/{tokenId}").Handler(http.HandlerFunc(apiServer.removeToken))

	//router.Methods("GET").Path("/v3/identities").Handler(http.HandlerFunc(apiServer.listIdentities))
	//router.Methods("GET").Path("/v1/identities").Handler(api.ApiHandler(schemas, http.HandlerFunc(SearchIdentities)))

	return router, nil
}
