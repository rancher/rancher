package cognito

import (
	"context"

	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/genericoidc"
	baseoidc "github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
)

// CognitoProvider represents AWS Cognito auth provider
type CognitoProvider struct {
	genericoidc.GenOIDCProvider
}

const (
	Name = "cognito"
)

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	return &CognitoProvider{
		GenOIDCProvider: genericoidc.GenOIDCProvider{
			OpenIDCProvider: baseoidc.OpenIDCProvider{
				Name:        Name,
				Type:        client.CognitoConfigType,
				CTX:         ctx,
				AuthConfigs: mgmtCtx.Management.AuthConfigs(""),
				Secrets:     mgmtCtx.Wrangler.Core.Secret(),
				UserMGR:     userMGR,
				TokenMGR:    tokenMGR,
			},
		},
	}
}

// GetName returns the name of this provider.
func (c *CognitoProvider) GetName() string {
	return Name
}
