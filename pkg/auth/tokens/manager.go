package tokens

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/rancher/rancher/pkg/catalog/utils"
	clientv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// TODO Cleanup error logging. If error is being returned, use errors.wrap to return and dont log here

const (
	userPrincipalIndex     = "authn.management.cattle.io/user-principal-index"
	UserIDLabel            = "authn.management.cattle.io/token-userId"
	TokenKindLabel         = "authn.management.cattle.io/kind"
	TokenHashed            = "authn.management.cattle.io/token-hashed"
	tokenKeyIndex          = "authn.management.cattle.io/token-key-index"
	secretNameEnding       = "-secret"
	SecretNamespace        = "cattle-system"
	KubeconfigResponseType = "kubeconfig"
)

var (
	toDeleteCookies = []string{CookieName, CSRFCookie}
)

func RegisterIndexer(apiContext *config.ScaledContext) error {
	informer := apiContext.Management.Users("").Controller().Informer()
	return informer.AddIndexers(map[string]cache.IndexFunc{userPrincipalIndex: userPrincipalIndexer})
}

func NewManager(ctx context.Context, apiContext *config.ScaledContext) *Manager {
	informer := apiContext.Management.Users("").Controller().Informer()
	tokenInformer := apiContext.Management.Tokens("").Controller().Informer()

	return &Manager{
		ctx:                 ctx,
		tokensClient:        apiContext.Management.Tokens(""),
		userIndexer:         informer.GetIndexer(),
		tokenIndexer:        tokenInformer.GetIndexer(),
		userAttributes:      apiContext.Management.UserAttributes(""),
		userAttributeLister: apiContext.Management.UserAttributes("").Controller().Lister(),
		userLister:          apiContext.Management.Users("").Controller().Lister(),
		secrets:             apiContext.Core.Secrets(""),
		secretLister:        apiContext.Core.Secrets("").Controller().Lister(),
	}
}

type Manager struct {
	ctx                 context.Context
	tokensClient        v3.TokenInterface
	userAttributes      v3.UserAttributeInterface
	userAttributeLister v3.UserAttributeLister
	userIndexer         cache.Indexer
	tokenIndexer        cache.Indexer
	userLister          v3.UserLister
	secrets             v1.SecretInterface
	secretLister        v1.SecretLister
}

func userPrincipalIndexer(obj interface{}) ([]string, error) {
	user, ok := obj.(*v3.User)
	if !ok {
		return []string{}, nil
	}

	return user.PrincipalIDs, nil
}

// createDerivedToken will create a jwt token for the authenticated user
func (m *Manager) createDerivedToken(jsonInput clientv3.Token, tokenAuthValue string) (v3.Token, string, int, error) {
	logrus.Debug("Create Derived Token Invoked")

	token, _, err := m.getToken(tokenAuthValue)
	if err != nil {
		return v3.Token{}, "", 401, err
	}

	tokenTTL, err := ClampToMaxTTL(time.Duration(int64(jsonInput.TTLMillis)) * time.Millisecond)
	if err != nil {
		return v3.Token{}, "", 500, fmt.Errorf("error validating max-ttl %v", err)
	}

	var unhashedTokenKey string
	derivedToken := v3.Token{
		UserPrincipal: token.UserPrincipal,
		IsDerived:     true,
		TTLMillis:     tokenTTL.Milliseconds(),
		UserID:        token.UserID,
		AuthProvider:  token.AuthProvider,
		ProviderInfo:  token.ProviderInfo,
		Description:   jsonInput.Description,
		ClusterName:   jsonInput.ClusterID,
	}
	derivedToken, unhashedTokenKey, err = m.createToken(&derivedToken)

	return derivedToken, unhashedTokenKey, 0, err

}

// createToken returns the token object and it's unhashed token key, which is stored hashed
func (m *Manager) createToken(k8sToken *v3.Token) (v3.Token, string, error) {
	key, err := randomtoken.Generate()
	if err != nil {
		logrus.Errorf("Failed to generate token key: %v", err)
		return v3.Token{}, "", errors.New("failed to generate token key")
	}

	if k8sToken.ObjectMeta.Labels == nil {
		k8sToken.ObjectMeta.Labels = make(map[string]string)
	}
	k8sToken.APIVersion = "management.cattle.io/v3"
	k8sToken.Kind = "Token"
	k8sToken.Token = key
	k8sToken.ObjectMeta.Labels[UserIDLabel] = k8sToken.UserID
	k8sToken.ObjectMeta.GenerateName = "token-"
	err = ConvertTokenKeyToHash(k8sToken)
	if err != nil {
		return v3.Token{}, "", err
	}
	createdToken, err := m.tokensClient.Create(k8sToken)

	if err != nil {
		return v3.Token{}, "", err
	}

	return *createdToken, key, nil
}

func (m *Manager) updateToken(token *v3.Token) (*v3.Token, error) {
	return m.tokensClient.Update(token)
}

func (m *Manager) getToken(tokenAuthValue string) (*v3.Token, int, error) {
	tokenName, tokenKey := SplitTokenParts(tokenAuthValue)

	lookupUsingClient := false

	objs, err := m.tokenIndexer.ByIndex(tokenKeyIndex, tokenKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			lookupUsingClient = true
		} else {
			return nil, 404, fmt.Errorf("failed to retrieve auth token from cache, error: %v", err)
		}
	} else if len(objs) == 0 {
		lookupUsingClient = true
	}

	var storedToken *v3.Token
	if lookupUsingClient {
		storedToken, err = m.tokensClient.Get(tokenName, metav1.GetOptions{})
		if err != nil {
			return nil, 404, fmt.Errorf("failed to retrieve auth token, error: %#v", err)
		}
	} else {
		storedToken = objs[0].(*v3.Token)
	}

	if code, err := VerifyToken(storedToken, tokenName, tokenKey); err != nil {
		return nil, code, err
	}

	return storedToken, 0, nil
}

// GetTokens will list all(login and derived, and even expired) tokens of the authenticated user
func (m *Manager) getTokens(tokenAuthValue string) ([]v3.Token, int, error) {
	logrus.Debug("LIST Tokens Invoked")
	tokens := make([]v3.Token, 0)

	storedToken, _, err := m.getToken(tokenAuthValue)
	if err != nil {
		return tokens, 401, err
	}

	userID := storedToken.UserID
	set := labels.Set(map[string]string{UserIDLabel: userID})
	tokenList, err := m.tokensClient.List(metav1.ListOptions{LabelSelector: set.AsSelector().String()})
	if err != nil {
		return tokens, 0, fmt.Errorf("error getting tokens for user: %v selector: %v  err: %v", userID, set.AsSelector().String(), err)
	}

	for _, t := range tokenList.Items {
		if IsExpired(t) {
			t.Expired = true
		}
		tokens = append(tokens, t)
	}
	return tokens, 0, nil
}

func (m *Manager) deleteToken(tokenAuthValue string) (int, error) {
	logrus.Debug("DELETE Token Invoked")

	storedToken, status, err := m.getToken(tokenAuthValue)
	if err != nil {
		if status == 404 {
			return 0, nil
		} else if status != 410 {
			return 401, err
		}
	}

	return m.deleteTokenByName(storedToken.Name)
}

func (m *Manager) deleteTokenByName(tokenName string) (int, error) {
	err := m.tokensClient.Delete(tokenName, &metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return 0, nil
		}
		return 500, fmt.Errorf("failed to delete token")
	}
	logrus.Debug("Deleted Token")
	return 0, nil
}

// getToken will get the token by ID
func (m *Manager) getTokenByID(tokenAuthValue string, tokenID string) (v3.Token, int, error) {
	logrus.Debug("GET Token Invoked")
	token := &v3.Token{}

	storedToken, _, err := m.getToken(tokenAuthValue)
	if err != nil {
		return *token, 401, err
	}

	token, err = m.tokensClient.Get(tokenID, metav1.GetOptions{})
	if err != nil {
		return v3.Token{}, 404, err
	}

	if token.UserID != storedToken.UserID {
		return v3.Token{}, 404, fmt.Errorf("%v not found", tokenID)
	}

	if IsExpired(*token) {
		token.Expired = true
	}

	return *token, 0, nil
}

func (m *Manager) deriveToken(request *types.APIContext) error {

	r := request.Request

	tokenAuthValue := GetTokenAuthFromRequest(r)
	if tokenAuthValue == "" {
		// no cookie or auth header, cannot authenticate
		return httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "No valid token cookie or auth header")
	}

	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%s", err))
	}

	jsonInput := clientv3.Token{}
	err = json.Unmarshal(bytes, &jsonInput)
	if err != nil {
		return httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("%s", err))
	}

	// create derived token
	token, unhashedTokenKey, status, err := m.createDerivedToken(jsonInput, tokenAuthValue)
	if err != nil {
		logrus.Errorf("deriveToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
	}

	tokenData, err := ConvertTokenResource(request.Schema, token)
	if err != nil {
		return err
	}
	tokenData["token"] = token.ObjectMeta.Name + ":" + unhashedTokenKey

	request.WriteResponse(http.StatusCreated, tokenData)

	return nil
}

func (m *Manager) listTokens(request *types.APIContext) error {
	r := request.Request

	// TODO switch to X-API-UserId header
	tokenAuthValue := GetTokenAuthFromRequest(r)
	if tokenAuthValue == "" {
		// no cookie or auth header, cannot authenticate
		return httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "No valid token cookie or auth header")
	}
	//getToken
	tokens, status, err := m.getTokens(tokenAuthValue)
	if err != nil {
		logrus.Errorf("GetToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
	}

	currentAuthToken, _, err := m.getToken(tokenAuthValue)
	if err != nil {
		return err
	}

	tokensFromStore := make([]map[string]interface{}, len(tokens))
	for _, token := range tokens {
		token.Current = currentAuthToken.Name == token.Name && !currentAuthToken.IsDerived
		tokenData, err := ConvertTokenResource(request.Schema, token)
		if err != nil {
			return err
		}

		tokensFromStore = append(tokensFromStore, tokenData)
	}

	request.WriteResponse(http.StatusOK, tokensFromStore)
	return nil
}

func (m *Manager) logout(actionName string, action *types.Action, request *types.APIContext) error {
	r := request.Request
	w := request.Response

	tokenAuthValue := GetTokenAuthFromRequest(r)
	if tokenAuthValue == "" {
		// no cookie or auth header, cannot authenticate
		return httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "No valid token cookie or auth header")
	}

	isSecure := false
	if r.URL.Scheme == "https" {
		isSecure = true
	}

	for _, cookieName := range toDeleteCookies {
		tokenCookie := &http.Cookie{
			Name:     cookieName,
			Value:    "",
			Secure:   isSecure,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
			Expires:  time.Date(1982, time.February, 10, 23, 0, 0, 0, time.UTC),
		}
		http.SetCookie(w, tokenCookie)
	}
	w.Header().Add("Content-type", "application/json")

	//getToken
	status, err := m.deleteToken(tokenAuthValue)
	if err != nil {
		logrus.Errorf("DeleteToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
	}
	return nil
}

func (m *Manager) getTokenFromRequest(request *types.APIContext) error {
	// TODO switch to X-API-UserId header
	r := request.Request

	tokenAuthValue := GetTokenAuthFromRequest(r)
	if tokenAuthValue == "" {
		// no cookie or auth header, cannot authenticate
		return httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "No valid token cookie or auth header")
	}

	tokenID := request.ID

	currentAuthToken, _, err := m.getToken(tokenAuthValue)
	if err != nil {
		return err
	}

	//getToken
	token, status, err := m.getTokenByID(tokenAuthValue, tokenID)
	if err != nil {
		logrus.Errorf("GetToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		} else if status == 410 {
			status = http.StatusNotFound
		}
		return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
	}
	token.Current = currentAuthToken.Name == token.Name && !currentAuthToken.IsDerived
	tokenData, err := ConvertTokenResource(request.Schema, token)
	if err != nil {
		return err
	}
	request.WriteResponse(http.StatusOK, tokenData)
	return nil
}

func (m *Manager) removeToken(request *types.APIContext) error {
	// TODO switch to X-API-UserId header
	r := request.Request

	tokenAuthValue := GetTokenAuthFromRequest(r)
	if tokenAuthValue == "" {
		// no cookie or auth header, cannot authenticate
		return httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "No valid token cookie or auth header")
	}
	tokenID := request.ID

	//getToken
	t, status, err := m.getTokenByID(tokenAuthValue, tokenID)
	if err != nil {
		if status != 410 {
			logrus.Errorf("DeleteToken Failed to fetch the token to delete with error: %v", err)
			if status == 0 {
				status = http.StatusInternalServerError
			}
			return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
		}
	}

	currentAuthToken, _, err := m.getToken(tokenAuthValue)
	if err != nil {
		return err
	}

	if currentAuthToken.Name == t.Name && !currentAuthToken.IsDerived {
		return httperror.NewAPIErrorLong(http.StatusBadRequest, util.GetHTTPErrorCode(http.StatusBadRequest), "Cannot delete token for current session. Use logout instead")
	}

	if _, err := m.deleteTokenByName(t.Name); err != nil {
		return err
	}

	request.WriteResponse(http.StatusNoContent, nil)
	return nil
}

// CreateSecret saves the secret in k8s. Secret is saved under the userID-secret with
// key being the provider and data being the providers secret
func (m *Manager) CreateSecret(userID, provider, secret string) error {
	_, err := m.secretLister.Get(SecretNamespace, userID+secretNameEnding)
	// An error either means it already exists or something bad happened
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// The secret doesn't exist so create it
		data := make(map[string]string)
		data[provider] = secret

		s := apicorev1.Secret{
			StringData: data,
		}
		s.ObjectMeta = metav1.ObjectMeta{
			Name:      userID + secretNameEnding,
			Namespace: SecretNamespace,
		}
		_, err = m.secrets.Create(&s)
		return err
	}

	// No error means the secret already exists and needs to be updated
	return m.UpdateSecret(userID, provider, secret)
}

func (m *Manager) GetSecret(userID string, provider string, fallbackTokens []*v3.Token) (string, error) {
	cachedSecret, err := m.secretLister.Get(SecretNamespace, userID+secretNameEnding)
	if err != nil && !apierrors.IsNotFound(err) {
		return "", err
	}

	if (err == nil) && cachedSecret != nil && string(cachedSecret.Data[provider]) != "" {
		return string(cachedSecret.Data[provider]), nil
	}

	for _, token := range fallbackTokens {
		secret := token.ProviderInfo["access_token"]
		if secret != "" {
			return secret, nil
		}
	}

	return "", err // The not found error from above
}

func (m *Manager) UpdateSecret(userID, provider, secret string) error {
	cachedSecret, err := m.secretLister.Get(SecretNamespace, userID+secretNameEnding)
	if err != nil {
		return err
	}

	cachedSecret = cachedSecret.DeepCopy()

	cachedSecret.Data[provider] = []byte(secret)

	_, err = m.secrets.Update(cachedSecret)
	return err
}

func (m *Manager) EnsureAndGetUserAttribute(userID string) (*v3.UserAttribute, bool, error) {
	attribs, err := m.userAttributeLister.Get("", userID)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, false, err
	}

	if attribs == nil {
		attribs, err = m.userAttributes.Get(userID, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, false, err
		}
	}

	if attribs != nil && attribs.Name != "" {
		return attribs.DeepCopy(), false, nil
	}

	user, err := m.userLister.Get("", userID)
	if err != nil {
		return nil, false, err
	}

	attribs = &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: user.APIVersion,
					Kind:       user.Kind,
					UID:        user.UID,
					Name:       user.Name,
				},
			},
		},
		GroupPrincipals: map[string]v32.Principals{},
		ExtraByProvider: map[string]map[string][]string{},
		LastRefresh:     "",
		NeedsRefresh:    false,
	}

	return attribs, true, nil
}

func (m *Manager) UserAttributeCreateOrUpdate(userID, provider string, groupPrincipals []v3.Principal, userExtraInfo map[string][]string, loginTime ...time.Time) error {
	attribs, needCreate, err := m.EnsureAndGetUserAttribute(userID)
	if err != nil {
		return err
	}

	if attribs.GroupPrincipals == nil {
		attribs.GroupPrincipals = make(map[string]v32.Principals)
	}
	attribs.GroupPrincipals[provider] = v32.Principals{Items: groupPrincipals}

	if attribs.ExtraByProvider == nil {
		attribs.ExtraByProvider = make(map[string]map[string][]string)
	}
	if userExtraInfo == nil {
		userExtraInfo = make(map[string][]string)
	}
	attribs.ExtraByProvider[provider] = userExtraInfo

	var shouldUpdate bool
	if len(loginTime) > 0 && !loginTime[0].IsZero() {
		// Login time is truncated to seconds as the corresponding user label is set as epoch time.
		lastLogin := metav1.NewTime(loginTime[0].Truncate(time.Second))
		attribs.LastLogin = &lastLogin
		shouldUpdate = true
	}

	if needCreate {
		_, err = m.userAttributes.Create(attribs)
		if err != nil {
			return fmt.Errorf("failed to create UserAttribute: %w", err)
		}

		return nil
	}

	if shouldUpdate || m.userAttributeChanged(attribs, provider, userExtraInfo, groupPrincipals) {
		_, err = m.userAttributes.Update(attribs)
		if err != nil {
			return fmt.Errorf("failed to update UserAttribute: %w", err)
		}
	}

	return nil
}

func (m *Manager) userAttributeChanged(attribs *v32.UserAttribute, provider string, extraInfo map[string][]string, groupPrincipals []v32.Principal) bool {
	oldSet := []string{}
	newSet := []string{}

	if len(attribs.GroupPrincipals[provider].Items) != len(groupPrincipals) {
		return true
	}

	for _, principal := range attribs.GroupPrincipals[provider].Items {
		oldSet = append(oldSet, principal.ObjectMeta.Name)
	}
	for _, principal := range groupPrincipals {
		newSet = append(newSet, principal.ObjectMeta.Name)
	}
	sort.Strings(oldSet)
	sort.Strings(newSet)

	for i := range oldSet {
		if oldSet[i] != newSet[i] {
			return true
		}
	}

	if attribs.ExtraByProvider == nil && extraInfo != nil {
		return true
	}

	return !reflect.DeepEqual(attribs.ExtraByProvider[provider], extraInfo)
}

// PerUserCacheProviders is a set of provider names for which the token manager creates a per-user login token.
var PerUserCacheProviders = []string{"github", "azuread", "googleoauth", "oidc", "keycloakoidc"}

func (m *Manager) NewLoginToken(userID string, userPrincipal v3.Principal, groupPrincipals []v3.Principal, providerToken string, ttl int64, description string) (v3.Token, string, error) {
	provider := userPrincipal.Provider
	// Providers that use oauth need to create a secret for storing the access token.
	if utils.Contains(PerUserCacheProviders, provider) && providerToken != "" {
		err := m.CreateSecret(userID, provider, providerToken)
		if err != nil {
			return v3.Token{}, "", fmt.Errorf("unable to create secret: %s", err)
		}
	}

	token := &v3.Token{
		UserPrincipal: userPrincipal,
		IsDerived:     false,
		TTLMillis:     ttl,
		UserID:        userID,
		AuthProvider:  provider,
		Description:   description,
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				TokenKindLabel: "session",
			},
		},
	}

	return m.createToken(token)
}

func (m *Manager) UpdateToken(token *v3.Token) (*v3.Token, error) {
	return m.updateToken(token)
}

func (m *Manager) GetGroupsForTokenAuthProvider(token *v3.Token) []v3.Principal {
	var groups []v3.Principal

	attribs, err := m.userAttributeLister.Get("", token.UserID)
	if err != nil && !apierrors.IsNotFound(err) {
		logrus.Warnf("Problem getting userAttribute while getting groups for %v: %v", token.UserID, err)
		// if err is not nil, then attribs will be. So, below code will handle it
	}

	hitProvider := false
	if attribs != nil {
		for provider, y := range attribs.GroupPrincipals {
			if provider == token.AuthProvider {
				hitProvider = true
				groups = append(groups, y.Items...)
			}
		}
	}

	// fallback to legacy token groupPrincipals
	if !hitProvider {
		groups = append(groups, token.GroupPrincipals...)
	}

	return groups
}

func (m *Manager) IsMemberOf(token v3.Token, group v3.Principal) bool {
	attribs, err := m.userAttributeLister.Get("", token.UserID)
	if err != nil && !apierrors.IsNotFound(err) {
		logrus.Warnf("Problem getting userAttribute while determining group membership for %v in %v (%v): %v", token.UserID,
			group.Name, group.DisplayName, err)
		// if err not nil, then attribs will be nil. So, below code will handle it
	}

	groups := map[string]bool{}
	hitProviders := map[string]bool{}
	if attribs != nil {
		for provider, gps := range attribs.GroupPrincipals {
			for _, principal := range gps.Items {
				hitProviders[provider] = true
				groups[principal.Name] = true
			}
		}
	}

	// fallback to legacy token groupPrincipals
	if _, ok := hitProviders[token.AuthProvider]; !ok {
		for _, principal := range token.GroupPrincipals {
			groups[principal.Name] = true
		}
	}

	return groups[group.Name]
}

func (m *Manager) CreateTokenAndSetCookie(userID string, userPrincipal v3.Principal, groupPrincipals []v3.Principal, providerToken string, ttl int, description string, request *types.APIContext) error {
	token, unhashedTokenKey, err := m.NewLoginToken(userID, userPrincipal, groupPrincipals, providerToken, 0, description)
	if err != nil {
		logrus.Errorf("Failed creating token with error: %v", err)
		return httperror.NewAPIErrorLong(500, "", fmt.Sprintf("Failed creating token with error: %v", err))
	}

	isSecure := false
	if request.Request.URL.Scheme == "https" {
		isSecure = true
	}

	tokenCookie := &http.Cookie{
		Name:     CookieName,
		Value:    token.ObjectMeta.Name + ":" + unhashedTokenKey,
		Secure:   isSecure,
		Path:     "/",
		HttpOnly: true,
	}
	http.SetCookie(request.Response, tokenCookie)
	request.WriteResponse(http.StatusOK, nil)

	return nil
}

// TokenStreamTransformer only filters out data for tokens that do not belong to the user
func (m *Manager) TokenStreamTransformer(
	apiContext *types.APIContext,
	schema *types.Schema,
	data chan map[string]interface{},
	opt *types.QueryOptions) (chan map[string]interface{}, error) {
	logrus.Debug("TokenStreamTransformer called")
	tokenAuthValue := GetTokenAuthFromRequest(apiContext.Request)
	if tokenAuthValue == "" {
		// no cookie or auth header, cannot authenticate
		return nil, httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "[TokenStreamTransformer] failed: No valid token cookie or auth header")
	}

	storedToken, code, err := m.getToken(tokenAuthValue)
	if err != nil {
		return nil, httperror.NewAPIErrorLong(code, http.StatusText(code), fmt.Sprintf("[TokenStreamTransformer] failed: %s", err.Error()))
	}

	userID := storedToken.UserID

	return convert.Chan(data, func(data map[string]interface{}) map[string]interface{} {
		labels, _ := data["labels"].(map[string]interface{})
		if labels[UserIDLabel] != userID {
			return nil
		}
		return data
	}), nil
}

// ParseTokenTTL parses an integer representing minutes as a string and returns its duration.
func ParseTokenTTL(ttl string) (time.Duration, error) {
	durString := fmt.Sprintf("%vm", ttl)
	dur, err := time.ParseDuration(durString)
	if err != nil {
		return 0, fmt.Errorf("error parsing token ttl: %v", err)
	}
	return dur, nil
}

// ClampToMaxTTL will return the duration of the provided TTL or the duration of settings.AuthTokenMaxTTLMinutes whichever is smaller.
func ClampToMaxTTL(ttl time.Duration) (time.Duration, error) {
	maxTTL, err := ParseTokenTTL(settings.AuthTokenMaxTTLMinutes.Get())
	if err != nil {
		return 0, fmt.Errorf("failed to parse setting '%s': %w", settings.AuthTokenMaxTTLMinutes.Name, err)
	}
	if maxTTL == 0 {
		return ttl, nil
	}
	if ttl == 0 {
		return maxTTL, nil
	}
	// return min(ttl, maxTTL)
	if ttl <= maxTTL {
		return ttl, nil
	}
	return maxTTL, nil
}

// GetKubeconfigDefaultTokenTTLInMilliSeconds will return the default TTL for kubeconfig tokens
func GetKubeconfigDefaultTokenTTLInMilliSeconds() (*int64, error) {
	defaultTokenTTL, err := ParseTokenTTL(settings.KubeconfigDefaultTokenTTLMinutes.Get())
	if err != nil {
		return nil, fmt.Errorf("failed to parse setting '%s': %w", settings.KubeconfigDefaultTokenTTLMinutes.Name, err)
	}

	tokenTTL, err := ClampToMaxTTL(defaultTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token ttl: %w", err)
	}
	ttlMilli := tokenTTL.Milliseconds()
	return &ttlMilli, nil
}
