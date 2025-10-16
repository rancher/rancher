package publicapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/store/empty"
	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/util"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func setAuthProvidersStore(schema *types.Schema, apiContext *config.ScaledContext) {
	schema.Store = &authProvidersStore{
		authConfigsRaw: apiContext.Management.AuthConfigs("").ObjectClient().UnstructuredClient(),
	}
}

type authProvidersStore struct {
	empty.Store
	authConfigsRaw objectclient.GenericClient
}

func (s *authProvidersStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]any, error) {
	o, err := s.authConfigsRaw.Get(id, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	u, _ := o.(runtime.Unstructured)
	config := u.UnstructuredContent()
	if t, ok := config["type"].(string); ok && t != "" {
		config[".host"] = util.GetHost(apiContext.Request)
		provider, err := providers.GetProviderByType(t).TransformToAuthProvider(config)
		if err != nil {
			return nil, err
		}
		return provider, nil
	}

	return nil, httperror.NewAPIError(httperror.NotFound, "")
}

func (s *authProvidersStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]any, error) {
	rrr, _ := s.authConfigsRaw.List(metav1.ListOptions{})
	var result []map[string]any
	list, _ := rrr.(*unstructured.UnstructuredList)
	for _, i := range list.Items {
		if t, ok := i.Object["type"].(string); ok && t != "" {
			if enabled, ok := i.Object["enabled"].(bool); ok && enabled {
				i.Object[".host"] = util.GetHost(apiContext.Request)
				provider, err := providers.GetProviderByType(t).TransformToAuthProvider(i.Object)
				if err != nil {
					return result, err
				}
				result = append(result, provider)
			}
		}
	}
	return result, nil
}

func setAuthTokensStore(schema *types.Schema, apiContext *config.ScaledContext) {
	schema.Store = &v3AuthTokensStore{
		tokens: apiContext.Wrangler.Mgmt.SamlToken(),
	}
}

type v3AuthTokensStore struct {
	empty.Store
	tokens mgmtv3.SamlTokenClient
}

func (t *v3AuthTokensStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]any, error) {
	token, err := t.tokens.Get(namespace.GlobalNamespace, id, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("token %s not found", id))
		}
		return nil, err
	}
	generated := transformToAuthToken(token)
	return generated, err
}

func (t *v3AuthTokensStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]any, error) {
	if err := t.tokens.Delete(namespace.GlobalNamespace, id, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	return nil, nil
}

func transformToAuthToken(token *apiv3.SamlToken) map[string]any {
	generated := map[string]any{}
	if token == nil {
		return generated
	}
	generated["id"] = token.Name
	generated["type"] = "authToken"
	generated["name"] = token.Name
	generated["token"] = token.Token
	generated["expiresAt"] = token.ExpiresAt
	return generated
}

type v1AuthProviderStore struct {
	authConfigCache         mgmtv3.AuthConfigCache
	authConfigsUnstructured dynamic.ResourceInterface
	getProviderByType       func(string) common.AuthProvider
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
		getProviderByType: providers.GetProviderByType,
	}, nil
}

func (s *v1AuthProviderStore) List(w http.ResponseWriter, r *http.Request) {
	list, err := s.authConfigCache.List(labels.Everything())
	if err != nil {
		logrus.Errorf("v1AuthProviderStore: Listing authconfigs: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	response := struct {
		Data []map[string]any `json:"data"`
	}{}

	for _, authConfig := range list {
		if !authConfig.Enabled {
			continue
		}

		raw, err := s.authConfigsUnstructured.Get(r.Context(), authConfig.Name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("v1AuthProviderStore: Getting authconfig %s: %s", authConfig.Name, err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		raw.Object[".host"] = util.GetHost(r)

		authProvider, err := s.getProviderByType(authConfig.Type).TransformToAuthProvider(raw.Object)
		if err != nil {
			logrus.Errorf("v1AuthProviderStore: Getting authprovider %s: %s", authConfig.Type, err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		response.Data = append(response.Data, authProvider)
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(&response); err != nil {
		logrus.Errorf("v1AuthProviderStore: Encoding authproviders response: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func newV1AuthTokenStore(wContext *wrangler.Context) *v1AuthTokenStore {
	return &v1AuthTokenStore{
		tokens: wContext.Mgmt.SamlToken(),
	}
}

type v1AuthTokenStore struct {
	tokens mgmtv3.SamlTokenClient
}

func (s *v1AuthTokenStore) Get(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	token, err := s.tokens.Get(namespace.GlobalNamespace, id, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			logrus.Errorf("v1AuthTokenStore: Getting authtoken %s: %v", id, err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	// Only return details that are actually used.
	response := map[string]any{
		"token":     token.Token,
		"expiresAt": token.ExpiresAt,
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(&response); err != nil {
		logrus.Errorf("v1AuthTokenStore: Encoding response for authtoken %s: %v", id, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (s *v1AuthTokenStore) Delete(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	err := s.tokens.Delete(namespace.GlobalNamespace, id, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logrus.Errorf("v1AuthTokenStore: Deleting authtoken %s: %v", id, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
