package azuread

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	Name      = "azuread"
	userType  = "user"
	groupType = "group"
)

type azureADProvider struct {
	ctx           context.Context
	authConfigs   v3.AuthConfigInterface
	azureADClient *Client
	userMGR       user.Manager
	tokenClient   v3.TokenInterface
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager) common.AuthProvider {
	azureADClient := &Client{
		httpClient: &http.Client{},
	}

	return &azureADProvider{
		ctx:           ctx,
		authConfigs:   mgmtCtx.Management.AuthConfigs(""),
		azureADClient: azureADClient,
		userMGR:       userMGR,
		tokenClient:   mgmtCtx.Management.Tokens(""),
	}
}

func (p *azureADProvider) GetName() string {
	return Name
}

func (p *azureADProvider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = p.actionHandler
	schema.Formatter = p.formatter
}

func (p *azureADProvider) TransformToAuthProvider(authConfig map[string]interface{}) map[string]interface{} {
	azureADP := common.TransformToAuthProvider(authConfig)
	return azureADP
}

func (p *azureADProvider) AuthenticateUser(input interface{}) (v3.Principal, []v3.Principal, map[string]string, error) {
	azureADCredential, ok := input.(*v3public.BasicLogin)
	if !ok {
		return v3.Principal{}, nil, nil, errors.New("unexpected input type")
	}

	return p.loginUser(azureADCredential, nil, false)
}

func (p *azureADProvider) SearchPrincipals(searchkey, principalType string, token v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal
	var err error

	config, err := p.getAzureADConfig()
	if err != nil {
		return principals, nil
	}
	azureAccessToken := token.ProviderInfo["access_token"]

	var currentPrincipalType string
	if principalType == "" || principalType == userType {
		currentPrincipalType = userType
		users, providerInfo, err := p.azureADClient.searchUsers(searchkey, azureAccessToken, config)
		if err != nil {
			return nil, err
		}
		if len(providerInfo) != 0 {
			token.ProviderInfo = providerInfo
			tu, err := p.tokenClient.Update(&token)
			if err != nil {
				return []v3.Principal{}, fmt.Errorf("failed to update token %v, err: %v", tu, err)
			}
		}
		for _, acct := range users {
			p := p.toPrincipal(currentPrincipalType, acct, nil)
			principals = append(principals, p)
		}
	}

	if principalType == "" || principalType == groupType {
		currentPrincipalType = groupType
		groups, providerInfo, err := p.azureADClient.searchGroups(searchkey, azureAccessToken, config)
		if err != nil {
			return nil, err
		}
		if len(providerInfo) != 0 {
			token.ProviderInfo = providerInfo
			tg, err := p.tokenClient.Update(&token)
			if err != nil {
				return []v3.Principal{}, fmt.Errorf("failed to update token %v, err: %v", tg, err)
			}
		}
		for _, acct := range groups {
			p := p.toPrincipal(currentPrincipalType, acct, nil)
			principals = append(principals, p)
		}
	}

	for _, principal := range principals {
		if principal.PrincipalType == userType {
			if p.isThisUserMe(token.UserPrincipal, principal) {
				principal.Me = true
			}
		} else if principal.PrincipalType == groupType {
			if p.isMemberOf(token.GroupPrincipals, principal) {
				principal.MemberOf = true
			}
		}
	}

	return principals, nil
}

func (p *azureADProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	config, err := p.getAzureADConfig()
	if err != nil {
		return v3.Principal{}, nil
	}
	accessToken := token.ProviderInfo["access_token"]

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
	var acct Account
	switch principalType {
	case userType:
		useracct, providerInfo, err := p.azureADClient.getUserByID(externalID, accessToken, config)
		if err != nil {
			return v3.Principal{}, err
		}
		if len(providerInfo) != 0 {
			token.ProviderInfo = providerInfo
			tu, err := p.tokenClient.Update(&token)
			if err != nil {
				return v3.Principal{}, fmt.Errorf("failed to update token %v, err: %v", tu, err)
			}
		}
		acct = useracct
	case groupType:
		groupacct, providerInfo, err := p.azureADClient.getGroupByID(externalID, accessToken, config)
		if err != nil {
			return v3.Principal{}, err
		}
		if len(providerInfo) != 0 {
			token.ProviderInfo = providerInfo
			tg, err := p.tokenClient.Update(&token)
			if err != nil {
				return v3.Principal{}, fmt.Errorf("failed to update token %v, err: %v", tg, err)
			}
		}
		acct = groupacct
	default:
		return v3.Principal{}, fmt.Errorf("Cannot get the azuread account due to invalid externalIDType %v", principalType)
	}

	princ := p.toPrincipal(principalType, acct, &token)
	return princ, nil
}

func (p *azureADProvider) isThisUserMe(me v3.Principal, other v3.Principal) bool {
	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}

func (p *azureADProvider) isMemberOf(myGroups []v3.Principal, other v3.Principal) bool {
	for _, mygroup := range myGroups {
		if mygroup.ObjectMeta.Name == other.ObjectMeta.Name && mygroup.PrincipalType == other.PrincipalType {
			return true
		}
	}
	return false
}

func (p *azureADProvider) getAzureADConfig() (*v3.AzureADConfig, error) {
	authConfigObj, err := p.authConfigs.ObjectClient().UnstructuredClient().Get(Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AzureADConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve AzureADConfig, cannot read k8s Unstructured data")
	}
	storedAzureADConfigMap := u.UnstructuredContent()

	storedAzureADConfig := &v3.AzureADConfig{}
	mapstructure.Decode(storedAzureADConfigMap, storedAzureADConfig)

	metadataMap, ok := storedAzureADConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to retrieve AzureADConfig metadata, cannot read k8s Unstructured data")
	}

	objectMeta := &metav1.ObjectMeta{}
	mapstructure.Decode(metadataMap, objectMeta)
	storedAzureADConfig.ObjectMeta = *objectMeta

	return storedAzureADConfig, nil
}

func (p *azureADProvider) loginUser(azureADCredential *v3public.BasicLogin, azureADConfig *v3.AzureADConfig, test bool) (v3.Principal, []v3.Principal, map[string]string, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal
	var providerInfo = make(map[string]string)
	var err error

	if azureADConfig == nil {
		azureADConfig, err = p.getAzureADConfig()
		if err != nil {
			return v3.Principal{}, nil, nil, err
		}
	}

	accessToken, refreshToken, err := p.azureADClient.getAccessToken(azureADCredential, azureADConfig)
	if err != nil {
		logrus.Infof("Error generating accessToken from azure %v", err)
		return v3.Principal{}, nil, nil, err
	}
	logrus.Debugf("Received AccessToken from azure %v", accessToken)

	providerInfo["access_token"] = accessToken //set accessToken token when login
	providerInfo["refresh_token"] = refreshToken

	userAcct, newProviderInfo, err := p.azureADClient.getUser(accessToken, azureADConfig)
	if err != nil {
		return v3.Principal{}, nil, nil, err
	}
	if len(newProviderInfo) != 0 {
		providerInfo = newProviderInfo
	}
	userPrincipal = p.toPrincipal(userType, userAcct, nil)
	userPrincipal.Me = true

	groupAccts, newProviderInfo, err := p.azureADClient.getGroups(accessToken, azureADConfig)
	if err != nil {
		return v3.Principal{}, nil, nil, err
	}
	if len(newProviderInfo) != 0 {
		providerInfo = newProviderInfo
	}
	for _, groupAcct := range groupAccts {
		groupPrincipal := p.toPrincipal(groupType, groupAcct, nil)
		groupPrincipal.MemberOf = true
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	testAllowedPrincipals := azureADConfig.AllowedPrincipalIDs
	if test && azureADConfig.AccessMode == "restricted" {
		testAllowedPrincipals = append(testAllowedPrincipals, userPrincipal.Name)
	}

	allowed, err := p.userMGR.CheckAccess(azureADConfig.AccessMode, testAllowedPrincipals, userPrincipal, groupPrincipals)
	if err != nil {
		return v3.Principal{}, nil, nil, err
	}
	if !allowed {
		return v3.Principal{}, nil, nil, httperror.NewAPIError(httperror.Unauthorized, "unauthorized")
	}

	return userPrincipal, groupPrincipals, providerInfo, nil
}

func (p *azureADProvider) toPrincipal(principalType string, acct Account, token *v3.Token) v3.Principal {
	displayName := acct.DisplayName
	if displayName == "" {
		displayName = acct.AccountName
	}

	if principalType == "" {
		principalType = userType
	}
	princ := v3.Principal{
		ObjectMeta:  metav1.ObjectMeta{Name: Name + "_" + principalType + "://" + acct.ObjectID},
		DisplayName: displayName,
		LoginName:   acct.AccountName,
		Provider:    Name,
		Me:          false,
	}

	if principalType == userType {
		princ.PrincipalType = userType
		if token != nil {
			princ.Me = p.isThisUserMe(token.UserPrincipal, princ)
		}
	} else {
		princ.PrincipalType = groupType
		if token != nil {
			princ.MemberOf = p.isMemberOf(token.GroupPrincipals, princ)
		}
	}

	return princ
}
