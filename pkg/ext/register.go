package ext

import (
	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/ext/resources/tokens"
	"github.com/rancher/rancher/pkg/wrangler"
)

func RegisterSubRoutes(router *mux.Router, wContext *wrangler.Context) {
	apiServer := NewAPIServer()
	tokenStore := tokens.NewTokenStore(wContext.Core.Secret(), wContext.Core.Secret().Cache(), wContext.K8s.AuthorizationV1().SubjectAccessReviews())
	tokenHandler := NewStoreDelegate(tokenStore, tokens.SchemeGroupVersion)
	apiServer.AddAPIResource(tokens.SchemeGroupVersion, tokens.TokenAPIResource, tokenHandler.Delegate)
	apiServer.RegisterRoutes(router)
}
