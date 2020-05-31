package requests

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/steve/pkg/auth"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/cache"
)

type Authenticator interface {
	Authenticate(req *http.Request) (authed bool, user string, groups []string, err error)
	TokenFromRequest(req *http.Request) (*v3.Token, error)
}

func ToAuthMiddleware(a Authenticator) auth.Middleware {
	f := func(req *http.Request) (user.Info, bool, error) {
		authed, u, groups, err := a.Authenticate(req)
		return &user.DefaultInfo{
			Name:   u,
			UID:    u,
			Groups: groups,
		}, authed, err
	}
	return auth.ToMiddleware(auth.AuthenticatorFunc(f))
}

type ClusterRouter func(req *http.Request) string

func NewAuthenticator(ctx context.Context, clusterRouter ClusterRouter, mgmtCtx *config.ScaledContext) Authenticator {
	tokenInformer := mgmtCtx.Management.Tokens("").Controller().Informer()
	tokenInformer.AddIndexers(map[string]cache.IndexFunc{tokenKeyIndex: tokenKeyIndexer})

	return &tokenAuthenticator{
		ctx:                 ctx,
		tokenIndexer:        tokenInformer.GetIndexer(),
		tokenClient:         mgmtCtx.Management.Tokens(""),
		userAttributeLister: mgmtCtx.Management.UserAttributes("").Controller().Lister(),
		userAttributes:      mgmtCtx.Management.UserAttributes(""),
		userLister:          mgmtCtx.Management.Users("").Controller().Lister(),
		clusterRouter:       clusterRouter,
		userAuthRefresher:   providerrefresh.NewUserAuthRefresher(ctx, mgmtCtx),
	}
}

type tokenAuthenticator struct {
	ctx                 context.Context
	tokenIndexer        cache.Indexer
	tokenClient         v3.TokenInterface
	userAttributes      v3.UserAttributeInterface
	userAttributeLister v3.UserAttributeLister
	userLister          v3.UserLister
	clusterRouter       ClusterRouter
	userAuthRefresher   providerrefresh.UserAuthRefresher
}

const (
	tokenKeyIndex = "authn.management.cattle.io/token-key-index"
)

func tokenKeyIndexer(obj interface{}) ([]string, error) {
	token, ok := obj.(*v3.Token)
	if !ok {
		return []string{}, nil
	}

	return []string{token.Token}, nil
}

func (a *tokenAuthenticator) Authenticate(req *http.Request) (bool, string, []string, error) {
	token, err := a.TokenFromRequest(req)
	if err != nil {
		return false, "", []string{}, err
	}

	if token.Enabled != nil && !*token.Enabled {
		return false, "", []string{}, fmt.Errorf("user's token is not enabled")
	}
	if token.ClusterName != "" && token.ClusterName != a.clusterRouter(req) {
		return false, "", []string{}, fmt.Errorf("clusterID does not match")
	}

	attribs, err := a.userAttributeLister.Get("", token.UserID)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, "", []string{}, err
	}

	u, err := a.userLister.Get("", token.UserID)
	if err != nil {
		return false, "", []string{}, err
	}

	if u.Enabled != nil && !*u.Enabled {
		return false, "", []string{}, fmt.Errorf("user is not enabled")
	}

	var groups []string
	hitProvider := false
	if attribs != nil {
		for provider, gps := range attribs.GroupPrincipals {
			if provider == token.AuthProvider {
				hitProvider = true
			}
			for _, principal := range gps.Items {
				name := strings.TrimPrefix(principal.Name, "local://")
				groups = append(groups, name)
			}
		}
	}

	// fallback to legacy token.GroupPrincipals
	if !hitProvider {
		for _, principal := range token.GroupPrincipals {
			// TODO This is a short cut for now. Will actually need to lookup groups in future
			name := strings.TrimPrefix(principal.Name, "local://")
			groups = append(groups, name)
		}
	}

	groups = append(groups, user.AllAuthenticated, "system:cattle:authenticated")

	if !strings.HasPrefix(token.UserID, "system:") {
		go a.userAuthRefresher.TriggerUserRefresh(token.UserID, false)
	}

	return true, token.UserID, groups, nil
}

func (a *tokenAuthenticator) TokenFromRequest(req *http.Request) (*v3.Token, error) {
	tokenAuthValue := tokens.GetTokenAuthFromRequest(req)
	if tokenAuthValue == "" {
		return nil, fmt.Errorf("must authenticate")
	}

	tokenName, tokenKey := tokens.SplitTokenParts(tokenAuthValue)
	if tokenName == "" || tokenKey == "" {
		return nil, fmt.Errorf("must authenticate")
	}

	lookupUsingClient := false
	objs, err := a.tokenIndexer.ByIndex(tokenKeyIndex, tokenKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			lookupUsingClient = true
		} else {
			return nil, fmt.Errorf("failed to retrieve auth token from cache, error: %v", err)
		}
	} else if len(objs) == 0 {
		lookupUsingClient = true
	}

	storedToken := &v3.Token{}
	if lookupUsingClient {
		storedToken, err = a.tokenClient.Get(tokenName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("must authenticate")
			}
			return nil, fmt.Errorf("failed to retrieve auth token, error: %#v", err)
		}
	} else {
		storedToken = objs[0].(*v3.Token)
	}

	if storedToken.Token != tokenKey || storedToken.ObjectMeta.Name != tokenName {
		return nil, fmt.Errorf("must authenticate")
	}

	if tokens.IsExpired(*storedToken) {
		return nil, fmt.Errorf("must authenticate")
	}

	return storedToken, nil
}
