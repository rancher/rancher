package github

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
	"strconv"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/auth/model"
)

//Constants for github
const (
	Name = "github"
)

//GProvider implements an PrincipalProvider for github
type GProvider struct {
	githubClient *GClient
}

func Configure(ctx context.Context, mgmtCtx *config.ManagementContext) *GProvider {
	configObj := model.DefaultGithubConfig()

	client := &http.Client{}
	githubClient := &GClient{}
	githubClient.httpClient = client
	githubClient.config = configObj.GithubConfig

	g := &GProvider{}
	g.githubClient = githubClient

	return g
}

//GetName returns the name of the provider
func (g *GProvider) GetName() string {
	return Name
}

func (g *GProvider) AuthenticateUser(loginInput v3.LoginInput) (v3.Principal, []v3.Principal, map[string]string, int, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal
	var providerInfo = make(map[string]string)

	securityCode := loginInput.GithubCredential.Code

	logrus.Debugf("GitHubIdentityProvider AuthenticateUser called for securityCode %v", securityCode)
	accessToken, err := g.githubClient.getAccessToken(securityCode)
	if err != nil {
		logrus.Infof("Error generating accessToken from github %v", err)
		return userPrincipal, groupPrincipals, providerInfo, 401, err
	}
	logrus.Debugf("Received AccessToken from github %v", accessToken)

	providerInfo["access_token"] = accessToken

	user, err := g.githubClient.getGithubUser(accessToken)
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

	orgAccts, err := g.githubClient.getGithubOrgs(accessToken)
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

	teamAccts, err := g.githubClient.getGithubTeams(accessToken)
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

	accessToken := myToken.ProviderInfo["access_token"]

	user, err := g.githubClient.getGithubUserByName(searchKey, accessToken)
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

	orgAcct, err := g.githubClient.getGithubOrgByName(searchKey, accessToken)
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
