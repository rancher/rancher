package tokens

import (
	"context"
	"fmt"

	"github.com/rancher/auth/providers"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// TODO Cleanup error logging. If error is being returned, use errors.wrap to return and dont log here

const (
	defaultTokenTTL    = 57600000
	userPrincipalIndex = "authn.management.cattle.io/user-principal-index"
	userIDLabel        = "authn.management.cattle.io/token-userId"
)

type tokenAPIServer struct {
	ctx          context.Context
	client       *config.ManagementContext
	tokensClient v3.TokenInterface
	userIndexer  cache.Indexer
}

func userPrincipalIndexer(obj interface{}) ([]string, error) {
	user, ok := obj.(*v3.User)
	if !ok {
		return []string{}, nil
	}

	return user.PrincipalIDs, nil
}

func newTokenAPIServer(ctx context.Context, mgmtCtx *config.ManagementContext) (*tokenAPIServer, error) {
	if mgmtCtx == nil {
		return nil, fmt.Errorf("failed to build tokenAPIHandler, nil ManagementContext")
	}
	providers.Configure(ctx, mgmtCtx)

	informer := mgmtCtx.Management.Users("").Controller().Informer()
	informer.AddIndexers(map[string]cache.IndexFunc{userPrincipalIndex: userPrincipalIndexer})
	apiServer := &tokenAPIServer{
		ctx:          ctx,
		client:       mgmtCtx,
		tokensClient: mgmtCtx.Management.Tokens(""),
		userIndexer:  informer.GetIndexer(),
	}

	return apiServer, nil
}

//createLoginToken will authenticate with provider and creates a token CR
func (s *tokenAPIServer) createLoginToken(jsonInput v3.LoginInput) (v3.Token, int, error) {
	logrus.Debugf("Create Token Invoked")

	// Authenticate User
	userPrincipal, groupPrincipals, providerInfo, status, err := providers.AuthenticateUser(jsonInput)
	if status != 0 || err != nil {
		return v3.Token{}, status, err
	}

	/*objs, err := s.userIndexer.ByIndex(userPrincipalIndex, userPrincipal.Name)
	if err != nil {
		return v3.Token{}, 500, err
	}
	if len(objs) == 0 {
		return v3.Token{}, 403, fmt.Errorf("forbidden")
	}
	if len(objs) > 1 {
		logrus.Errorf("Found more than one user matching %v", userPrincipal.Name)
		return v3.Token{}, 403, fmt.Errorf("forbidden")
	}
	localUser, ok := objs[0].(*v3.User)
	if !ok {
		logrus.Errorf("User isnt a user %v", objs[0])
		return v3.Token{}, 500, fmt.Errorf("fatal error. User is not a user")
	}*/

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
	}
	rToken, err := s.createK8sTokenCR(key, k8sToken)
	return rToken, 0, err
}

//CreateDerivedToken will create a jwt token for the authenticated user
func (s *tokenAPIServer) createDerivedToken(jsonInput v3.Token, tokenID string) (v3.Token, int, error) {

	logrus.Debug("Create Derived Token Invoked")

	token, err := s.getK8sTokenCR(tokenID)
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
	}
	rToken, err := s.createK8sTokenCR(key, k8sToken)
	return rToken, 0, err

}

func (s *tokenAPIServer) createK8sTokenCR(key string, k8sToken *v3.Token) (v3.Token, error) {
	labels := make(map[string]string)
	labels[userIDLabel] = k8sToken.UserID

	k8sToken.APIVersion = "management.cattle.io/v3"
	k8sToken.Kind = "Token"
	k8sToken.ObjectMeta = metav1.ObjectMeta{
		Name:   key,
		Labels: labels,
	}
	createdToken, err := s.tokensClient.Create(k8sToken)

	if err != nil {
		return v3.Token{}, err
	}
	return *createdToken, nil
}

func (s *tokenAPIServer) getK8sTokenCR(tokenID string) (*v3.Token, error) {
	storedToken, err := s.tokensClient.Get(tokenID, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return storedToken, nil
}

//GetTokens will list all derived tokens of the authenticated user - only derived
func (s *tokenAPIServer) getTokens(tokenID string) ([]v3.Token, int, error) {
	logrus.Debug("LIST Tokens Invoked")
	tokens := make([]v3.Token, 0)

	storedToken, err := s.tokensClient.Get(tokenID, metav1.GetOptions{})
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
		if t.IsDerived {
			tokens = append(tokens, t)
		}
	}
	return tokens, 0, nil
}

func (s *tokenAPIServer) deleteToken(tokenKey string) (int, error) {
	logrus.Debug("DELETE Token Invoked")

	err := s.tokensClient.Delete(tokenKey, &metav1.DeleteOptions{})
	if err != nil {
		if e2, ok := err.(*errors.StatusError); ok && e2.Status().Code == 404 {
			return 0, nil
		}
		return 500, fmt.Errorf("failed to delete token")
	}
	logrus.Debug("Deleted Token")
	return 0, nil
}

//getDerivedToken will get the derived token - only derived
func (s *tokenAPIServer) getDerivedToken(tokenID string, derivedTokenID string) (v3.Token, int, error) {
	logrus.Debug("GET Derived Token Invoked")
	derivedToken := &v3.Token{}

	storedToken, err := s.getK8sTokenCR(tokenID)
	if err != nil {
		return *derivedToken, 401, err
	}

	derivedToken, err = s.getK8sTokenCR(derivedTokenID)

	if err != nil {
		return v3.Token{}, 404, err
	}

	if derivedToken.UserID != storedToken.UserID {
		return v3.Token{}, 403, fmt.Errorf("access denied: cannot get derived token")
	}

	return *derivedToken, 0, nil
}

//deleteDerivedToken will delete the derived token - only derived
func (s *tokenAPIServer) deleteDerivedToken(tokenID string, derivedTokenID string) (int, error) {
	logrus.Debug("DELETE Derived Token Invoked")

	_, status, err := s.getDerivedToken(tokenID, derivedTokenID)

	if err != nil {
		return status, err
	}

	return s.deleteToken(derivedTokenID)
}
