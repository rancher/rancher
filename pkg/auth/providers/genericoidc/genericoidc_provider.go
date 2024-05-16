package genericoidc

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/pkg/errors"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	baseoidc "github.com/rancher/rancher/pkg/auth/providers/oidc"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type GenOIDCProvider struct {
	baseoidc.OpenIDCProvider
}

const (
	Name      = "genericoidc"
	UserType  = "user"
	GroupType = "group"
)

type ClaimInfo struct {
	Subject           string   `json:"sub"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	GivenName         string   `json:"given_name"`
	FamilyName        string   `json:"family_name"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	Groups            []string `json:"groups"`
	FullGroupPath     []string `json:"full_group_path"`
}

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

// SearchPrincipals will return a principal of the requested principalType with a displayName and loginName
// that match the searchValue.  This is done because OIDC does not have a proper lookup mechanism.  In order
// to provide some degree of functionality that allows manual entry for users/groups, this is the compromise.
func (g *GenOIDCProvider) SearchPrincipals(searchValue, principalType string, _ v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal

	if principalType == "" {
		principalType = UserType
	}

	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: g.Name + "_" + principalType + "://" + searchValue},
		DisplayName:   searchValue,
		LoginName:     searchValue,
		PrincipalType: principalType,
		Provider:      g.Name,
	}

	principals = append(principals, p)
	return principals, nil
}

func (g *GenOIDCProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	var p v3.Principal

	// parsing id to get the external id and type. Example genericoidc_<user|group>://<user sub | group name>
	var externalID string
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return p, errors.Errorf("invalid id %v", principalID)
	}
	externalID = strings.TrimPrefix(parts[1], "//")
	parts = strings.SplitN(parts[0], "_", 2)
	if len(parts) != 2 {
		return p, errors.Errorf("invalid id %v", principalID)
	}
	provider := parts[0]
	principalType := parts[1]
	if externalID == "" && principalType == "" {
		return p, fmt.Errorf("invalid id %v", principalID)
	}
	if principalType != UserType && principalType != GroupType {
		return p, fmt.Errorf("invalid principal type")
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

// GetOIDCConfig fetches necessary details from the AuthConfig object and returns the parts required in order
// to configure an OIDC library provider object.
func (g *GenOIDCProvider) GetOIDCConfig() (*v32.OIDCConfig, error) {
	authConfigObj, err := g.AuthConfigs.ObjectClient().UnstructuredClient().Get(g.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve OIDCConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve OIDCConfig, cannot read k8s Unstructured data")
	}
	storedOidcConfigMap := u.UnstructuredContent()

	storedOidcConfig := &v32.OIDCConfig{}
	err = common.Decode(storedOidcConfigMap, storedOidcConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode OidcConfig: %w", err)
	}

	if storedOidcConfig.PrivateKey != "" {
		value, err := common.ReadFromSecret(g.Secrets, storedOidcConfig.PrivateKey, strings.ToLower(client.GenericOIDCConfigFieldPrivateKey))
		if err != nil {
			return nil, err
		}
		storedOidcConfig.PrivateKey = value
	}
	if storedOidcConfig.ClientSecret != "" {
		data, err := common.ReadFromSecretData(g.Secrets, storedOidcConfig.ClientSecret)
		if err != nil {
			return nil, err
		}
		for _, v := range data {
			storedOidcConfig.ClientSecret = string(v)
		}
	}

	return storedOidcConfig, nil
}

// userToPrincipal takes user-related information from OIDC's UserInfo and combines it with information present
// in the claims to form and return a v3.Principal object.
func (g *GenOIDCProvider) userToPrincipal(userInfo *oidc.UserInfo, claimInfo ClaimInfo) v3.Principal {
	displayName := claimInfo.Name
	if displayName == "" {
		displayName = userInfo.Email
	}
	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: g.Name + "_" + UserType + "://" + userInfo.Subject},
		DisplayName:   displayName,
		LoginName:     userInfo.Email,
		Provider:      g.Name,
		PrincipalType: UserType,
		Me:            false,
	}
	return p
}

// getGroupsFromClaimInfo takes the claims that we get from the OIDC provider and returns a slice of groupPrincipals.
func (g *GenOIDCProvider) getGroupsFromClaimInfo(claimInfo ClaimInfo) []v3.Principal {
	var groupPrincipals []v3.Principal

	if claimInfo.FullGroupPath != nil {
		for _, groupPath := range claimInfo.FullGroupPath {
			groupsFromPath := strings.Split(groupPath, "/")
			for _, group := range groupsFromPath {
				if group != "" {
					groupPrincipal := g.groupToPrincipal(group)
					groupPrincipal.MemberOf = true
					groupPrincipals = append(groupPrincipals, groupPrincipal)
				}
			}
		}
	} else {
		for _, group := range claimInfo.Groups {
			groupPrincipal := g.groupToPrincipal(group)
			groupPrincipal.MemberOf = true
			groupPrincipals = append(groupPrincipals, groupPrincipal)
		}
	}
	return groupPrincipals
}

// groupToPrincipal takes a bare group name and turns it into a v3.Principal group object by filling-in other fields
// with basic provider information.
func (g *GenOIDCProvider) groupToPrincipal(groupName string) v3.Principal {
	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: g.Name + "_" + GroupType + "://" + groupName},
		DisplayName:   groupName,
		Provider:      g.Name,
		PrincipalType: GroupType,
		Me:            false,
	}
	return p
}

// getRedirectURL uses the AuthConfig map to build-up the redirect URL passed to the OIDC provider at login-time.
func (g *GenOIDCProvider) getRedirectURL(config map[string]interface{}) string {
	authURL, _ := baseoidc.FetchAuthURL(config)

	return fmt.Sprintf(
		"%s?client_id=%s&response_type=code&redirect_uri=%s",
		authURL,
		config["clientId"],
		config["rancherUrl"],
	)
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
