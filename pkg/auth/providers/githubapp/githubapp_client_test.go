package githubapp

import (
	"cmp"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-github/v73/github"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGithubAppClientGetAccessToken(t *testing.T) {
	srv := httptest.NewServer(newFakeGitHubServer(t,
		withTestCode("test_client_id", "1234567", "http://localhost:3000/callback", "testing")))
	defer srv.Close()

	appClient := githubAppClient{httpClient: http.DefaultClient}
	token, err := appClient.getAccessToken(t.Context(), "1234567", &apiv3.GithubAppConfig{
		Hostname:     stripScheme(t, srv),
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	if token == "" {
		t.Error("did not get a token")
	}
}

func TestGithubAppClientGetUser(t *testing.T) {
	srv := httptest.NewServer(newFakeGitHubServer(t,
		withTestCode("test_client_id", "1234567", "http://localhost:3000/callback", "testing")))
	defer srv.Close()
	cfg := &apiv3.GithubAppConfig{
		Hostname:     stripScheme(t, srv),
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
	}

	appClient := githubAppClient{httpClient: http.DefaultClient}
	token, err := appClient.getAccessToken(t.Context(), "1234567", cfg)
	if err != nil {
		t.Fatal(err)
	}
	account, err := appClient.getUser(t.Context(), token, cfg)
	if err != nil {
		t.Fatal(err)
	}

	want := common.GitHubAccount{
		ID:        1234,
		Login:     "octocat",
		Name:      "monalisa octocat",
		AvatarURL: "https://github.com/images/error/octocat_happy.gif",
		HTMLURL:   "https://github.com/octocat",
		Type:      "User",
	}

	assert.Equal(t, want, account)
}

func TestGithubAppClientGetUserWithInvalidToken(t *testing.T) {
	srv := httptest.NewServer(newFakeGitHubServer(t,
		withTestCode("test_client_id", "1234567", "http://localhost:3000/callback", "testing")))
	defer srv.Close()
	cfg := &apiv3.GithubAppConfig{
		Hostname:     stripScheme(t, srv),
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
	}

	appClient := githubAppClient{httpClient: http.DefaultClient}
	_, err := appClient.getUser(t.Context(), "invalid token", cfg)
	assert.ErrorContains(t, err, "Access token is invalid or expired")
}

func TestGithubAppClientGetOrgsForUser(t *testing.T) {
	privateKey := newTestCertificate(t)
	srv := httptest.NewServer(newFakeGitHubServer(t,
		withTestCode("test_client_id", "1234567", "http://localhost:3000/callback", "testing"),
		withPrivateKey("23456", privateKey),
	))
	defer srv.Close()
	cfg := &apiv3.GithubAppConfig{
		Hostname:     stripScheme(t, srv),
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		AppID:        "23456",
		PrivateKey:   string(privateKey),
	}

	appClient := githubAppClient{httpClient: http.DefaultClient}
	orgs, err := appClient.getOrgsForUser(t.Context(), "example", cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := []common.GitHubAccount{
		{
			ID:        1,
			Login:     "example-org-1",
			Name:      "Example Org 1",
			AvatarURL: "https://example.com/example-org-1-avatar.jpg",
			Type:      "Organization",
		},
		{
			ID:        2,
			Login:     "example-org-2",
			Name:      "Example Org 2",
			AvatarURL: "https://example.com/example-org-2-avatar.jpg",
			Type:      "Organization",
		},
	}
	slices.SortFunc(orgs, func(a, b common.GitHubAccount) int {
		return strings.Compare(a.Login, b.Login)
	})
	assert.Equal(t, want, orgs)
}

func TestGithubAppClientGetOrgsForUserNotProvidingInstallationID(t *testing.T) {
	cert := newTestCertificate(t)
	srv := httptest.NewServer(newFakeGitHubServer(t,
		withTestCode("test_client_id", "1234567", "http://localhost:3000/callback", "testing"),
		withPrivateKey("1234567", cert)))
	defer srv.Close()
	cfg := &apiv3.GithubAppConfig{
		Hostname:     stripScheme(t, srv),
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		AppID:        "1234567",
		PrivateKey:   string(cert),
	}

	appClient := githubAppClient{httpClient: http.DefaultClient}
	orgs, err := appClient.getOrgsForUser(t.Context(), "example", cfg)
	slices.SortFunc(orgs, func(a, b common.GitHubAccount) int {
		return strings.Compare(a.Login, b.Login)
	})
	require.NoError(t, err)

	want := []common.GitHubAccount{
		{
			ID:        1,
			Login:     "example-org-1",
			Name:      "Example Org 1",
			AvatarURL: "https://example.com/example-org-1-avatar.jpg",
			Type:      "Organization",
		},
		{
			ID:        2,
			Login:     "example-org-2",
			Name:      "Example Org 2",
			AvatarURL: "https://example.com/example-org-2-avatar.jpg",
			Type:      "Organization",
		},
	}
	assert.Equal(t, want, orgs)
}

func TestGithubAppClientGetOrgsForUserProvidingInstallationID(t *testing.T) {
	cert := newTestCertificate(t)
	srv := httptest.NewServer(newFakeGitHubServer(t,
		withTestCode("test_client_id", "1234567", "http://localhost:3000/callback", "testing"),
		withPrivateKey("1234567", cert)))
	defer srv.Close()
	cfg := &apiv3.GithubAppConfig{
		Hostname:       stripScheme(t, srv),
		ClientID:       "test_client_id",
		ClientSecret:   "test_client_secret",
		AppID:          "1234567",
		PrivateKey:     string(cert),
		InstallationID: "1",
	}

	appClient := githubAppClient{httpClient: http.DefaultClient}
	orgs, err := appClient.getOrgsForUser(t.Context(), "example", cfg)
	require.NoError(t, err)
	want := []common.GitHubAccount{
		{
			ID:        1,
			Login:     "example-org-1",
			Name:      "Example Org 1",
			AvatarURL: "https://example.com/example-org-1-avatar.jpg",
			Type:      "Organization",
		},
	}
	assert.Equal(t, want, orgs)
}

func TestGithubAppClientGetTeamsForUserNotProvidingInstallationID(t *testing.T) {
	cert := newTestCertificate(t)
	srv := httptest.NewServer(newFakeGitHubServer(t,
		withTestCode("test_client_id", "1234567", "http://localhost:3000/callback", "testing"),
		withPrivateKey("1234567", cert)))
	defer srv.Close()
	cfg := &apiv3.GithubAppConfig{
		Hostname:     stripScheme(t, srv),
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		AppID:        "1234567",
		PrivateKey:   string(cert),
	}

	appClient := githubAppClient{httpClient: http.DefaultClient}
	orgs, err := appClient.getTeamsForUser(t.Context(), "octocat", cfg)
	require.NoError(t, err)

	want := []common.GitHubAccount{
		{
			ID:        1215,
			Login:     "dev-team",
			Name:      "dev-team",
			AvatarURL: "https://example.com/example-org-1-avatar.jpg",
			HTMLURL:   "https://github.com/orgs/example-org-1/dev-team",
		},
		{
			ID:        1216,
			Login:     "dev-team",
			Name:      "dev-team",
			AvatarURL: "https://example.com/example-org-2-avatar.jpg",
			HTMLURL:   "https://github.com/orgs/example-org-2/dev-team",
		},
		{
			ID:        1217,
			Login:     "test-team",
			Name:      "test-team",
			AvatarURL: "https://example.com/example-org-1-avatar.jpg",
			HTMLURL:   "https://github.com/orgs/example-org-1/test-team",
		},
	}
	slices.SortFunc(orgs, func(a, b common.GitHubAccount) int {
		return cmp.Compare(a.ID, b.ID)
	})
	assert.Equal(t, want, orgs)
}

func TestGithubAppClientGetTeamsForUserProvidingInstallationID(t *testing.T) {
	cert := newTestCertificate(t)
	srv := httptest.NewServer(newFakeGitHubServer(t,
		withTestCode("test_client_id", "1234567", "http://localhost:3000/callback", "testing"),
		withPrivateKey("1234567", cert)))
	defer srv.Close()
	cfg := &apiv3.GithubAppConfig{
		Hostname:       stripScheme(t, srv),
		ClientID:       "test_client_id",
		ClientSecret:   "test_client_secret",
		AppID:          "1234567",
		InstallationID: "1",
		PrivateKey:     string(cert),
	}

	appClient := githubAppClient{httpClient: http.DefaultClient}
	orgs, err := appClient.getTeamsForUser(t.Context(), "octocat", cfg)
	require.NoError(t, err)

	want := []common.GitHubAccount{
		{
			ID:        1215,
			Login:     "dev-team",
			Name:      "dev-team",
			AvatarURL: "https://example.com/example-org-1-avatar.jpg",
			HTMLURL:   "https://github.com/orgs/example-org-1/dev-team",
		},
		{
			ID:        1217,
			Login:     "test-team",
			Name:      "test-team",
			AvatarURL: "https://example.com/example-org-1-avatar.jpg",
			HTMLURL:   "https://github.com/orgs/example-org-1/test-team",
		},
	}
	slices.SortFunc(orgs, func(a, b common.GitHubAccount) int {
		return cmp.Compare(a.ID, b.ID)
	})
	assert.Equal(t, want, orgs)
}

func stripScheme(t *testing.T, ts *httptest.Server) string {
	parsed, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	return parsed.Host
}

// This will communicate with GitHub using the provided credentials.
// And verify that the APIs calls are correct.
func TestGitHubAppClient(t *testing.T) {
	privateKey := []byte(os.Getenv("GITHUB_APP_KEY"))
	if len(privateKey) == 0 {
		t.Skip("No GITHUB_APP_KEY provided")
	}
	app := os.Getenv("GITHUB_APP_ID")
	appID, err := strconv.ParseInt(app, 10, 64)
	require.NoError(t, err)

	var installationID int64
	if v := os.Getenv("GITHUB_INSTALLATION_ID"); v != "" {
		i, err := strconv.ParseInt(v, 10, 64)
		require.NoError(t, err)
		installationID = i
	}

	data, err := getDataForApp(t.Context(), appID, privateKey, installationID, "")
	require.NoError(t, err)

	assert.NotEmpty(t, data.members)
	assert.NotEmpty(t, data.orgs)
}

func TestGitHubAppInstallationClient(t *testing.T) {
	privateKey := []byte(os.Getenv("GITHUB_APP_KEY"))
	if len(privateKey) == 0 {
		t.Skip("No GITHUB_APP_KEY provided")
	}
	app := os.Getenv("GITHUB_APP_ID")
	appID, err := strconv.ParseInt(app, 10, 64)
	if err != nil {
		t.Fatalf("invalid app ID: %q", app)
	}

	var installationID int64
	if v := os.Getenv("GITHUB_INSTALLATION_ID"); v != "" {
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			t.Fatalf("invalid installation id %q", v)
		}
		installationID = i
	}
	client, err := getInstallationClient(context.TODO(), appID, privateKey, installationID, "")
	require.NoError(t, err)

	result, _, err := client.Search.Users(context.TODO(), "rancher", &github.SearchOptions{})
	require.NoError(t, err)

	assert.NotEmpty(t, result.Users)
}
