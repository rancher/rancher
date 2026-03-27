package publicapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/azure"
	"github.com/rancher/rancher/pkg/auth/providers/cognito"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/githubapp"
	"github.com/rancher/rancher/pkg/auth/providers/googleoauth"
	"github.com/rancher/rancher/pkg/auth/providers/keycloakoidc"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3public "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	publicSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3public"
	"github.com/rancher/rancher/pkg/types/config"
)

// NewV1Handler returns an http handler for /v1-public endpoints.
func NewV1Handler(ctx context.Context, scaledContext *config.ScaledContext) (http.Handler, error) {
	providerStore, err := newV1AuthProviderStore(scaledContext.Wrangler)
	if err != nil {
		return nil, fmt.Errorf("creating authprovider store: %w", err)
	}

	authTokenStore := newV1AuthTokenStore(scaledContext.Wrangler)

	r := http.NewServeMux()
	r.HandleFunc("GET /v1-public/authproviders", providerStore.List)
	r.HandleFunc("GET /v1-public/authprovider-types", listAuthProviderTypes)
	r.HandleFunc("POST /v1-public/login", newV1LoginHandler(scaledContext).login)
	r.HandleFunc("GET /v1-public/authtokens/{id}", authTokenStore.Get)
	r.HandleFunc("DELETE /v1-public/authtokens/{id}", authTokenStore.Delete)

	return r, nil
}

type ServerOption func(server *normanapi.Server)

// NewV3Handler returns an http handler for /v3-public endpoints.
// Deprecated. Use NewV1Handler instead. Will be removed in future releases.
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

type authProvider struct {
	ID          string `json:"id"`
	ConfigType  string `json:"configType"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

var authProviderTypes = map[string]authProvider{
	v3public.ActiveDirectoryProviderType: {
		ID:          activedirectory.ProviderName,
		Description: "Active Directory authentication provider",
		Type:        "ldap",
		ConfigType:  client.ActiveDirectoryConfigType,
	},
	v3public.AzureADProviderType: {
		ID:          azure.ProviderName,
		Description: "Microsoft Entra authentication provider",
		Type:        "oidc",
		ConfigType:  client.AzureADConfigType,
	},
	v3public.GithubProviderType: {
		ID:          github.ProviderName,
		Description: "GitHub authentication OAuth provider",
		Type:        "oauth",
		ConfigType:  client.GithubConfigType,
	},
	v3public.GithubAppProviderType: {
		ID:          githubapp.ProviderName,
		Description: "GitHub App authentication via OAuth provider",
		Type:        "oauth",
		ConfigType:  client.GithubAppConfigType,
	},
	v3public.LocalProviderType: {
		ID:          local.Name,
		Description: "Local authentication provider",
		Type:        "local",
		ConfigType:  client.LocalConfigType,
	},
	v3public.OpenLdapProviderType: {
		ID:          ldap.OpenLdapName,
		Description: "OpenLDAP authentication provider",
		Type:        "ldap",
		ConfigType:  client.OpenLdapConfigType,
	},
	v3public.FreeIpaProviderType: {
		ID:          ldap.FreeIpaName,
		Description: "FreeIPA authentication provider",
		Type:        "saml",
		ConfigType:  client.FreeIpaConfigType,
	},
	v3public.PingProviderType: {
		ID:          saml.PingName,
		Description: "Ping Identity authentication provider",
		Type:        "saml",
		ConfigType:  client.PingConfigType,
	},
	v3public.ADFSProviderType: {
		ID:          saml.ADFSName,
		Description: "ADFS authentication provider",
		Type:        "saml",
		ConfigType:  client.ADFSConfigType,
	},
	v3public.KeyCloakProviderType: {
		ID:          saml.KeyCloakName,
		Description: "Keycloak authentication provider",
		Type:        "saml",
		ConfigType:  client.KeyCloakConfigType,
	},
	v3public.OKTAProviderType: {
		ID:          saml.OKTAName,
		Description: "Okta authentication provider",
		Type:        "saml",
		ConfigType:  client.OKTAConfigType,
	},
	v3public.ShibbolethProviderType: {
		ID:          saml.ShibbolethName,
		Description: "Shibboleth authentication provider",
		Type:        "saml",
		ConfigType:  client.ShibbolethConfigType,
	},
	v3public.GenericSAMLProviderType: {
		ID:          saml.GenericSAMLName,
		Description: "Generic SAML authentication provider",
		Type:        "saml",
		ConfigType:  client.GenericSAMLConfigType,
	},
	v3public.GoogleOAuthProviderType: {
		ID:          googleoauth.ProviderName,
		Description: "Google OAuth authentication provider",
		Type:        "oauth",
		ConfigType:  client.GoogleOauthConfigType,
	},
	v3public.OIDCProviderType: {
		ID:          oidc.ProviderName,
		Description: "OpenID Connect authentication provider",
		Type:        "oidc",
		ConfigType:  client.OIDCConfigType,
	},
	v3public.KeyCloakOIDCProviderType: {
		ID:          keycloakoidc.ProviderName,
		Description: "Keycloak OIDC authentication provider",
		Type:        "oidc",
		ConfigType:  client.KeyCloakOIDCConfigType,
	},
	v3public.GenericOIDCProviderType: {
		ID:          genericoidc.ProviderName,
		Description: "Generic OIDC authentication provider",
		Type:        "oidc",
		ConfigType:  client.GenericOIDCConfigType,
	},
	v3public.CognitoProviderType: {
		ID:          cognito.ProviderName,
		Description: "Amazon Cognito authentication provider",
		Type:        "oidc",
		ConfigType:  client.CognitoConfigType,
	},
}

func authProviderSchemas(management *config.ScaledContext, schemas *types.Schemas) error {
	schema := schemas.Schema(&publicSchema.PublicVersion, v3public.AuthProviderType)
	setAuthProvidersStore(schema, management)
	lh := newV3LoginHandler(management)

	for apSubtype := range authProviderTypes {
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

func listAuthProviderTypes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(authProviderTypes); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
