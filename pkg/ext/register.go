package ext

import (
	"github.com/gorilla/mux"
	openapitokens "github.com/rancher/rancher/pkg/ext/generated/openapi/tokens"
	openapiuseractivity "github.com/rancher/rancher/pkg/ext/generated/openapi/useractivity"
	"github.com/rancher/rancher/pkg/ext/resources/tokens"
	"github.com/rancher/rancher/pkg/ext/resources/useractivity"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/common"
)

func RegisterSubRoutes(router *mux.Router, wContext *wrangler.Context) {
	apiServer := NewAPIServer(getDefinitions)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tokens.TokenNamespace,
		},
	}
	wContext.Core.Namespace().Create(ns)

	tokenStore := tokens.NewTokenStore(wContext.Core.Secret(), wContext.Core.Secret().Cache(), wContext.K8s.AuthorizationV1().SubjectAccessReviews())
	tokenHandler := NewStoreDelegate(tokenStore, tokens.SchemeGroupVersion.WithKind(tokens.TokenAPIResource.Kind))
	tokenWebService := tokenHandler.WebService(tokens.RancherTokenName, tokens.TokenAPIResource.Namespaced)

	userActivityStore := useractivity.NewUserActivityStore(wContext.Mgmt.Token())
	userActivityHandler := NewStoreDelegate(userActivityStore, useractivity.SchemeGroupVersion.WithKind(useractivity.UserActivityAPIResource.Kind))
	userActivityWebService := userActivityHandler.WebService(useractivity.UserActivityName, useractivity.UserActivityAPIResource.Namespaced)

	apiServer.AddAPIResource(tokens.SchemeGroupVersion, tokens.TokenAPIResource, tokenHandler.Delegate, tokenWebService)
	apiServer.AddAPIResource(useractivity.SchemeGroupVersion, useractivity.UserActivityAPIResource, userActivityHandler.Delegate, userActivityWebService)
	apiServer.RegisterRoutes(router)
}

// getDefinitions merges many GetDefinitions into one map
func getDefinitions(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	result := make(map[string]common.OpenAPIDefinition)
	for _, getDefs := range []func(common.ReferenceCallback) map[string]common.OpenAPIDefinition{
		openapitokens.GetOpenAPIDefinitions,
		openapiuseractivity.GetOpenAPIDefinitions,
	} {
		defs := getDefs(ref)
		for key, val := range defs {
			result[key] = val
		}
	}
	return result
}
