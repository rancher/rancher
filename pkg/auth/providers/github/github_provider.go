package github

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//Constants for github
const (
	Name = "github"
)

//GProvider implements an PrincipalProvider for github
type GProvider struct {
	ctx          context.Context
	authConfigs  v3.AuthConfigInterface
	githubClient *GClient
}

func Configure(ctx context.Context, mgmtCtx *config.ManagementContext) *GProvider {
	githubClient := &GClient{
		httpClient: &http.Client{},
	}

	return &GProvider{
		ctx:          ctx,
		authConfigs:  mgmtCtx.Management.AuthConfigs(""),
		githubClient: githubClient,
	}
}

//GetName returns the name of the provider
func (g *GProvider) GetName() string {
	return Name
}

func (g *GProvider) getGithubConfigCR() (*v3.GithubConfig, error) {

	authConfigObj, err := g.authConfigs.ObjectClient().UnstructuredClient().Get("github", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve GithubConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve GithubConfig, cannot read k8s Unstructured data")
	}
	storedGithubConfigMap := u.UnstructuredContent()

	storedGithubConfig := &v3.GithubConfig{}
	mapstructure.Decode(storedGithubConfigMap, storedGithubConfig)

	metadataMap, ok := storedGithubConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to retrieve GithubConfig metadata, cannot read k8s Unstructured data")
	}

	typemeta := &metav1.ObjectMeta{}
	mapstructure.Decode(metadataMap, typemeta)
	storedGithubConfig.ObjectMeta = *typemeta

	return storedGithubConfig, nil
}

func (g *GProvider) SaveGithubConfig(config *v3.GithubConfig) error {
	storedGithubConfig, err := g.getGithubConfigCR()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = client.GithubConfigType
	config.ObjectMeta = storedGithubConfig.ObjectMeta

	logrus.Debugf("updating githubConfig")
	_, err = g.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}

func (g *GProvider) AuthenticateUser(input interface{}) (v3.Principal, []v3.Principal, map[string]string, int, error) {
	login, ok := input.(*v3public.GithubLogin)
	if !ok {
		return v3.Principal{}, nil, nil, 500, errors.New("unexpected input type")
	}

	return g.LoginUser(login, nil)
}

func (g *GProvider) LoginUser(githubCredential *v3public.GithubLogin, config *v3.GithubConfig) (v3.Principal, []v3.Principal, map[string]string, int, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal
	var providerInfo = make(map[string]string)
	var err error

	if config == nil {
		config, err = g.getGithubConfigCR()
		if err != nil {
			return userPrincipal, groupPrincipals, providerInfo, 401, err
		}
	}

	securityCode := githubCredential.Code

	logrus.Debugf("GitHubIdentityProvider AuthenticateUser called for securityCode %v", securityCode)
	accessToken, err := g.githubClient.getAccessToken(securityCode, config)
	if err != nil {
		logrus.Infof("Error generating accessToken from github %v", err)
		return userPrincipal, groupPrincipals, providerInfo, 401, err
	}
	logrus.Debugf("Received AccessToken from github %v", accessToken)

	providerInfo["access_token"] = accessToken

	user, err := g.githubClient.getGithubUser(accessToken, config)
	if err != nil {
		return userPrincipal, groupPrincipals, providerInfo, 401, fmt.Errorf("Error getting github user %v", err)
	}
	userPrincipal = v3.Principal{
		ObjectMeta:  metav1.ObjectMeta{Name: Name + "_user://" + strconv.Itoa(user.ID)},
		DisplayName: user.Name,
		LoginName:   user.Login,
		Kind:        "user",
		Me:          true,
	}

	orgAccts, err := g.githubClient.getGithubOrgs(accessToken, config)
	if err != nil {
		logrus.Errorf("Failed to get orgs for github user: %v, err: %v", user.Name, err)
		return userPrincipal, groupPrincipals, providerInfo, 500, fmt.Errorf("Error getting orgs for github user %v", err)
	}

	for _, orgAcct := range orgAccts {
		groupPrincipal := v3.Principal{
			ObjectMeta:  metav1.ObjectMeta{Name: Name + "_org://" + strconv.Itoa(orgAcct.ID)},
			DisplayName: orgAcct.Name,
			Kind:        "group",
		}
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}

	teamAccts, err := g.githubClient.getGithubTeams(accessToken, config)
	if err != nil {
		logrus.Errorf("Failed to get teams for github user: %v, err: %v", user.Name, err)
		return userPrincipal, groupPrincipals, providerInfo, 500, fmt.Errorf("Error getting teams for github user %v", err)
	}
	for _, teamAcct := range teamAccts {
		groupPrincipal := v3.Principal{
			ObjectMeta:  metav1.ObjectMeta{Name: Name + "_team://" + strconv.Itoa(teamAcct.ID)},
			DisplayName: teamAcct.Name,
			Kind:        "group",
		}
		groupPrincipals = append(groupPrincipals, groupPrincipal)
	}
	return userPrincipal, groupPrincipals, providerInfo, 0, nil
}

func (g *GProvider) SearchPrincipals(searchKey string, myToken v3.Token) ([]v3.Principal, int, error) {
	var principals []v3.Principal
	var err error

	//is this github token?
	if myToken.AuthProvider != g.GetName() {
		return principals, 0, nil
	}

	config, err := g.getGithubConfigCR()
	if err != nil {
		return principals, 0, nil
	}

	accessToken := myToken.ProviderInfo["access_token"]

	user, err := g.githubClient.getGithubUserByName(searchKey, accessToken, config)
	if err == nil {
		userPrincipal := v3.Principal{
			ObjectMeta:  metav1.ObjectMeta{Name: Name + "_user://" + strconv.Itoa(user.ID)},
			DisplayName: user.Name,
			LoginName:   user.Login,
			Kind:        "user",
			Me:          false,
		}
		if g.isThisUserMe(myToken.UserPrincipal, userPrincipal) {
			userPrincipal.Me = true
		}
		principals = append(principals, userPrincipal)
	}

	orgAcct, err := g.githubClient.getGithubOrgByName(searchKey, accessToken, config)
	if err == nil {
		groupPrincipal := v3.Principal{
			ObjectMeta:  metav1.ObjectMeta{Name: Name + "_org://" + strconv.Itoa(orgAcct.ID)},
			DisplayName: orgAcct.Name,
			Kind:        "group",
		}
		if g.isMemberOf(myToken.GroupPrincipals, groupPrincipal) {
			groupPrincipal.MemberOf = true
		}
		principals = append(principals, groupPrincipal)
	}

	return principals, 0, nil
}

func (g *GProvider) isThisUserMe(me v3.Principal, other v3.Principal) bool {

	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.Kind == other.Kind {
		return true
	}
	return false
}

func (g *GProvider) isMemberOf(myGroups []v3.Principal, other v3.Principal) bool {

	for _, mygroup := range myGroups {
		if mygroup.ObjectMeta.Name == other.ObjectMeta.Name && mygroup.Kind == other.Kind {
			return true
		}
	}
	return false
}
