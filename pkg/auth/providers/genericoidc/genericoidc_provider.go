package genericoidc

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	baseoidc "github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GenOIDCProvider struct {
	baseoidc.OpenIDCProvider
}

const (
	Name      = "genericoidc"
	UserType  = "user"
	GroupType = "group"
)

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	return &GenOIDCProvider{
		baseoidc.OpenIDCProvider{
			Name:        Name,
			Type:        client.GenericOIDCConfigType,
			CTX:         ctx,
			AuthConfigs: mgmtCtx.Management.AuthConfigs(""),
			Secrets:     mgmtCtx.Core.Secrets(""),
			UserMGR:     userMGR,
			TokenMGR:    tokenMGR,
		},
	}
}

// GetName returns the name of this provider.
func (g *GenOIDCProvider) GetName() string {
	return Name
}

// SearchPrincipals will return a principal of the requested principalType with a displayName
// that matches the searchValue.  If principalType is empty, both a user principal and a group principal will
// be returned.  This is done because OIDC does not have a proper lookup mechanism.  In order
// to provide some degree of functionality that allows manual entry for users/groups, this is the compromise.
func (g *GenOIDCProvider) SearchPrincipals(searchValue, principalType string, _ v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal

	if principalType != GroupType {
		p := v3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: g.Name + "_" + UserType + "://" + searchValue},
			DisplayName:   searchValue,
			LoginName:     searchValue,
			PrincipalType: UserType,
			Provider:      g.Name,
		}
		principals = append(principals, p)
	}

	if principalType != UserType {
		gp := v3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: g.Name + "_" + GroupType + "://" + searchValue},
			DisplayName:   searchValue,
			PrincipalType: GroupType,
			Provider:      g.Name,
		}
		principals = append(principals, gp)
	}
	return principals, nil
}

func (g *GenOIDCProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	var p v3.Principal

	// parsing id to get the external id and type. Example genericoidc_<user|group>://<user sub | group name>
	principalScheme, externalID, found := strings.Cut(principalID, "://")
	if !found {
		return p, fmt.Errorf("invalid principal id: %s", principalID)
	}
	provider, principalType, found := strings.Cut(principalScheme, "_")
	if !found {
		return p, fmt.Errorf("invalid principal scheme: %s", principalScheme)
	}

	if externalID == "" && principalType == "" {
		return p, fmt.Errorf("invalid id %v", principalID)
	}
	if principalType != UserType && principalType != GroupType {
		return p, fmt.Errorf("invalid principal type: %s", principalType)
	}
	if principalType == UserType {
		p = v3.Principal{
			ObjectMeta:    metav1.ObjectMeta{Name: provider + "_" + principalType + "://" + externalID},
			DisplayName:   externalID,
			LoginName:     externalID,
			PrincipalType: UserType,
			Provider:      g.Name,
		}
	} else {
		p = g.groupToPrincipal(externalID)
	}
	p = g.toPrincipalFromToken(principalType, p, &token)
	return p, nil
}

// TransformToAuthProvider yields information used, typically by the UI, to be able to form URLs used to perform login.
func (g *GenOIDCProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	p := common.TransformToAuthProvider(authConfig)
	p[publicclient.GenericOIDCProviderFieldRedirectURL] = g.getRedirectURL(authConfig)
	p[publicclient.GenericOIDCProviderFieldScopes] = authConfig["scope"]
	return p, nil
}

// RefetchGroupPrincipals is not implemented for OIDC.
func (g *GenOIDCProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	return nil, errors.New("Not implemented")
}

// groupToPrincipal takes a bare group name and turns it into a v3.Principal group object by filling-in other fields
// with basic provider information.
func (g *GenOIDCProvider) groupToPrincipal(groupName string) v3.Principal {
	return v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: g.Name + "_" + GroupType + "://" + groupName},
		DisplayName:   groupName,
		Provider:      g.Name,
		PrincipalType: GroupType,
		Me:            false,
	}
}

// getRedirectURL uses the AuthConfig map to build-up the redirect URL passed to the OIDC provider at login-time.
func (g *GenOIDCProvider) getRedirectURL(config map[string]interface{}) string {
	authURL, _ := baseoidc.FetchAuthURL(config)

	redirectURL := fmt.Sprintf(
		"%s?client_id=%s&response_type=code&redirect_uri=%s",
		authURL,
		config["clientId"],
		config["rancherUrl"],
	)

	if config["acrValue"] != nil {
		redirectURL += fmt.Sprintf("&acr_values=%s", config["acrValue"])
	}

	return redirectURL
}

// toPrincipalFromToken uses additional information about the principal found in the token, if available, to provide
// a more detailed, useful Principal object.
func (g *GenOIDCProvider) toPrincipalFromToken(principalType string, princ v3.Principal, token *v3.Token) v3.Principal {
	if principalType == UserType {
		princ.PrincipalType = UserType
		if token != nil {
			princ.Me = g.IsThisUserMe(token.UserPrincipal, princ)
			if princ.Me {
				princ.LoginName = token.UserPrincipal.LoginName
				princ.DisplayName = token.UserPrincipal.DisplayName
			}
		}
	} else {
		princ.PrincipalType = GroupType
		if token != nil {
			princ.MemberOf = g.TokenMGR.IsMemberOf(*token, princ)
		}
	}
	return princ
}
