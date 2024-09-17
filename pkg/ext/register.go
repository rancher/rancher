package ext

import (
	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/ext/generated/openapi"
	"github.com/rancher/rancher/pkg/ext/resources/tokens"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/common"
)

func RegisterSubRoutes(router *mux.Router, wContext *wrangler.Context, sc *config.ScaledContext) {
	apiServer := NewAPIServer(getDefinitions)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tokens.TokenNamespace,
		},
	}
	wContext.Core.Namespace().Create(ns)

	tokenStore := tokens.NewTokenStore(wContext.Core.Secret(), wContext.Core.Secret().Cache(),
		wContext.K8s.AuthorizationV1().SubjectAccessReviews(),
		sc.Management.UserAttributes("").Controller().Lister())
	tokenHandler := NewStoreDelegate(tokenStore, tokens.SchemeGroupVersion.WithKind(tokens.TokenAPIResource.Kind))
	tokenWebService := tokenHandler.WebService(tokens.TokenName, tokens.TokenAPIResource.Namespaced)

	apiServer.AddAPIResource(tokens.SchemeGroupVersion, tokens.TokenAPIResource, tokenHandler.Delegate, tokenWebService)
	apiServer.RegisterRoutes(router)
}

// getDefinitions merges many GetDefinitions into one map
func getDefinitions(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	result := make(map[string]common.OpenAPIDefinition)
	for _, getDefs := range []func(common.ReferenceCallback) map[string]common.OpenAPIDefinition{
		openapi.GetOpenAPIDefinitions,
	} {
		defs := getDefs(ref)
		for key, val := range defs {
			result[key] = val
		}
	}
	return result
}
