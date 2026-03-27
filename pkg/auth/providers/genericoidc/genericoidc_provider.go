package genericoidc

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	baseoidc "github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GenOIDCProvider struct {
	baseoidc.OpenIDCProvider
}

const (
	ProviderName = "genericoidc"
	UserType     = "user"
	GroupType    = "group"
)

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	p := &GenOIDCProvider{
		baseoidc.OpenIDCProvider{
			Name:         ProviderName,
			Type:         client.GenericOIDCConfigType,
			CTX:          ctx,
			AuthConfigs:  mgmtCtx.Management.AuthConfigs(""),
			Secrets:      mgmtCtx.Wrangler.Core.Secret(),
			UserMGR:      userMGR,
			TokenMgr:     tokenMGR,
			UserSearcher: common.NewUserSearcher(mgmtCtx.Management.Users("").Controller().Lister()),
		},
	}
	p.GetConfig = p.GetOIDCConfig
	return p
}

// GetName returns the name of this provider.
func (g *GenOIDCProvider) GetName() string {
	return ProviderName
}

// SearchPrincipals will return the users Rancher already knows about whose
// display name or username matches the searchValue, followed by a principal of
// the requested principalType holding the searchValue itself.
// If principalType is empty, both a user principal and a group principal will
// be returned.  This is done because OIDC does not have a proper lookup
// mechanism, so that last principal lets an admin enter a subject or group ID by
// hand for an identity Rancher has not seen yet.
func (g *GenOIDCProvider) SearchPrincipals(searchValue, principalType string, token accessor.TokenAccessor) ([]apiv3.Principal, error) {
	// TODO: This will not work across providers.
	configName, err := common.ConfigNameFromToken(token)
	if err != nil {
		return nil, err
	}

	var principals []apiv3.Principal
	if principalType != GroupType {
		fromSearchValue := apiv3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: configName + "_" + UserType + "://" + searchValue},
			DisplayName:   searchValue,
			LoginName:     searchValue,
			PrincipalType: UserType,
			Provider:      g.Name,
		}

		users, err := common.PrincipalsWithFallback(g.UserSearcher, g.Name, searchValue, fromSearchValue)
		if err != nil {
			return nil, err
		}
		principals = append(principals, users...)
	}

	if principalType != UserType {
		gp := apiv3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: g.Name + "_" + GroupType + "://" + searchValue},
			DisplayName:   searchValue,
			PrincipalType: GroupType,
			Provider:      g.Name,
		}
		principals = append(principals, gp)
	}
	return principals, nil
}

func (g *GenOIDCProvider) GetPrincipal(principalID string, token accessor.TokenAccessor) (apiv3.Principal, error) {
	var p apiv3.Principal
	// TODO: this should compare the principalID configName and the token configName
	// And return an error if they don't match?

	principalConfigName, principalType, externalID, err := common.SplitPrincipalID(principalID)
	if err != nil {
		return p, err
	}
	if externalID == "" && principalType == "" {
		return p, fmt.Errorf("invalid id %v", principalID)
	}
	if principalType != UserType && principalType != GroupType {
		return p, fmt.Errorf("invalid principal type: %s", principalType)
	}
	if principalType == UserType {
		p = apiv3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: principalConfigName + "_" + principalType + "://" + externalID},
			DisplayName:   externalID,
			LoginName:     externalID,
			PrincipalType: UserType,
			Provider:      g.Name,
		}
	} else {
		p = g.groupToPrincipal(externalID, principalConfigName)
	}
	p = g.toPrincipalFromToken(principalType, p, token)

	return p, nil
}

// TransformToAuthProvider yields information used, typically by the UI, to be able to form URLs used to perform login.
func (g *GenOIDCProvider) TransformToAuthProvider(authConfig map[string]any) (map[string]any, error) {
	p, err := g.OpenIDCProvider.TransformToAuthProvider(authConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to transform auth config: %w", err)
	}

	if authConfig["acrValue"] != nil {
		redirectURL := p[publicclient.GenericOIDCProviderFieldRedirectURL].(string)
		p[publicclient.GenericOIDCProviderFieldRedirectURL] = redirectURL + fmt.Sprintf("&acr_values=%s", authConfig["acrValue"])
	}

	p[publicclient.GenericOIDCProviderFieldScopes] = authConfig["scope"]

	return p, nil
}

// RefetchGroupPrincipals is not implemented for OIDC.
func (g *GenOIDCProvider) RefetchGroupPrincipals(principalID string, secret string) ([]apiv3.Principal, error) {
	return nil, errors.New("Not implemented")
}

func (g *GenOIDCProvider) UsesUserSecrets() bool      { return false }
func (g *GenOIDCProvider) CanRefreshPrincipals() bool { return false }

// groupToPrincipal takes a bare group name and turns it into a apiv3.Principal group object by filling-in other fields
// with basic provider information.
func (g *GenOIDCProvider) groupToPrincipal(configName, groupName string) apiv3.Principal {
	return apiv3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: configName + "_" + GroupType + "://" + groupName},
		DisplayName:   groupName,
		Provider:      g.Name,
		PrincipalType: GroupType,
		Me:            false,
	}
}

// toPrincipalFromToken uses additional information about the principal found in the token, if available, to provide
// a more detailed, useful Principal object.
func (g *GenOIDCProvider) toPrincipalFromToken(principalType string, princ apiv3.Principal, token accessor.TokenAccessor) apiv3.Principal {
	if principalType == UserType {
		princ.PrincipalType = UserType
		if token != nil {
			princ.Me = g.IsThisUserMe(token.GetUserPrincipal(), princ)
			if princ.Me {
				tokenPrincipal := token.GetUserPrincipal()
				princ.LoginName = tokenPrincipal.LoginName
				princ.DisplayName = tokenPrincipal.DisplayName
			}
		}
	} else {
		princ.PrincipalType = GroupType
		if token != nil {
			princ.MemberOf = g.UserMGR.IsMemberOf(token, princ)
		}
	}

	return princ
}
