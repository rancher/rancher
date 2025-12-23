package cognito

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/rancher/norman/httperror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	baseoidc "github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
)

// CognitoProvider represents AWS Cognito auth provider
type CognitoProvider struct {
	genericoidc.GenOIDCProvider
}

const (
	Name = "cognito"
)

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMgr *tokens.Manager) common.AuthProvider {
	p := &CognitoProvider{
		GenOIDCProvider: genericoidc.GenOIDCProvider{
			OpenIDCProvider: baseoidc.OpenIDCProvider{
				Name:        Name,
				Type:        client.CognitoConfigType,
				CTX:         ctx,
				AuthConfigs: mgmtCtx.Management.AuthConfigs(""),
				Secrets:     mgmtCtx.Wrangler.Core.Secret(),
				UserMGR:     userMGR,
				TokenMgr:    tokenMgr,
			},
		},
	}
	p.GetConfig = p.GetOIDCConfig
	return p
}

// GetName returns the name of this provider.
func (p *CognitoProvider) GetName() string {
	return Name
}

func (p *CognitoProvider) Logout(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	providerName := token.GetAuthProvider()
	logrus.Debugf("CognitoProvider [logout]: triggered by provider %s", providerName)
	oidcConfig, err := p.GetConfig()
	if err != nil {
		return fmt.Errorf("getting config for OIDC Logout: %w", err)
	}
	if oidcConfig.LogoutAllForced {
		logrus.Debugf("CognitoProvider [logout]: Rancher provider resource `%v` configured for forced SLO, rejecting regular logout", providerName)
		return fmt.Errorf("CognitoProvider [logout]: Rancher provider resource `%v` configured for forced SLO, rejecting regular logout", providerName)
	}

	return nil
}

func (p *CognitoProvider) LogoutAll(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	logrus.Debugf("CognitoProvider [logout-all]: triggered by provider %s", token.GetAuthProvider())
	oidcConfig, err := p.GetConfig()
	if err != nil {
		return err
	}

	providerName := token.GetAuthProvider()
	if !oidcConfig.LogoutAllEnabled {
		logrus.Debugf("CognitoProvider [logout-all]: Rancher provider resource `%v` not configured for SLO", providerName)
		return fmt.Errorf("CognitoProvider [logout-all]: Rancher provider resource `%v` not configured for SLO", providerName)
	}

	idpRedirectURL, err := createIDPRedirectURL(r, oidcConfig)
	if err != nil {
		return err
	}

	data := map[string]any{
		"idpRedirectUrl": idpRedirectURL,
		"type":           "authConfigLogoutOutput",
		"baseType":       "authConfigLogoutOutput",
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}

// Based on https://docs.aws.amazon.com/cognito/latest/developerguide/logout-endpoint.html#get-logout
func createIDPRedirectURL(r *http.Request, config *v3.OIDCConfig) (string, error) {
	if config.EndSessionEndpoint == "" {
		return "", httperror.NewAPIError(httperror.ServerError, "LogoutAll triggered with no endSessionEndpoint")
	}

	idpRedirectURL, err := url.Parse(config.EndSessionEndpoint)
	if err != nil {
		logrus.Errorf("CognitoProvider: failed parsing end session endpoint: %v", err)
		return "", httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("CognitoProvider: parsing end session endpoint: %s", err))
	}

	authLogout := &v3.AuthConfigLogoutInput{}
	if err := json.NewDecoder(r.Body).Decode(authLogout); err != nil {
		return "", httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("CognitoProvider: parsing request body: %s", err))
	}

	params := idpRedirectURL.Query()
	params.Set("client_id", config.ClientID)
	params.Set("logout_uri", authLogout.FinalRedirectURL)
	idpRedirectURL.RawQuery = params.Encode()

	return idpRedirectURL.String(), nil
}
