package githubapp

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/stretchr/testify/assert"
)

func TestGitHubAppData(t *testing.T) {
	data := newGitHubAppData()
	data.addOrg(1234567, "example", "Example Org", "https://example.com/avatar1.jpg")
	data.addTeamToOrg("example", 34567, "dev-team", "dev team", "https://example.com/avatar.jpg", "https://example.com/org/example/team/dev-team")
	data.addMemberToTeamInOrg("example", "dev-team", 1001, "test-user", "Test User", "https://example.com/avatar2.jpg", "https://example.com/html")
	data.addTeamToOrg("example", 45678, "admin-team", "admin team", "https://example.com/avatar.jpg", "https://example.com/org/example/team/admin-team")
	data.addMemberToTeamInOrg("example", "example", 1001, "test-user", "Test User", "https://example.com/avatar2.jpg", "https://example.com/html")

	data.addOrg(2345678, "other-org", "Other Org", "https://example.com/avatar2.jpg")
	data.addTeamToOrg("other-org", 23468, "dev-team2", "dev team 2", "https://example.com/avatar.jpg", "https://example.com/org/other-org/team/dev-team2")
	data.addMemberToTeamInOrg("other-org", "dev-team2", 1001, "test-user", "Test User", "https://example.com/avatar2.jpg", "https://example.com/html")
	data.addTeamToOrg("other-org", 34579, "admin-team2", "admin team 2", "https://example.com/avatar.jpg", "https://example.com/org/other-org/team-dev-team2")
	data.addMemberToTeamInOrg("other-org", "admin-team2", 1001, "test-user", "Test User", "https://example.com/avatar2.jpg", "https://example.com/html")
	data.addMemberToTeamInOrg("other-org", "admin-team2", 1002, "test-user2", "Other User", "https://example.com/avatar3.jpg", "https://example.com/htmlpage")
	data.addTeamToOrg("other-org", 45790, "admin-team2", "admin team 2", "https://example.com/otheravatar.jpg", "https://example.com/org/other-org/team-dev-team2")

	data.addOrg(3456789, "example2", "Example Org 2", "https://example.com/avatar2.jpg")

	t.Run("member data", func(t *testing.T) {
		want := member{
			gitHubObject: gitHubObject{
				id:        1001,
				name:      "Test User",
				login:     "test-user",
				avatarURL: "https://example.com/avatar2.jpg",
				htmlURL:   "https://example.com/html",
			},
			orgs: map[string][]string{
				"example":   {"dev-team", "example"},
				"other-org": {"dev-team2", "admin-team2"},
			},
		}
		assert.Equal(t, want, data.members["test-user"])
	})

	t.Run("org data", func(t *testing.T) {
		want := map[string]*org{
			"example": {
				gitHubObject: gitHubObject{
					id:        1234567,
					name:      "Example Org",
					login:     "example",
					avatarURL: "https://example.com/avatar1.jpg",
				},

				teams: map[string]orgTeam{
					"admin-team": {
						gitHubObject: gitHubObject{
							id:        45678,
							avatarURL: "https://example.com/avatar.jpg",
							htmlURL:   "https://example.com/org/example/team/admin-team",
							login:     "admin-team",
							name:      "admin team",
						},
						members: []string{},
					},
					"dev-team": {
						gitHubObject: gitHubObject{
							id:        34567,
							avatarURL: "https://example.com/avatar.jpg",
							htmlURL:   "https://example.com/org/example/team/dev-team",
							login:     "dev-team",
							name:      "dev team",
						},
						members: []string{
							"test-user",
						},
					},
				},
			},
			"other-org": {
				gitHubObject: gitHubObject{
					id:        2345678,
					name:      "Other Org",
					login:     "other-org",
					avatarURL: "https://example.com/avatar2.jpg",
				},

				teams: map[string]orgTeam{
					"admin-team2": {
						gitHubObject: gitHubObject{
							id:        34579,
							login:     "admin-team2",
							name:      "admin team 2",
							htmlURL:   "https://example.com/org/other-org/team-dev-team2",
							avatarURL: "https://example.com/avatar.jpg",
						},
						members: []string{
							"test-user",
							"test-user2",
						},
					},
					"dev-team2": {
						gitHubObject: gitHubObject{
							id:        23468,
							htmlURL:   "https://example.com/org/other-org/team/dev-team2",
							name:      "dev team 2",
							login:     "dev-team2",
							avatarURL: "https://example.com/avatar.jpg",
						},
						members: []string{
							"test-user",
						},
					},
				},
			},
			"example2": {
				gitHubObject: gitHubObject{
					name:      "Example Org 2",
					login:     "example2",
					avatarURL: "https://example.com/avatar2.jpg",
					id:        3456789,
				},
				teams: map[string]orgTeam{},
			},
		}
		assert.Equal(t, want, data.orgs)
	})

	t.Run("orgs for user", func(t *testing.T) {
		orgs := data.listOrgsForUser("unknown")
		assert.Empty(t, orgs)

		// Does not include the third Organisation
		want := []common.GitHubAccount{
			{
				Login:     "example",
				ID:        1234567,
				Name:      "Example Org",
				AvatarURL: "https://example.com/avatar1.jpg",
				Type:      "Organization",
			},
			{

				ID:        2345678,
				Login:     "other-org",
				Name:      "Other Org",
				AvatarURL: "https://example.com/avatar2.jpg",
				Type:      "Organization",
			},
		}

		orgs = data.listOrgsForUser("test-user")
		slices.SortFunc(orgs, func(a, b common.GitHubAccount) int {
			return strings.Compare(a.Login, b.Login)
		})
		assert.Equal(t, want, orgs)

		want = []common.GitHubAccount{
			{
				ID:        2345678,
				Login:     "other-org",
				Name:      "Other Org",
				AvatarURL: "https://example.com/avatar2.jpg",
				Type:      "Organization",
			},
		}

		orgs = data.listOrgsForUser("test-user2")
		slices.SortFunc(orgs, func(a, b common.GitHubAccount) int {
			return strings.Compare(a.Login, b.Login)
		})
		assert.Equal(t, want, orgs)
	})

	t.Run("list all orgs", func(t *testing.T) {
		want := []common.GitHubAccount{
			{
				Login:     "example",
				ID:        1234567,
				Name:      "Example Org",
				AvatarURL: "https://example.com/avatar1.jpg",
				Type:      "Organization",
			},
			{
				ID:        3456789,
				Login:     "example2",
				Name:      "Example Org 2",
				AvatarURL: "https://example.com/avatar2.jpg",
				Type:      "Organization",
			},
			{

				ID:        2345678,
				Login:     "other-org",
				Name:      "Other Org",
				AvatarURL: "https://example.com/avatar2.jpg",
				Type:      "Organization",
			},
		}
		orgs := data.listOrgs()
		slices.SortFunc(orgs, func(a, b common.GitHubAccount) int {
			return strings.Compare(a.Login, b.Login)
		})
		assert.Equal(t, want, orgs)
	})

	t.Run("teams for user", func(t *testing.T) {
		want := []common.GitHubAccount{
			{
				ID:        45678,
				Login:     "admin-team",
				Name:      "admin-team",
				AvatarURL: "https://example.com/avatar1.jpg",
				HTMLURL:   "https://example.com/org/example/team/admin-team",
			},
			{
				ID:        34579,
				Login:     "admin-team2",
				Name:      "admin-team2",
				AvatarURL: "https://example.com/avatar2.jpg",
				HTMLURL:   "https://example.com/org/other-org/team-dev-team2",
			},
			{
				ID:        34567,
				Login:     "dev-team",
				Name:      "dev-team",
				AvatarURL: "https://example.com/avatar1.jpg",
				HTMLURL:   "https://example.com/org/example/team/dev-team",
			},
			{
				ID:        23468,
				Login:     "dev-team2",
				Name:      "dev-team2",
				AvatarURL: "https://example.com/avatar2.jpg",
				HTMLURL:   "https://example.com/org/other-org/team/dev-team2",
			},
		}
		accounts := data.listTeamsForUser("test-user")
		slices.SortFunc(accounts, func(a, b common.GitHubAccount) int {
			return strings.Compare(a.Login, b.Login)
		})

		assert.Equal(t, want, accounts)

		want = []common.GitHubAccount{
			{
				ID:        34579,
				Login:     "admin-team2",
				Name:      "admin-team2",
				AvatarURL: "https://example.com/avatar2.jpg",
				HTMLURL:   "https://example.com/org/other-org/team-dev-team2",
			},
			{
				ID:        23468,
				Login:     "dev-team2",
				Name:      "dev-team2",
				AvatarURL: "https://example.com/avatar2.jpg",
				HTMLURL:   "https://example.com/org/other-org/team/dev-team2",
			},
		}
		accounts = data.listTeamsForUser("test-user2")
		slices.SortFunc(accounts, func(a, b common.GitHubAccount) int {
			return strings.Compare(a.Login, b.Login)
		})
		assert.Equal(t, want, accounts)
	})

	t.Run("list all teams", func(t *testing.T) {
		want := []common.GitHubAccount{
			{
				ID:        45678,
				Login:     "admin-team",
				Name:      "admin-team",
				AvatarURL: "https://example.com/avatar1.jpg",
				HTMLURL:   "https://example.com/org/example/team/admin-team",
			},
			{
				ID:        34579,
				Login:     "admin-team2",
				Name:      "admin-team2",
				AvatarURL: "https://example.com/avatar2.jpg",
				HTMLURL:   "https://example.com/org/other-org/team-dev-team2",
			},
			{
				ID:        34567,
				Login:     "dev-team",
				Name:      "dev-team",
				AvatarURL: "https://example.com/avatar1.jpg",
				HTMLURL:   "https://example.com/org/example/team/dev-team",
			},
			{
				ID:        23468,
				Login:     "dev-team2",
				Name:      "dev-team2",
				AvatarURL: "https://example.com/avatar2.jpg",
				HTMLURL:   "https://example.com/org/other-org/team/dev-team2",
			},
		}

		teams := data.listTeams()
		slices.SortFunc(teams, func(a, b common.GitHubAccount) int {
			return strings.Compare(a.Login, b.Login)
		})
		assert.Equal(t, want, teams)
	})

	t.Run("search teams", func(t *testing.T) {
		teams := data.searchTeams("unknown")
		assert.Empty(t, teams)

		devTeams := []common.GitHubAccount{
			{
				ID:        34567,
				Login:     "dev-team",
				Name:      "dev team",
				AvatarURL: "https://example.com/avatar1.jpg",
				HTMLURL:   "https://example.com/org/example/team/dev-team",
			},
			{
				ID:        23468,
				Login:     "dev-team2",
				Name:      "dev team 2",
				AvatarURL: "https://example.com/avatar2.jpg",
				HTMLURL:   "https://example.com/org/other-org/team/dev-team2",
			},
		}

		teams = data.searchTeams("dev-team")
		slices.SortFunc(teams, func(a, b common.GitHubAccount) int {
			return strings.Compare(a.Login, b.Login)
		})
		assert.Equal(t, devTeams, teams)

		want := []common.GitHubAccount{
			{
				ID:        23468,
				Login:     "dev-team2",
				Name:      "dev team 2",
				AvatarURL: "https://example.com/avatar2.jpg",
				HTMLURL:   "https://example.com/org/other-org/team/dev-team2",
			},
		}

		teams = data.searchTeams("dev-team2")
		assert.Equal(t, want, teams)

		// searching is case-insensitive
		teams = data.searchTeams("Dev")
		slices.SortFunc(teams, func(a, b common.GitHubAccount) int {
			return strings.Compare(a.Login, b.Login)
		})
		assert.Equal(t, devTeams, teams)
	})

	t.Run("search orgs", func(t *testing.T) {
		want := []common.GitHubAccount{
			{

				ID:        2345678,
				Login:     "other-org",
				Name:      "Other Org",
				AvatarURL: "https://example.com/avatar2.jpg",
				Type:      "Organization",
			},
		}
		orgs := data.searchOrgs("other")
		slices.SortFunc(orgs, func(a, b common.GitHubAccount) int {
			return strings.Compare(a.Login, b.Login)
		})
		assert.Equal(t, want, orgs)

		want = []common.GitHubAccount{
			{
				ID:        1234567,
				Login:     "example",
				Name:      "Example Org",
				AvatarURL: "https://example.com/avatar1.jpg",
				Type:      "Organization",
			},
			{
				ID:        3456789,
				Login:     "example2",
				Name:      "Example Org 2",
				AvatarURL: "https://example.com/avatar2.jpg",
				Type:      "Organization",
			},
		}
		orgs = data.searchOrgs("example")
		slices.SortFunc(orgs, func(a, b common.GitHubAccount) int {
			return strings.Compare(a.Login, b.Login)
		})
		assert.Equal(t, want, orgs)
	})

	t.Run("find member by id", func(t *testing.T) {
		unknown := data.findMemberByID(1)
		assert.Nil(t, unknown)

		member1 := data.findMemberByID(1001)
		want1 := &common.GitHubAccount{
			ID:        1001,
			Login:     "test-user",
			Name:      "Test User",
			AvatarURL: "https://example.com/avatar2.jpg",
			HTMLURL:   "https://example.com/html",
			Type:      "User",
		}
		assert.Equal(t, want1, member1)

		member2 := data.findMemberByID(1002)
		want2 := &common.GitHubAccount{
			ID:        1002,
			Login:     "test-user2",
			Name:      "Other User",
			AvatarURL: "https://example.com/avatar3.jpg",
			HTMLURL:   "https://example.com/htmlpage",
			Type:      "User",
		}
		assert.Equal(t, want2, member2)
	})

	t.Run("find org by id", func(t *testing.T) {
		unknown := data.findOrgByID(1)
		assert.Nil(t, unknown)

		org1 := data.findOrgByID(1234567)
		want1 := &common.GitHubAccount{
			Login:     "example",
			ID:        1234567,
			Name:      "Example Org",
			AvatarURL: "https://example.com/avatar1.jpg",
			Type:      "Organization",
		}
		assert.Equal(t, want1, org1)

		org2 := data.findOrgByID(2345678)
		want2 := &common.GitHubAccount{
			ID:        2345678,
			Login:     "other-org",
			Name:      "Other Org",
			AvatarURL: "https://example.com/avatar2.jpg",
			Type:      "Organization",
		}
		assert.Equal(t, want2, org2)
	})

}

func TestGitHubAppData_Errors(t *testing.T) {
	// Duplicate Org
	// Duplicate Team in Org
	// Duplicate Member in team
}

func TestTeamDataFromApp(t *testing.T) {
	ts := startFakeGitHubServer(t)
	testPEM := newTestCertificate(t)

	t.Run("providing an installation ID queries only the installation", func(t *testing.T) {
		// Provides Installation 1
		data, err := getDataForApp(t.Context(), 123456, testPEM, 1, ts.URL)
		if err != nil {
			t.Fatal(err)
		}

		want := &gitHubAppData{
			orgs: map[string]*org{
				"example-org-1": {
					gitHubObject: gitHubObject{
						id:        1,
						login:     "example-org-1",
						avatarURL: "https://example.com/avatar.jpg",
						name:      "Example Org 1",
					},
					teams: map[string]orgTeam{
						"example-team": {
							gitHubObject: gitHubObject{
								id:        12,
								htmlURL:   "https://example.com/org/example-org-1/team/example-team",
								login:     "example-team",
								name:      "Example Team",
								avatarURL: "https://example.com/avatar.jpg",
							},
							members: []string{"test-user"},
						},
					},
				},
			},
			members: map[string]member{
				"test-user": {
					gitHubObject: gitHubObject{
						id:        1,
						name:      "Test User",
						login:     "test-user",
						avatarURL: "https://example.com/1.jpg",
						htmlURL:   "https://example.com/",
					},
					orgs: map[string][]string{
						"example-org-1": {"example-team"},
					},
				},
			},
		}
		assert.Equal(t, want, data)

		wantTeams := []common.GitHubAccount{
			{
				ID:        12,
				Login:     "example-team",
				Name:      "example-team",
				AvatarURL: "https://example.com/avatar.jpg",
				HTMLURL:   "https://example.com/org/example-org-1/team/example-team",
			},
		}

		teams := data.listTeamsForUser("test-user")
		slices.SortFunc(teams, func(a, b common.GitHubAccount) int {
			return strings.Compare(a.HTMLURL, b.HTMLURL)
		})
		assert.Equal(t, wantTeams, teams)

	})

	t.Run("not providing an installation ID gets all installations", func(t *testing.T) {
		data, err := getDataForApp(t.Context(), 123456, testPEM, 0, ts.URL)
		if err != nil {
			t.Fatal(err)
		}

		want := &gitHubAppData{
			orgs: map[string]*org{
				"example-org-1": {
					gitHubObject: gitHubObject{
						id:        1,
						login:     "example-org-1",
						avatarURL: "https://example.com/avatar.jpg",
						name:      "Example Org 1",
					},
					teams: map[string]orgTeam{
						"example-team": {
							gitHubObject: gitHubObject{
								id:        12,
								login:     "example-team",
								name:      "Example Team",
								avatarURL: "https://example.com/avatar.jpg",
								htmlURL:   "https://example.com/org/example-org-1/team/example-team",
							},
							members: []string{"test-user"},
						},
					},
				},
				"example-org-2": {
					gitHubObject: gitHubObject{
						id:        2,
						login:     "example-org-2",
						avatarURL: "https://example.com/avatar.jpg",
						name:      "Example Org 2",
					},
					teams: map[string]orgTeam{
						"example-team": {
							gitHubObject: gitHubObject{
								id:        12,
								htmlURL:   "https://example.com/org/example-org-2/team/example-team",
								avatarURL: "https://example.com/avatar.jpg",
								name:      "Example Team",
								login:     "example-team",
							},
							members: []string{"test-user"},
						},
					},
				},
			},
			members: map[string]member{
				"test-user": {
					gitHubObject: gitHubObject{
						id:        1,
						name:      "Test User",
						login:     "test-user",
						avatarURL: "https://example.com/1.jpg",
						htmlURL:   "https://example.com/",
					},
					orgs: map[string][]string{
						"example-org-1": {"example-team"},
						"example-org-2": {"example-team"},
					},
				},
			},
		}
		assert.Equal(t, want, data)

		wantTeams := []common.GitHubAccount{
			{
				ID:        12,
				Login:     "example-team",
				Name:      "example-team",
				AvatarURL: "https://example.com/avatar.jpg",
				HTMLURL:   "https://example.com/org/example-org-1/team/example-team",
			},
			{
				ID:        12,
				Login:     "example-team",
				Name:      "example-team",
				AvatarURL: "https://example.com/avatar.jpg",
				HTMLURL:   "https://example.com/org/example-org-2/team/example-team",
			},
		}

		teams := data.listTeamsForUser("test-user")
		slices.SortFunc(teams, func(a, b common.GitHubAccount) int {
			return strings.Compare(a.HTMLURL, b.HTMLURL)
		})
		assert.Equal(t, wantTeams, teams)
	})
}

// this starts a very simple HTTP server to respond to GitHub client requests.
// No attempt is done to authenticate the request.
func startFakeGitHubServer(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	// https://docs.github.com/en/rest/apps/apps?apiVersion=2022-11-28#get-an-installation-for-the-authenticated-app
	mux.HandleFunc("GET /api/v3/app/installations/1", func(w http.ResponseWriter, r *http.Request) {
		marshalJSON(t, w, map[string]any{
			"id":          1,
			"target_type": "Organization",
			"target_id":   1,
		})
	})

	// https://docs.github.com/en/rest/apps/apps?apiVersion=2022-11-28#list-installations-for-the-authenticated-app
	mux.HandleFunc("GET /api/v3/app/installations", func(w http.ResponseWriter, r *http.Request) {
		marshalJSON(t, w, []any{
			map[string]any{
				"id":          1,
				"target_type": "Organization",
				"target_id":   1,
			},
			map[string]any{
				"id":          2,
				"target_type": "Organization",
				"target_id":   2,
			},
		})
	})

	// https://docs.github.com/en/rest/orgs/orgs?apiVersion=2022-11-28#get-an-organization
	mux.HandleFunc("GET /api/v3/organizations/{organizationID}", func(w http.ResponseWriter, r *http.Request) {
		organizationID := r.PathValue("organizationID")
		organizationIDInt := testInt64(t, organizationID)
		marshalJSON(t, w, map[string]any{
			"login":      "example-org-" + organizationID,
			"id":         organizationIDInt,
			"type":       "Organization",
			"avatar_url": "https://example.com/avatar.jpg",
			"name":       "Example Org " + organizationID,
		})
	})

	// https://docs.github.com/en/rest/teams/teams?apiVersion=2022-11-28#list-teams
	mux.HandleFunc("GET /api/v3/orgs/{orgName}/teams", func(w http.ResponseWriter, r *http.Request) {
		marshalJSON(t, w, []any{
			map[string]any{
				"id":         12,
				"name":       "Example Team",
				"privacy":    "closed",
				"permission": "admin",
				"slug":       "example-team",
				"html_url":   "https://example.com/org/" + r.PathValue("orgName") + "/team/example-team",
			},
		})
	})

	mux.HandleFunc("GET /api/v3/organizations/{organizationID}/team/{teamID}/members", func(w http.ResponseWriter, r *http.Request) {
		marshalJSON(t, w, []any{
			map[string]any{
				"login":      "test-user",
				"id":         1,
				"node_id":    "MDQ6VXNlcjE=",
				"name":       "Test User",
				"avatar_url": "https://example.com/1.jpg",
				"html_url":   "https://example.com/",
			},
		})
	})

	mux.HandleFunc("POST /api/v3/app/installations/{installationID}/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		marshalJSON(t, w, map[string]any{
			"token":      "ghs_16C7e42F292c6912E7710c838347Ae178B4a",
			"expires_at": "2016-07-11T22:14:10Z",
			"permissions": map[string]any{
				"issues":   "write",
				"contents": "read",
			}})
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(func() {
		ts.Close()
	})

	return ts
}

func marshalJSON(t *testing.T, w http.ResponseWriter, s any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s); err != nil {
		t.Fatalf("encoding JSON response: %s", err)
	}
}

func newTestCertificate(t *testing.T) []byte {
	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatal(err)
	}

	certPrivKeyPEM := &bytes.Buffer{}
	if err := pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	}); err != nil {
		t.Fatal(err)
	}

	return certPrivKeyPEM.Bytes()
}

func testInt64(t *testing.T, s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
