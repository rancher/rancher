package tokens

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/util"
	clientv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	ctrlv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/rancher/pkg/wrangler"
	ctrlv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

// TODO Cleanup error logging. If error is being returned, use errors.wrap to return and dont log here

const (
	userPrincipalIndex     = "authn.management.cattle.io/user-principal-index"
	UserIDLabel            = "authn.management.cattle.io/token-userId"
	TokenKindLabel         = "authn.management.cattle.io/kind"
	TokenKubeconfigIDLabel = "authn.management.cattle.io/kubeconfig-id"
	TokenHashed            = "authn.management.cattle.io/token-hashed"
	tokenKeyIndex          = "authn.management.cattle.io/token-key-index"
	secretNameEnding       = "-secret"
	SecretNamespace        = "cattle-system"
	KubeconfigResponseType = "kubeconfig"
)

type tokenClient interface {
	Create(*apiv3.Token) (*apiv3.Token, error)
	Update(*apiv3.Token) (*apiv3.Token, error)
	Get(string, metav1.GetOptions) (*apiv3.Token, error)
	List(metav1.ListOptions) (*apiv3.TokenList, error)
	Delete(string, *metav1.DeleteOptions) error
}

func RegisterIndexer(wContext *wrangler.Context) error {
	informer := wContext.Mgmt.User().Informer()
	return informer.AddIndexers(map[string]cache.IndexFunc{userPrincipalIndex: userPrincipalIndexer})
}

func NewManager(wContext *wrangler.Context) *Manager {
	return &Manager{
		tokenCache:   wContext.Mgmt.Token().Cache(),
		tokens:       wContext.Mgmt.Token(),
		tokenIndexer: wContext.Mgmt.Token().Informer().GetIndexer(),
		userCache:    wContext.Mgmt.User().Cache(),
		secrets:      wContext.Core.Secret(),
		secretCache:  wContext.Core.Secret().Cache(),
	}
}

type Manager struct {
	tokens       tokenClient
	tokenCache   ctrlv3.TokenCache
	tokenIndexer cache.Indexer
	userCache    ctrlv3.UserCache
	secrets      ctrlv1.SecretClient
	secretCache  ctrlv1.SecretCache
}

func userPrincipalIndexer(obj any) ([]string, error) {
	user, ok := obj.(*apiv3.User)
	if !ok {
		return []string{}, nil
	}

	return user.PrincipalIDs, nil
}

// createDerivedToken will create a jwt token for the authenticated user
func (m *Manager) createDerivedToken(jsonInput clientv3.Token, tokenAuthValue string) (apiv3.Token, string, int, error) {
	logrus.Debug("Create Derived Token Invoked")

	token, _, err := m.GetToken(tokenAuthValue)
	if err != nil {
		return apiv3.Token{}, "", http.StatusUnauthorized, err
	}

	tokenTTL, err := ClampToMaxTTL(time.Duration(int64(jsonInput.TTLMillis)) * time.Millisecond)
	if err != nil {
		return apiv3.Token{}, "", http.StatusInternalServerError, fmt.Errorf("error validating max-ttl %v", err)
	}

	var unhashedTokenKey string
	derivedToken := &apiv3.Token{
		UserPrincipal: token.UserPrincipal,
		IsDerived:     true,
		TTLMillis:     tokenTTL.Milliseconds(),
		UserID:        token.UserID,
		AuthProvider:  token.AuthProvider,
		ProviderInfo:  token.ProviderInfo,
		Description:   jsonInput.Description,
		ClusterName:   jsonInput.ClusterID,
	}
	derivedToken, unhashedTokenKey, err = m.createToken(derivedToken)

	return *derivedToken, unhashedTokenKey, 0, err

}

// createToken returns the token object and it's unhashed token key, which is stored hashed
func (m *Manager) createToken(k8sToken *apiv3.Token) (*apiv3.Token, string, error) {
	key, err := randomtoken.Generate()
	if err != nil {
		logrus.Errorf("Failed to generate token key: %v", err)
		return nil, "", errors.New("failed to generate token key")
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
		return nil, "", err
	}
	createdToken, err := m.tokens.Create(k8sToken)

	if err != nil {
		return nil, "", err
	}

	return createdToken, key, nil
}

func (m *Manager) updateToken(token *apiv3.Token) (*apiv3.Token, error) {
	return m.tokens.Update(token)
}

func (m *Manager) GetToken(tokenAuthValue string) (*apiv3.Token, int, error) {
	tokenName, tokenKey := SplitTokenParts(tokenAuthValue)
	var lookupUsingClient bool

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

	var storedToken *apiv3.Token
	if lookupUsingClient {
		storedToken, err = m.tokens.Get(tokenName, metav1.GetOptions{})
		if err != nil {
			return nil, 404, fmt.Errorf("failed to retrieve auth token, error: %#v", err)
		}
	} else {
		storedToken = objs[0].(*apiv3.Token)
	}

	if code, err := VerifyToken(storedToken, tokenName, tokenKey); err != nil {
		return nil, code, err
	}

	return storedToken, 0, nil
}

// GetTokens will list all (login and derived, and even expired) tokens of the authenticated user
func (m *Manager) getTokens(tokenAuthValue string) ([]apiv3.Token, int, error) {
	tokens := make([]apiv3.Token, 0)

	storedToken, _, err := m.GetToken(tokenAuthValue)
	if err != nil {
		return tokens, 401, err
	}

	userID := storedToken.UserID
	set := labels.Set(map[string]string{UserIDLabel: userID})
	tokenList, err := m.tokens.List(metav1.ListOptions{LabelSelector: set.AsSelector().String()})
	if err != nil {
		return tokens, 0, fmt.Errorf("error getting tokens for user: %v selector: %v  err: %v", userID, set.AsSelector().String(), err)
	}

	for _, t := range tokenList.Items {
		if IsExpired(&t) {
			t.Expired = true
		}
		tokens = append(tokens, t)
	}
	return tokens, 0, nil
}

func (m *Manager) DeleteTokenByName(tokenName string) (int, error) {
	err := m.tokens.Delete(tokenName, &metav1.DeleteOptions{})
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
func (m *Manager) getTokenByID(tokenAuthValue string, tokenID string) (apiv3.Token, int, error) {
	logrus.Debug("GET Token Invoked")
	token := &apiv3.Token{}

	storedToken, _, err := m.GetToken(tokenAuthValue)
	if err != nil {
		return *token, http.StatusUnauthorized, err
	}

	token, err = m.tokens.Get(tokenID, metav1.GetOptions{})
	if err != nil {
		return apiv3.Token{}, http.StatusNotFound, err
	}

	if token.UserID != storedToken.UserID {
		return apiv3.Token{}, http.StatusNotFound, fmt.Errorf("%s not found", tokenID)
	}

	if IsExpired(token) {
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

	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%s", err))
	}

	jsonInput := clientv3.Token{}
	err = json.Unmarshal(bytes, &jsonInput)
	if err != nil {
		return httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("%s", err))
	}

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

	currentAuthToken, _, err := m.GetToken(tokenAuthValue)
	if err != nil {
		return err
	}

	tokensFromStore := []map[string]any{}
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

func (m *Manager) getTokenFromRequest(request *types.APIContext) error {
	// TODO switch to X-API-UserId header
	r := request.Request

	tokenAuthValue := GetTokenAuthFromRequest(r)
	if tokenAuthValue == "" {
		// no cookie or auth header, cannot authenticate
		return httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "No valid token cookie or auth header")
	}

	tokenID := request.ID

	currentAuthToken, _, err := m.GetToken(tokenAuthValue)
	if err != nil {
		return err
	}

	token, status, err := m.getTokenByID(tokenAuthValue, tokenID)
	if err != nil {
		switch status {
		case 0:
			status = http.StatusInternalServerError
		case 410:
			status = http.StatusNotFound
		default:
		}
		logrus.Errorf("GetToken failed with error: %v", err)
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

	currentAuthToken, _, err := m.GetToken(tokenAuthValue)
	if err != nil {
		return err
	}

	if currentAuthToken.Name == t.Name && !currentAuthToken.IsDerived {
		return httperror.NewAPIErrorLong(http.StatusBadRequest, util.GetHTTPErrorCode(http.StatusBadRequest), "Cannot delete token for current session. Use logout instead")
	}

	if _, err := m.DeleteTokenByName(t.Name); err != nil {
		return err
	}

	request.WriteResponse(http.StatusNoContent, nil)
	return nil
}

// CreateSecret saves the secret in k8s. Secret is saved under the userID-secret with
// key being the provider and data being the providers secret
func (m *Manager) CreateSecret(userID, provider, secret string) error {
	_, err := m.secretCache.Get(SecretNamespace, userID+secretNameEnding)
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

func (m *Manager) GetSecret(userID string, provider string, fallbackTokens []accessor.TokenAccessor) (string, error) {
	cachedSecret, err := m.secretCache.Get(SecretNamespace, userID+secretNameEnding)
	if err != nil && !apierrors.IsNotFound(err) {
		return "", err
	}

	if (err == nil) && cachedSecret != nil && string(cachedSecret.Data[provider]) != "" {
		return string(cachedSecret.Data[provider]), nil
	}

	for _, token := range fallbackTokens {
		secret := token.GetProviderInfo()["access_token"]
		if secret != "" {
			return secret, nil
		}
	}

	return "", err // The not found error from above
}

func (m *Manager) UpdateSecret(userID, provider, secret string) error {
	cachedSecret, err := m.secretCache.Get(SecretNamespace, userID+secretNameEnding)
	if err != nil {
		return err
	}

	cachedSecret = cachedSecret.DeepCopy()

	cachedSecret.Data[provider] = []byte(secret)

	_, err = m.secrets.Update(cachedSecret)
	return err
}

// PerUserCacheProviders is a set of provider names for which the token manager creates a per-user login token.
var PerUserCacheProviders = []string{"github", "azuread", "googleoauth", "oidc", "keycloakoidc"}

func (m *Manager) NewLoginToken(userID string, userPrincipal apiv3.Principal, groupPrincipals []apiv3.Principal, providerToken string, ttl int64, description string) (*apiv3.Token, string, error) {
	provider := userPrincipal.Provider
	// Providers that use oauth need to create a secret for storing the access token.
	if slices.Contains(PerUserCacheProviders, provider) && providerToken != "" {
		err := m.CreateSecret(userID, provider, providerToken)
		if err != nil {
			return nil, "", fmt.Errorf("unable to create secret: %s", err)
		}
	}

	return m.createToken(&apiv3.Token{
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
	})
}

func (m *Manager) UpdateToken(token *apiv3.Token) (*apiv3.Token, error) {
	return m.updateToken(token)
}

func (m *Manager) CreateTokenAndSetCookie(userID string, userPrincipal apiv3.Principal, groupPrincipals []apiv3.Principal, providerToken string, ttl int, description string, request *types.APIContext) error {
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
	data chan map[string]any,
	opt *types.QueryOptions,
) (chan map[string]any, error) {
	logrus.Debug("TokenStreamTransformer called")

	tokenAuthValue := GetTokenAuthFromRequest(apiContext.Request)
	if tokenAuthValue == "" {
		// no cookie or auth header, cannot authenticate
		return nil, httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "[TokenStreamTransformer] failed: No valid token cookie or auth header")
	}

	storedToken, code, err := m.GetToken(tokenAuthValue)
	if err != nil {
		return nil, httperror.NewAPIErrorLong(code, http.StatusText(code), fmt.Sprintf("[TokenStreamTransformer] failed: %s", err.Error()))
	}

	userID := storedToken.UserID

	return convert.Chan(data, func(data map[string]any) map[string]any {
		labels, _ := data["labels"].(map[string]any)
		if labels[UserIDLabel] != userID {
			return nil
		}

		name, _ := data["name"].(string)
		isDerived, _ := data["isDerived"].(bool)
		data["current"] = name == storedToken.Name && !isDerived

		return data
	}), nil
}

var backoff = wait.Backoff{
	Duration: 100 * time.Millisecond,
	Factor:   1,
	Jitter:   0,
	Steps:    7,
}

func (m *Manager) EnsureToken(input user.TokenInput) (string, runtime.Object, error) {
	return m.EnsureClusterToken("", input)
}

func (m *Manager) EnsureClusterToken(clusterName string, input user.TokenInput) (string, runtime.Object, error) {
	if strings.HasPrefix(input.TokenName, "token-") {
		return "", nil, errors.New("token names can't start with token-")
	}

	var err error
	var token *apiv3.Token
	if !input.Randomize {
		token, err = m.tokenCache.Get(input.TokenName)
		if err != nil && !apierrors.IsNotFound(err) {
			return "", nil, err
		}
		if err == nil {
			if err := m.tokens.Delete(token.Name, &metav1.DeleteOptions{}); err != nil {
				return "", nil, err
			}
		}
	}

	key, err := randomtoken.Generate()
	if err != nil {
		return "", nil, errors.New("failed to generate token key")
	}

	labels := map[string]string{}
	if input.Labels != nil {
		for k, v := range input.Labels {
			labels[k] = v
		}
	}
	labels[UserIDLabel] = input.UserName
	labels[TokenKindLabel] = input.Kind

	token = &apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:   input.TokenName,
			Labels: labels,
		},
		TTLMillis:     0,
		Description:   input.Description,
		UserID:        input.UserName,
		AuthProvider:  input.AuthProvider,
		UserPrincipal: input.UserPrincipal,
		IsDerived:     true,
		Token:         key,
		ClusterName:   clusterName,
	}
	if input.TTL != nil {
		token.TTLMillis = *input.TTL
	}
	if input.Randomize {
		token.ObjectMeta.Name = ""
		token.ObjectMeta.GenerateName = input.TokenName
	}
	err = ConvertTokenKeyToHash(token)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert token key to hash: %w", err)
	}

	logrus.Infof("Creating token for user %s", input.UserName)
	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		// Backoff was added here because it is possible the token is in the process of deleting.
		// This should cause the create to retry until the delete is finished.
		newToken, err := m.tokens.Create(token)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				return false, nil
			}
			return false, err
		}
		token = newToken
		return true, nil
	})
	if err != nil {
		return "", nil, err
	}

	return token.Name + ":" + key, token, nil
}

// newTokenForKubeconfig creates a new token for a generated kubeconfig.
func (m *Manager) newTokenForKubeconfig(clusterName, tokenName, description, kind, userName string, userPrincipal apiv3.Principal) (string, error) {
	tokenTTL, err := GetKubeconfigDefaultTokenTTLInMilliSeconds()
	if err != nil {
		return "", fmt.Errorf("failed to get default token TTL: %w", err)
	}

	input := user.TokenInput{
		TokenName:     tokenName,
		Description:   description,
		Kind:          kind,
		UserName:      userName,
		AuthProvider:  userPrincipal.Provider,
		TTL:           tokenTTL,
		Randomize:     true,
		UserPrincipal: userPrincipal,
	}

	tokenKey, _, err := m.EnsureClusterToken(clusterName, input)
	if err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}

	return tokenKey, nil
}

// GetKubeconfigToken creates a new token for use in a kubeconfig generated through the CLI.
func (m *Manager) GetKubeconfigToken(clusterName, tokenName, description, kind, userName string, userPrincipal apiv3.Principal) (*apiv3.Token, string, error) {
	fullCreatedToken, err := m.newTokenForKubeconfig(clusterName, tokenName, description, kind, userName, userPrincipal)
	if err != nil {
		return nil, "", err
	}

	randomizedTokenName, createdTokenValue := SplitTokenParts(fullCreatedToken)
	token, err := m.tokens.Get(randomizedTokenName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, createdTokenValue, err
	}

	if token.ExpiresAt != "" {
		return token, createdTokenValue, nil
	}

	// SetTokenExpiresAt requires creationTS, so can only be set post create
	tokenCopy := token.DeepCopy()
	SetTokenExpiresAt(tokenCopy)

	token, err = m.tokens.Update(tokenCopy)
	if err != nil {
		if !apierrors.IsConflict(err) {
			return nil, "", fmt.Errorf("getToken: updating token [%s] failed [%v]", randomizedTokenName, err)
		}

		err = wait.ExponentialBackoff(backoff, func() (bool, error) {
			token, err = m.tokens.Get(randomizedTokenName, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			if token.ExpiresAt == "" {
				tokenCopy := token.DeepCopy()
				SetTokenExpiresAt(tokenCopy)

				token, err = m.tokens.Update(tokenCopy)
				if err != nil {
					logrus.Debugf("getToken: updating token [%s] failed [%v]", randomizedTokenName, err)
					if apierrors.IsConflict(err) {
						return false, nil
					}
					return false, err
				}
			}
			return true, nil
		})

		if err != nil {
			return nil, "", fmt.Errorf("getToken: retry updating token [%s] failed [%v]", randomizedTokenName, err)
		}
	}

	logrus.Debugf("getToken: token %s expiresAt %s", token.Name, token.ExpiresAt)
	return token, createdTokenValue, nil
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
