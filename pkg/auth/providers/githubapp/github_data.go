package githubapp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v73/github"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"golang.org/x/oauth2"
	"k8s.io/utils/ptr"
)

// gitHubObject represents the basic values that all GitHub resources have.
type gitHubObject struct {
	name      string
	login     string
	avatarURL string
	htmlURL   string
	id        int
}

// org represents an Organization.
type org struct {
	gitHubObject

	// Teams is a mapping "slug" -> OrgTeam
	teams map[string]orgTeam
}

func (o org) toAccount() common.GitHubAccount {
	return common.GitHubAccount{Name: o.name, Login: o.login, AvatarURL: o.avatarURL, ID: o.id, HTMLURL: o.htmlURL, Type: "Organization"}
}

// orgTeam represents a team within an Organization.
type orgTeam struct {
	gitHubObject

	members []string
}

type member struct {
	gitHubObject
	// Mapping of org -> teams
	orgs map[string][]string
}

func (m member) toAccount() common.GitHubAccount {
	return common.GitHubAccount{Name: m.name, Login: m.login, AvatarURL: m.avatarURL, ID: m.id, HTMLURL: m.htmlURL, Type: "User"}
}

// Add a team to this Organization.
func (o *org) addTeam(id int, login, name, avatarURL, htmlURL string) {
	_, ok := o.teams[login]
	if ok {
		return
	}
	o.teams[login] = orgTeam{
		gitHubObject: gitHubObject{
			id:        id,
			htmlURL:   htmlURL,
			avatarURL: avatarURL,
			name:      name,
			login:     login,
		},
		members: []string{},
	}
}

// Add a member to a team within this Organization.
func (o *org) addTeamMember(teamName, name string) {
	team, ok := o.teams[teamName]
	if !ok {
		return
	}
	team.members = append(team.members, name)
	o.teams[teamName] = team
}

// Aggregates the data for members.
type gitHubAppData struct {
	// Orgs is a mapping "name" -> *Org
	orgs map[string]*org
	// Members is a mapping "membername" -> org -> teams (slice of strings)
	members map[string]member
}

// listOrgsForUser returns a set of Accounts derived from the Organizations the
// provided username is a member of.
func (g *gitHubAppData) listOrgsForUser(username string) []common.GitHubAccount {
	var accounts []common.GitHubAccount
	for orgName := range g.members[username].orgs {
		org := g.orgs[orgName]
		accounts = append(accounts, org.toAccount())
	}

	return accounts
}

// listOrgs returns a set of Accounts derived from all Organizations queried
// with the GitHub App credentials.
func (g *gitHubAppData) listOrgs() []common.GitHubAccount {
	var accounts []common.GitHubAccount

	for _, org := range g.orgs {
		accounts = append(accounts, common.GitHubAccount{Name: org.name, Login: org.login, AvatarURL: org.avatarURL, ID: org.id, Type: "Organization"})
	}

	return accounts
}

// listTeamsForUser returns a set of Accounts derived from the Teams the provided
// username is a member of.
func (g *gitHubAppData) listTeamsForUser(username string) []common.GitHubAccount {
	var accounts []common.GitHubAccount

	for orgName := range g.members[username].orgs {
		org := g.orgs[orgName]
		for teamName, team := range org.teams {
			accounts = append(accounts, common.GitHubAccount{Name: teamName, Login: team.login, AvatarURL: org.avatarURL, ID: team.id, HTMLURL: team.htmlURL})
		}
	}

	return accounts
}

// listTeams returns a set of Accounts derived from all Organizations queried
// with the GitHub App credentials.
func (g *gitHubAppData) listTeams() []common.GitHubAccount {
	var accounts []common.GitHubAccount
	for _, org := range g.orgs {
		for teamName, team := range org.teams {
			accounts = append(accounts, common.GitHubAccount{Name: teamName, Login: team.login, AvatarURL: org.avatarURL, ID: team.id, HTMLURL: team.htmlURL})
		}
	}

	return accounts
}

// searchTeams returns a set of Accounts derived from all teams with a
// simple string match.
func (g *gitHubAppData) searchTeams(s string) []common.GitHubAccount {
	var accounts []common.GitHubAccount

	for _, org := range g.orgs {
		for _, team := range org.teams {
			if hasInsensitivePrefix(team.login, s) {
				accounts = append(accounts, common.GitHubAccount{Name: team.name, Login: team.login, AvatarURL: org.avatarURL, ID: team.id, HTMLURL: team.htmlURL})
			}
		}
	}

	return accounts
}

// searchOrgs returns a set of Accounts derived from all orgs with a
// simple string match.
func (g *gitHubAppData) searchOrgs(s string) []common.GitHubAccount {
	var accounts []common.GitHubAccount

	for orgName, org := range g.orgs {
		if hasInsensitivePrefix(orgName, s) {
			accounts = append(accounts, common.GitHubAccount{Name: org.name, Login: org.login, AvatarURL: org.avatarURL, ID: org.id, HTMLURL: org.htmlURL, Type: "Organization"})
		}
	}

	return accounts
}

func (g *gitHubAppData) addOrg(id int, login, name, avatarURL string) {
	if _, ok := g.orgs[login]; ok {
		return
	}
	g.orgs[login] = &org{gitHubObject: gitHubObject{login: login, id: id, name: name, avatarURL: avatarURL}, teams: map[string]orgTeam{}}
}

func (g *gitHubAppData) addTeamToOrg(org string, id int, slug, name, avatarURL, htmlURL string) {
	o, ok := g.orgs[org]
	if !ok {
		return
	}

	o.addTeam(id, slug, name, avatarURL, htmlURL)
}

func (g *gitHubAppData) addMemberToTeamInOrg(org, team string, id int, login, name, avatarURL, htmlURL string) {
	o, ok := g.orgs[org]
	if !ok {
		return
	}
	o.addTeamMember(team, login)

	m, ok := g.members[login]
	if !ok {
		m = member{
			gitHubObject: gitHubObject{id: id, name: name, login: login, avatarURL: avatarURL, htmlURL: htmlURL},
			orgs:         map[string][]string{}}
	}

	orgTeams, ok := m.orgs[org]
	if !ok {
		orgTeams = []string{}
	}
	orgTeams = append(orgTeams, team)
	m.orgs[org] = orgTeams

	g.members[login] = m
}

func (g *gitHubAppData) findMemberByID(memberID int) *common.GitHubAccount {
	for _, member := range g.members {
		if member.id == memberID {
			acct := member.toAccount()
			return &acct
		}
	}
	return nil
}

func (g *gitHubAppData) findOrgByID(orgID int) *common.GitHubAccount {
	for _, org := range g.orgs {
		if org.id == orgID {
			acct := org.toAccount()
			return &acct
		}
	}
	return nil
}

func newGitHubAppData() *gitHubAppData {
	return &gitHubAppData{
		orgs:    map[string]*org{},
		members: map[string]member{},
	}
}

func newGitHubClient(c *http.Client, endpoint string) (*github.Client, error) {
	client := github.NewClient(c)
	if endpoint != "" {
		c, err := client.WithEnterpriseURLs(endpoint, endpoint)
		if err != nil {
			// TODO: Improve error message
			return nil, err
		}
		client = c
	}

	return client, nil
}

// Create an installation specific token and return a client configured to use the
// token.
func newClientForInstallation(ctx context.Context, client *github.Client, installationID int64, endpoint string) (*github.Client, error) {
	token, _, err := client.Apps.CreateInstallationToken(ctx, installationID, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create installation token: %w", err)
	}

	return newGitHubClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token.GetToken()},
	)), endpoint)
}

func newClientForApp(ctx context.Context, appID int64, privateKey []byte, endpoint string) (*github.Client, error) {
	// Create the JWT token signed by the GitHub App, and a GitHub client using it.
	jwtToken := createJWT(appID, privateKey)

	return newGitHubClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: jwtToken},
	)), endpoint)
}

func createJWT(appID int64, privateKey []byte) string {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(privateKey)
	if err != nil {
		log.Fatalf("failed to parse private key: %v", err)
	}

	iss := time.Now().Add(-30 * time.Second).Truncate(time.Second)
	claims := &jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(iss),
		ExpiresAt: jwt.NewNumericDate(iss.Add(2 * time.Minute)),
		Issuer:    fmt.Sprintf("%v", appID),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	ss, err := token.SignedString(key)
	if err != nil {
		log.Fatalf("failed to sign JWT: %v", err)
	}

	return ss
}

func gatherDataForInstallation(ctx context.Context, data *gitHubAppData, installationClient *github.Client, organization int64) error {
	org, _, err := installationClient.Organizations.GetByID(ctx, organization)
	if err != nil {
		return fmt.Errorf("getting GitHub organization %v: %w", organization, err)
	}
	data.addOrg(int(*org.ID), *org.Login, ptr.Deref(org.Name, ""), *org.AvatarURL)

	opts := &github.ListOptions{PerPage: 100}
	var allTeams []*github.Team
	for {
		teams, resp, err := installationClient.Teams.ListTeams(ctx, *org.Login, opts)
		if err != nil {
			return fmt.Errorf("listing teams in GitHub organization %v: %w", *org.Login, err)
		}
		allTeams = append(allTeams, teams...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	for _, team := range allTeams {
		data.addTeamToOrg(*org.Login, int(*team.ID), *team.Slug, *team.Name, *org.AvatarURL, *team.HTMLURL)
		membersOpts := &github.TeamListTeamMembersOptions{ListOptions: github.ListOptions{PerPage: 100}}
		var allMembers []*github.User
		for {
			members, resp, err := installationClient.Teams.ListTeamMembersByID(ctx, *org.ID, *team.ID, membersOpts)
			if err != nil {
				return fmt.Errorf("listing team members: %w", err)
			}
			allMembers = append(allMembers, members...)
			if resp.NextPage == 0 {
				break
			}
			membersOpts.Page = resp.NextPage
		}

		for _, member := range allMembers {
			name := ""
			if member.Name != nil {
				name = *member.Name
			}

			data.addMemberToTeamInOrg(*org.Login, *team.Slug, int(*member.ID), *member.Login, name, *member.AvatarURL, *member.HTMLURL)
		}
	}

	return nil
}

// Extract the team memberships of organisations that the App has been installed
// into.
//
// If the installationID is zero (0) all installations for the app will be
// queried.
func getDataForApp(ctx context.Context, appID int64, privateKey []byte, installationID int64, endpoint string) (*gitHubAppData, error) {
	itr, err := ghinstallation.NewAppsTransport(http.DefaultTransport, appID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("creating transport to access GitHub: %w", err)
	}

	client := github.NewClient(
		&http.Client{
			Transport: itr,
			Timeout:   time.Second * 30,
		},
	)

	if endpoint != "" {
		c, err := client.WithEnterpriseURLs(endpoint, endpoint)
		if err != nil {
			return nil, fmt.Errorf("creating a github client: %w", err)
		}
		client = c
	}

	appClient, err := newClientForApp(ctx, appID, privateKey, endpoint)
	if err != nil {
		return nil, fmt.Errorf("creating a client for app %v: %w", appID, err)
	}

	data := newGitHubAppData()
	if installationID > 0 {
		installation, _, err := client.Apps.GetInstallation(ctx, installationID)
		if err != nil {
			log.Fatalf("failed to get installation %v: %s", installationID, err)
		}
		installationClient, err := newClientForInstallation(ctx, appClient, installationID, endpoint)
		if err != nil {
			return nil, err
		}

		if err := gatherDataForInstallation(ctx, data, installationClient, *installation.TargetID); err != nil {
			return nil, err
		}
	} else {
		installations, _, err := client.Apps.ListInstallations(ctx, &github.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing app installations: %w", err)
		}

		for _, i := range installations {
			installationClient, err := newClientForInstallation(ctx, appClient, *i.ID, endpoint)
			if err != nil {
				return nil, err
			}

			if err := gatherDataForInstallation(ctx, data, installationClient, *i.TargetID); err != nil {
				return nil, err
			}
		}
	}

	return data, nil
}

// Get a client for one of the installations available to the app.
//
// If the installationID is not zero, this installation will be used, otherwise
// we'll pick one of the installations for the app.
func getInstallationClient(ctx context.Context, appID int64, privateKey []byte, installationID int64, endpoint string) (*github.Client, error) {
	itr, err := ghinstallation.NewAppsTransport(http.DefaultTransport, appID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("creating transport to access GitHub: %w", err)
	}

	client := github.NewClient(
		&http.Client{
			Transport: itr,
			Timeout:   time.Second * 30,
		},
	)

	if endpoint != "" {
		c, err := client.WithEnterpriseURLs(endpoint, endpoint)
		if err != nil {
			return nil, fmt.Errorf("creating a github client: %w", err)
		}
		client = c
	}

	appClient, err := newClientForApp(ctx, appID, privateKey, endpoint)
	if err != nil {
		return nil, fmt.Errorf("creating a client for app %v: %w", appID, err)
	}

	if installationID > 0 {
		return newClientForInstallation(ctx, appClient, installationID, endpoint)
	}

	installations, _, err := client.Apps.ListInstallations(ctx, &github.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing app installations: %w", err)
	}

	// Should this choose randomly?
	selectedInstallationID := *installations[0].ID
	return newClientForInstallation(ctx, appClient, selectedInstallationID, endpoint)
}

func hasInsensitivePrefix(s, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix))
}
