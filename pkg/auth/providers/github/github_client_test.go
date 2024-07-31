package github

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

func TestGitHubClientGetOrgTeams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
		[{
			"id": 9933605,
			"name": "developers",
			"slug": "developers"
		}]`))
	}))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	gcClient := &GClient{httpClient: srv.Client()}

	config := &v32.GithubConfig{
		Hostname: srvURL.Host,
	}

	org := Account{
		ID:        9343010,
		Login:     "org",
		AvatarURL: srvURL.Host + "/u/9343010/avatar",
	}
	teams, err := gcClient.getOrgTeams("", config, org)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 1, len(teams); want != got {
		t.Fatalf("Expected teams %d got %d", want, got)
	}

	if want, got := 9933605, teams[0].ID; want != got {
		t.Errorf("Expected ID %d got %d", want, got)
	}
	if want, got := "developers", teams[0].Login; want != got {
		t.Errorf("Expected login %s got %s", want, got)
	}
	if want, got := "developers", teams[0].Name; want != got {
		t.Errorf("Expected name %s got %s", want, got)
	}
	if want, got := org.AvatarURL, teams[0].AvatarURL; want != got {
		t.Errorf("Expected avatarURL %s got %s", want, got)
	}
	if !strings.HasSuffix(teams[0].HTMLURL, "/orgs/org/teams/developers") {
		t.Errorf("Unexpected htmlURL %s", teams[0].HTMLURL)
	}
}

func TestGetUrlForOrgTeams(t *testing.T) {
	var userOrgs, org1Teams, org2Teams []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch path := r.URL.Path; path {
		case "/api/v3/user/orgs":
			w.Write(userOrgs)
		case "/api/v3/orgs/org1/teams":
			w.Write(org1Teams)
		case "/api/v3/orgs/org2/teams":
			w.Write(org2Teams)
		default:
			t.Errorf("Unexpected client call %s", path)
		}
	}))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	userOrgs = []byte(`
	[{
		"id": 9343010,
		"login": "org1",
		"avatar_url": "` + srvURL.Host + `/u/9343010/avatar"
	},{
		"id": 9343011,
		"login": "org2",
		"avatar_url": "` + srvURL.Host + `/u/9343011/avatar"
	}]`)
	org1Teams = []byte(`
	[{
		"id": 9933605,
		"name": "developers",
		"slug": "developers"
	},{
		"id": 9933606,
		"name": "security",
		"slug": "security"
	}]`)
	org2Teams = []byte(`
	[{
		"id": 9933607,
		"name": "dev-ops",
		"slug": "dev-ops"
	}]`)

	gcClient := &GClient{httpClient: srv.Client()}

	config := &v32.GithubConfig{
		Hostname: srvURL.Host,
	}

	teams, err := gcClient.searchTeams("dev", "", config)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 2, len(teams); want != got {
		t.Fatalf("Expected teams %d got %d", want, got)
	}

	for _, team := range teams {
		switch team.ID {
		case 9933605, 9933607:
		default:
			t.Errorf("Unexpected team %d", team.ID)
		}
	}

	teams, err = gcClient.searchTeams("foo", "", config)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 0, len(teams); want != got {
		t.Fatalf("Expected teams %d got %d", want, got)
	}
}
