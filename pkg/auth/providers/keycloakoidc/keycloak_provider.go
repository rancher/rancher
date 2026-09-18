package keycloakoidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	authsecrets "github.com/rancher/rancher/pkg/auth/api/secrets"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	ldapprovider "github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
)

const (
	Name      = "keycloakoidc"
	UserType  = "user"
	GroupType = "group"
)

type keyCloakOIDCProvider struct {
	oidc.OpenIDCProvider
	ldapProvider common.AuthProvider
}

type keyCloakOIDCConfigInput struct {
	apiv3.OIDCConfig `json:",inline"`
	OpenLdapConfig   *apiv3.LdapFields `json:"openLdapConfig,omitempty"`
}

type keyCloakOIDCApplyInput struct {
	OIDCConfig keyCloakOIDCConfigInput `json:"oidcConfig,omitempty"`
	Code       string                  `json:"code,omitempty"`
	Enabled    bool                    `json:"enabled,omitempty"`
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	p := &keyCloakOIDCProvider{
		OpenIDCProvider: oidc.OpenIDCProvider{
			Name:        Name,
			Type:        client.KeyCloakOIDCConfigType,
			CTX:         ctx,
			AuthConfigs: mgmtCtx.Management.AuthConfigs(""),
			Secrets:     mgmtCtx.Wrangler.Core.Secret(),
			UserMGR:     userMGR,
			TokenMgr:    tokenMGR,
		},
	}
	p.ldapProvider = ldapprovider.Configure(mgmtCtx, userMGR, tokenMGR, Name)

	p.GetConfig = p.GetOIDCConfig
	return p
}

func (k *keyCloakOIDCProvider) GetName() string {
	return Name
}

func (k *keyCloakOIDCProvider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = k.ActionHandler
	schema.Formatter = k.Formatter
}

func (k *keyCloakOIDCProvider) ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	handled, err := common.HandleCommonAction(actionName, action, request, k.Name, k.AuthConfigs)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	switch actionName {
	case "configureTest":
		return k.ConfigureTest(request)
	case "testAndApply":
		return k.TestAndApply(request)
	default:
		return httperror.NewAPIError(httperror.ActionNotAvailable, "")
	}
}

func (k *keyCloakOIDCProvider) TestAndApply(request *types.APIContext) error {
	oidcConfigApplyInput := &keyCloakOIDCApplyInput{}
	if err := json.NewDecoder(request.Request.Body).Decode(oidcConfigApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("[keycloak oidc] testAndApply: failed to parse body: %v", err))
	}

	oidcConfig := apiv3.KeyCloakOIDCConfig{
		OIDCConfig: oidcConfigApplyInput.OIDCConfig.OIDCConfig,
	}
	ldapConfigProvided := oidcConfigApplyInput.OIDCConfig.OpenLdapConfig != nil
	if ldapConfigProvided {
		oidcConfig.OpenLdapConfig = *oidcConfigApplyInput.OIDCConfig.OpenLdapConfig
	}
	if !validateScopes(oidcConfig.Scopes) {
		return fmt.Errorf("scopes are invalid: scopes must be space delimited and openid is a required scope. %s", oidcConfig.Scopes)
	}

	issuerURL, err := url.Parse(oidcConfig.Issuer)
	if err != nil {
		if httperror.IsAPIError(err) {
			return err
		}
		return fmt.Errorf("[keycloak oidc]: failed to parse the issuer URL while authenticating: %w", err)
	}
	if issuerURL.Scheme == "" || issuerURL.Host == "" {
		return fmt.Errorf("[keycloak oidc]: issuer must be an absolute URL")
	}
	oidcConfig.Issuer = issuerURL.String()
	if oidcConfig.Scopes == "" {
		storedConfig, err := k.getKeyCloakOIDCConfig()
		if err == nil {
			k.mergeStoredConfigDefaults(&oidcConfig, storedConfig, ldapConfigProvided)
		}
	}

	oidcLogin := &apiv3.OIDCLogin{Code: oidcConfigApplyInput.Code}
	userPrincipal, groupPrincipals, providerToken, _, err := k.LoginUser(
		request.Response, request.Request,
		oidcLogin, &oidcConfig.OIDCConfig)
	if err != nil {
		if httperror.IsAPIError(err) {
			return err
		}
		return fmt.Errorf("[keycloak oidc]: server error while authenticating: %w", err)
	}
	user, err := k.UserMGR.SetPrincipalOnCurrentUser(request.Request, userPrincipal)
	if err != nil {
		return err
	}

	if err := k.saveKeyCloakOIDCConfig(&oidcConfig, ldapConfigProvided); err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("[keycloak oidc]: failed to save oidc config: %v", err))
	}

	userExtraInfo := k.GetUserExtraAttributes(userPrincipal)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return k.UserMGR.UserAttributeCreateOrUpdate(user.Name, userPrincipal.Provider, groupPrincipals, userExtraInfo)
	}); err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("[keycloak oidc]: Failed to create or update userAttribute: %v", err))
	}

	return k.TokenMgr.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerToken, 0, "Token via OIDC Configuration", request)
}

func (k *keyCloakOIDCProvider) newClient(config *apiv3.OIDCConfig, token accessor.TokenAccessor) (*KeyCloakClient, error) {
	// creating context for new client and for refreshing oauth token if needed
	ctx, err := oidc.AddCertKeyToContext(context.Background(), config.Certificate, config.PrivateKey)
	if err != nil {
		return nil, err
	}
	provider, err := gooidc.NewProvider(ctx, config.Issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create new oidc provider: %w", err)
	}

	oauthConfig := oidc.ConfigToOauthConfig(provider.Endpoint(), config)
	var oauthToken *oauth2.Token
	if config.ClientAuthenticatedSearch {
		token, err := k.getClientCredentialsToken(ctx, provider, config)
		if err != nil {
			return nil, fmt.Errorf("failed to get client credentials: %w", err)
		}
		oauthToken = token
	} else {
		// get, refresh and update token
		token, err := k.getRefreshAndUpdateToken(ctx, oauthConfig, token)
		if err != nil {
			return nil, err
		}
		oauthToken = token

	}
	keyCloakClient := &KeyCloakClient{
		httpClient: oauthConfig.Client(ctx, oauthToken),
	}

	return keyCloakClient, err
}

func (k *keyCloakOIDCProvider) SearchPrincipals(searchValue, principalType string, token accessor.TokenAccessor) ([]apiv3.Principal, error) {
	var principals []apiv3.Principal

	if principalType == "" || principalType == UserType {
		userPrincipals, err := k.searchKeycloakPrincipals(searchValue, UserType, token)
		if err != nil {
			return principals, err
		}
		principals = append(principals, userPrincipals...)
	}

	if principalType == "" || principalType == GroupType {
		groupPrincipals, err := k.searchGroupPrincipals(searchValue, token)
		if err != nil {
			return principals, err
		}
		principals = append(principals, groupPrincipals...)
	}

	return principals, nil
}

// TransformToAuthProvider yields information used, typically by the UI, to be able to form URLs used to perform login.
func (k *keyCloakOIDCProvider) TransformToAuthProvider(authConfig map[string]any) (map[string]any, error) {
	p, err := k.OpenIDCProvider.TransformToAuthProvider(authConfig)
	if err != nil {
		return nil, err
	}

	p[publicclient.KeyCloakOIDCProviderFieldScopes] = authConfig["scope"]

	return p, nil
}

func (k *keyCloakOIDCProvider) toPrincipal(principalType string, acct account, token accessor.TokenAccessor) apiv3.Principal {
	displayName := acct.Name
	if displayName == "" {
		displayName = acct.Username
	}
	princ := apiv3.Principal{
		ObjectMeta:  metav1.ObjectMeta{Name: k.GetName() + "_" + principalType + "://" + acct.ID},
		DisplayName: displayName,
		LoginName:   acct.Username,
		Provider:    k.GetName(),
		Me:          false,
	}

	if principalType == UserType {
		princ.PrincipalType = UserType
		if token != nil {
			princ.Me = k.IsThisUserMe(token.GetUserPrincipal(), princ)
		}
	} else {
		princ.PrincipalType = GroupType
		princ.ObjectMeta = metav1.ObjectMeta{Name: k.GetName() + "_" + principalType + "://" + acct.Name}
		if token != nil {
			princ.MemberOf = k.UserMGR.IsMemberOf(token, princ)
		}
	}
	return princ
}

func (k *keyCloakOIDCProvider) GetPrincipal(principalID string, token accessor.TokenAccessor) (apiv3.Principal, error) {
	var externalID string
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return apiv3.Principal{}, fmt.Errorf("invalid id %v", principalID)
	}
	externalID = strings.TrimPrefix(parts[1], "//")
	parts = strings.SplitN(parts[0], "_", 2)
	if len(parts) != 2 {
		return apiv3.Principal{}, fmt.Errorf("invalid id %v", principalID)
	}
	principalType := parts[1]

	if principalType == GroupType {
		principal, found, err := k.getLDAPGroupPrincipal(externalID, token)
		if err != nil {
			return apiv3.Principal{}, err
		}
		if found {
			return principal, nil
		}
	}

	config, err := k.GetOIDCConfig()
	if err != nil {
		return apiv3.Principal{}, err
	}
	keyCloakClient, err := k.newClient(config, token)
	if err != nil {
		logrus.Warnf("[keycloak oidc] GetPrincipal: error creating new http client: %v", err)
		return apiv3.Principal{}, err
	}
	acct, err := keyCloakClient.getFromKeyCloakByID(externalID, principalType, config)
	if err != nil {
		return apiv3.Principal{}, err
	}
	princ := k.toPrincipal(principalType, acct, token)
	return princ, err
}

func (k *keyCloakOIDCProvider) GetOIDCConfig() (*apiv3.OIDCConfig, error) {
	cfg, err := k.getKeyCloakOIDCConfig()
	if err != nil {
		return nil, err
	}
	return &cfg.OIDCConfig, nil
}

func (k *keyCloakOIDCProvider) getKeyCloakOIDCConfig() (*apiv3.KeyCloakOIDCConfig, error) {
	authConfigObj, err := k.AuthConfigs.ObjectClient().UnstructuredClient().Get(k.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve OIDCConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve OIDCConfig, cannot read k8s Unstructured data")
	}
	storedConfigMap := u.UnstructuredContent()

	storedConfig := &apiv3.KeyCloakOIDCConfig{}
	if err := common.Decode(storedConfigMap, storedConfig); err != nil {
		return nil, fmt.Errorf("unable to decode OidcConfig: %w", err)
	}

	if storedConfig.PrivateKey != "" {
		value, err := common.ReadFromSecret(k.Secrets, storedConfig.PrivateKey, strings.ToLower(client.KeyCloakOIDCConfigFieldPrivateKey))
		if err != nil {
			return nil, err
		}
		storedConfig.PrivateKey = value
	}
	if storedConfig.ClientSecret != "" {
		data, err := common.ReadFromSecretData(k.Secrets, storedConfig.ClientSecret)
		if err != nil {
			return nil, err
		}
		for _, v := range data {
			storedConfig.ClientSecret = string(v)
		}
	}
	if strings.HasPrefix(storedConfig.OpenLdapConfig.ServiceAccountPassword, common.SecretsNamespace+":") {
		value, err := common.ReadFromSecret(k.Secrets, storedConfig.OpenLdapConfig.ServiceAccountPassword, strings.ToLower(client.LdapConfigFieldServiceAccountPassword))
		if err != nil {
			return nil, err
		}
		storedConfig.OpenLdapConfig.ServiceAccountPassword = value
	}

	return storedConfig, nil
}

func (k *keyCloakOIDCProvider) saveKeyCloakOIDCConfig(config *apiv3.KeyCloakOIDCConfig, ldapConfigProvided bool) error {
	storedConfig, err := k.getKeyCloakOIDCConfig()
	if err != nil {
		return err
	}
	k.mergeStoredConfigDefaults(config, storedConfig, ldapConfigProvided)
	if !validateScopes(config.Scopes) {
		return fmt.Errorf("scopes are invalid: scopes must be space delimited and openid is a required scope. %s", config.Scopes)
	}

	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = client.KeyCloakOIDCConfigType
	config.ObjectMeta = storedConfig.ObjectMeta
	if config.PrivateKey != "" {
		name, err := common.CreateOrUpdateSecrets(k.Secrets, config.PrivateKey, strings.ToLower(client.KeyCloakOIDCConfigFieldPrivateKey), strings.ToLower(config.Type))
		if err != nil {
			return err
		}
		config.PrivateKey = name
	}

	name, err := common.CreateOrUpdateSecrets(k.Secrets, config.ClientSecret, strings.ToLower(client.KeyCloakOIDCConfigFieldClientSecret), strings.ToLower(config.Type))
	if err != nil {
		return err
	}
	config.ClientSecret = name

	if ldapConfigProvided && config.OpenLdapConfig.ServiceAccountPassword == "" &&
		strings.HasPrefix(storedConfig.OpenLdapConfig.ServiceAccountPassword, common.SecretsNamespace+":") {
		if err := common.DeleteSecret(k.Secrets, config.Type, client.LdapConfigFieldServiceAccountPassword); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	if ldapConfigProvided && reflect.DeepEqual(config.OpenLdapConfig, apiv3.LdapFields{}) &&
		strings.HasPrefix(storedConfig.OpenLdapConfig.ServiceAccountPassword, common.SecretsNamespace+":") {
		if err := k.cleanupEmbeddedLDAPSecrets(config.Type); err != nil {
			return err
		}
		config.OpenLdapConfig = apiv3.LdapFields{}
	}
	if config.OpenLdapConfig.ServiceAccountPassword != "" {
		name, err := common.CreateOrUpdateSecrets(
			k.Secrets,
			config.OpenLdapConfig.ServiceAccountPassword,
			strings.ToLower(client.LdapConfigFieldServiceAccountPassword),
			strings.ToLower(config.Type),
		)
		if err != nil {
			return err
		}
		config.OpenLdapConfig.ServiceAccountPassword = name
	}

	logrus.Debugf("KeycloakOIDCProvider: saveKeyCloakOIDCConfig: updating config")
	_, err = k.AuthConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	return err
}

func (k *keyCloakOIDCProvider) searchKeycloakPrincipals(searchValue, principalType string, token accessor.TokenAccessor) ([]apiv3.Principal, error) {
	var principals []apiv3.Principal

	config, err := k.GetConfig()
	if err != nil {
		return principals, err
	}
	if principalType == GroupType && config.GroupSearchEnabled != nil && !*config.GroupSearchEnabled {
		return principals, nil
	}
	keyCloakClient, err := k.newClient(config, token)
	if err != nil {
		logrus.Errorf("[keycloak oidc] SearchPrincipals: error creating new http client: %v", err)
		return principals, err
	}
	accts, err := keyCloakClient.searchPrincipals(searchValue, principalType, config)
	if err != nil {
		logrus.Errorf("[keycloak oidc] SearchPrincipals: problem searching keycloak: %v", err)
		return principals, err
	}
	for _, acct := range accts {
		principals = append(principals, k.toPrincipal(acct.Type, acct, token))
	}
	return principals, nil
}

func (k *keyCloakOIDCProvider) searchGroupPrincipals(searchValue string, token accessor.TokenAccessor) ([]apiv3.Principal, error) {
	ldapGroups, err := k.searchLDAPGroupPrincipals(searchValue, token)
	if err != nil {
		return nil, err
	}

	keycloakGroups, err := k.searchKeycloakPrincipals(searchValue, GroupType, token)
	if err != nil {
		return nil, err
	}

	groups := append(ldapGroups, keycloakGroups...)
	return dedupePrincipals(groups), nil
}

func (k *keyCloakOIDCProvider) searchLDAPGroupPrincipals(searchValue string, token accessor.TokenAccessor) ([]apiv3.Principal, error) {
	principals, err := k.fetchLDAPGroupPrincipals(searchValue, token)
	if err != nil {
		return nil, err
	}

	normalized := make([]apiv3.Principal, 0, len(principals))
	for _, principal := range principals {
		groupName := strings.TrimSpace(principal.DisplayName)
		if groupName == "" {
			groupName = strings.TrimSpace(principal.LoginName)
		}
		if groupName == "" {
			continue
		}
		normalized = append(normalized, k.groupToPrincipal(groupName, token))
	}

	return dedupePrincipals(normalized), nil
}

func (k *keyCloakOIDCProvider) fetchLDAPGroupPrincipals(searchValue string, token accessor.TokenAccessor) ([]apiv3.Principal, error) {
	if k.ldapProvider == nil {
		return nil, nil
	}

	principals, err := k.ldapProvider.SearchPrincipals(searchValue, GroupType, token)
	if err != nil {
		if ldapprovider.IsNotConfigured(err) {
			return nil, nil
		}
		return nil, err
	}
	return principals, nil
}

func (k *keyCloakOIDCProvider) getLDAPGroupPrincipal(groupName string, token accessor.TokenAccessor) (apiv3.Principal, bool, error) {
	principals, err := k.fetchLDAPGroupPrincipals(groupName, token)
	if err != nil {
		return apiv3.Principal{}, false, err
	}

	matchSet := map[string]struct{}{}
	matched := false
	for _, principal := range principals {
		if principal.ObjectMeta.Name == k.GetName()+"_"+GroupType+"://"+groupName ||
			principal.DisplayName == groupName ||
			principal.LoginName == groupName {
			matched = true
			matchName := strings.TrimSpace(principal.DisplayName)
			if matchName == "" {
				matchName = strings.TrimSpace(principal.LoginName)
			}
			if matchName == "" {
				continue
			}
			matchSet[matchName] = struct{}{}
		}
	}
	if matched && len(matchSet) == 0 {
		return k.groupToPrincipal(groupName, token), true, nil
	}
	if len(matchSet) == 1 {
		for matchName := range matchSet {
			return k.groupToPrincipal(matchName, token), true, nil
		}
	}

	return apiv3.Principal{}, false, nil
}

func (k *keyCloakOIDCProvider) groupToPrincipal(groupName string, token accessor.TokenAccessor) apiv3.Principal {
	principal := apiv3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: k.GetName() + "_" + GroupType + "://" + groupName},
		DisplayName:   groupName,
		LoginName:     groupName,
		PrincipalType: GroupType,
		Provider:      k.GetName(),
	}
	if token != nil {
		principal.MemberOf = k.UserMGR.IsMemberOf(token, principal)
	}
	return principal
}

func dedupePrincipals(principals []apiv3.Principal) []apiv3.Principal {
	deduped := make([]apiv3.Principal, 0, len(principals))
	seen := make(map[string]struct{}, len(principals))
	for _, principal := range principals {
		if _, ok := seen[principal.ObjectMeta.Name]; ok {
			continue
		}
		seen[principal.ObjectMeta.Name] = struct{}{}
		deduped = append(deduped, principal)
	}
	return deduped
}

func validateScopes(input string) bool {
	if strings.Contains(input, ",") {
		return false
	}
	values := strings.Fields(input)
	return slices.Contains(values, "openid")
}

func (k *keyCloakOIDCProvider) mergeStoredConfigDefaults(config, storedConfig *apiv3.KeyCloakOIDCConfig, ldapConfigProvided bool) {
	if config.Scopes == "" {
		config.Scopes = storedConfig.Scopes
	}
	if config.GroupSearchEnabled == nil {
		config.GroupSearchEnabled = storedConfig.GroupSearchEnabled
	}
	if !ldapConfigProvided {
		config.OpenLdapConfig = storedConfig.OpenLdapConfig
	}
}

func (k *keyCloakOIDCProvider) cleanupEmbeddedLDAPSecrets(configType string) error {
	var result error
	if fieldsMap, ok := authsecrets.SubTypeToFields[configType]; ok {
		for _, field := range fieldsMap[client.KeyCloakOIDCConfigFieldOpenLdapConfig] {
			if err := common.DeleteSecret(k.Secrets, configType, field); err != nil && !apierrors.IsNotFound(err) {
				result = errors.Join(result, err)
			}
		}
	}
	return result
}

func (k *keyCloakOIDCProvider) getRefreshAndUpdateToken(ctx context.Context, oauthConfig oauth2.Config, token accessor.TokenAccessor) (*oauth2.Token, error) {
	var oauthToken *oauth2.Token
	storedOauthToken, err := k.TokenMgr.GetSecret(token.GetUserID(), token.GetAuthProvider(), []accessor.TokenAccessor{token})
	if err != nil {
		// If the secret lookup failed for a reason other than NotFound, surface the error.
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("getting access token for user: %w", err)
		}

		// Secret not found: fall back to the access token stored in ProviderInfo.
		accessToken, ok := token.GetProviderInfo()["access_token"]
		if !ok || strings.TrimSpace(accessToken) == "" {
			return nil, fmt.Errorf("no stored access token found for user %s", token.GetUserID())
		}
		oauthToken = &oauth2.Token{
			AccessToken: strings.TrimSpace(accessToken),
		}
	} else {
		// Secret retrieved successfully. First, try to interpret it as JSON-encoded oauth2.Token.
		stored := strings.TrimSpace(storedOauthToken)
		if stored == "" {
			return nil, fmt.Errorf("empty access token secret for user %s", token.GetUserID())
		}
		if unmarshalErr := json.Unmarshal([]byte(stored), &oauthToken); unmarshalErr != nil || oauthToken == nil {
			// If unmarshalling fails or yields nil, fall back to treating the secret as a raw access token string.
			oauthToken = &oauth2.Token{
				AccessToken: stored,
			}
		}
	}

	// Valid will return false if access token is expired
	if !oauthToken.Valid() {
		// since token is not valid, the TokenSource func used in the Client func will attempt to refresh the access token
		// if the refresh token has not expired
		logrus.Debugf("[generic oidc] RefreshAndUpdateToken: attempting to refresh access token")
	}

	reusedToken, err := oauth2.ReuseTokenSource(oauthToken, oauthConfig.TokenSource(ctx, oauthToken)).Token()
	if err != nil {
		// invalid_grant means the refresh token was revoked at the IdP (e.g. session
		// deleted in Keycloak). This won't resolve on retry, so surface it as
		// NonTransientError to stop the controller from requeuing.
		var retrieveErr *oauth2.RetrieveError
		if errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "invalid_grant" {
			return oauthToken, &common.NonTransientError{Err: fmt.Errorf("setting up reusable token: %w", err)}
		}
		return oauthToken, fmt.Errorf("setting up reusable token: %w", err)
	}

	if !reflect.DeepEqual(oauthToken, reusedToken) {
		if err := k.UpdateToken(reusedToken, token.GetUserID()); err != nil {
			logrus.Errorf("updating cached oauth token for user %s: %s", token.GetUserID(), err)
		}
	}

	return reusedToken, nil
}

func (k *keyCloakOIDCProvider) getClientCredentialsToken(ctx context.Context, provider *gooidc.Provider, config *apiv3.OIDCConfig) (*oauth2.Token, error) {
	var oauthToken *oauth2.Token
	secretExists := true
	storedOauthToken, err := k.TokenMgr.GetSecret(k.GetName(), k.GetName(), nil)
	if err != nil {
		if apierrors.IsNotFound(err) {
			secretExists = false
		} else {
			return nil, fmt.Errorf("finding oauth token for provider %s: %w", k.GetName(), err)
		}
	}
	if storedOauthToken != "" {
		if err := json.Unmarshal([]byte(storedOauthToken), &oauthToken); err != nil {
			return nil, fmt.Errorf("unmarshalling cached oauth token for provider %s: %w", k.GetName(), err)
		}
	}
	oauthConfig := oidc.ConfigToOauthConfig(provider.Endpoint(), config)
	clientConf := clientcredentials.Config{
		ClientID:     oauthConfig.ClientID,
		ClientSecret: oauthConfig.ClientSecret,
		TokenURL:     provider.Endpoint().TokenURL,
		AuthStyle:    oauth2.AuthStyleInParams,
		Scopes:       oauthConfig.Scopes,
	}
	if oauthToken != nil && (!oauthToken.Valid() || oauthToken.AccessToken == "") {
		logrus.Debugf("[keycloak oidc] RefreshAndUpdateToken: attempting to refresh access token from client credentials")
		tok, err := clientConf.Token(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch token: %w", err)
		}
		oauthToken = tok
	}

	reusedToken, err := oauth2.ReuseTokenSource(oauthToken, clientConf.TokenSource(ctx)).Token()
	if err != nil {
		return nil, fmt.Errorf("reusing the token source for provider %s: %w", k.GetName(), err)
	}

	if !reflect.DeepEqual(oauthToken, reusedToken) {
		if !secretExists {
			tokenBytes, err := json.Marshal(reusedToken)
			if err != nil {
				logrus.Errorf("marshalling oauth token for provider %s: %s", k.GetName(), err)
			} else {
				if err := k.TokenMgr.CreateSecret(k.GetName(), k.GetName(), string(tokenBytes)); err != nil {
					logrus.Errorf("creating cached oauth token for provider %s: %s", k.GetName(), err)
				}
			}
		} else {
			if err := k.UpdateToken(reusedToken, k.GetName()); err != nil {
				logrus.Errorf("updating cached oauth token for provider %s: %s", k.GetName(), err)
			}
		}
	}

	return reusedToken, nil
}
