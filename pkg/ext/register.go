package ext

import (
	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/ext/resources/tokens"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/kube-openapi/pkg/common"
)

func RegisterSubRoutes(router *mux.Router, wContext *wrangler.Context) {
	apiServer := NewAPIServer(getDefinitions)
	tokenStore := tokens.NewTokenStore(wContext.Core.Secret(), wContext.Core.Secret().Cache(), wContext.K8s.AuthorizationV1().SubjectAccessReviews())
	tokenHandler := NewStoreDelegate(tokenStore, tokens.SchemeGroupVersion.WithKind("RancherToken"))
	tokenWebService := tokenHandler.WebService(tokens.RancherTokenName, tokens.TokenAPIResource.Namespaced)
	apiServer.AddAPIResource(tokens.SchemeGroupVersion, tokens.TokenAPIResource, tokenHandler.Delegate, tokenWebService)
	apiServer.RegisterRoutes(router)
}

// getDefinitions merges many GetDefinitions into one map
func getDefinitions(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	result := make(map[string]common.OpenAPIDefinition)
	for _, getDefs := range []func(common.ReferenceCallback) map[string]common.OpenAPIDefinition{
		tokens.GetDefinitions,
	} {
		defs := getDefs(ref)
		for key, val := range defs {
			result[key] = val
		}
	}
	return result
}