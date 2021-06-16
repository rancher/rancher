package requests

import (
	"context"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/steve/pkg/auth"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/cache"
)

var (
	ErrMustAuthenticate = httperror.NewAPIError(httperror.Unauthorized, "must authenticate")
)

type Authenticator interface {
	Authenticate(req *http.Request) (*AuthenticatorResponse, error)
	TokenFromRequest(req *http.Request) (*v3.Token, error)
}

type AuthenticatorResponse struct {
	IsAuthed      bool
	User          string
	UserPrincipal string
	Groups        []string
	Extras        map[string][]string
}

func ToAuthMiddleware(a Authenticator) auth.Middleware {
	f := func(req *http.Request) (user.Info, bool, error) {
		authResp, err := a.Authenticate(req)
		if err != nil {
			return nil, false, err
		}
		return &user.DefaultInfo{
			Name:   authResp.User,
			UID:    authResp.User,
			Groups: authResp.Groups,
			Extra:  authResp.Extras,
		}, authResp.IsAuthed, err
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

func (a *tokenAuthenticator) Authenticate(req *http.Request) (*AuthenticatorResponse, error) {
	authResp := &AuthenticatorResponse{
		Extras: make(map[string][]string),
	}
	token, err := a.TokenFromRequest(req)
	if err != nil {
		return nil, err
	}

	if token.Enabled != nil && !*token.Enabled {
		return nil, errors.Wrapf(ErrMustAuthenticate, "user's token is not enabled")
	}
	if token.ClusterName != "" && token.ClusterName != a.clusterRouter(req) {
		return nil, errors.Wrapf(ErrMustAuthenticate, "clusterID does not match")
	}
	logrus.Infof("request token %v", token)

	attribs, err := a.userAttributeLister.Get("", token.UserID)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	u, err := a.userLister.Get("", token.UserID)
	if err != nil {
		return nil, err
	}

	if u.Enabled != nil && !*u.Enabled {
		return nil, errors.Wrap(ErrMustAuthenticate, "user is not enabled")
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

	authResp.IsAuthed = true
	authResp.User = token.UserID
	authResp.UserPrincipal = token.UserPrincipal.Name
	authResp.Groups = groups

	if attribs != nil {
		if strings.EqualFold(token.AuthProvider, "local") || token.AuthProvider == "" {
			//gather all extraInfo for all external auth providers present in the userAttributes
			extraInfo := make(map[string][]string)
			for provider, extra := range attribs.Extra {
				if provider != "local" {
					for key, value := range extra {
						extraInfo[key] = append(extraInfo[key], value...)
					}
				}
			}
			authResp.Extras = extraInfo
		} else {
			//non-local authProvider is set in token
			var ok bool
			authResp.Extras, ok = attribs.Extra[token.AuthProvider]
			if !ok {
				extraFromProvider := providers.GetUserExtraAttributes(token.AuthProvider, token.UserPrincipal)
				if extraFromProvider != nil {
					authResp.Extras = extraFromProvider
				}
			}
		}
	}
	return authResp, nil
}

func (a *tokenAuthenticator) TokenFromRequest(req *http.Request) (*v3.Token, error) {
	tokenAuthValue := tokens.GetTokenAuthFromRequest(req)
	if tokenAuthValue == "" {
		return nil, ErrMustAuthenticate
	}

	tokenName, tokenKey := tokens.SplitTokenParts(tokenAuthValue)
	if tokenName == "" || tokenKey == "" {
		return nil, ErrMustAuthenticate
	}

	lookupUsingClient := false
	objs, err := a.tokenIndexer.ByIndex(tokenKeyIndex, tokenKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			lookupUsingClient = true
		} else {
			return nil, errors.Wrapf(ErrMustAuthenticate, "failed to retrieve auth token from cache, error: %v", err)
		}
	} else if len(objs) == 0 {
		lookupUsingClient = true
	}

	var storedToken *v3.Token
	if lookupUsingClient {
		storedToken, err = a.tokenClient.Get(tokenName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, ErrMustAuthenticate
			}
			return nil, errors.Wrapf(ErrMustAuthenticate, "failed to retrieve auth token, error: %#v", err)
		}
	} else {
		storedToken = objs[0].(*v3.Token)
	}

	if _, err := tokens.VerifyToken(storedToken, tokenName, tokenKey); err != nil {
		return nil, errors.Wrapf(ErrMustAuthenticate, "failed to verify token: %v", err)
	}

	return storedToken, nil
}
