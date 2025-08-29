package cognito

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
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
func (c *CognitoProvider) GetName() string {
	return Name
}

func (s *CognitoProvider) Logout(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	providerName := token.GetAuthProvider()
	logrus.Debugf("CognitoProvider [logout]: triggered by provider %s", providerName)
	oidcConfig, err := s.GetConfig()
	if err != nil {
		return fmt.Errorf("getting config for OIDC Logout: %w", err)
	}
	if oidcConfig.LogoutAllForced {
		logrus.Debugf("CognitoProvider [logout]: Rancher provider resource `%v` configured for forced SLO, rejecting regular logout", providerName)
		return fmt.Errorf("CognitoProvider [logout]: Rancher provider resource `%v` configured for forced SLO, rejecting regular logout", providerName)
	}

	return nil
}

func (s *CognitoProvider) LogoutAll(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	logrus.Debugf("CognitoProvider [logout-all]: triggered by provider %s", token.GetAuthProvider())
	oidcConfig, err := s.GetConfig()
	if err != nil {
		return err
	}

	idpRedirectURL, err := createIDPRedirectURL(apiContext, oidcConfig)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"idpRedirectUrl": idpRedirectURL,
		"type":           "oidcConfigLogoutOutput",
	}
	apiContext.WriteResponse(http.StatusOK, data)

	return nil
}

// Based on https://docs.aws.amazon.com/cognito/latest/developerguide/logout-endpoint.html#get-logout
func createIDPRedirectURL(apiContext *types.APIContext, config *v3.OIDCConfig) (string, error) {
	if config.EndSessionEndpoint == "" {
		logrus.Debug("OIDC: LogoutAll not redirecting without endSessionEndpoint")
		return "", nil
	}

	idpRedirectURL, err := url.Parse(config.EndSessionEndpoint)
	if err != nil {
		logrus.Infof("OIDC: failed parsing end session endpoint: %v", err)
		return "", httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("OIDC: parsing end session endpoint: %s", err))
	}

	oidcLogout := &v3.OIDCConfigLogoutInput{}
	r := apiContext.Request
	if err := json.NewDecoder(r.Body).Decode(oidcLogout); err != nil {
		return "", httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("OIDC: parsing request body: %s", err))
	}

	params := idpRedirectURL.Query()
	params.Set("client_id", config.ClientID)
	params.Set("logout_uri", oidcLogout.FinalRedirectURL)
	idpRedirectURL.RawQuery = params.Encode()

	return idpRedirectURL.String(), nil
}
