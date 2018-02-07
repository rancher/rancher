package requests

import (
	"context"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
)

type Authenticator interface {
	Authenticate(req *http.Request) (authed bool, user string, groups []string, err error)
	TokenFromRequest(req *http.Request) (*v3.Token, error)
}

func NewAuthenticator(ctx context.Context, mgmtCtx *config.ManagementContext) Authenticator {
	tokenInformer := mgmtCtx.Management.Tokens("").Controller().Informer()
	tokenInformer.AddIndexers(map[string]cache.IndexFunc{tokenKeyIndex: tokenKeyIndexer})

	return &tokenAuthenticator{
		ctx:          ctx,
		tokenIndexer: tokenInformer.GetIndexer(),
		tokenClient:  mgmtCtx.Management.Tokens(""),
	}
}

type tokenAuthenticator struct {
	ctx          context.Context
	tokenIndexer cache.Indexer
	tokenClient  v3.TokenInterface
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

	var groups []string
	for _, principal := range token.GroupPrincipals {
		// TODO This is a short cut for now. Will actually need to lookup groups in future
		name := strings.TrimPrefix(principal.Name, "local://")
		groups = append(groups, name)
	}

	return true, token.UserID, groups, nil
}

func (a *tokenAuthenticator) TokenFromRequest(req *http.Request) (*v3.Token, error) {
	tokenAuthValue := tokens.GetTokenAuthFromRequest(req)
	if tokenAuthValue == "" {
		return nil, fmt.Errorf("must log in")
	}

	tokenName, tokenKey := tokens.SplitTokenParts(tokenAuthValue)
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
			return nil, fmt.Errorf("failed to retrieve auth token, error: %#v", err)
		}
	} else {
		storedToken = objs[0].(*v3.Token)
	}

	if storedToken.Token != tokenKey || storedToken.ObjectMeta.Name != tokenName {
		return nil, fmt.Errorf("Invalid auth token value")
	}

	if !tokens.IsNotExpired(*storedToken) {
		return nil, fmt.Errorf("Auth Token has expired")
	}

	return storedToken, nil
}
