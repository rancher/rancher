package ext

import (
	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/ext/resources/tokens"
	"github.com/rancher/rancher/pkg/wrangler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func RegisterSubRoutes(router *mux.Router, wContext *wrangler.Context) {
	apiServer := NewAPIServer()
	tokenHandler := StoreDelegate[*tokens.RancherToken]{
		GroupVersion: tokens.SchemeGroupVersion,
		Store:        tokens.NewTokenStore(wContext.Core.Secret(), wContext.Core.Secret().Cache()),
	}
	apiServer.AddAPIResource(tokens.SchemeGroupVersion, metav1.APIResource{}, tokenHandler.Delegate)
	apiServer.RegisterRoutes(router)
}
