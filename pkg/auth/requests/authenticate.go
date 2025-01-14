package requests

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/steve/pkg/auth"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/cache"
)

const tokenKeyIndex = "authn.management.cattle.io/token-key-index"

const (
	ExtraRequestTokenID = "requesttokenid"
	ExtraRequestHost    = "requesthost"
)

var ErrMustAuthenticate = httperror.NewAPIError(httperror.Unauthorized, "must authenticate")

// Authenticator authenticates a request.
type Authenticator interface {
	Authenticate(req *http.Request) (*AuthenticatorResponse, error)
	TokenFromRequest(req *http.Request) (*v3.Token, error)
}

// AuthenticatorResponse is the response returned by an Authenticator.
type AuthenticatorResponse struct {
	IsAuthed      bool
	User          string
	UserPrincipal string
	Groups        []string
	Extras        map[string][]string
}

// ClusterRouter returns the cluster ID based on the request URL's path.
type ClusterRouter func(req *http.Request) string

type tokenAuthenticator struct {
	ctx                 context.Context
	tokenIndexer        cache.Indexer
	tokenClient         mgmtcontrollers.TokenClient
	userAttributes      v3.UserAttributeInterface
	userAttributeLister v3.UserAttributeLister
	userLister          v3.UserLister
	clusterRouter       ClusterRouter
	refreshUser         func(userID string, force bool)
	now                 func() time.Time // Make it easier to test.
}

// ToAuthMiddleware converts an Authenticator to an auth.Middleware.
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

// NewAuthenticator creates a new token authenticator instance.
func NewAuthenticator(ctx context.Context, clusterRouter ClusterRouter, mgmtCtx *config.ScaledContext) Authenticator {
	tokenInformer := mgmtCtx.Management.Tokens("").Controller().Informer()
	// Deliberately ignore the error if the indexer was already added.
	_ = tokenInformer.AddIndexers(map[string]cache.IndexFunc{tokenKeyIndex: tokenKeyIndexer})
	providerRefresher := providerrefresh.NewUserAuthRefresher(ctx, mgmtCtx)

	return &tokenAuthenticator{
		ctx:                 ctx,
		tokenIndexer:        tokenInformer.GetIndexer(),
		tokenClient:         mgmtCtx.Wrangler.Mgmt.Token(),
		userAttributeLister: mgmtCtx.Management.UserAttributes("").Controller().Lister(),
		userAttributes:      mgmtCtx.Management.UserAttributes(""),
		userLister:          mgmtCtx.Management.Users("").Controller().Lister(),
		clusterRouter:       clusterRouter,
		refreshUser: func(userID string, force bool) {
			go providerRefresher.TriggerUserRefresh(userID, force)
		},
		now: time.Now,
	}
}

func tokenKeyIndexer(obj interface{}) ([]string, error) {
	token, ok := obj.(*v3.Token)
	if !ok {
		return []string{}, nil
	}

	return []string{token.Token}, nil
}

// Authenticate authenticates a request using a request's token.
func (a *tokenAuthenticator) Authenticate(req *http.Request) (*AuthenticatorResponse, error) {
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

	// If the auth provider is specified make sure it exists and enabled.
	if token.AuthProvider != "" {
		disabled, err := providers.IsDisabledProvider(token.AuthProvider)
		if err != nil {
			return nil, errors.Wrapf(ErrMustAuthenticate, "error checking if provider %s is disabled: %v", token.AuthProvider, err)
		}
		if disabled {
			return nil, errors.Wrapf(ErrMustAuthenticate, "provider %s is disabled", token.AuthProvider)
		}
	}

	attribs, err := a.userAttributeLister.Get("", token.UserID)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, errors.Wrapf(ErrMustAuthenticate, "failed to retrieve userattribute %s: %v", token.UserID, err)
	}

	authUser, err := a.userLister.Get("", token.UserID)
	if err != nil {
		return nil, errors.Wrapf(ErrMustAuthenticate, "failed to retrieve user %s: %v", token.UserID, err)
	}

	if authUser.Enabled != nil && !*authUser.Enabled {
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

	if !(authUser.IsSystem() || strings.HasPrefix(token.UserID, "system:")) {
		a.refreshUser(token.UserID, false)
	}

	extras := make(map[string][]string)
	if userExtra := getUserExtraInfo(token, authUser, attribs); userExtra != nil {
		extras = userExtra
	}

	// Add request-specific extra information.
	extras[ExtraRequestTokenID] = []string{token.Name}
	extras[ExtraRequestHost] = []string{req.Host}

	authResp := &AuthenticatorResponse{
		IsAuthed:      true,
		User:          token.UserID,
		UserPrincipal: token.UserPrincipal.Name,
		Groups:        groups,
		Extras:        extras,
	}

	logrus.Debugf("Extras returned %v", authResp.Extras)

	now := a.now().Truncate(time.Second) // Use the second precision.
	if token.LastUsedAt != nil {
		if now.Equal(token.LastUsedAt.Time.Truncate(time.Second)) {
			// Throttle subsecond updates.
			return authResp, nil
		}
	}

	if err := func() error {
		patch, err := json.Marshal([]struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value any    `json:"value"`
		}{{
			Op:    "replace",
			Path:  "/lastUsedAt",
			Value: metav1.NewTime(now),
		}})
		if err != nil {
			return err
		}

		_, err = a.tokenClient.Patch(token.Name, types.JSONPatchType, patch)
		return err
	}(); err != nil {
		// Log the error and move on to avoid failing the request.
		logrus.Errorf("Error updating lastUsedAt for token %s: %v", token.Name, err)
		return authResp, nil
	}

	logrus.Debugf("Updated lastUsedAt for token %s", token.Name)
	return authResp, nil
}

func getUserExtraInfo(token *v3.Token, user *v3.User, attribs *v3.UserAttribute) map[string][]string {
	extraInfo := make(map[string][]string)

	if attribs != nil && attribs.ExtraByProvider != nil && len(attribs.ExtraByProvider) != 0 {
		if token.AuthProvider == "local" || token.AuthProvider == "" {
			// Gather all extraInfo for all external auth providers present in the userAttributes.
			for _, extra := range attribs.ExtraByProvider {
				for key, value := range extra {
					extraInfo[key] = append(extraInfo[key], value...)
				}
			}
			return extraInfo
		}
		// AuthProvider is set in the token.
		if extraInfo, ok := attribs.ExtraByProvider[token.AuthProvider]; ok {
			return extraInfo
		}
	}

	extraInfo = providers.GetUserExtraAttributes(token.AuthProvider, token.UserPrincipal)
	// If principal id is not set in extra, read from user.
	if extraInfo != nil {
		if len(extraInfo[common.UserAttributePrincipalID]) == 0 {
			extraInfo[common.UserAttributePrincipalID] = user.PrincipalIDs
		}
		if len(extraInfo[common.UserAttributeUserName]) == 0 {
			extraInfo[common.UserAttributeUserName] = []string{user.DisplayName}
		}
	}

	return extraInfo
}

// TokenFromRequest retrieves and verifies the token from the request.
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
			return nil, errors.Wrapf(ErrMustAuthenticate, "failed to retrieve auth token, error: %v", err)
		}
	} else {
		storedToken = objs[0].(*v3.Token)
	}

	if _, err := tokens.VerifyToken(storedToken, tokenName, tokenKey); err != nil {
		return nil, errors.Wrapf(ErrMustAuthenticate, "failed to verify token: %v", err)
	}

	return storedToken, nil
}
