package tokens

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
)

// TODO Cleanup error logging. If error is being returned, use errors.wrap to return and dont log here

const (
	defaultTokenTTL    = 57600000
	userPrincipalIndex = "authn.management.cattle.io/user-principal-index"
	userIDLabel        = "authn.management.cattle.io/token-userId"
	tokenKeyIndex      = "authn.management.cattle.io/token-key-index"
)

type tokenAPIServer struct {
	ctx          context.Context
	client       *config.ManagementContext
	tokensClient v3.TokenInterface
	userIndexer  cache.Indexer
	tokenIndexer cache.Indexer
}

var tokenServer *tokenAPIServer

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

func NewTokenAPIServer(ctx context.Context, mgmtCtx *config.ManagementContext) error {
	if mgmtCtx == nil {
		return fmt.Errorf("failed to build tokenAPIHandler, nil ManagementContext")
	}
	providers.Configure(ctx, mgmtCtx)

	informer := mgmtCtx.Management.Users("").Controller().Informer()
	informer.AddIndexers(map[string]cache.IndexFunc{userPrincipalIndex: userPrincipalIndexer})
	tokenInformer := mgmtCtx.Management.Tokens("").Controller().Informer()
	tokenInformer.AddIndexers(map[string]cache.IndexFunc{tokenKeyIndex: tokenKeyIndexer})
	tokenServer = &tokenAPIServer{
		ctx:          ctx,
		client:       mgmtCtx,
		tokensClient: mgmtCtx.Management.Tokens(""),
		userIndexer:  informer.GetIndexer(),
		tokenIndexer: tokenInformer.GetIndexer(),
	}

	return nil
}

//createLoginToken will authenticate with provider and creates a token CR
func (s *tokenAPIServer) createLoginToken(jsonInput v3.LoginInput) (v3.Token, int, error) {
	logrus.Debugf("Create Token Invoked")

	// Authenticate User
	userPrincipal, groupPrincipals, providerInfo, status, err := providers.AuthenticateUser(jsonInput)
	if status != 0 || err != nil {
		return v3.Token{}, status, err
	}

	logrus.Debug("User Authenticated")

	key, err := generateKey()
	if err != nil {
		logrus.Errorf("Failed to generate token key: %v", err)
		return v3.Token{}, 0, fmt.Errorf("failed to generate token key")
	}

	ttl := jsonInput.TTLMillis
	if ttl == 0 {
		ttl = defaultTokenTTL //16 hrs
	}

	k8sToken := &v3.Token{
		UserPrincipal:   userPrincipal,
		GroupPrincipals: groupPrincipals,
		IsDerived:       false,
		TTLMillis:       ttl,
		UserID:          getUserID(userPrincipal.Name),
		AuthProvider:    getAuthProviderName(userPrincipal.Name),
		ProviderInfo:    providerInfo,
		Description:     jsonInput.Description,
	}
	rToken, err := s.createK8sTokenCR(key, k8sToken)
	return rToken, 0, err
}

//CreateDerivedToken will create a jwt token for the authenticated user
func (s *tokenAPIServer) createDerivedToken(jsonInput v3.Token, tokenAuthValue string) (v3.Token, int, error) {

	logrus.Debug("Create Derived Token Invoked")

	token, _, err := s.getK8sTokenCR(tokenAuthValue)
	if err != nil {
		return v3.Token{}, 401, err
	}

	key, err := generateKey()
	if err != nil {
		logrus.Errorf("Failed to generate token key: %v", err)
		return v3.Token{}, 0, fmt.Errorf("failed to generate token key")
	}

	ttl := jsonInput.TTLMillis
	if ttl == 0 {
		ttl = defaultTokenTTL //16 hrs
	}

	k8sToken := &v3.Token{
		UserPrincipal:   token.UserPrincipal,
		GroupPrincipals: token.GroupPrincipals,
		IsDerived:       true,
		TTLMillis:       ttl,
		UserID:          token.UserID,
		AuthProvider:    token.AuthProvider,
		ProviderInfo:    token.ProviderInfo,
		Description:     jsonInput.Description,
	}
	rToken, err := s.createK8sTokenCR(key, k8sToken)

	return rToken, 0, err

}

func (s *tokenAPIServer) createK8sTokenCR(key string, k8sToken *v3.Token) (v3.Token, error) {
	labels := make(map[string]string)
	labels[userIDLabel] = k8sToken.UserID

	k8sToken.APIVersion = "management.cattle.io/v3"
	k8sToken.Kind = "Token"
	k8sToken.Token = key
	k8sToken.ObjectMeta = metav1.ObjectMeta{
		GenerateName: "token-",
		Labels:       labels,
	}
	createdToken, err := s.tokensClient.Create(k8sToken)

	if err != nil {
		return v3.Token{}, err
	}

	return *createdToken, nil
}

func (s *tokenAPIServer) getK8sTokenCR(tokenAuthValue string) (*v3.Token, int, error) {
	tokenName, tokenKey := SplitTokenParts(tokenAuthValue)

	lookupUsingClient := false

	objs, err := s.tokenIndexer.ByIndex(tokenKeyIndex, tokenKey)
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
		storedToken, err = s.tokensClient.Get(tokenName, metav1.GetOptions{})
		if err != nil {
			return nil, 404, fmt.Errorf("failed to retrieve auth token, error: %#v", err)
		}
	} else {
		storedToken = objs[0].(*v3.Token)
	}

	if storedToken.Token != tokenKey || storedToken.ObjectMeta.Name != tokenName {
		return nil, 0, fmt.Errorf("Invalid auth token value")
	}

	if !IsNotExpired(*storedToken) {
		return storedToken, 410, fmt.Errorf("Auth Token has expired")
	}

	return storedToken, 0, nil
}

//GetTokens will list all derived tokens of the authenticated user - only derived
func (s *tokenAPIServer) getTokens(tokenAuthValue string) ([]v3.Token, int, error) {
	logrus.Debug("LIST Tokens Invoked")
	tokens := make([]v3.Token, 0)

	storedToken, _, err := s.getK8sTokenCR(tokenAuthValue)
	if err != nil {
		return tokens, 401, err
	}

	userID := storedToken.UserID
	set := labels.Set(map[string]string{userIDLabel: userID})
	tokenList, err := s.tokensClient.List(metav1.ListOptions{LabelSelector: set.AsSelector().String()})
	if err != nil {
		return tokens, 0, fmt.Errorf("error getting tokens for user: %v selector: %v  err: %v", userID, set.AsSelector().String(), err)
	}

	for _, t := range tokenList.Items {
		if IsNotExpired(t) {
			tokens = append(tokens, t)
		}
	}
	return tokens, 0, nil
}

func (s *tokenAPIServer) deleteToken(tokenAuthValue string) (int, error) {
	logrus.Debug("DELETE Token Invoked")

	storedToken, status, err := s.getK8sTokenCR(tokenAuthValue)
	if err != nil {
		if status == 404 {
			return 0, nil
		} else if status != 410 {
			return 401, err
		}
	}
	err = s.tokensClient.Delete(storedToken.ObjectMeta.Name, &metav1.DeleteOptions{})
	if err != nil {
		if e2, ok := err.(*errors.StatusError); ok && e2.Status().Code == 404 {
			return 0, nil
		}
		return 500, fmt.Errorf("failed to delete token")
	}
	logrus.Debug("Deleted Token")
	return 0, nil
}

//getToken will get the token by ID if not expired
func (s *tokenAPIServer) getTokenByID(tokenAuthValue string, tokenID string) (v3.Token, int, error) {
	logrus.Debug("GET Token Invoked")
	token := &v3.Token{}

	storedToken, _, err := s.getK8sTokenCR(tokenAuthValue)
	if err != nil {
		return *token, 401, err
	}

	token, err = s.tokensClient.Get(tokenID, metav1.GetOptions{})
	if err != nil {
		return v3.Token{}, 404, err
	}

	if token.UserID != storedToken.UserID {
		return v3.Token{}, 403, fmt.Errorf("access denied: cannot get token")
	}

	if !IsNotExpired(*token) {
		return v3.Token{}, 410, fmt.Errorf("expired token")
	}

	return *token, 0, nil
}
