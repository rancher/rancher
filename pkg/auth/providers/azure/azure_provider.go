// Package azure provides functions and types to register and use Azure AD as the auth provider in Rancher.
package azure

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/azure/clients"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	Name = "azuread"

	// IDTokenCookie is the name of the cookie that holds the raw Azure AD ID token for SSO logout.
	IDTokenCookie = "R_AZUREAD_ID"

	// maxSafeCookieSize is the threshold above which we warn that a cookie may exceed
	// the browser's ~4 KB per-cookie limit and be silently dropped.
	maxSafeCookieSize = 3800
)

type unstructuredGetter interface {
	Get(string, metav1.GetOptions) (runtime.Object, error)
}

type Provider struct {
	authConfigs v3.AuthConfigInterface
	Retriever   unstructuredGetter
	secrets     wcorev1.SecretController
	userMGR     user.Manager
	tokenMGR    *tokens.Manager
	// getConfig is used to retrieve the AzureAD configuration; injectable for testing.
	getConfig func() (*apiv3.AzureADConfig, error)
}

func Configure(mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	var err error
	clients.GroupCache, err = lru.New(settings.AzureGroupCacheSize.GetInt())
	if err != nil {
		logrus.Warnf("initial azure-group-cache-size was invalid value, setting to 10000 error:%v", err)
		clients.GroupCache, _ = lru.New(10000)
	}

	p := &Provider{
		Retriever:   mgmtCtx.Management.AuthConfigs("").ObjectClient().UnstructuredClient(),
		authConfigs: mgmtCtx.Management.AuthConfigs(""),
		secrets:     mgmtCtx.Wrangler.Core.Secret(),
		userMGR:     userMGR,
		tokenMGR:    tokenMGR,
	}
	p.getConfig = p.GetAzureConfigK8s
	return p
}

func (ap *Provider) LogoutAll(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	cfg, err := ap.getConfig()
	if err != nil {
		return fmt.Errorf("getting Azure AD config for LogoutAll: %w", err)
	}

	if !cfg.LogoutAllEnabled {
		return fmt.Errorf("azure AD [logout-all]: provider not configured for SSO logout")
	}

	endSessionEndpoint := cfg.EndSessionEndpoint
	if endSessionEndpoint == "" {
		endSessionEndpoint, err = url.JoinPath(cfg.Endpoint, cfg.TenantID, "/oauth2/v2.0/logout")
		if err != nil {
			return fmt.Errorf("building Azure AD logout endpoint: %w", err)
		}
	}

	endSessionURL, err := url.Parse(endSessionEndpoint)
	if err != nil {
		return fmt.Errorf("parsing Azure AD logout endpoint: %w", err)
	}

	authLogout := &apiv3.AuthConfigLogoutInput{}
	if err := json.NewDecoder(r.Body).Decode(authLogout); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("azure AD: parsing request body: %v", err))
	}

	params := endSessionURL.Query()
	params.Set("client_id", cfg.ApplicationID)
	if authLogout.FinalRedirectURL != "" {
		params.Set("post_logout_redirect_uri", authLogout.FinalRedirectURL)
	}
	if idToken := getIDTokenCookie(r); idToken != "" {
		params.Set("id_token_hint", idToken)
	}
	endSessionURL.RawQuery = params.Encode()

	logrus.Debugf("azure AD [logout-all]: redirecting to %s", endSessionURL.String())

	data := map[string]any{
		"idpRedirectUrl": endSessionURL.String(),
		"type":           "authConfigLogoutOutput",
		"baseType":       "authConfigLogoutOutput",
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}

func (ap *Provider) Logout(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	cfg, err := ap.getConfig()
	if err != nil {
		return fmt.Errorf("azure AD [logout]: getting Azure AD config for Logout: %w", err)
	}
	if cfg.LogoutAllForced {
		return fmt.Errorf("azure AD [logout]: provider configured for forced SSO logout, rejecting regular logout")
	}
	return nil
}

func (ap *Provider) GetName() string {
	return Name
}

func (ap *Provider) AuthenticateUser(w http.ResponseWriter, r *http.Request, input any) (apiv3.Principal, []apiv3.Principal, string, error) {
	login, ok := input.(*apiv3.AzureADLogin)
	if !ok {
		return apiv3.Principal{}, nil, "", errors.New("unexpected input type")
	}
	cfg, err := ap.getConfig()
	if err != nil {
		return apiv3.Principal{}, nil, "", err
	}
	userPrincipal, groupPrincipals, providerToken, idToken, err := ap.loginUser(cfg, login, false)
	if err != nil {
		return apiv3.Principal{}, nil, "", err
	}
	if idToken != "" {
		setIDTokenCookie(r, w, idToken)
	}
	return userPrincipal, groupPrincipals, providerToken, nil
}

func (ap *Provider) RefetchGroupPrincipals(principalID, secret string) ([]apiv3.Principal, error) {
	cfg, err := ap.GetAzureConfigK8s()
	if err != nil {
		return nil, err
	}
	azureClient, err := clients.NewAzureClient(cfg, ap.secrets)
	if err != nil {
		return nil, err
	}

	logrus.Debug("[AZURE_PROVIDER] Started getting user info from AzureAD")

	parsed, err := clients.ParsePrincipalID(principalID)
	if err != nil {
		return nil, err
	}
	userPrincipal, err := azureClient.GetUser(parsed["ID"])
	if err != nil {
		return nil, err
	}

	logrus.Debug("[AZURE_PROVIDER] Completed getting user info from AzureAD")

	userGroups, err := azureClient.ListGroupMemberships(clients.GetPrincipalID(userPrincipal), cfg.GroupMembershipFilter)
	if err != nil {
		return nil, err
	}

	groupPrincipals, err := clients.UserGroupsToPrincipals(azureClient, userGroups)
	if err != nil {
		return nil, err
	}

	return groupPrincipals, nil
}

func (ap *Provider) SearchPrincipals(name, principalType string, token accessor.TokenAccessor) ([]apiv3.Principal, error) {
	cfg, err := ap.GetAzureConfigK8s()
	if err != nil {
		return nil, err
	}
	var principals []apiv3.Principal

	azureClient, err := ap.newAzureClient(cfg)
	if err != nil {
		return nil, err
	}

	switch principalType {
	case "user":
		principals, err = ap.searchUserPrincipalsByName(azureClient, name, token)
		if err != nil {
			return nil, err
		}
	case "group":
		principals, err = ap.searchGroupPrincipalsByName(azureClient, name, token)
		if err != nil {
			return nil, err
		}
	case "":
		users, err := ap.searchUserPrincipalsByName(azureClient, name, token)
		if err != nil {
			return nil, err
		}
		groups, err := ap.searchGroupPrincipalsByName(azureClient, name, token)
		if err != nil {
			return nil, err
		}
		principals = append(principals, users...)
		principals = append(principals, groups...)
	}

	return principals, nil
}

func (ap *Provider) GetPrincipal(principalID string, token accessor.TokenAccessor) (apiv3.Principal, error) {
	var principal apiv3.Principal
	var err error
	cfg, err := ap.GetAzureConfigK8s()
	if err != nil {
		return apiv3.Principal{}, err
	}

	azureClient, err := ap.newAzureClient(cfg)
	if err != nil {
		return principal, err
	}

	parsed, err := clients.ParsePrincipalID(principalID)
	if err != nil {
		return apiv3.Principal{}, httperror.NewAPIError(httperror.NotFound, "invalid principal")
	}

	switch parsed["type"] {
	case "user":
		principal, err = ap.getUserPrincipal(azureClient, parsed["ID"], token)
	case "group":
		principal, err = ap.getGroupPrincipal(azureClient, parsed["ID"], token)
	}

	if err != nil {
		return apiv3.Principal{}, err
	}

	return principal, nil
}

func (ap *Provider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = ap.actionHandler
	schema.Formatter = ap.formatter
}

func (ap *Provider) TransformToAuthProvider(
	authConfig map[string]any,
) (map[string]any, error) {
	p := common.TransformToAuthProvider(authConfig)
	p[publicclient.AzureADProviderFieldRedirectURL] = formAzureRedirectURL(authConfig)

	tenantID, _ := authConfig["tenantId"].(string)
	p[publicclient.AzureADProviderFieldTenantID] = tenantID
	applicationID, _ := authConfig["applicationId"].(string)
	p[publicclient.AzureADProviderFieldClientID] = applicationID

	p[publicclient.AzureADProviderFieldScopes] = []string{"openid", "profile", "email"}

	// this is the default base endpoint, i.e.: https://login.microsoftonline.com/
	baseEndpoint, _ := authConfig["endpoint"].(string)

	// getString will return the string value from the map, or a blank string if the value is not a string
	getString := func(data map[string]any, key string) string {
		v, ok := data[key]
		if !ok {
			return ""
		}

		s, _ := v.(string)
		return s
	}

	// Set default authEndpoint, or custom if provided
	joined, err := url.JoinPath(baseEndpoint, tenantID, "/oauth2/v2.0/authorize")
	if err != nil {
		return nil, err
	}
	p[publicclient.AzureADProviderFieldAuthURL] = joined

	if customEndpoint := getString(authConfig, "authEndpoint"); customEndpoint != "" {
		p[publicclient.AzureADProviderFieldAuthURL] = customEndpoint
	}

	// Set default tokenEndpoint, or custom if provided
	joined, err = url.JoinPath(baseEndpoint, tenantID, "/oauth2/v2.0/token")
	if err != nil {
		return nil, err
	}
	p[publicclient.AzureADProviderFieldTokenURL] = joined

	if customEndpoint := getString(authConfig, "tokenEndpoint"); customEndpoint != "" {
		p[publicclient.AzureADProviderFieldTokenURL] = customEndpoint
	}

	// Set default deviceAuthEndpoint, or custom if provided
	joined, err = url.JoinPath(baseEndpoint, tenantID, "/oauth2/v2.0/devicecode")
	if err != nil {
		return nil, err
	}
	p[publicclient.AzureADProviderFieldDeviceAuthURL] = joined

	if customEndpoint := getString(authConfig, "deviceAuthEndpoint"); customEndpoint != "" {
		p[publicclient.AzureADProviderFieldDeviceAuthURL] = customEndpoint
	}

	return p, nil
}

func (ap *Provider) loginUser(config *apiv3.AzureADConfig, azureCredential *apiv3.AzureADLogin, test bool) (apiv3.Principal, []apiv3.Principal, string, string, error) {
	azureClient, err := clients.NewAzureClient(config, ap.secrets)
	if err != nil {
		return apiv3.Principal{}, nil, "", "", err
	}
	userPrincipal, groupPrincipals, providerToken, idToken, err := azureClient.LoginUser(config, azureCredential)
	if err != nil {
		return apiv3.Principal{}, nil, "", "", err
	}
	testAllowedPrincipals := config.AllowedPrincipalIDs
	if test && config.AccessMode == "restricted" {
		testAllowedPrincipals = append(testAllowedPrincipals, userPrincipal.Name)
	}

	allowed, err := ap.userMGR.CheckAccess(config.AccessMode, testAllowedPrincipals, userPrincipal.Name, groupPrincipals)
	if err != nil {
		return apiv3.Principal{}, nil, "", "", err
	}
	if !allowed {
		return apiv3.Principal{}, nil, "", "", apierror.NewAPIError(validation.Unauthorized, "unauthorized")
	}

	return userPrincipal, groupPrincipals, providerToken, idToken, nil
}

func (ap *Provider) getUserPrincipal(client clients.AzureClient, principalID string, token accessor.TokenAccessor) (apiv3.Principal, error) {
	principal, err := client.GetUser(principalID)
	if err != nil {
		return apiv3.Principal{}, &common.NonTransientError{Err: httperror.NewAPIError(httperror.NotFound, err.Error())}
	}
	principal.Me = common.SamePrincipal(token.GetUserPrincipal(), principal)
	return principal, nil
}

func (ap *Provider) getGroupPrincipal(client clients.AzureClient, id string, token accessor.TokenAccessor) (apiv3.Principal, error) {
	principal, err := client.GetGroup(id)
	if err != nil {
		return apiv3.Principal{}, &common.NonTransientError{Err: httperror.NewAPIError(httperror.NotFound, err.Error())}
	}
	principal.MemberOf = ap.userMGR.IsMemberOf(token, principal)
	return principal, nil
}

func (ap *Provider) searchUserPrincipalsByName(client clients.AzureClient, name string, token accessor.TokenAccessor) ([]apiv3.Principal, error) {
	filter := fmt.Sprintf("startswith(userPrincipalName,'%[1]s') or startswith(displayName,'%[1]s') or startswith(givenName,'%[1]s') or startswith(surname,'%[1]s')", name)
	principals, err := client.ListUsers(filter)
	if err != nil {
		return nil, err
	}
	for _, principal := range principals {
		principal.Me = common.SamePrincipal(token.GetUserPrincipal(), principal)
	}
	return principals, nil
}

func (ap *Provider) searchGroupPrincipalsByName(client clients.AzureClient, name string, token accessor.TokenAccessor) ([]apiv3.Principal, error) {
	filter := fmt.Sprintf("startswith(displayName,'%[1]s') or startswith(mail,'%[1]s') or startswith(mailNickname,'%[1]s')", name)
	principals, err := client.ListGroups(filter)
	if err != nil {
		return nil, err
	}
	for _, principal := range principals {
		principal.MemberOf = ap.userMGR.IsMemberOf(token, principal)
	}
	return principals, nil
}

func (ap *Provider) newAzureClient(cfg *apiv3.AzureADConfig) (clients.AzureClient, error) {
	return clients.NewAzureClient(cfg, ap.secrets)
}

func (ap *Provider) saveAzureConfigK8s(config *apiv3.AzureADConfig) error {
	// Copy the annotations.
	annotations := config.Annotations
	storedAzureConfig, err := ap.GetAzureConfigK8s()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = client.AzureADConfigType
	config.ObjectMeta = storedAzureConfig.ObjectMeta

	// Ensure the passed in config's annotations are applied to the object to be persisted.
	if config.Annotations == nil {
		config.Annotations = map[string]string{}
	}
	for k, v := range annotations {
		config.Annotations[k] = v
	}

	field := strings.ToLower(client.AzureADConfigFieldApplicationSecret)
	name, err := common.CreateOrUpdateSecrets(ap.secrets, config.ApplicationSecret, field, strings.ToLower(config.Type))
	if err != nil {
		return err
	}

	config.ApplicationSecret = name

	logrus.Debugf("updating AzureADConfig")
	_, err = ap.authConfigs.ObjectClient().Update(config.Name, config)
	if err != nil {
		return err
	}
	return nil
}

func (ap *Provider) GetAzureConfigK8s() (*apiv3.AzureADConfig, error) {
	authConfigObj, err := ap.Retriever.Get(Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AzureADConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve AzureADConfig, cannot read k8s Unstructured data")
	}
	storedAzureADConfigMap := u.UnstructuredContent()

	storedAzureADConfig := &apiv3.AzureADConfig{}
	err = common.Decode(storedAzureADConfigMap, storedAzureADConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode Azure Config: %w", err)
	}

	if storedAzureADConfig.ApplicationSecret != "" {
		value, err := common.ReadFromSecret(ap.secrets, storedAzureADConfig.ApplicationSecret,
			strings.ToLower(client.AzureADConfigFieldApplicationSecret))
		if err != nil {
			return nil, err
		}
		storedAzureADConfig.ApplicationSecret = value
	}

	return storedAzureADConfig, nil
}

func formAzureRedirectURL(config map[string]interface{}) string {
	var ac apiv3.AzureADConfig
	err := common.Decode(config, &ac)
	if err != nil {
		logrus.Warnf("error decoding AzureAD configuration: %v", err)
	}

	// Return the redirect URL for Microsoft Graph.
	return fmt.Sprintf(
		"%s?client_id=%s&redirect_uri=%s&response_type=code&scope=openid",
		ac.AuthEndpoint,
		ac.ApplicationID,
		ac.RancherURL,
	)
}

func (ap *Provider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []apiv3.Principal) (bool, error) {
	cfg, err := ap.GetAzureConfigK8s()
	if err != nil {
		logrus.Errorf("Error fetching azure config: %v", err)
		return false, err
	}
	allowed, err := ap.userMGR.CheckAccess(cfg.AccessMode, cfg.AllowedPrincipalIDs, userPrincipalID, groupPrincipals)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

// UpdateGroupCacheSize attempts to update the size of the group cache defined at the package level.
func UpdateGroupCacheSize(size string) {
	if size == "" {
		return
	}

	i, err := strconv.Atoi(size)
	if err != nil {
		logrus.Errorf("Error parsing azure-group-cache-size, skipping update %v", err)
		return
	}
	if i < 0 {
		logrus.Error("Azure-group-cache-size must be >= 0, skipping update")
		return
	}
	clients.GroupCache.Resize(i)
}

func (ap *Provider) GetUserExtraAttributes(userPrincipal apiv3.Principal) map[string][]string {
	return common.GetCommonUserExtraAttributes(userPrincipal)
}

// IsDisabledProvider checks if the Azure AD auth provider is currently disabled in Rancher.
func (ap *Provider) IsDisabledProvider() (bool, error) {
	azureConfig, err := ap.GetAzureConfigK8s()
	if err != nil {
		return false, err
	}
	return !azureConfig.Enabled, nil
}

// setIDTokenCookie writes the Azure AD ID token to the R_AZUREAD_ID cookie.
func setIDTokenCookie(req *http.Request, w http.ResponseWriter, token string) {
	if len(token) > maxSafeCookieSize {
		logrus.Warnf("[AZURE_PROVIDER] ID token is %d bytes, may exceed browser cookie limit; SSO logout will fall back to client_id only", len(token))
	}
	isSecure := req.URL.Scheme == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     IDTokenCookie,
		Value:    token,
		Secure:   isSecure,
		Path:     "/",
		HttpOnly: true,
	})
}

// getIDTokenCookie retrieves the Azure AD ID token from the R_AZUREAD_ID cookie.
func getIDTokenCookie(req *http.Request) string {
	cookie, err := req.Cookie(IDTokenCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}
