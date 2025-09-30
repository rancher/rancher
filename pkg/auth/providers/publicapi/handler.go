package publicapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/util"
	v3public "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	publicSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3public"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type v1AuthProviderStore struct {
	authConfigCache         v3.AuthConfigCache
	authConfigsUnstructured dynamic.ResourceInterface
}

func newV1AuthProviderStore(wContext *wrangler.Context) (*v1AuthProviderStore, error) {
	dynamicClient, err := dynamic.NewForConfig(wContext.RESTConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &v1AuthProviderStore{
		authConfigCache: wContext.Mgmt.AuthConfig().Cache(),
		authConfigsUnstructured: dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "management.cattle.io",
			Version:  "v3",
			Resource: "authconfigs",
		}),
	}, nil
}

func (s *v1AuthProviderStore) List() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		list, err := s.authConfigCache.List(labels.Everything())
		if err != nil {
			http.Error(w, "failed to list auth providers", http.StatusInternalServerError)
			return
		}

		response := struct {
			Type string           `json:"type"`
			Data []map[string]any `json:"data"`
		}{
			Type: "Collection",
		}

		for _, authConfig := range list {
			if !authConfig.Enabled {
				continue
			}

			raw, err := s.authConfigsUnstructured.Get(r.Context(), authConfig.Name, metav1.GetOptions{})
			if err != nil {
				http.Error(w, "failed to get auth config", http.StatusInternalServerError)
				return
			}

			raw.Object[".host"] = util.GetHost(r)

			authProvider, err := providers.GetProviderByType(authConfig.Type).TransformToAuthProvider(raw.Object)
			if err != nil {
				http.Error(w, "failed to get auth provider", http.StatusInternalServerError)
				return
			}

			response.Data = append(response.Data, authProvider)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(&response); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	})
}

func NewV1Handler(ctx context.Context, scaledContext *config.ScaledContext) (http.Handler, error) {
	providerStore, err := newV1AuthProviderStore(scaledContext.Wrangler)
	if err != nil {
		return nil, err
	}

	r := mux.NewRouter()
	r.Methods(http.MethodGet).Path("/v1-public/authProviders").Handler(providerStore.List())
	r.Methods(http.MethodPost).Path("/v1-public/login").HandlerFunc(newV1LoginHandler(scaledContext).login)

	return r, nil
}

type ServerOption func(server *normanapi.Server)

func NewV3Handler(ctx context.Context, mgmtCtx *config.ScaledContext, opts ...ServerOption) (http.Handler, error) {
	schemas := types.NewSchemas().AddSchemas(publicSchema.PublicSchemas)
	if err := authProviderSchemas(mgmtCtx, schemas); err != nil {
		return nil, err
	}

	server := normanapi.NewAPIServer()
	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(server)
	}

	return server, nil
}

var authProviderTypes = []string{
	v3public.ActiveDirectoryProviderType,
	v3public.AzureADProviderType,
	v3public.GithubProviderType,
	v3public.GithubAppProviderType,
	v3public.LocalProviderType,
	v3public.OpenLdapProviderType,
	v3public.FreeIpaProviderType,
	v3public.PingProviderType,
	v3public.ADFSProviderType,
	v3public.KeyCloakProviderType,
	v3public.OKTAProviderType,
	v3public.ShibbolethProviderType,
	v3public.GoogleOAuthProviderType,
	v3public.OIDCProviderType,
	v3public.KeyCloakOIDCProviderType,
	v3public.GenericOIDCProviderType,
	v3public.CognitoProviderType,
}

func authProviderSchemas(management *config.ScaledContext, schemas *types.Schemas) error {
	schema := schemas.Schema(&publicSchema.PublicVersion, v3public.AuthProviderType)
	setAuthProvidersStore(schema, management)
	lh := newV3LoginHandler(management)

	for _, apSubtype := range authProviderTypes {
		subSchema := schemas.Schema(&publicSchema.PublicVersion, apSubtype)
		subSchema.Store = subtype.NewSubTypeStore(apSubtype, schema.Store)
		subSchema.ActionHandler = lh.login
		subSchema.Formatter = loginActionFormatter
	}

	schema = schemas.Schema(&publicSchema.PublicVersion, v3public.AuthTokenType)
	setAuthTokensStore(schema, management)
	return nil
}

func loginActionFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "login")
}
