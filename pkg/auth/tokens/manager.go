package tokens

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/rancher/rancher/pkg/randomtoken"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// TODO Cleanup error logging. If error is being returned, use errors.wrap to return and dont log here

const (
	userPrincipalIndex = "authn.management.cattle.io/user-principal-index"
	UserIDLabel        = "authn.management.cattle.io/token-userId"
	tokenKeyIndex      = "authn.management.cattle.io/token-key-index"
)

func NewManager(ctx context.Context, apiContext *config.ScaledContext) *Manager {
	informer := apiContext.Management.Users("").Controller().Informer()
	informer.AddIndexers(map[string]cache.IndexFunc{userPrincipalIndex: userPrincipalIndexer})
	tokenInformer := apiContext.Management.Tokens("").Controller().Informer()
	tokenInformer.AddIndexers(map[string]cache.IndexFunc{tokenKeyIndex: tokenKeyIndexer})
	return &Manager{
		ctx:          ctx,
		tokensClient: apiContext.Management.Tokens(""),
		userIndexer:  informer.GetIndexer(),
		tokenIndexer: tokenInformer.GetIndexer(),
	}
}

type Manager struct {
	ctx          context.Context
	tokensClient v3.TokenInterface
	userIndexer  cache.Indexer
	tokenIndexer cache.Indexer
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

//CreateDerivedToken will create a jwt token for the authenticated user
func (m *Manager) createDerivedToken(jsonInput v3.Token, tokenAuthValue string) (v3.Token, int, error) {

	logrus.Debug("Create Derived Token Invoked")

	token, _, err := m.getK8sTokenCR(tokenAuthValue)
	if err != nil {
		return v3.Token{}, 401, err
	}

	k8sToken := &v3.Token{
		UserPrincipal:   token.UserPrincipal,
		GroupPrincipals: token.GroupPrincipals,
		IsDerived:       true,
		TTLMillis:       jsonInput.TTLMillis,
		UserID:          token.UserID,
		AuthProvider:    token.AuthProvider,
		ProviderInfo:    token.ProviderInfo,
		Description:     jsonInput.Description,
	}
	rToken, err := m.createK8sTokenCR(k8sToken)

	return rToken, 0, err

}

func (m *Manager) createK8sTokenCR(k8sToken *v3.Token) (v3.Token, error) {
	key, err := randomtoken.Generate()
	if err != nil {
		logrus.Errorf("Failed to generate token key: %v", err)
		return v3.Token{}, fmt.Errorf("failed to generate token key")
	}

	labels := make(map[string]string)
	labels[UserIDLabel] = k8sToken.UserID

	k8sToken.APIVersion = "management.cattle.io/v3"
	k8sToken.Kind = "Token"
	k8sToken.Token = key
	k8sToken.ObjectMeta = metav1.ObjectMeta{
		GenerateName: "token-",
		Labels:       labels,
	}
	createdToken, err := m.tokensClient.Create(k8sToken)

	if err != nil {
		return v3.Token{}, err
	}

	return *createdToken, nil
}

func (m *Manager) updateK8sTokenCR(token *v3.Token) (*v3.Token, error) {
	return m.tokensClient.Update(token)
}

func (m *Manager) getK8sTokenCR(tokenAuthValue string) (*v3.Token, int, error) {
	tokenName, tokenKey := SplitTokenParts(tokenAuthValue)

	lookupUsingClient := false

	objs, err := m.tokenIndexer.ByIndex(tokenKeyIndex, tokenKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			lookupUsingClient = true
		} else {
			return nil, 0, fmt.Errorf("failed to retrieve auth token from cache, error: %v", err)
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
		return nil, 0, fmt.Errorf("Invalid auth token value")
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

	storedToken, _, err := m.getK8sTokenCR(tokenAuthValue)
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

	storedToken, status, err := m.getK8sTokenCR(tokenAuthValue)
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
		if e2, ok := err.(*errors.StatusError); ok && e2.Status().Code == 404 {
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

	storedToken, _, err := m.getK8sTokenCR(tokenAuthValue)
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
		logrus.Errorf("GetToken failed with error: %v", err)
	}
	jsonInput := v3.Token{}

	err = json.Unmarshal(bytes, &jsonInput)
	if err != nil {
		logrus.Errorf("unmarshal failed with error: %v", err)
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

	tokensFromStore := make([]map[string]interface{}, len(tokens))
	for _, token := range tokens {
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

	tokenCookie := &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Secure:   isSecure,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Date(1982, time.February, 10, 23, 0, 0, 0, time.UTC),
	}
	http.SetCookie(w, tokenCookie)

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

func (m *Manager) getToken(request *types.APIContext) error {
	// TODO switch to X-API-UserId header
	r := request.Request

	tokenAuthValue := GetTokenAuthFromRequest(r)
	if tokenAuthValue == "" {
		// no cookie or auth header, cannot authenticate
		return httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "No valid token cookie or auth header")
	}

	tokenID := request.ID

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

	currentAuthToken, _, err := m.getK8sTokenCR(tokenAuthValue)
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

func (m *Manager) NewLoginToken(userID string, userPrincipal v3.Principal, groupPrincipals []v3.Principal, providerInfo map[string]string, ttl int64, description string) (v3.Token, error) {
	token := &v3.Token{
		UserPrincipal:   userPrincipal,
		GroupPrincipals: groupPrincipals,
		IsDerived:       false,
		TTLMillis:       ttl,
		UserID:          userID,
		AuthProvider:    getAuthProviderName(userPrincipal.Name),
		ProviderInfo:    providerInfo,
		Description:     description,
	}

	return m.createK8sTokenCR(token)
}

func (m *Manager) UpateLoginToken(token *v3.Token) (*v3.Token, error) {
	return m.updateK8sTokenCR(token)
}

func (m *Manager) CreateTokenAndSetCookie(userID string, userPrincipal v3.Principal, groupPrincipals []v3.Principal, providerInfo map[string]string, ttl int, description string, request *types.APIContext) error {
	token, err := m.NewLoginToken(userID, userPrincipal, groupPrincipals, providerInfo, 0, description)
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
