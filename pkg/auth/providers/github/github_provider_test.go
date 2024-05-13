package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type fakeTokensManager struct {
	getSecretFunc               func(userID string, provider string, fallbackTokens []*v3.Token) (string, error)
	isMemberOfFunc              func(token v3.Token, group v3.Principal) bool
	createTokenAndSetCookieFunc func(userID string, userPrincipal v3.Principal, groupPrincipals []v3.Principal, providerToken string, ttl int, description string, request *types.APIContext, userExtraInfo map[string][]string) error
}

func (m *fakeTokensManager) GetSecret(userID string, provider string, fallbackTokens []*v3.Token) (string, error) {
	if m.getSecretFunc != nil {
		return m.getSecretFunc(userID, provider, fallbackTokens)
	}
	return "", nil
}

func (m *fakeTokensManager) IsMemberOf(token v3.Token, group v3.Principal) bool {
	if m.isMemberOfFunc != nil {
		return m.isMemberOfFunc(token, group)
	}
	return false
}

func (m *fakeTokensManager) CreateTokenAndSetCookie(userID string, userPrincipal v3.Principal, groupPrincipals []v3.Principal, providerToken string, ttl int, description string, request *types.APIContext, userExtraInfo map[string][]string) error {
	if m.createTokenAndSetCookieFunc != nil {
		return m.createTokenAndSetCookieFunc(userID, userPrincipal, groupPrincipals, providerToken, ttl, description, request, userExtraInfo)
	}
	return nil
}

func TestSearchPrincipals(t *testing.T) {
	var userOrgs, orgTeams, searchUsersAll, searchUsersGroup, searchUsersUser []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch path := r.URL.Path; path {
		case "/api/v3/user/orgs":
			w.Write(userOrgs)
		case "/api/v3/orgs/devorg/teams":
			w.Write(orgTeams)
		case "/api/v3/search/users":
			q := r.URL.Query().Get("q")
			if strings.Contains(q, " type:org") {
				w.Write(searchUsersGroup)
			} else if strings.Contains(q, " type:user") {
				w.Write(searchUsersUser)
			} else {
				w.Write(searchUsersAll)
			}
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
		"login": "devorg",
		"avatar_url": "` + srvURL.Host + `/u/9343010/avatar"
	}]`)
	orgTeams = []byte(`
	[{
		"id": 9933605,
		"name": "developers",
		"slug": "developers"
	},{
		"id": 9933606,
		"name": "security",
		"slug": "security"
	}]`)
	searchUsersAll = []byte(`{
	"total_count": 2,
  	"incomplete_results": false,
  		"items": [{
			"id": 9253000,
			"login": "developer",
			"avatar_url": "` + srvURL.Host + `/u/9253000/avatar",
			"html_url": "` + srvURL.Host + `/developer",
			"type": "User"
		},{
			"id": 9343010,
			"login": "devorg",
			"avatar_url": "` + srvURL.Host + `/u/9343010/avatar",
			"html_url": "` + srvURL.Host + `/devorg",
			"type": "Organization"
		}]
	}`)
	searchUsersGroup = []byte(`{
	"total_count": 1,
  	"incomplete_results": false,
  		"items": [{
			"id": 9343010,
			"login": "devorg",
			"avatar_url": "` + srvURL.Host + `/u/9343010/avatar",
			"html_url": "` + srvURL.Host + `/devorg",
			"type": "Organization"
		}]
	}`)
	searchUsersUser = []byte(`{
	"total_count": 1,
  	"incomplete_results": false,
  		"items": [{
			"id": 9253000,
			"login": "developer",
			"avatar_url": "` + srvURL.Host + `/u/9253000/avatar",
			"html_url": "` + srvURL.Host + `/developer",
			"type": "User"
		}]
	}`)

	fakeTokensManager := &fakeTokensManager{
		isMemberOfFunc: func(token v3.Token, group v3.Principal) bool {
			return true
		},
	}
	config := &v32.GithubConfig{
		Hostname: srvURL.Host,
	}

	provider := ghProvider{
		ctx:          context.Background(),
		githubClient: &GClient{httpClient: srv.Client()},
		getConfig:    func() (*v32.GithubConfig, error) { return config, nil },
		tokenMGR:     fakeTokensManager,
	}

	token := v32.Token{
		UserPrincipal: v32.Principal{
			ObjectMeta: metav1.ObjectMeta{
				Name: "github_user://9253000",
			},
			LoginName:     "developer",
			PrincipalType: "user",
		},
	}

	// Search for groups and users.
	found, err := provider.SearchPrincipals("dev", "", token)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 3, len(found); want != got {
		t.Fatalf("Expected principals %d got %d", want, got)
	}

	for _, p := range found {
		switch p.LoginName {
		case "devorg":
			if want, got := false, p.Me; want != got {
				t.Errorf("[%s] Expected Me %t, got %t", p.LoginName, want, got)
			}
			if want, got := true, p.MemberOf; want != got {
				t.Errorf("[%s] Expected MemberOf %t, got %t", p.LoginName, want, got)
			}
			if want, got := "group", p.PrincipalType; want != got {
				t.Errorf("[%s] Expected PrincipalType %s, got %s", p.LoginName, want, got)
			}
		case "developer":
			if want, got := true, p.Me; want != got {
				t.Errorf("[%s] Expected Me %t, got %t", p.LoginName, want, got)
			}
			if want, got := "user", p.PrincipalType; want != got {
				t.Errorf("[%s] Expected PrincipalType %s, got %s", p.LoginName, want, got)
			}
		case "developers":
			if want, got := false, p.Me; want != got {
				t.Errorf("[%s] Expected Me %t, got %t", p.LoginName, want, got)
			}
			if want, got := true, p.MemberOf; want != got {
				t.Errorf("[%s] Expected MemberOf %t, got %t", p.LoginName, want, got)
			}
			if want, got := "group", p.PrincipalType; want != got {
				t.Errorf("[%s] Expected PrincipalType %s, got %s", p.LoginName, want, got)
			}
		default:
			t.Errorf("Unexpected principal %s", p.LoginName)
		}
	}

	// Search for groups only.
	found, err = provider.SearchPrincipals("dev", "group", token)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 2, len(found); want != got {
		t.Fatalf("Expected principals %d got %d", want, got)
	}

	for _, p := range found {
		switch p.LoginName {
		case "devorg", "developers":
		default:
			t.Errorf("Unexpected principal %s", p.LoginName)
		}
	}

	// Search for users only.
	found, err = provider.SearchPrincipals("dev", "user", token)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 1, len(found); want != got {
		t.Fatalf("Expected principals %d got %d", want, got)
	}

	if found[0].LoginName != "developer" {
		t.Errorf("Unexpected principal %s", found[0].LoginName)
	}
}
