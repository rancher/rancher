package github

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	userMocks "github.com/rancher/rancher/pkg/user/mocks"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type fakeTokensManager struct {
	getSecretFunc               func(userID string, provider string, fallbackTokens []accessor.TokenAccessor) (string, error)
	createTokenAndSetCookieFunc func(userID string, userPrincipal apiv3.Principal, groupPrincipals []apiv3.Principal, providerToken string, ttl int, description string, request *types.APIContext) error
}

func (m *fakeTokensManager) GetSecret(userID string, provider string, fallbackTokens []accessor.TokenAccessor) (string, error) {
	if m.getSecretFunc != nil {
		return m.getSecretFunc(userID, provider, fallbackTokens)
	}
	return "", nil
}

func (m *fakeTokensManager) CreateTokenAndSetCookie(userID string, userPrincipal apiv3.Principal, groupPrincipals []apiv3.Principal, providerToken string, ttl int, description string, request *types.APIContext) error {
	if m.createTokenAndSetCookieFunc != nil {
		return m.createTokenAndSetCookieFunc(userID, userPrincipal, groupPrincipals, providerToken, ttl, description, request)
	}
	return nil
}

func (m *fakeTokensManager) UserAttributeCreateOrUpdate(userID, provider string, groupPrincipals []apiv3.Principal, userExtraInfo map[string][]string, loginTime ...time.Time) error {
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

	ctrl := gomock.NewController(t)
	userManager := userMocks.NewMockManager(ctrl)
	userManager.EXPECT().IsMemberOf(gomock.Any(), gomock.Any()).Return(true).AnyTimes()

	config := &apiv3.GithubConfig{
		Hostname: srvURL.Host,
	}

	provider := Provider{
		githubClient: &GClient{httpClient: srv.Client()},
		getConfig:    func() (*apiv3.GithubConfig, error) { return config, nil },
		userMGR:      userManager,
		tokenMGR:     &fakeTokensManager{},
	}

	token := apiv3.Token{
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{
				Name: "github_user://9253000",
			},
			LoginName:     "developer",
			PrincipalType: "user",
		},
	}

	// Search for groups and users.
	found, err := provider.SearchPrincipals("dev", "", &token)
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
	found, err = provider.SearchPrincipals("dev", "group", &token)
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
	found, err = provider.SearchPrincipals("dev", "user", &token)
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

func TestSearchPrincipalsExt(t *testing.T) {
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

	ctrl := gomock.NewController(t)
	userManager := userMocks.NewMockManager(ctrl)
	userManager.EXPECT().IsMemberOf(gomock.Any(), gomock.Any()).Return(true).AnyTimes()

	config := &apiv3.GithubConfig{
		Hostname: srvURL.Host,
	}

	provider := Provider{
		githubClient: &GClient{httpClient: srv.Client()},
		getConfig:    func() (*apiv3.GithubConfig, error) { return config, nil },
		userMGR:      userManager,
		tokenMGR:     &fakeTokensManager{},
	}

	token := ext.Token{
		Spec: ext.TokenSpec{
			UserPrincipal: ext.TokenPrincipal{
				Name:          "github_user://9253000",
				LoginName:     "developer",
				PrincipalType: "user",
			},
		},
	}

	// Search for groups and users.
	found, err := provider.SearchPrincipals("dev", "", &token)
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
	found, err = provider.SearchPrincipals("dev", "group", &token)
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
	found, err = provider.SearchPrincipals("dev", "user", &token)
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
