package requests

// XXX TODO AK -- marker of code modified for ext token support

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	exttokens "github.com/rancher/rancher/pkg/ext/resources/tokens"
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
	TokenFromRequest(req *http.Request) (accessor.TokenAccessor, error)
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
	providerRefresher := providerrefresh.NewUserAuthRefresher(ctx, mgmtCtx)

	// Rancher Backend direct Ext Token Access via in-built custom handler store instance
	wContext := mgmtCtx.Wrangler
	extTokenStore := exttokens.NewSystemTokenStore(
		wContext.Core.Secret(),
		wContext.Core.Secret().Cache(),
		wContext.Mgmt.UserAttribute(),
		wContext.Mgmt.User(),
	)

	return &tokenAuthenticator{
		ctx:                 ctx,
		tokenIndexer:        tokenInformer.GetIndexer(),
		tokenClient:         mgmtCtx.Management.Tokens(""),
		userAttributeLister: mgmtCtx.Management.UserAttributes("").Controller().Lister(),
		userAttributes:      mgmtCtx.Management.UserAttributes(""),
		userLister:          mgmtCtx.Management.Users("").Controller().Lister(),
		clusterRouter:       clusterRouter,
		refreshUser: func(userID string, force bool) {
			go providerRefresher.TriggerUserRefresh(userID, force)
		},
		extTokenStore: extTokenStore,
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
	refreshUser         func(userID string, force bool)
	extTokenStore       *exttokens.SystemTokenStore
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

	if !token.GetIsEnabled() {
		return nil, errors.Wrapf(ErrMustAuthenticate, "user's token is not enabled")
	}
	cluster := token.ObjClusterName()
	if cluster != "" && cluster != a.clusterRouter(req) {
		return nil, errors.Wrapf(ErrMustAuthenticate, "clusterID does not match")
	}

	// If the auth provider is specified make sure it exists and enabled.
	if token.GetAuthProvider() != "" {
		disabled, err := providers.IsDisabledProvider(token.GetAuthProvider())
		if err != nil {
			return nil, errors.Wrapf(ErrMustAuthenticate,
				"error checking if provider %s is disabled: %v",
				token.GetAuthProvider(), err)
		}
		if disabled {
			return nil, errors.Wrapf(ErrMustAuthenticate, "provider %s is disabled",
				token.GetAuthProvider())
		}
	}

	attribs, err := a.userAttributeLister.Get("", token.GetUserID())
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, errors.Wrapf(ErrMustAuthenticate,
			"failed to retrieve userattribute %s: %v", token.GetUserID(), err)
	}

	authUser, err := a.userLister.Get("", token.GetUserID())
	if err != nil {
		return nil, errors.Wrapf(ErrMustAuthenticate,
			"failed to retrieve user %s: %v", token.GetUserID(), err)
	}

	if authUser.Enabled != nil && !*authUser.Enabled {
		return nil, errors.Wrap(ErrMustAuthenticate, "user is not enabled")
	}

	var groups []string
	hitProvider := false
	if attribs != nil {
		authp := token.GetAuthProvider()
		for provider, gps := range attribs.GroupPrincipals {
			if provider == authp {
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
		for _, principal := range token.GetGroupPrincipals() {
			// TODO This is a short cut for now. Will actually need to lookup groups in future
			name := strings.TrimPrefix(principal.Name, "local://")
			groups = append(groups, name)
		}
	}
	groups = append(groups, user.AllAuthenticated, "system:cattle:authenticated")

	if !(authUser.IsSystem() || strings.HasPrefix(token.GetUserID(), "system:")) {
		a.refreshUser(token.GetUserID(), false)
	}

	authResp.IsAuthed = true
	authResp.User = token.GetUserID()
	authResp.UserPrincipal = token.GetUserPrincipal().Name
	authResp.Groups = groups
	authResp.Extras = getUserExtraInfo(token, authUser, attribs)
	logrus.Debugf("Extras returned %v", authResp.Extras)

	return authResp, nil
}

func getUserExtraInfo(token accessor.TokenAccessor, u *v3.User, attribs *v3.UserAttribute) map[string][]string {
	extraInfo := make(map[string][]string)

	ap := token.GetAuthProvider()
	if attribs != nil && attribs.ExtraByProvider != nil && len(attribs.ExtraByProvider) != 0 {
		if ap == "local" || ap == "" {
			//gather all extraInfo for all external auth providers present in the userAttributes
			for _, extra := range attribs.ExtraByProvider {
				for key, value := range extra {
					extraInfo[key] = append(extraInfo[key], value...)
				}
			}
			return extraInfo
		}
		//authProvider is set in token
		if extraInfo, ok := attribs.ExtraByProvider[ap]; ok {
			return extraInfo
		}
	}

	extraInfo = providers.GetUserExtraAttributes(ap, token.GetUserPrincipal())
	//if principalid is not set in extra, read from user
	if extraInfo != nil {
		if len(extraInfo[common.UserAttributePrincipalID]) == 0 {
			extraInfo[common.UserAttributePrincipalID] = u.PrincipalIDs
		}
		if len(extraInfo[common.UserAttributeUserName]) == 0 {
			extraInfo[common.UserAttributeUserName] = []string{u.DisplayName}
		}
	}

	return extraInfo
}

func (a *tokenAuthenticator) TokenFromRequest(req *http.Request) (accessor.TokenAccessor, error) {
	tokenAuthValue := tokens.GetTokenAuthFromRequest(req)
	if tokenAuthValue == "" {
		return nil, ErrMustAuthenticate
	}

	tokenName, tokenKey := tokens.SplitTokenParts(tokenAuthValue)
	if tokenName == "" || tokenKey == "" {
		return nil, ErrMustAuthenticate
	}

	lookupUsingClient := false

	// General process:
	// 1. look for token in the indexer. use that if found.
	// 2. in all other cases (not found, or error), retrieve directly via kube client

	if extTokenName, found := strings.CutPrefix(tokenName, "ext-"); found {
		// Process ext token
		// Roughly the same process as for legacy tokens, using a different store.
		// No indexer/cache in play here.

		storedToken, err := a.extTokenStore.Get(extTokenName, &metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, ErrMustAuthenticate
			}
			return nil, errors.Wrapf(ErrMustAuthenticate,
				"failed to retrieve auth token, error: %#v", err)
		}
		if _, err := extVerifyToken(storedToken, extTokenName, tokenKey); err != nil {
			return nil, errors.Wrapf(ErrMustAuthenticate, "failed to verify token: %v", err)
		}

		return storedToken, nil
	}

	// Process legacy norman/v3 token

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

// Given a stored token with hashed key, check if the provided (unhashed) tokenKey matches and is valid
func extVerifyToken(storedToken *exttokens.Token, tokenName, tokenKey string) (int, error) {
	invalidAuthTokenErr := errors.New("Invalid auth token value")

	if storedToken == nil || storedToken.ObjectMeta.Name != tokenName {
		return http.StatusUnprocessableEntity, invalidAuthTokenErr
	}

	// Ext token always has a hash. Only a hash.

	hasher, err := hashers.GetHasherForHash(storedToken.Status.TokenHash)
	if err != nil {
		logrus.Errorf("unable to get a hasher for token with error %v", err)
		return http.StatusInternalServerError, fmt.Errorf("unable to verify hash")
	}

	if err := hasher.VerifyHash(storedToken.Status.TokenHash, tokenKey); err != nil {
		logrus.Errorf("VerifyHash failed with error: %v", err)
		return http.StatusUnprocessableEntity, invalidAuthTokenErr
	}

	if extIsExpired(storedToken) {
		return http.StatusGone, errors.New("must authenticate")
	}
	return http.StatusOK, nil
}

func extIsExpired(token *exttokens.Token) bool {
	if token.Spec.TTL == 0 {
		return false
	}

	created := token.ObjectMeta.CreationTimestamp.Time
	durationElapsed := time.Since(created)
	ttlDuration := time.Duration(token.Spec.TTL) * time.Millisecond

	return durationElapsed.Seconds() >= ttlDuration.Seconds()
}
