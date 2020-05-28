package tokens

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/auth/util"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	clientv3 "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/wrangler/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

// TODO Cleanup error logging. If error is being returned, use errors.wrap to return and dont log here

const (
	userPrincipalIndex = "authn.management.cattle.io/user-principal-index"
	UserIDLabel        = "authn.management.cattle.io/token-userId"
	TokenKindLabel     = "authn.management.cattle.io/kind"
	tokenKeyIndex      = "authn.management.cattle.io/token-key-index"
	secretNameEnding   = "-secret"
	secretNamespace    = "cattle-system"
)

var (
	toDeleteCookies = []string{CookieName, CSRFCookie}
)

func NewManager(ctx context.Context, apiContext *config.ScaledContext) *Manager {
	informer := apiContext.Management.Users("").Controller().Informer()
	informer.AddIndexers(map[string]cache.IndexFunc{userPrincipalIndex: userPrincipalIndexer})

	tokenInformer := apiContext.Management.Tokens("").Controller().Informer()
	tokenInformer.AddIndexers(map[string]cache.IndexFunc{tokenKeyIndex: tokenKeyIndexer})

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

func tokenKeyIndexer(obj interface{}) ([]string, error) {
	token, ok := obj.(*v3.Token)
	if !ok {
		return []string{}, nil
	}

	return []string{token.Token}, nil
}

// createDerivedToken will create a jwt token for the authenticated user
func (m *Manager) createDerivedToken(jsonInput clientv3.Token, tokenAuthValue string) (v3.Token, int, error) {
	logrus.Debug("Create Derived Token Invoked")

	token, _, err := m.getToken(tokenAuthValue)
	if err != nil {
		return v3.Token{}, 401, err
	}

	derivedToken := v3.Token{
		UserPrincipal: token.UserPrincipal,
		IsDerived:     true,
		TTLMillis:     jsonInput.TTLMillis,
		UserID:        token.UserID,
		AuthProvider:  token.AuthProvider,
		ProviderInfo:  token.ProviderInfo,
		Description:   jsonInput.Description,
		ClusterName:   jsonInput.ClusterID,
	}
	derivedToken, err = m.createToken(&derivedToken)

	return derivedToken, 0, err

}

func (m *Manager) createToken(k8sToken *v3.Token) (v3.Token, error) {
	key, err := randomtoken.Generate()
	if err != nil {
		logrus.Errorf("Failed to generate token key: %v", err)
		return v3.Token{}, fmt.Errorf("failed to generate token key")
	}

	if k8sToken.ObjectMeta.Labels == nil {
		k8sToken.ObjectMeta.Labels = make(map[string]string)
	}
	k8sToken.APIVersion = "management.cattle.io/v3"
	k8sToken.Kind = "Token"
	k8sToken.Token = key
	k8sToken.ObjectMeta.Labels[UserIDLabel] = k8sToken.UserID
	k8sToken.ObjectMeta.GenerateName = "token-"
	createdToken, err := m.tokensClient.Create(k8sToken)

	if err != nil {
		return v3.Token{}, err
	}

	return *createdToken, nil
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

	storedToken := &v3.Token{}
	if lookupUsingClient {
		storedToken, err = m.tokensClient.Get(tokenName, metav1.GetOptions{})
		if err != nil {
			return nil, 404, fmt.Errorf("failed to retrieve auth token, error: %#v", err)
		}
	} else {
		storedToken = objs[0].(*v3.Token)
	}

	if storedToken.Token != tokenKey || storedToken.ObjectMeta.Name != tokenName {
		return nil, 422, fmt.Errorf("Invalid auth token value")
	}

	if IsExpired(*storedToken) {
		return storedToken, 410, fmt.Errorf("Auth Token has expired")
	}

	return storedToken, 0, nil
}

//GetTokens will list all(login and derived, and even expired) tokens of the authenticated user
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

//getToken will get the token by ID
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

	var token v3.Token
	var status int

	// create derived token
	token, status, err = m.createDerivedToken(jsonInput, tokenAuthValue)
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
	tokenData["token"] = token.ObjectMeta.Name + ":" + token.Token

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
	_, err := m.secretLister.Get(secretNamespace, userID+secretNameEnding)
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
			Namespace: secretNamespace,
		}
		_, err = m.secrets.Create(&s)
		return err
	}

	// No error means the secret already exists and needs to be updated
	return m.UpdateSecret(userID, provider, secret)
}

func (m *Manager) GetSecret(userID string, provider string, fallbackTokens []*v3.Token) (string, error) {
	cachedSecret, err := m.secretLister.Get(secretNamespace, userID+secretNameEnding)
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
	cachedSecret, err := m.secretLister.Get(secretNamespace, userID+secretNameEnding)
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
		GroupPrincipals: map[string]v3.Principals{},
		UserPrincipal:   v3.Principal{},
		LastRefresh:     "",
		NeedsRefresh:    false,
	}

	return attribs, true, nil
}

func (m *Manager) UserAttributeCreateOrUpdate(userID, provider string, userPrincipal v3.Principal, groupPrincipals []v3.Principal) error {
	attribs, needCreate, err := m.EnsureAndGetUserAttribute(userID)
	if err != nil {
		return err
	}

	if needCreate {
		updateAttribs(attribs, provider, userPrincipal, groupPrincipals)
		_, err := m.userAttributes.Create(attribs)
		if err != nil {
			return err
		}
		return nil
	}

	// Exists, just update if necessary
	if m.UserAttributeChanged(attribs, provider, userPrincipal, groupPrincipals) {
		updateAttribs(attribs, provider, userPrincipal, groupPrincipals)
		_, err := m.userAttributes.Update(attribs)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateAttribs(attribs *v3.UserAttribute, provider string, userPrincipal v3.Principal, groupPrincipals []v3.Principal) {
	attribs.GroupPrincipals[provider] = v3.Principals{Items: groupPrincipals}
	attribs.UserPrincipal = userPrincipal
	attribs.UserName = userPrincipal.DisplayName
}

func (m *Manager) UserAttributeChanged(attribs *v3.UserAttribute, provider string, userPrincipal v3.Principal, groupPrincipals []v3.Principal) bool {
	oldSet := []string{}
	newSet := []string{}
	for _, principal := range attribs.GroupPrincipals[provider].Items {
		oldSet = append(oldSet, principal.ObjectMeta.Name)
	}
	for _, principal := range groupPrincipals {
		newSet = append(newSet, principal.ObjectMeta.Name)
	}
	sort.Strings(oldSet)
	sort.Strings(newSet)

	if len(oldSet) != len(newSet) {
		return true
	}

	for i := range oldSet {
		if oldSet[i] != newSet[i] {
			return true
		}
	}
	if !reflect.DeepEqual(attribs.UserPrincipal, userPrincipal) {
		return true
	}
	if attribs.UserName != userPrincipal.DisplayName {
		return true
	}
	return false
}

var uaBackoff = wait.Backoff{
	Duration: time.Millisecond * 100,
	Factor:   2,
	Jitter:   .2,
	Steps:    5,
}

func (m *Manager) NewLoginToken(userID string, userPrincipal v3.Principal, groupPrincipals []v3.Principal, providerToken string, ttl int64, description string) (v3.Token, error) {
	provider := userPrincipal.Provider
	if (provider == "github" || provider == "azuread" || provider == "googleoauth") && providerToken != "" {
		err := m.CreateSecret(userID, provider, providerToken)
		if err != nil {
			return v3.Token{}, fmt.Errorf("unable to create secret: %s", err)
		}
	}

	err := wait.ExponentialBackoff(uaBackoff, func() (bool, error) {
		err := m.UserAttributeCreateOrUpdate(userID, provider, userPrincipal, groupPrincipals)
		if err != nil {
			logrus.Warnf("Problem creating or updating userAttribute for %v: %v", userID, err)
		}
		return err == nil, nil
	})

	if err != nil {
		return v3.Token{}, fmt.Errorf("Unable to create userAttribute")
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
				for _, principal := range y.Items {
					groups = append(groups, principal)
				}
			}
		}
	}

	// fallback to legacy token groupPrincipals
	if !hitProvider {
		for _, principal := range token.GroupPrincipals {
			groups = append(groups, principal)
		}
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
	token, err := m.NewLoginToken(userID, userPrincipal, groupPrincipals, providerToken, 0, description)
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
		Value:    token.ObjectMeta.Name + ":" + token.Token,
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
