package keycloakoidc

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/pkg/errors"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	Name      = "keycloakoidc"
	UserType  = "user"
	GroupType = "group"
)

type keyCloakOIDCProvider struct {
	oidc.OpenIDCProvider
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	return &keyCloakOIDCProvider{
		oidc.OpenIDCProvider{
			Name:        Name,
			Type:        client.KeyCloakOIDCConfigType,
			CTX:         ctx,
			AuthConfigs: mgmtCtx.Management.AuthConfigs(""),
			Secrets:     mgmtCtx.Core.Secrets(""),
			UserMGR:     userMGR,
			TokenMGR:    tokenMGR,
		},
	}
}

func (k *keyCloakOIDCProvider) GetName() string {
	return Name
}

func (k *keyCloakOIDCProvider) newClient(config *v32.OIDCConfig, token v3.Token) (*KeyCloakClient, error) {
	// creating context for new client and for refreshing oauth token if needed
	ctx, err := oidc.AddCertKeyToContext(context.Background(), config.Certificate, config.PrivateKey)
	if err != nil {
		return nil, err
	}
	provider, err := gooidc.NewProvider(ctx, config.Issuer)
	if err != nil {
		return nil, err
	}
	oauthConfig := oidc.ConfigToOauthConfig(provider.Endpoint(), config)
	// get, refresh and update token
	oauthToken, err := k.getRefreshAndUpdateToken(ctx, oauthConfig, token)
	if err != nil {
		return nil, err
	}
	keyCloakClient := &KeyCloakClient{
		httpClient: oauthConfig.Client(ctx, oauthToken),
	}

	return keyCloakClient, err
}

func (k *keyCloakOIDCProvider) SearchPrincipals(searchValue, principalType string, token v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal
	var err error

	config, err := k.GetOIDCConfig()
	if err != nil {
		return principals, err
	}
	keyCloakClient, err := k.newClient(config, token)
	if err != nil {
		logrus.Errorf("[keycloak oidc] SsearchPrincipals: error creating new http client: %v", err)
		return principals, err
	}
	accts, err := keyCloakClient.searchPrincipals(searchValue, principalType, config)
	if err != nil {
		logrus.Errorf("[keycloak oidc] SearchPrincipals: problem searching keycloak: %v", err)
		return principals, err
	}
	for _, acct := range accts {
		p := k.toPrincipal(acct.Type, acct, &token)
		principals = append(principals, p)
	}
	return principals, nil
}

func (k *keyCloakOIDCProvider) toPrincipal(principalType string, acct account, token *v3.Token) v3.Principal {
	displayName := acct.Name
	if displayName == "" {
		displayName = acct.Username
	}
	princ := v3.Principal{
		ObjectMeta:  metav1.ObjectMeta{Name: k.GetName() + "_" + principalType + "://" + acct.ID},
		DisplayName: displayName,
		LoginName:   acct.Username,
		Provider:    k.GetName(),
		Me:          false,
	}

	if principalType == UserType {
		princ.PrincipalType = UserType
		if token != nil {
			princ.Me = k.IsThisUserMe(token.UserPrincipal, princ)
		}
	} else {
		princ.PrincipalType = GroupType
		princ.ObjectMeta = metav1.ObjectMeta{Name: k.GetName() + "_" + principalType + "://" + acct.Name}
		if token != nil {
			princ.MemberOf = k.TokenMGR.IsMemberOf(*token, princ)
		}
	}
	return princ
}

func (k *keyCloakOIDCProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	config, err := k.GetOIDCConfig()
	if err != nil {
		return v3.Principal{}, err
	}
	var externalID string
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return v3.Principal{}, errors.Errorf("invalid id %v", principalID)
	}
	externalID = strings.TrimPrefix(parts[1], "//")
	parts = strings.SplitN(parts[0], "_", 2)
	if len(parts) != 2 {
		return v3.Principal{}, errors.Errorf("invalid id %v", principalID)
	}
	principalType := parts[1]
	keyCloakClient, err := k.newClient(config, token)
	if err != nil {
		logrus.Errorf("[keycloak oidc] GetPrincipal: error creating new http client: %v", err)
		return v3.Principal{}, err
	}
	acct, err := keyCloakClient.getFromKeyCloakByID(externalID, principalType, config)
	if err != nil {
		return v3.Principal{}, err
	}
	princ := k.toPrincipal(principalType, acct, &token)
	return princ, err
}

func (k *keyCloakOIDCProvider) getRefreshAndUpdateToken(ctx context.Context, oauthConfig oauth2.Config, token v3.Token) (*oauth2.Token, error) {
	var oauthToken *oauth2.Token
	storedOauthToken, err := k.TokenMGR.GetSecret(token.UserID, token.AuthProvider, []*v3.Token{&token})
	if err := json.Unmarshal([]byte(storedOauthToken), &oauthToken); err != nil {
		return oauthToken, err
	}
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return oauthToken, err
		}
		oauthToken.AccessToken = token.ProviderInfo["access_token"]
	}
	// Valid will return false if access token is expired
	if !oauthToken.Valid() {
		// since token is not valid, the TokenSource func used in the Client func will attempt to refresh the access token
		// if the refresh token has not expired
		logrus.Debugf("[generic oidc] RefeshAndUpdateToken: attempting to refresh access token")
	}
	reusedToken, err := oauth2.ReuseTokenSource(oauthToken, oauthConfig.TokenSource(ctx, oauthToken)).Token()
	if err != nil {
		return oauthToken, err
	}

	if !reflect.DeepEqual(oauthToken, reusedToken) {
		k.UpdateToken(reusedToken, token.UserID)
	}
	return reusedToken, nil
}
